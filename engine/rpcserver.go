package engine

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io/ioutil"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/gofrs/uuid"
	"github.com/golang/protobuf/ptypes"
	grpcauth "github.com/grpc-ecosystem/go-grpc-middleware/auth"
	grpcruntime "github.com/grpc-ecosystem/grpc-gateway/runtime"
	"github.com/yurulab/gocryptotrader/common"
	"github.com/yurulab/gocryptotrader/common/crypto"
	"github.com/yurulab/gocryptotrader/common/file"
	"github.com/yurulab/gocryptotrader/common/file/archive"
	"github.com/yurulab/gocryptotrader/currency"
	"github.com/yurulab/gocryptotrader/database"
	"github.com/yurulab/gocryptotrader/database/models/postgres"
	"github.com/yurulab/gocryptotrader/database/models/sqlite3"
	"github.com/yurulab/gocryptotrader/database/repository/audit"
	"github.com/yurulab/gocryptotrader/exchanges/account"
	"github.com/yurulab/gocryptotrader/exchanges/asset"
	"github.com/yurulab/gocryptotrader/exchanges/kline"
	"github.com/yurulab/gocryptotrader/exchanges/order"
	"github.com/yurulab/gocryptotrader/exchanges/orderbook"
	"github.com/yurulab/gocryptotrader/exchanges/ticker"
	"github.com/yurulab/gocryptotrader/gctrpc"
	"github.com/yurulab/gocryptotrader/gctrpc/auth"
	gctscript "github.com/yurulab/gocryptotrader/gctscript/vm"
	"github.com/yurulab/gocryptotrader/log"
	"github.com/yurulab/gocryptotrader/portfolio"
	"github.com/yurulab/gocryptotrader/portfolio/banking"
	"github.com/yurulab/gocryptotrader/portfolio/withdraw"
	"github.com/yurulab/gocryptotrader/utils"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
	"google.golang.org/grpc/metadata"
)

const (
	errExchangeNameUnset = "exchange name unset"
	errCurrencyPairUnset = "currency pair unset"
	errAssetTypeUnset    = "asset type unset"
	errDispatchSystem    = "dispatch system offline"
)

var (
	errExchangeNotLoaded    = errors.New("exchange is not loaded/doesn't exist")
	errExchangeBaseNotFound = errors.New("cannot get exchange base")
)

// RPCServer struct
type RPCServer struct{}

func authenticateClient(ctx context.Context) (context.Context, error) {
	md, ok := metadata.FromIncomingContext(ctx)
	if !ok {
		return ctx, fmt.Errorf("unable to extract metadata")
	}

	authStr, ok := md["authorization"]
	if !ok {
		return ctx, fmt.Errorf("authorization header missing")
	}

	if !strings.Contains(authStr[0], "Basic") {
		return ctx, fmt.Errorf("basic not found in authorization header")
	}

	decoded, err := crypto.Base64Decode(strings.Split(authStr[0], " ")[1])
	if err != nil {
		return ctx, fmt.Errorf("unable to base64 decode authorization header")
	}

	username := strings.Split(string(decoded), ":")[0]
	password := strings.Split(string(decoded), ":")[1]

	if username != Bot.Config.RemoteControl.Username || password != Bot.Config.RemoteControl.Password {
		return ctx, fmt.Errorf("username/password mismatch")
	}

	return ctx, nil
}

// StartRPCServer starts a gRPC server with TLS auth
func StartRPCServer() {
	targetDir := utils.GetTLSDir(Bot.Settings.DataDir)
	err := checkCerts(targetDir)
	if err != nil {
		log.Errorf(log.GRPCSys, "gRPC checkCerts failed. err: %s\n", err)
		return
	}

	log.Debugf(log.GRPCSys, "gRPC server support enabled. Starting gRPC server on https://%v.\n", Bot.Config.RemoteControl.GRPC.ListenAddress)
	lis, err := net.Listen("tcp", Bot.Config.RemoteControl.GRPC.ListenAddress)
	if err != nil {
		log.Errorf(log.GRPCSys, "gRPC server failed to bind to port: %s", err)
		return
	}

	creds, err := credentials.NewServerTLSFromFile(filepath.Join(targetDir, "cert.pem"), filepath.Join(targetDir, "key.pem"))
	if err != nil {
		log.Errorf(log.GRPCSys, "gRPC server could not load TLS keys: %s\n", err)
		return
	}

	opts := []grpc.ServerOption{
		grpc.Creds(creds),
		grpc.UnaryInterceptor(grpcauth.UnaryServerInterceptor(authenticateClient)),
	}
	server := grpc.NewServer(opts...)
	s := RPCServer{}
	gctrpc.RegisterGoCryptoTraderServer(server, &s)

	go func() {
		if err := server.Serve(lis); err != nil {
			log.Errorf(log.GRPCSys, "gRPC server failed to serve: %s\n", err)
			return
		}
	}()

	log.Debugln(log.GRPCSys, "gRPC server started!")

	if Bot.Settings.EnableGRPCProxy {
		StartRPCRESTProxy()
	}
}

// StartRPCRESTProxy starts a gRPC proxy
func StartRPCRESTProxy() {
	log.Debugf(log.GRPCSys, "gRPC proxy server support enabled. Starting gRPC proxy server on http://%v.\n", Bot.Config.RemoteControl.GRPC.GRPCProxyListenAddress)

	targetDir := utils.GetTLSDir(Bot.Settings.DataDir)
	creds, err := credentials.NewClientTLSFromFile(filepath.Join(targetDir, "cert.pem"), "")
	if err != nil {
		log.Errorf(log.GRPCSys, "Unabled to start gRPC proxy. Err: %s\n", err)
		return
	}

	mux := grpcruntime.NewServeMux()
	opts := []grpc.DialOption{grpc.WithTransportCredentials(creds),
		grpc.WithPerRPCCredentials(auth.BasicAuth{
			Username: Bot.Config.RemoteControl.Username,
			Password: Bot.Config.RemoteControl.Password,
		}),
	}
	err = gctrpc.RegisterGoCryptoTraderHandlerFromEndpoint(context.Background(),
		mux, Bot.Config.RemoteControl.GRPC.ListenAddress, opts)
	if err != nil {
		log.Errorf(log.GRPCSys, "Failed to register gRPC proxy. Err: %s\n", err)
		return
	}

	go func() {
		if err := http.ListenAndServe(Bot.Config.RemoteControl.GRPC.GRPCProxyListenAddress, mux); err != nil {
			log.Errorf(log.GRPCSys, "gRPC proxy failed to server: %s\n", err)
			return
		}
	}()

	log.Debugln(log.GRPCSys, "gRPC proxy server started!")
}

// GetInfo returns info about the current GoCryptoTrader session
func (s *RPCServer) GetInfo(_ context.Context, r *gctrpc.GetInfoRequest) (*gctrpc.GetInfoResponse, error) {
	d := time.Since(Bot.Uptime)
	resp := gctrpc.GetInfoResponse{
		Uptime:               d.String(),
		EnabledExchanges:     int64(Bot.Config.CountEnabledExchanges()),
		AvailableExchanges:   int64(len(Bot.Config.Exchanges)),
		DefaultFiatCurrency:  Bot.Config.Currency.FiatDisplayCurrency.String(),
		DefaultForexProvider: Bot.Config.GetPrimaryForexProvider(),
		SubsystemStatus:      GetSubsystemsStatus(),
	}
	endpoints := GetRPCEndpoints()
	resp.RpcEndpoints = make(map[string]*gctrpc.RPCEndpoint)
	for k, v := range endpoints {
		resp.RpcEndpoints[k] = &gctrpc.RPCEndpoint{
			Started:       v.Started,
			ListenAddress: v.ListenAddr,
		}
	}
	return &resp, nil
}

// GetSubsystems returns a list of subsystems and their status
func (s *RPCServer) GetSubsystems(_ context.Context, r *gctrpc.GetSubsystemsRequest) (*gctrpc.GetSusbsytemsResponse, error) {
	return &gctrpc.GetSusbsytemsResponse{SubsystemsStatus: GetSubsystemsStatus()}, nil
}

// EnableSubsystem enables a engine subsytem
func (s *RPCServer) EnableSubsystem(_ context.Context, r *gctrpc.GenericSubsystemRequest) (*gctrpc.GenericResponse, error) {
	err := SetSubsystem(r.Subsystem, true)
	if err != nil {
		return nil, err
	}
	return &gctrpc.GenericResponse{Status: MsgStatusSuccess,
		Data: fmt.Sprintf("subsystem %s enabled", r.Subsystem)}, nil
}

// DisableSubsystem disables a engine subsytem
func (s *RPCServer) DisableSubsystem(_ context.Context, r *gctrpc.GenericSubsystemRequest) (*gctrpc.GenericResponse, error) {
	err := SetSubsystem(r.Subsystem, false)
	if err != nil {
		return nil, err
	}
	return &gctrpc.GenericResponse{Status: MsgStatusSuccess,
		Data: fmt.Sprintf("subsystem %s disabled", r.Subsystem)}, nil
}

// GetRPCEndpoints returns a list of API endpoints
func (s *RPCServer) GetRPCEndpoints(_ context.Context, r *gctrpc.GetRPCEndpointsRequest) (*gctrpc.GetRPCEndpointsResponse, error) {
	endpoints := GetRPCEndpoints()
	var resp gctrpc.GetRPCEndpointsResponse
	resp.Endpoints = make(map[string]*gctrpc.RPCEndpoint)
	for k, v := range endpoints {
		resp.Endpoints[k] = &gctrpc.RPCEndpoint{
			Started:       v.Started,
			ListenAddress: v.ListenAddr,
		}
	}
	return &resp, nil
}

// GetCommunicationRelayers returns the status of the engines communication relayers
func (s *RPCServer) GetCommunicationRelayers(_ context.Context, r *gctrpc.GetCommunicationRelayersRequest) (*gctrpc.GetCommunicationRelayersResponse, error) {
	relayers, err := Bot.CommsManager.GetStatus()
	if err != nil {
		return nil, err
	}

	var resp gctrpc.GetCommunicationRelayersResponse
	resp.CommunicationRelayers = make(map[string]*gctrpc.CommunicationRelayer)
	for k, v := range relayers {
		resp.CommunicationRelayers[k] = &gctrpc.CommunicationRelayer{
			Enabled:   v.Enabled,
			Connected: v.Connected,
		}
	}
	return &resp, nil
}

// GetExchanges returns a list of exchanges
// Param is whether or not you wish to list enabled exchanges
func (s *RPCServer) GetExchanges(_ context.Context, r *gctrpc.GetExchangesRequest) (*gctrpc.GetExchangesResponse, error) {
	exchanges := strings.Join(GetExchangeNames(r.Enabled), ",")
	return &gctrpc.GetExchangesResponse{Exchanges: exchanges}, nil
}

// DisableExchange disables an exchange
func (s *RPCServer) DisableExchange(_ context.Context, r *gctrpc.GenericExchangeNameRequest) (*gctrpc.GenericResponse, error) {
	err := UnloadExchange(r.Exchange)
	if err != nil {
		return nil, err
	}
	return &gctrpc.GenericResponse{Status: MsgStatusSuccess}, nil
}

// EnableExchange enables an exchange
func (s *RPCServer) EnableExchange(_ context.Context, r *gctrpc.GenericExchangeNameRequest) (*gctrpc.GenericResponse, error) {
	err := LoadExchange(r.Exchange, false, nil)
	if err != nil {
		return nil, err
	}
	return &gctrpc.GenericResponse{Status: MsgStatusSuccess}, nil
}

// GetExchangeOTPCode retrieves an exchanges OTP code
func (s *RPCServer) GetExchangeOTPCode(_ context.Context, r *gctrpc.GenericExchangeNameRequest) (*gctrpc.GetExchangeOTPReponse, error) {
	result, err := GetExchangeoOTPByName(r.Exchange)
	return &gctrpc.GetExchangeOTPReponse{OtpCode: result}, err
}

// GetExchangeOTPCodes retrieves OTP codes for all exchanges which have an
// OTP secret installed
func (s *RPCServer) GetExchangeOTPCodes(_ context.Context, r *gctrpc.GetExchangeOTPsRequest) (*gctrpc.GetExchangeOTPsResponse, error) {
	result, err := GetExchangeOTPs()
	return &gctrpc.GetExchangeOTPsResponse{OtpCodes: result}, err
}

// GetExchangeInfo gets info for a specific exchange
func (s *RPCServer) GetExchangeInfo(_ context.Context, r *gctrpc.GenericExchangeNameRequest) (*gctrpc.GetExchangeInfoResponse, error) {
	exchCfg, err := Bot.Config.GetExchangeConfig(r.Exchange)
	if err != nil {
		return nil, err
	}

	resp := &gctrpc.GetExchangeInfoResponse{
		Name:           exchCfg.Name,
		Enabled:        exchCfg.Enabled,
		Verbose:        exchCfg.Verbose,
		UsingSandbox:   exchCfg.UseSandbox,
		HttpTimeout:    exchCfg.HTTPTimeout.String(),
		HttpUseragent:  exchCfg.HTTPUserAgent,
		HttpProxy:      exchCfg.ProxyAddress,
		BaseCurrencies: strings.Join(exchCfg.BaseCurrencies.Strings(), ","),
	}

	resp.SupportedAssets = make(map[string]*gctrpc.PairsSupported)
	assets := exchCfg.CurrencyPairs.GetAssetTypes()
	for i := range assets {
		ps, err := exchCfg.CurrencyPairs.Get(assets[i])
		if err != nil {
			return nil, err
		}

		resp.SupportedAssets[assets[i].String()] = &gctrpc.PairsSupported{
			EnabledPairs:   ps.Enabled.Join(),
			AvailablePairs: ps.Available.Join(),
		}
	}
	return resp, nil
}

// GetTicker returns the ticker for a specified exchange, currency pair and
// asset type
func (s *RPCServer) GetTicker(_ context.Context, r *gctrpc.GetTickerRequest) (*gctrpc.TickerResponse, error) {
	t, err := GetSpecificTicker(currency.Pair{
		Delimiter: r.Pair.Delimiter,
		Base:      currency.NewCode(r.Pair.Base),
		Quote:     currency.NewCode(r.Pair.Quote),
	},
		r.Exchange,
		asset.Item(r.AssetType),
	)
	if err != nil {
		return nil, err
	}

	resp := &gctrpc.TickerResponse{
		Pair:        r.Pair,
		LastUpdated: t.LastUpdated.Unix(),
		Last:        t.Last,
		High:        t.High,
		Low:         t.Low,
		Bid:         t.Bid,
		Ask:         t.Ask,
		Volume:      t.Volume,
		PriceAth:    t.PriceATH,
	}

	return resp, nil
}

// GetTickers returns a list of tickers for all enabled exchanges and all
// enabled currency pairs
func (s *RPCServer) GetTickers(_ context.Context, r *gctrpc.GetTickersRequest) (*gctrpc.GetTickersResponse, error) {
	activeTickers := GetAllActiveTickers()
	var tickers []*gctrpc.Tickers

	for x := range activeTickers {
		var ticker gctrpc.Tickers
		ticker.Exchange = activeTickers[x].ExchangeName
		for y := range activeTickers[x].ExchangeValues {
			t := activeTickers[x].ExchangeValues[y]
			ticker.Tickers = append(ticker.Tickers, &gctrpc.TickerResponse{
				Pair: &gctrpc.CurrencyPair{
					Delimiter: t.Pair.Delimiter,
					Base:      t.Pair.Base.String(),
					Quote:     t.Pair.Quote.String(),
				},
				LastUpdated: t.LastUpdated.Unix(),
				Last:        t.Last,
				High:        t.High,
				Low:         t.Low,
				Bid:         t.Bid,
				Ask:         t.Ask,
				Volume:      t.Volume,
				PriceAth:    t.PriceATH,
			})
		}
		tickers = append(tickers, &ticker)
	}

	return &gctrpc.GetTickersResponse{Tickers: tickers}, nil
}

// GetOrderbook returns an orderbook for a specific exchange, currency pair
// and asset type
func (s *RPCServer) GetOrderbook(_ context.Context, r *gctrpc.GetOrderbookRequest) (*gctrpc.OrderbookResponse, error) {
	ob, err := GetSpecificOrderbook(currency.Pair{
		Delimiter: r.Pair.Delimiter,
		Base:      currency.NewCode(r.Pair.Base),
		Quote:     currency.NewCode(r.Pair.Quote),
	},
		r.Exchange,
		asset.Item(r.AssetType),
	)
	if err != nil {
		return nil, err
	}

	var bids []*gctrpc.OrderbookItem
	for x := range ob.Bids {
		bids = append(bids, &gctrpc.OrderbookItem{
			Amount: ob.Bids[x].Amount,
			Price:  ob.Bids[x].Price,
		})
	}

	var asks []*gctrpc.OrderbookItem
	for x := range ob.Asks {
		asks = append(asks, &gctrpc.OrderbookItem{
			Amount: ob.Asks[x].Amount,
			Price:  ob.Asks[x].Price,
		})
	}

	resp := &gctrpc.OrderbookResponse{
		Pair:        r.Pair,
		Bids:        bids,
		Asks:        asks,
		LastUpdated: ob.LastUpdated.Unix(),
		AssetType:   r.AssetType,
	}

	return resp, nil
}

// GetOrderbooks returns a list of orderbooks for all enabled exchanges and all
// enabled currency pairs
func (s *RPCServer) GetOrderbooks(_ context.Context, r *gctrpc.GetOrderbooksRequest) (*gctrpc.GetOrderbooksResponse, error) {
	activeOrderbooks := GetAllActiveOrderbooks()
	var orderbooks []*gctrpc.Orderbooks

	for x := range activeOrderbooks {
		var ob gctrpc.Orderbooks
		ob.Exchange = activeOrderbooks[x].ExchangeName
		for y := range activeOrderbooks[x].ExchangeValues {
			o := activeOrderbooks[x].ExchangeValues[y]
			var bids []*gctrpc.OrderbookItem
			for z := range o.Bids {
				bids = append(bids, &gctrpc.OrderbookItem{
					Amount: o.Bids[z].Amount,
					Price:  o.Bids[z].Price,
				})
			}

			var asks []*gctrpc.OrderbookItem
			for z := range o.Asks {
				asks = append(asks, &gctrpc.OrderbookItem{
					Amount: o.Asks[z].Amount,
					Price:  o.Asks[z].Price,
				})
			}

			ob.Orderbooks = append(ob.Orderbooks, &gctrpc.OrderbookResponse{
				Pair: &gctrpc.CurrencyPair{
					Delimiter: o.Pair.Delimiter,
					Base:      o.Pair.Base.String(),
					Quote:     o.Pair.Quote.String(),
				},
				LastUpdated: o.LastUpdated.Unix(),
				Bids:        bids,
				Asks:        asks,
			})
		}
		orderbooks = append(orderbooks, &ob)
	}

	return &gctrpc.GetOrderbooksResponse{Orderbooks: orderbooks}, nil
}

// GetAccountInfo returns an account balance for a specific exchange
func (s *RPCServer) GetAccountInfo(_ context.Context, r *gctrpc.GetAccountInfoRequest) (*gctrpc.GetAccountInfoResponse, error) {
	exch := GetExchangeByName(r.Exchange)
	if exch == nil {
		return nil, errExchangeNotLoaded
	}

	resp, err := exch.FetchAccountInfo()
	if err != nil {
		return nil, err
	}

	var accounts []*gctrpc.Account
	for x := range resp.Accounts {
		var a gctrpc.Account
		a.Id = resp.Accounts[x].ID
		for _, y := range resp.Accounts[x].Currencies {
			a.Currencies = append(a.Currencies, &gctrpc.AccountCurrencyInfo{
				Currency:   y.CurrencyName.String(),
				Hold:       y.Hold,
				TotalValue: y.TotalValue,
			})
		}
		accounts = append(accounts, &a)
	}

	return &gctrpc.GetAccountInfoResponse{Exchange: r.Exchange, Accounts: accounts}, nil
}

// GetAccountInfoStream streams an account balance for a specific exchange
func (s *RPCServer) GetAccountInfoStream(r *gctrpc.GetAccountInfoRequest, stream gctrpc.GoCryptoTrader_GetAccountInfoStreamServer) error {
	if r.Exchange == "" {
		return errors.New(errExchangeNameUnset)
	}

	exch := GetExchangeByName(r.Exchange)
	if exch == nil {
		return errExchangeNotLoaded
	}

	initAcc, err := exch.FetchAccountInfo()
	if err != nil {
		return err
	}

	var accounts []*gctrpc.Account
	for x := range initAcc.Accounts {
		var subAccounts []*gctrpc.AccountCurrencyInfo
		for y := range initAcc.Accounts[x].Currencies {
			subAccounts = append(subAccounts, &gctrpc.AccountCurrencyInfo{
				Currency:   initAcc.Accounts[x].Currencies[y].CurrencyName.String(),
				TotalValue: initAcc.Accounts[x].Currencies[y].TotalValue,
				Hold:       initAcc.Accounts[x].Currencies[y].Hold,
			})
		}
		accounts = append(accounts, &gctrpc.Account{
			Id:         initAcc.Accounts[x].ID,
			Currencies: subAccounts,
		})
	}

	err = stream.Send(&gctrpc.GetAccountInfoResponse{
		Exchange: initAcc.Exchange,
		Accounts: accounts,
	})
	if err != nil {
		return err
	}

	pipe, err := account.SubscribeToExchangeAccount(r.Exchange)
	if err != nil {
		return err
	}

	defer pipe.Release()

	for {
		data, ok := <-pipe.C
		if !ok {
			return errors.New(errDispatchSystem)
		}

		acc := (*data.(*interface{})).(account.Holdings)

		var accounts []*gctrpc.Account
		for x := range acc.Accounts {
			var subAccounts []*gctrpc.AccountCurrencyInfo
			for y := range acc.Accounts[x].Currencies {
				subAccounts = append(subAccounts, &gctrpc.AccountCurrencyInfo{
					Currency:   acc.Accounts[x].Currencies[y].CurrencyName.String(),
					TotalValue: acc.Accounts[x].Currencies[y].TotalValue,
					Hold:       acc.Accounts[x].Currencies[y].Hold,
				})
			}
			accounts = append(accounts, &gctrpc.Account{
				Id:         acc.Accounts[x].ID,
				Currencies: subAccounts,
			})
		}

		err := stream.Send(&gctrpc.GetAccountInfoResponse{
			Exchange: acc.Exchange,
			Accounts: accounts,
		})
		if err != nil {
			return err
		}
	}
}

// GetConfig returns the bots config
func (s *RPCServer) GetConfig(_ context.Context, r *gctrpc.GetConfigRequest) (*gctrpc.GetConfigResponse, error) {
	return &gctrpc.GetConfigResponse{}, common.ErrNotYetImplemented
}

// GetPortfolio returns the portfolio details
func (s *RPCServer) GetPortfolio(_ context.Context, r *gctrpc.GetPortfolioRequest) (*gctrpc.GetPortfolioResponse, error) {
	var addrs []*gctrpc.PortfolioAddress
	botAddrs := Bot.Portfolio.Addresses

	for x := range botAddrs {
		addrs = append(addrs, &gctrpc.PortfolioAddress{
			Address:     botAddrs[x].Address,
			CoinType:    botAddrs[x].CoinType.String(),
			Description: botAddrs[x].Description,
			Balance:     botAddrs[x].Balance,
		})
	}

	resp := &gctrpc.GetPortfolioResponse{
		Portfolio: addrs,
	}

	return resp, nil
}

// GetPortfolioSummary returns the portfolio summary
func (s *RPCServer) GetPortfolioSummary(_ context.Context, r *gctrpc.GetPortfolioSummaryRequest) (*gctrpc.GetPortfolioSummaryResponse, error) {
	result := Bot.Portfolio.GetPortfolioSummary()
	var resp gctrpc.GetPortfolioSummaryResponse

	p := func(coins []portfolio.Coin) []*gctrpc.Coin {
		var c []*gctrpc.Coin
		for x := range coins {
			c = append(c,
				&gctrpc.Coin{
					Coin:       coins[x].Coin.String(),
					Balance:    coins[x].Balance,
					Address:    coins[x].Address,
					Percentage: coins[x].Percentage,
				},
			)
		}
		return c
	}

	resp.CoinTotals = p(result.Totals)
	resp.CoinsOffline = p(result.Offline)
	resp.CoinsOfflineSummary = make(map[string]*gctrpc.OfflineCoins)
	for k, v := range result.OfflineSummary {
		var o []*gctrpc.OfflineCoinSummary
		for x := range v {
			o = append(o,
				&gctrpc.OfflineCoinSummary{
					Address:    v[x].Address,
					Balance:    v[x].Balance,
					Percentage: v[x].Percentage,
				},
			)
		}
		resp.CoinsOfflineSummary[k.String()] = &gctrpc.OfflineCoins{
			Addresses: o,
		}
	}
	resp.CoinsOnline = p(result.Online)
	resp.CoinsOnlineSummary = make(map[string]*gctrpc.OnlineCoins)
	for k, v := range result.OnlineSummary {
		o := make(map[string]*gctrpc.OnlineCoinSummary)
		for x, y := range v {
			o[x.String()] = &gctrpc.OnlineCoinSummary{
				Balance:    y.Balance,
				Percentage: y.Percentage,
			}
		}
		resp.CoinsOnlineSummary[k] = &gctrpc.OnlineCoins{
			Coins: o,
		}
	}

	return &resp, nil
}

// AddPortfolioAddress adds an address to the portfolio manager
func (s *RPCServer) AddPortfolioAddress(_ context.Context, r *gctrpc.AddPortfolioAddressRequest) (*gctrpc.GenericResponse, error) {
	err := Bot.Portfolio.AddAddress(r.Address,
		r.Description,
		currency.NewCode(r.CoinType),
		r.Balance)
	if err != nil {
		return nil, err
	}
	return &gctrpc.GenericResponse{Status: MsgStatusSuccess}, nil
}

// RemovePortfolioAddress removes an address from the portfolio manager
func (s *RPCServer) RemovePortfolioAddress(_ context.Context, r *gctrpc.RemovePortfolioAddressRequest) (*gctrpc.GenericResponse, error) {
	err := Bot.Portfolio.RemoveAddress(r.Address,
		r.Description,
		currency.NewCode(r.CoinType))
	if err != nil {
		return nil, err
	}
	return &gctrpc.GenericResponse{Status: MsgStatusSuccess}, nil
}

// GetForexProviders returns a list of available forex providers
func (s *RPCServer) GetForexProviders(_ context.Context, r *gctrpc.GetForexProvidersRequest) (*gctrpc.GetForexProvidersResponse, error) {
	providers := Bot.Config.GetForexProviders()
	if len(providers) == 0 {
		return nil, fmt.Errorf("forex providers is empty")
	}

	var forexProviders []*gctrpc.ForexProvider
	for x := range providers {
		forexProviders = append(forexProviders, &gctrpc.ForexProvider{
			Name:             providers[x].Name,
			Enabled:          providers[x].Enabled,
			Verbose:          providers[x].Verbose,
			RestPollingDelay: providers[x].RESTPollingDelay.String(),
			ApiKey:           providers[x].APIKey,
			ApiKeyLevel:      int64(providers[x].APIKeyLvl),
			PrimaryProvider:  providers[x].PrimaryProvider,
		})
	}
	return &gctrpc.GetForexProvidersResponse{ForexProviders: forexProviders}, nil
}

// GetForexRates returns a list of forex rates
func (s *RPCServer) GetForexRates(_ context.Context, r *gctrpc.GetForexRatesRequest) (*gctrpc.GetForexRatesResponse, error) {
	rates, err := currency.GetExchangeRates()
	if err != nil {
		return nil, err
	}

	if len(rates) == 0 {
		return nil, fmt.Errorf("forex rates is empty")
	}

	var forexRates []*gctrpc.ForexRatesConversion
	for x := range rates {
		rate, err := rates[x].GetRate()
		if err != nil {
			continue
		}

		// TODO
		// inverseRate, err := rates[x].GetInversionRate()
		// if err != nil {
		//	 continue
		// }

		forexRates = append(forexRates, &gctrpc.ForexRatesConversion{
			From:        rates[x].From.String(),
			To:          rates[x].To.String(),
			Rate:        rate,
			InverseRate: 0,
		})
	}
	return &gctrpc.GetForexRatesResponse{ForexRates: forexRates}, nil
}

// GetOrders returns all open orders, filtered by exchange, currency pair or
// asset type
func (s *RPCServer) GetOrders(_ context.Context, r *gctrpc.GetOrdersRequest) (*gctrpc.GetOrdersResponse, error) {
	exch := GetExchangeByName(r.Exchange)
	if exch == nil {
		return nil, errExchangeNotLoaded
	}

	resp, err := exch.GetActiveOrders(&order.GetOrdersRequest{
		Pairs: []currency.Pair{
			currency.NewPairWithDelimiter(r.Pair.Base,
				r.Pair.Quote, r.Pair.Delimiter),
		},
	})
	if err != nil {
		return nil, err
	}

	var orders []*gctrpc.OrderDetails
	for x := range resp {
		orders = append(orders, &gctrpc.OrderDetails{
			Exchange:      r.Exchange,
			Id:            resp[x].ID,
			BaseCurrency:  resp[x].Pair.Base.String(),
			QuoteCurrency: resp[x].Pair.Quote.String(),
			AssetType:     asset.Spot.String(),
			OrderType:     resp[x].Type.String(),
			OrderSide:     resp[x].Side.String(),
			CreationTime:  resp[x].Date.Unix(),
			Status:        resp[x].Status.String(),
			Price:         resp[x].Price,
			Amount:        resp[x].Amount,
		})
	}

	return &gctrpc.GetOrdersResponse{Orders: orders}, nil
}

// GetOrder returns order information based on exchange and order ID
func (s *RPCServer) GetOrder(_ context.Context, r *gctrpc.GetOrderRequest) (*gctrpc.OrderDetails, error) {
	exch := GetExchangeByName(r.Exchange)
	if exch == nil {
		return nil, errExchangeNotLoaded
	}
	result, err := exch.GetOrderInfo(r.OrderId)
	if err != nil {
		return nil, fmt.Errorf("error whilst trying to retrieve info for order %s: %s", r.OrderId, err)
	}
	var trades []*gctrpc.TradeHistory
	for i := range result.Trades {
		trades = append(trades, &gctrpc.TradeHistory{
			CreationTime: result.Trades[i].Timestamp.Unix(),
			Id:           result.Trades[i].TID,
			Price:        result.Trades[i].Price,
			Amount:       result.Trades[i].Amount,
			Exchange:     result.Trades[i].Exchange,
			AssetType:    result.Trades[i].Type.String(),
			OrderSide:    result.Trades[i].Side.String(),
			Fee:          result.Trades[i].Fee,
		})
	}
	return &gctrpc.OrderDetails{
		Exchange:      result.Exchange,
		Id:            result.ID,
		BaseCurrency:  result.Pair.Base.String(),
		QuoteCurrency: result.Pair.Quote.String(),
		AssetType:     result.AssetType.String(),
		OrderSide:     result.Side.String(),
		OrderType:     result.Type.String(),
		CreationTime:  result.Date.Unix(),
		Status:        result.Status.String(),
		Price:         result.Price,
		Amount:        result.Amount,
		OpenVolume:    result.RemainingAmount,
		Fee:           result.Fee,
		Trades:        trades,
	}, err
}

// SubmitOrder submits an order specified by exchange, currency pair and asset
// type
func (s *RPCServer) SubmitOrder(_ context.Context, r *gctrpc.SubmitOrderRequest) (*gctrpc.SubmitOrderResponse, error) {
	exch := GetExchangeByName(r.Exchange)
	if exch == nil {
		return nil, errExchangeNotLoaded
	}

	p, err := currency.NewPairFromStrings(r.Pair.Base, r.Pair.Quote)
	if err != nil {
		return nil, err
	}

	submission := &order.Submit{
		Pair:     p,
		Side:     order.Side(r.Side),
		Type:     order.Type(r.OrderType),
		Amount:   r.Amount,
		Price:    r.Price,
		ClientID: r.ClientId,
		Exchange: r.Exchange,
	}

	resp, err := exch.SubmitOrder(submission)
	if err != nil {
		return &gctrpc.SubmitOrderResponse{}, err
	}

	return &gctrpc.SubmitOrderResponse{
		OrderId:     resp.OrderID,
		OrderPlaced: resp.IsOrderPlaced,
	}, err
}

// SimulateOrder simulates an order specified by exchange, currency pair and asset
// type
func (s *RPCServer) SimulateOrder(_ context.Context, r *gctrpc.SimulateOrderRequest) (*gctrpc.SimulateOrderResponse, error) {
	exch := GetExchangeByName(r.Exchange)
	if exch == nil {
		return nil, errExchangeNotLoaded
	}

	p, err := currency.NewPairFromStrings(r.Pair.Base, r.Pair.Quote)
	if err != nil {
		return nil, err
	}

	o, err := exch.FetchOrderbook(p, asset.Spot)
	if err != nil {
		return nil, err
	}

	var buy = true
	if !strings.EqualFold(r.Side, order.Buy.String()) &&
		!strings.EqualFold(r.Side, order.Bid.String()) {
		buy = false
	}

	result := o.SimulateOrder(r.Amount, buy)
	var resp gctrpc.SimulateOrderResponse
	for x := range result.Orders {
		resp.Orders = append(resp.Orders, &gctrpc.OrderbookItem{
			Price:  result.Orders[x].Price,
			Amount: result.Orders[x].Amount,
		})
	}

	resp.Amount = result.Amount
	resp.MaximumPrice = result.MaximumPrice
	resp.MinimumPrice = result.MinimumPrice
	resp.PercentageGainLoss = result.PercentageGainOrLoss
	resp.Status = result.Status
	return &resp, nil
}

// WhaleBomb finds the amount required to reach a specific price target for a given exchange, pair
// and asset type
func (s *RPCServer) WhaleBomb(_ context.Context, r *gctrpc.WhaleBombRequest) (*gctrpc.SimulateOrderResponse, error) {
	exch := GetExchangeByName(r.Exchange)
	if exch == nil {
		return nil, errExchangeNotLoaded
	}

	p, err := currency.NewPairFromStrings(r.Pair.Base, r.Pair.Quote)
	if err != nil {
		return nil, err
	}

	o, err := exch.FetchOrderbook(p, asset.Spot)
	if err != nil {
		return nil, err
	}

	var buy = true
	if !strings.EqualFold(r.Side, order.Buy.String()) &&
		!strings.EqualFold(r.Side, order.Bid.String()) {
		buy = false
	}

	result, err := o.WhaleBomb(r.PriceTarget, buy)
	var resp gctrpc.SimulateOrderResponse
	for x := range result.Orders {
		resp.Orders = append(resp.Orders, &gctrpc.OrderbookItem{
			Price:  result.Orders[x].Price,
			Amount: result.Orders[x].Amount,
		})
	}

	resp.Amount = result.Amount
	resp.MaximumPrice = result.MaximumPrice
	resp.MinimumPrice = result.MinimumPrice
	resp.PercentageGainLoss = result.PercentageGainOrLoss
	resp.Status = result.Status
	return &resp, err
}

// CancelOrder cancels an order specified by exchange, currency pair and asset
// type
func (s *RPCServer) CancelOrder(_ context.Context, r *gctrpc.CancelOrderRequest) (*gctrpc.GenericResponse, error) {
	exch := GetExchangeByName(r.Exchange)
	if exch == nil {
		return nil, errExchangeNotLoaded
	}

	p, err := currency.NewPairFromStrings(r.Pair.Base, r.Pair.Quote)
	if err != nil {
		return nil, err
	}

	err = exch.CancelOrder(&order.Cancel{
		AccountID:     r.AccountId,
		ID:            r.OrderId,
		Side:          order.Side(r.Side),
		WalletAddress: r.WalletAddress,
		Pair:          p,
	})
	if err != nil {
		return nil, err
	}
	return &gctrpc.GenericResponse{Status: MsgStatusSuccess,
		Data: fmt.Sprintf("order %s cancelled", r.OrderId)}, nil
}

// CancelAllOrders cancels all orders, filterable by exchange
func (s *RPCServer) CancelAllOrders(_ context.Context, r *gctrpc.CancelAllOrdersRequest) (*gctrpc.CancelAllOrdersResponse, error) {
	return &gctrpc.CancelAllOrdersResponse{}, common.ErrNotYetImplemented
}

// GetEvents returns the stored events list
func (s *RPCServer) GetEvents(_ context.Context, r *gctrpc.GetEventsRequest) (*gctrpc.GetEventsResponse, error) {
	return &gctrpc.GetEventsResponse{}, common.ErrNotYetImplemented
}

// AddEvent adds an event
func (s *RPCServer) AddEvent(_ context.Context, r *gctrpc.AddEventRequest) (*gctrpc.AddEventResponse, error) {
	evtCondition := EventConditionParams{
		CheckBids:        r.ConditionParams.CheckBids,
		CheckBidsAndAsks: r.ConditionParams.CheckBidsAndAsks,
		Condition:        r.ConditionParams.Condition,
		OrderbookAmount:  r.ConditionParams.OrderbookAmount,
		Price:            r.ConditionParams.Price,
	}

	p := currency.NewPairWithDelimiter(r.Pair.Base,
		r.Pair.Quote, r.Pair.Delimiter)

	id, err := Add(r.Exchange, r.Item, evtCondition, p, asset.Item(r.AssetType), r.Action)
	if err != nil {
		return nil, err
	}

	return &gctrpc.AddEventResponse{Id: id}, nil
}

// RemoveEvent removes an event, specified by an event ID
func (s *RPCServer) RemoveEvent(_ context.Context, r *gctrpc.RemoveEventRequest) (*gctrpc.GenericResponse, error) {
	if !Remove(r.Id) {
		return nil, fmt.Errorf("event %d not removed", r.Id)
	}
	return &gctrpc.GenericResponse{Status: MsgStatusSuccess,
		Data: fmt.Sprintf("event %d removed", r.Id)}, nil
}

// GetCryptocurrencyDepositAddresses returns a list of cryptocurrency deposit
// addresses specified by an exchange
func (s *RPCServer) GetCryptocurrencyDepositAddresses(_ context.Context, r *gctrpc.GetCryptocurrencyDepositAddressesRequest) (*gctrpc.GetCryptocurrencyDepositAddressesResponse, error) {
	exch := GetExchangeByName(r.Exchange)
	if exch == nil {
		return nil, errExchangeNotLoaded
	}

	result, err := GetCryptocurrencyDepositAddressesByExchange(r.Exchange)
	return &gctrpc.GetCryptocurrencyDepositAddressesResponse{Addresses: result}, err
}

// GetCryptocurrencyDepositAddress returns a cryptocurrency deposit address
// specified by exchange and cryptocurrency
func (s *RPCServer) GetCryptocurrencyDepositAddress(_ context.Context, r *gctrpc.GetCryptocurrencyDepositAddressRequest) (*gctrpc.GetCryptocurrencyDepositAddressResponse, error) {
	exch := GetExchangeByName(r.Exchange)
	if exch == nil {
		return nil, errExchangeNotLoaded
	}

	addr, err := GetExchangeCryptocurrencyDepositAddress(r.Exchange, "", currency.NewCode(r.Cryptocurrency))
	return &gctrpc.GetCryptocurrencyDepositAddressResponse{Address: addr}, err
}

// WithdrawCryptocurrencyFunds withdraws cryptocurrency funds specified by
// exchange
func (s *RPCServer) WithdrawCryptocurrencyFunds(_ context.Context, r *gctrpc.WithdrawCryptoRequest) (*gctrpc.WithdrawResponse, error) {
	exch := GetExchangeByName(r.Exchange)
	if exch == nil {
		return nil, errExchangeNotLoaded
	}

	request := &withdraw.Request{
		Amount:      r.Amount,
		Currency:    currency.NewCode(strings.ToUpper(r.Currency)),
		Type:        withdraw.Crypto,
		Description: r.Description,
		Crypto: &withdraw.CryptoRequest{
			Address:    r.Address,
			AddressTag: r.AddressTag,
			FeeAmount:  r.Fee,
		},
	}

	resp, err := SubmitWithdrawal(r.Exchange, request)
	if err != nil {
		return nil, err
	}

	return &gctrpc.WithdrawResponse{
		Id:     resp.ID.String(),
		Status: resp.Exchange.Status,
	}, nil
}

// WithdrawFiatFunds withdraws fiat funds specified by exchange
func (s *RPCServer) WithdrawFiatFunds(_ context.Context, r *gctrpc.WithdrawFiatRequest) (*gctrpc.WithdrawResponse, error) {
	exch := GetExchangeByName(r.Exchange)
	if exch == nil {
		return nil, errExchangeNotLoaded
	}

	var bankAccount *banking.Account
	bankAccount, err := banking.GetBankAccountByID(r.BankAccountId)
	if err != nil {
		base := exch.GetBase()
		if base == nil {
			return nil, errExchangeBaseNotFound
		}
		bankAccount, err = base.GetExchangeBankAccounts(r.BankAccountId, r.Currency)
		if err != nil {
			return nil, err
		}
	}

	request := &withdraw.Request{
		Amount:      r.Amount,
		Currency:    currency.NewCode(strings.ToUpper(r.Currency)),
		Type:        withdraw.Fiat,
		Description: r.Description,
		Fiat: &withdraw.FiatRequest{
			Bank: bankAccount,
		},
	}
	resp, err := SubmitWithdrawal(r.Exchange, request)
	if err != nil {
		return nil, err
	}

	return &gctrpc.WithdrawResponse{
		Id:     resp.ID.String(),
		Status: resp.Exchange.Status,
	}, nil
}

// WithdrawalEventByID returns previous withdrawal request details
func (s *RPCServer) WithdrawalEventByID(_ context.Context, r *gctrpc.WithdrawalEventByIDRequest) (*gctrpc.WithdrawalEventByIDResponse, error) {
	if !Bot.Config.Database.Enabled {
		return nil, database.ErrDatabaseSupportDisabled
	}
	v, err := WithdrawalEventByID(r.Id)
	if err != nil {
		return nil, err
	}

	resp := &gctrpc.WithdrawalEventByIDResponse{
		Event: &gctrpc.WithdrawalEventResponse{
			Id: v.ID.String(),
			Exchange: &gctrpc.WithdrawlExchangeEvent{
				Name:   v.Exchange.Name,
				Id:     v.Exchange.Name,
				Status: v.Exchange.Status,
			},
			Request: &gctrpc.WithdrawalRequestEvent{
				Currency:    v.RequestDetails.Currency.String(),
				Description: v.RequestDetails.Description,
				Amount:      v.RequestDetails.Amount,
				Type:        int32(v.RequestDetails.Type),
			},
		},
	}
	createdAtPtype, err := ptypes.TimestampProto(v.CreatedAt)
	if err != nil {
		log.Errorf(log.Global, "failed to convert time: %v", err)
	}
	resp.Event.CreatedAt = createdAtPtype

	updatedAtPtype, err := ptypes.TimestampProto(v.UpdatedAt)
	if err != nil {
		log.Errorf(log.Global, "failed to convert time: %v", err)
	}
	resp.Event.UpdatedAt = updatedAtPtype

	if v.RequestDetails.Type == withdraw.Crypto {
		resp.Event.Request.Crypto = new(gctrpc.CryptoWithdrawalEvent)
		resp.Event.Request.Crypto = &gctrpc.CryptoWithdrawalEvent{
			Address:    v.RequestDetails.Crypto.Address,
			AddressTag: v.RequestDetails.Crypto.AddressTag,
			Fee:        v.RequestDetails.Crypto.FeeAmount,
		}
	} else if v.RequestDetails.Type == withdraw.Fiat {
		if v.RequestDetails.Fiat != nil {
			resp.Event.Request.Fiat = new(gctrpc.FiatWithdrawalEvent)
			resp.Event.Request.Fiat = &gctrpc.FiatWithdrawalEvent{
				BankName:      v.RequestDetails.Fiat.Bank.BankName,
				AccountName:   v.RequestDetails.Fiat.Bank.AccountName,
				AccountNumber: v.RequestDetails.Fiat.Bank.AccountNumber,
				Bsb:           v.RequestDetails.Fiat.Bank.BSBNumber,
				Swift:         v.RequestDetails.Fiat.Bank.SWIFTCode,
				Iban:          v.RequestDetails.Fiat.Bank.IBAN,
			}
		}
	}

	return resp, nil
}

// WithdrawalEventsByExchange returns previous withdrawal request details by exchange
func (s *RPCServer) WithdrawalEventsByExchange(_ context.Context, r *gctrpc.WithdrawalEventsByExchangeRequest) (*gctrpc.WithdrawalEventsByExchangeResponse, error) {
	if !Bot.Config.Database.Enabled {
		return nil, database.ErrDatabaseSupportDisabled
	}
	if r.Id == "" {
		ret, err := WithdrawalEventByExchange(r.Exchange, int(r.Limit))
		if err != nil {
			return nil, err
		}
		return parseMultipleEvents(ret), nil
	}

	ret, err := WithdrawalEventByExchangeID(r.Exchange, r.Id)
	if err != nil {
		return nil, err
	}

	return parseSingleEvents(ret), nil
}

// WithdrawalEventsByDate returns previous withdrawal request details by exchange
func (s *RPCServer) WithdrawalEventsByDate(_ context.Context, r *gctrpc.WithdrawalEventsByDateRequest) (*gctrpc.WithdrawalEventsByExchangeResponse, error) {
	UTCStartTime, err := time.Parse(common.SimpleTimeFormat, r.Start)
	if err != nil {
		return nil, err
	}

	UTCSEndTime, err := time.Parse(common.SimpleTimeFormat, r.End)
	if err != nil {
		return nil, err
	}

	ret, err := WithdrawEventByDate(r.Exchange, UTCStartTime, UTCSEndTime, int(r.Limit))
	if err != nil {
		return nil, err
	}
	return parseMultipleEvents(ret), nil
}

// GetLoggerDetails returns a loggers details
func (s *RPCServer) GetLoggerDetails(_ context.Context, r *gctrpc.GetLoggerDetailsRequest) (*gctrpc.GetLoggerDetailsResponse, error) {
	levels, err := log.Level(r.Logger)
	if err != nil {
		return nil, err
	}

	return &gctrpc.GetLoggerDetailsResponse{
		Info:  levels.Info,
		Debug: levels.Debug,
		Warn:  levels.Warn,
		Error: levels.Error,
	}, nil
}

// SetLoggerDetails sets a loggers details
func (s *RPCServer) SetLoggerDetails(_ context.Context, r *gctrpc.SetLoggerDetailsRequest) (*gctrpc.GetLoggerDetailsResponse, error) {
	levels, err := log.SetLevel(r.Logger, r.Level)
	if err != nil {
		return nil, err
	}

	return &gctrpc.GetLoggerDetailsResponse{
		Info:  levels.Info,
		Debug: levels.Debug,
		Warn:  levels.Warn,
		Error: levels.Error,
	}, nil
}

// GetExchangePairs returns a list of exchange supported assets and related pairs
func (s *RPCServer) GetExchangePairs(_ context.Context, r *gctrpc.GetExchangePairsRequest) (*gctrpc.GetExchangePairsResponse, error) {
	exchCfg, err := Bot.Config.GetExchangeConfig(r.Exchange)
	if err != nil {
		return nil, err
	}

	if r.Asset != "" &&
		!exchCfg.CurrencyPairs.GetAssetTypes().Contains(asset.Item(r.Asset)) {
		return nil, errors.New("specified asset type does not exist")
	}

	var resp gctrpc.GetExchangePairsResponse
	resp.SupportedAssets = make(map[string]*gctrpc.PairsSupported)
	assetTypes := exchCfg.CurrencyPairs.GetAssetTypes()
	for x := range assetTypes {
		if r.Asset != "" && !strings.EqualFold(assetTypes[x].String(), r.Asset) {
			continue
		}

		ps, err := exchCfg.CurrencyPairs.Get(assetTypes[x])
		if err != nil {
			return nil, err
		}

		resp.SupportedAssets[assetTypes[x].String()] = &gctrpc.PairsSupported{
			AvailablePairs: ps.Available.Join(),
			EnabledPairs:   ps.Enabled.Join(),
		}
	}
	return &resp, nil
}

// SetExchangePair enables/disabled the specified pair(s) on an exchange
func (s *RPCServer) SetExchangePair(_ context.Context, r *gctrpc.SetExchangePairRequest) (*gctrpc.GenericResponse, error) {
	exchCfg, err := Bot.Config.GetExchangeConfig(r.Exchange)
	if err != nil {
		return nil, err
	}

	if r.AssetType == "" {
		return nil, errors.New("asset type must be specified")
	}

	if !exchCfg.CurrencyPairs.GetAssetTypes().Contains(asset.Item(r.AssetType)) {
		return nil, errors.New("specified asset type does not exist")
	}

	a := asset.Item(r.AssetType)

	exch := GetExchangeByName(r.Exchange)
	if exch == nil {
		return nil, errExchangeNotLoaded
	}

	base := exch.GetBase()
	if base == nil {
		return nil, errExchangeBaseNotFound
	}

	pairFmt, err := Bot.Config.GetPairFormat(r.Exchange, a)
	if err != nil {
		return nil, err
	}
	var pass bool
	var newErrors common.Errors
	for i := range r.Pairs {
		var p currency.Pair
		p, err = currency.NewPairFromStrings(r.Pairs[i].Base, r.Pairs[i].Quote)
		if err != nil {
			return nil, err
		}

		if r.Enable {
			err = exchCfg.CurrencyPairs.EnablePair(a,
				p.Format(pairFmt.Delimiter, pairFmt.Uppercase))
			if err != nil {
				newErrors = append(newErrors, err)
				continue
			}
			err = base.CurrencyPairs.EnablePair(asset.Item(r.AssetType), p)
			if err != nil {
				newErrors = append(newErrors, err)
				continue
			}
			pass = true
			continue
		}

		err = exchCfg.CurrencyPairs.DisablePair(asset.Item(r.AssetType),
			p.Format(pairFmt.Delimiter, pairFmt.Uppercase))
		if err != nil {
			newErrors = append(newErrors, err)
			continue
		}
		err = base.CurrencyPairs.DisablePair(asset.Item(r.AssetType), p)
		if err != nil {
			newErrors = append(newErrors, err)
			continue
		}
		pass = true
	}

	if exch.IsWebsocketEnabled() && pass && base.Websocket.IsConnected() {
		err = exch.FlushWebsocketChannels()
		if err != nil {
			newErrors = append(newErrors, err)
		}
	}

	if newErrors != nil {
		return nil, newErrors
	}

	return &gctrpc.GenericResponse{Status: MsgStatusSuccess}, nil
}

// GetOrderbookStream streams the requested updated orderbook
func (s *RPCServer) GetOrderbookStream(r *gctrpc.GetOrderbookStreamRequest, stream gctrpc.GoCryptoTrader_GetOrderbookStreamServer) error {
	if r.Exchange == "" {
		return errors.New(errExchangeNameUnset)
	}

	if r.Pair.String() == "" {
		return errors.New(errCurrencyPairUnset)
	}

	if r.AssetType == "" {
		return errors.New(errAssetTypeUnset)
	}

	p, err := currency.NewPairFromStrings(r.Pair.Base, r.Pair.Quote)
	if err != nil {
		return err
	}

	pipe, err := orderbook.SubscribeOrderbook(r.Exchange, p, asset.Item(r.AssetType))
	if err != nil {
		return err
	}

	defer pipe.Release()

	for {
		data, ok := <-pipe.C
		if !ok {
			return errors.New(errDispatchSystem)
		}

		ob := (*data.(*interface{})).(orderbook.Base)
		var bids, asks []*gctrpc.OrderbookItem
		for i := range ob.Bids {
			bids = append(bids, &gctrpc.OrderbookItem{
				Amount: ob.Bids[i].Amount,
				Price:  ob.Bids[i].Price,
				Id:     ob.Bids[i].ID,
			})
		}
		for i := range ob.Asks {
			asks = append(asks, &gctrpc.OrderbookItem{
				Amount: ob.Asks[i].Amount,
				Price:  ob.Asks[i].Price,
				Id:     ob.Asks[i].ID,
			})
		}
		err := stream.Send(&gctrpc.OrderbookResponse{
			Pair: &gctrpc.CurrencyPair{Base: ob.Pair.Base.String(),
				Quote: ob.Pair.Quote.String()},
			Bids:      bids,
			Asks:      asks,
			AssetType: ob.AssetType.String(),
		})
		if err != nil {
			return err
		}
	}
}

// GetExchangeOrderbookStream streams all orderbooks associated with an exchange
func (s *RPCServer) GetExchangeOrderbookStream(r *gctrpc.GetExchangeOrderbookStreamRequest, stream gctrpc.GoCryptoTrader_GetExchangeOrderbookStreamServer) error {
	if r.Exchange == "" {
		return errors.New(errExchangeNameUnset)
	}

	pipe, err := orderbook.SubscribeToExchangeOrderbooks(r.Exchange)
	if err != nil {
		return err
	}

	defer pipe.Release()

	for {
		data, ok := <-pipe.C
		if !ok {
			return errors.New(errDispatchSystem)
		}

		ob := (*data.(*interface{})).(orderbook.Base)
		var bids, asks []*gctrpc.OrderbookItem
		for i := range ob.Bids {
			bids = append(bids, &gctrpc.OrderbookItem{
				Amount: ob.Bids[i].Amount,
				Price:  ob.Bids[i].Price,
				Id:     ob.Bids[i].ID,
			})
		}
		for i := range ob.Asks {
			asks = append(asks, &gctrpc.OrderbookItem{
				Amount: ob.Asks[i].Amount,
				Price:  ob.Asks[i].Price,
				Id:     ob.Asks[i].ID,
			})
		}
		err := stream.Send(&gctrpc.OrderbookResponse{
			Pair: &gctrpc.CurrencyPair{Base: ob.Pair.Base.String(),
				Quote: ob.Pair.Quote.String()},
			Bids:      bids,
			Asks:      asks,
			AssetType: ob.AssetType.String(),
		})
		if err != nil {
			return err
		}
	}
}

// GetTickerStream streams the requested updated ticker
func (s *RPCServer) GetTickerStream(r *gctrpc.GetTickerStreamRequest, stream gctrpc.GoCryptoTrader_GetTickerStreamServer) error {
	if r.Exchange == "" {
		return errors.New(errExchangeNameUnset)
	}

	if r.Pair.String() == "" {
		return errors.New(errCurrencyPairUnset)
	}

	if r.AssetType == "" {
		return errors.New(errAssetTypeUnset)
	}

	p, err := currency.NewPairFromStrings(r.Pair.Base, r.Pair.Quote)
	if err != nil {
		return err
	}

	pipe, err := ticker.SubscribeTicker(r.Exchange, p, asset.Item(r.AssetType))
	if err != nil {
		return err
	}

	defer pipe.Release()

	for {
		data, ok := <-pipe.C
		if !ok {
			return errors.New(errDispatchSystem)
		}
		t := (*data.(*interface{})).(ticker.Price)

		err := stream.Send(&gctrpc.TickerResponse{
			Pair: &gctrpc.CurrencyPair{
				Base:      t.Pair.Base.String(),
				Quote:     t.Pair.Quote.String(),
				Delimiter: t.Pair.Delimiter},
			LastUpdated: t.LastUpdated.Unix(),
			Last:        t.Last,
			High:        t.High,
			Low:         t.Low,
			Bid:         t.Bid,
			Ask:         t.Ask,
			Volume:      t.Volume,
			PriceAth:    t.PriceATH,
		})
		if err != nil {
			return err
		}
	}
}

// GetExchangeTickerStream streams all tickers associated with an exchange
func (s *RPCServer) GetExchangeTickerStream(r *gctrpc.GetExchangeTickerStreamRequest, stream gctrpc.GoCryptoTrader_GetExchangeTickerStreamServer) error {
	if r.Exchange == "" {
		return errors.New(errExchangeNameUnset)
	}

	pipe, err := ticker.SubscribeToExchangeTickers(r.Exchange)
	if err != nil {
		return err
	}

	defer pipe.Release()

	for {
		data, ok := <-pipe.C
		if !ok {
			return errors.New(errDispatchSystem)
		}
		t := (*data.(*interface{})).(ticker.Price)

		err := stream.Send(&gctrpc.TickerResponse{
			Pair: &gctrpc.CurrencyPair{
				Base:      t.Pair.Base.String(),
				Quote:     t.Pair.Quote.String(),
				Delimiter: t.Pair.Delimiter},
			LastUpdated: t.LastUpdated.Unix(),
			Last:        t.Last,
			High:        t.High,
			Low:         t.Low,
			Bid:         t.Bid,
			Ask:         t.Ask,
			Volume:      t.Volume,
			PriceAth:    t.PriceATH,
		})
		if err != nil {
			return err
		}
	}
}

// GetAuditEvent returns matching audit events from database
func (s *RPCServer) GetAuditEvent(_ context.Context, r *gctrpc.GetAuditEventRequest) (*gctrpc.GetAuditEventResponse, error) {
	UTCStartTime, err := time.Parse(common.SimpleTimeFormat, r.StartDate)
	if err != nil {
		return nil, err
	}

	UTCSEndTime, err := time.Parse(common.SimpleTimeFormat, r.EndDate)
	if err != nil {
		return nil, err
	}

	loc := time.FixedZone("", int(r.Offset))

	events, err := audit.GetEvent(UTCStartTime, UTCSEndTime, r.OrderBy, int(r.Limit))
	if err != nil {
		return nil, err
	}

	resp := gctrpc.GetAuditEventResponse{}

	switch v := events.(type) {
	case postgres.AuditEventSlice:
		for x := range v {
			tempEvent := &gctrpc.AuditEvent{
				Type:       v[x].Type,
				Identifier: v[x].Identifier,
				Message:    v[x].Message,
				Timestamp:  v[x].CreatedAt.In(loc).Format(common.SimpleTimeFormat),
			}

			resp.Events = append(resp.Events, tempEvent)
		}
	case sqlite3.AuditEventSlice:
		for x := range v {
			tempEvent := &gctrpc.AuditEvent{
				Type:       v[x].Type,
				Identifier: v[x].Identifier,
				Message:    v[x].Message,
				Timestamp:  v[x].CreatedAt,
			}
			resp.Events = append(resp.Events, tempEvent)
		}
	}

	return &resp, nil
}

// GetHistoricCandles returns historical candles for a given exchange
func (s *RPCServer) GetHistoricCandles(_ context.Context, req *gctrpc.GetHistoricCandlesRequest) (*gctrpc.GetHistoricCandlesResponse, error) {
	if req.Exchange == "" {
		return nil, errors.New(errExchangeNameUnset)
	}

	if req.Pair.String() == "" {
		return nil, errors.New(errCurrencyPairUnset)
	}

	exchange := GetExchangeByName(req.Exchange)
	if exchange == nil {
		return nil, errors.New("Exchange " + req.Exchange + " not found")
	}

	var candles kline.Item
	var err error
	if req.ExRequest {
		candles, err = exchange.GetHistoricCandlesExtended(currency.Pair{
			Delimiter: req.Pair.Delimiter,
			Base:      currency.NewCode(req.Pair.Base),
			Quote:     currency.NewCode(req.Pair.Quote),
		},
			asset.Item(strings.ToLower(req.AssetType)),
			time.Unix(req.Start, 0),
			time.Unix(req.End, 0),
			kline.Interval(req.TimeInterval))
		if err != nil {
			return nil, err
		}
	} else {
		candles, err = exchange.GetHistoricCandles(currency.Pair{
			Delimiter: req.Pair.Delimiter,
			Base:      currency.NewCode(req.Pair.Base),
			Quote:     currency.NewCode(req.Pair.Quote),
		},
			asset.Item(strings.ToLower(req.AssetType)),
			time.Unix(req.Start, 0),
			time.Unix(req.End, 0),
			kline.Interval(req.TimeInterval))
		if err != nil {
			return nil, err
		}
	}
	resp := gctrpc.GetHistoricCandlesResponse{
		Exchange: exchange.GetName(),
		Interval: kline.Interval(req.TimeInterval).Short(),
		Start:    req.Start,
		End:      req.End,
	}

	for i := range candles.Candles {
		resp.Candle = append(resp.Candle, &gctrpc.Candle{
			Time:   candles.Candles[i].Time.Unix(),
			Low:    candles.Candles[i].Low,
			High:   candles.Candles[i].High,
			Open:   candles.Candles[i].Open,
			Close:  candles.Candles[i].Close,
			Volume: candles.Candles[i].Volume,
		})
	}
	return &resp, nil
}

// GCTScriptStatus returns a slice of current running scripts that includes next run time and uuid
func (s *RPCServer) GCTScriptStatus(_ context.Context, r *gctrpc.GCTScriptStatusRequest) (*gctrpc.GCTScriptStatusResponse, error) {
	if !gctscript.GCTScriptConfig.Enabled {
		return &gctrpc.GCTScriptStatusResponse{Status: gctscript.ErrScriptingDisabled.Error()}, nil
	}

	if gctscript.VMSCount.Len() < 1 {
		return &gctrpc.GCTScriptStatusResponse{Status: "no scripts running"}, nil
	}

	resp := &gctrpc.GCTScriptStatusResponse{
		Status: fmt.Sprintf("%v of %v virtual machines running", gctscript.VMSCount.Len(), gctscript.GCTScriptConfig.MaxVirtualMachines),
	}

	gctscript.AllVMSync.Range(func(k, v interface{}) bool {
		vm := v.(*gctscript.VM)
		resp.Scripts = append(resp.Scripts, &gctrpc.GCTScript{
			UUID:    vm.ID.String(),
			Name:    vm.ShortName(),
			NextRun: vm.NextRun.String(),
		})

		return true
	})

	return resp, nil
}

// GCTScriptQuery queries a running script and returns script running information
func (s *RPCServer) GCTScriptQuery(_ context.Context, r *gctrpc.GCTScriptQueryRequest) (*gctrpc.GCTScriptQueryResponse, error) {
	if !gctscript.GCTScriptConfig.Enabled {
		return &gctrpc.GCTScriptQueryResponse{Status: gctscript.ErrScriptingDisabled.Error()}, nil
	}

	UUID, err := uuid.FromString(r.Script.UUID)
	if err != nil {
		return &gctrpc.GCTScriptQueryResponse{Status: MsgStatusError, Data: err.Error()}, nil
	}

	if v, f := gctscript.AllVMSync.Load(UUID); f {
		resp := &gctrpc.GCTScriptQueryResponse{
			Status: MsgStatusOK,
			Script: &gctrpc.GCTScript{
				Name:    v.(*gctscript.VM).ShortName(),
				UUID:    v.(*gctscript.VM).ID.String(),
				Path:    v.(*gctscript.VM).Path,
				NextRun: v.(*gctscript.VM).NextRun.String(),
			},
		}
		data, err := v.(*gctscript.VM).Read()
		if err != nil {
			return nil, err
		}
		resp.Data = string(data)
		return resp, nil
	}
	return &gctrpc.GCTScriptQueryResponse{Status: MsgStatusError, Data: "UUID not found"}, nil
}

// GCTScriptExecute execute a script
func (s *RPCServer) GCTScriptExecute(_ context.Context, r *gctrpc.GCTScriptExecuteRequest) (*gctrpc.GenericResponse, error) {
	if !gctscript.GCTScriptConfig.Enabled {
		return &gctrpc.GenericResponse{Status: gctscript.ErrScriptingDisabled.Error()}, nil
	}

	if r.Script.Path == "" {
		r.Script.Path = gctscript.ScriptPath
	}

	gctVM := gctscript.New()
	if gctVM == nil {
		return &gctrpc.GenericResponse{Status: MsgStatusError, Data: "unable to create VM instance"}, nil
	}

	script := filepath.Join(r.Script.Path, r.Script.Name)
	err := gctVM.Load(script)
	if err != nil {
		return &gctrpc.GenericResponse{
			Status: MsgStatusError,
			Data:   err.Error(),
		}, nil
	}

	go gctVM.CompileAndRun()

	return &gctrpc.GenericResponse{
		Status: MsgStatusOK,
		Data:   gctVM.ShortName() + " (" + gctVM.ID.String() + ") executed",
	}, nil
}

// GCTScriptStop terminate a running script
func (s *RPCServer) GCTScriptStop(_ context.Context, r *gctrpc.GCTScriptStopRequest) (*gctrpc.GenericResponse, error) {
	if !gctscript.GCTScriptConfig.Enabled {
		return &gctrpc.GenericResponse{Status: gctscript.ErrScriptingDisabled.Error()}, nil
	}

	UUID, err := uuid.FromString(r.Script.UUID)
	if err != nil {
		return &gctrpc.GenericResponse{Status: MsgStatusError, Data: err.Error()}, nil
	}

	if v, f := gctscript.AllVMSync.Load(UUID); f {
		err = v.(*gctscript.VM).Shutdown()
		status := " terminated"
		if err != nil {
			status = " " + err.Error()
		}
		return &gctrpc.GenericResponse{Status: MsgStatusOK, Data: v.(*gctscript.VM).ID.String() + status}, nil
	}
	return &gctrpc.GenericResponse{Status: MsgStatusError, Data: "no running script found"}, nil
}

// GCTScriptUpload upload a new script to ScriptPath
func (s *RPCServer) GCTScriptUpload(_ context.Context, r *gctrpc.GCTScriptUploadRequest) (*gctrpc.GenericResponse, error) {
	if !gctscript.GCTScriptConfig.Enabled {
		return &gctrpc.GenericResponse{Status: gctscript.ErrScriptingDisabled.Error()}, nil
	}

	fPath := filepath.Join(gctscript.ScriptPath, r.ScriptName)
	var fPathExits = fPath
	if filepath.Ext(fPath) == ".zip" {
		fPathExits = fPathExits[0 : len(fPathExits)-4]
	}

	if s, err := os.Stat(fPathExits); !os.IsNotExist(err) {
		if !r.Overwrite {
			return nil, fmt.Errorf("%s script found and overwrite set to false", r.ScriptName)
		}
		f := filepath.Join(gctscript.ScriptPath, "version_history")
		err = os.MkdirAll(f, 0770)
		if err != nil {
			return nil, err
		}
		timeString := strconv.FormatInt(time.Now().UnixNano(), 10)
		renamedFile := filepath.Join(f, timeString+"-"+filepath.Base(fPathExits))
		if s.IsDir() {
			err = archive.Zip(fPathExits, renamedFile+".zip")
			if err != nil {
				return nil, err
			}
		} else {
			err = file.Move(fPathExits, renamedFile)
			if err != nil {
				return nil, err
			}
		}
	}

	newFile, err := os.Create(fPath)
	if err != nil {
		return nil, err
	}

	_, err = newFile.Write(r.Data)
	if err != nil {
		return nil, err
	}
	err = newFile.Close()
	if err != nil {
		log.Errorln(log.Global, "Failed to close file handle, archive removal may fail")
	}

	if r.Archived {
		files, errExtract := archive.UnZip(fPath, filepath.Join(gctscript.ScriptPath, r.ScriptName[:len(r.ScriptName)-4]))
		if errExtract != nil {
			log.Errorf(log.Global, "Failed to archive zip file %v", errExtract)
			return &gctrpc.GenericResponse{Status: MsgStatusError, Data: errExtract.Error()}, nil
		}
		var failedFiles []string
		for x := range files {
			err = gctscript.Validate(files[x])
			if err != nil {
				failedFiles = append(failedFiles, files[x])
			}
		}
		err = os.Remove(fPath)
		if err != nil {
			return nil, err
		}
		if len(failedFiles) > 0 {
			err = os.RemoveAll(filepath.Join(gctscript.ScriptPath, r.ScriptName[:len(r.ScriptName)-4]))
			if err != nil {
				log.Errorf(log.GCTScriptMgr, "Failed to remove file %v (%v), manual deletion required", filepath.Base(fPath), err)
			}
			return &gctrpc.GenericResponse{Status: ErrScriptFailedValidation, Data: strings.Join(failedFiles, ", ")}, nil
		}
	} else {
		err = gctscript.Validate(fPath)
		if err != nil {
			errRemove := os.Remove(fPath)
			if errRemove != nil {
				log.Errorf(log.GCTScriptMgr, "Failed to remove file %v, manual deletion required: %v", filepath.Base(fPath), errRemove)
			}
			return &gctrpc.GenericResponse{Status: ErrScriptFailedValidation, Data: err.Error()}, nil
		}
	}

	return &gctrpc.GenericResponse{
		Status: MsgStatusOK,
		Data:   fmt.Sprintf("script %s written", newFile.Name()),
	}, nil
}

// GCTScriptReadScript read a script and return contents
func (s *RPCServer) GCTScriptReadScript(_ context.Context, r *gctrpc.GCTScriptReadScriptRequest) (*gctrpc.GCTScriptQueryResponse, error) {
	if !gctscript.GCTScriptConfig.Enabled {
		return &gctrpc.GCTScriptQueryResponse{Status: gctscript.ErrScriptingDisabled.Error()}, nil
	}

	filename := filepath.Join(gctscript.ScriptPath, r.Script.Name)
	if !strings.HasPrefix(filename, filepath.Clean(gctscript.ScriptPath)+string(os.PathSeparator)) {
		return nil, fmt.Errorf("%s: invalid file path", filename)
	}
	data, err := ioutil.ReadFile(filename)
	if err != nil {
		return nil, err
	}

	return &gctrpc.GCTScriptQueryResponse{
		Status: MsgStatusOK,
		Script: &gctrpc.GCTScript{
			Name: filepath.Base(filename),
			Path: filepath.Dir(filename),
		},
		Data: string(data),
	}, nil
}

// GCTScriptListAll lists all scripts inside the default script path
func (s *RPCServer) GCTScriptListAll(context.Context, *gctrpc.GCTScriptListAllRequest) (*gctrpc.GCTScriptStatusResponse, error) {
	if !gctscript.GCTScriptConfig.Enabled {
		return &gctrpc.GCTScriptStatusResponse{Status: gctscript.ErrScriptingDisabled.Error()}, nil
	}

	resp := &gctrpc.GCTScriptStatusResponse{}
	err := filepath.Walk(gctscript.ScriptPath,
		func(path string, info os.FileInfo, err error) error {
			if err != nil {
				return err
			}
			if filepath.Ext(path) == ".gct" {
				resp.Scripts = append(resp.Scripts, &gctrpc.GCTScript{
					Name: path,
				})
			}
			return nil
		})
	if err != nil {
		return nil, err
	}

	return resp, nil
}

// GCTScriptStopAll stops all running scripts
func (s *RPCServer) GCTScriptStopAll(context.Context, *gctrpc.GCTScriptStopAllRequest) (*gctrpc.GenericResponse, error) {
	if !gctscript.GCTScriptConfig.Enabled {
		return &gctrpc.GenericResponse{Status: gctscript.ErrScriptingDisabled.Error()}, nil
	}

	err := gctscript.ShutdownAll()
	if err != nil {
		return &gctrpc.GenericResponse{Status: "error", Data: err.Error()}, nil
	}

	return &gctrpc.GenericResponse{
		Status: MsgStatusOK,
		Data:   "all running scripts have been stopped",
	}, nil
}

// GCTScriptAutoLoadToggle adds or removes an entry to the autoload list
func (s *RPCServer) GCTScriptAutoLoadToggle(_ context.Context, r *gctrpc.GCTScriptAutoLoadRequest) (*gctrpc.GenericResponse, error) {
	if !gctscript.GCTScriptConfig.Enabled {
		return &gctrpc.GenericResponse{Status: gctscript.ErrScriptingDisabled.Error()}, nil
	}

	if r.Status {
		err := gctscript.Autoload(r.Script, true)
		if err != nil {
			return &gctrpc.GenericResponse{Status: "error", Data: err.Error()}, nil
		}
		return &gctrpc.GenericResponse{Status: "success", Data: "script " + r.Script + " removed from autoload list"}, nil
	}

	err := gctscript.Autoload(r.Script, false)
	if err != nil {
		return &gctrpc.GenericResponse{Status: "error", Data: err.Error()}, nil
	}
	return &gctrpc.GenericResponse{Status: "success", Data: "script " + r.Script + " added to autoload list"}, nil
}

// SetExchangeAsset enables or disables an exchanges asset type
func (s *RPCServer) SetExchangeAsset(_ context.Context, r *gctrpc.SetExchangeAssetRequest) (*gctrpc.GenericResponse, error) {
	exch := GetExchangeByName(r.Exchange)
	if exch == nil {
		return nil, errExchangeNotLoaded
	}

	exchCfg, err := Bot.Config.GetExchangeConfig(r.Exchange)
	if err != nil {
		return nil, err
	}

	base := exch.GetBase()
	if base == nil {
		return nil, errExchangeBaseNotFound
	}

	if r.Asset == "" {
		return nil, errors.New("asset type must be specified")
	}

	a := asset.Item(r.Asset)
	err = base.CurrencyPairs.SetAssetEnabled(a, r.Enable)
	if err != nil {
		return nil, err
	}
	err = exchCfg.CurrencyPairs.SetAssetEnabled(a, r.Enable)
	if err != nil {
		return nil, err
	}

	return &gctrpc.GenericResponse{Status: MsgStatusSuccess}, nil
}

// SetAllExchangePairs enables or disables an exchanges pairs
func (s *RPCServer) SetAllExchangePairs(_ context.Context, r *gctrpc.SetExchangeAllPairsRequest) (*gctrpc.GenericResponse, error) {
	exch := GetExchangeByName(r.Exchange)
	if exch == nil {
		return nil, errExchangeNotLoaded
	}

	exchCfg, err := Bot.Config.GetExchangeConfig(r.Exchange)
	if err != nil {
		return nil, err
	}

	base := exch.GetBase()
	if base == nil {
		return nil, errExchangeBaseNotFound
	}

	assets := base.CurrencyPairs.GetAssetTypes()

	if r.Enable {
		for i := range assets {
			var pairs currency.Pairs
			pairs, err = base.CurrencyPairs.GetPairs(assets[i], false)
			if err != nil {
				return nil, err
			}
			exchCfg.CurrencyPairs.StorePairs(assets[i], pairs, true)
			base.CurrencyPairs.StorePairs(assets[i], pairs, true)
		}
	} else {
		for i := range assets {
			exchCfg.CurrencyPairs.StorePairs(assets[i], nil, true)
			base.CurrencyPairs.StorePairs(assets[i], nil, true)
		}
	}

	if exch.IsWebsocketEnabled() && base.Websocket.IsConnected() {
		err = exch.FlushWebsocketChannels()
		if err != nil {
			return nil, err
		}
	}

	return &gctrpc.GenericResponse{Status: MsgStatusSuccess}, nil
}

// UpdateExchangeSupportedPairs forces an update of the supported pairs which
// will update the available pairs list and remove any assets that are disabled
// by the exchange
func (s *RPCServer) UpdateExchangeSupportedPairs(_ context.Context, r *gctrpc.UpdateExchangeSupportedPairsRequest) (*gctrpc.GenericResponse, error) {
	exch := GetExchangeByName(r.Exchange)
	if exch == nil {
		return nil, errExchangeNotLoaded
	}

	base := exch.GetBase()
	if base == nil {
		return nil, errExchangeBaseNotFound
	}

	if !base.GetEnabledFeatures().AutoPairUpdates {
		return nil,
			errors.New("cannot auto pair update for exchange, a manual update is needed")
	}

	err := exch.UpdateTradablePairs(false)
	if err != nil {
		return nil, err
	}

	if exch.IsWebsocketEnabled() {
		err = exch.FlushWebsocketChannels()
		if err != nil {
			return nil, err
		}
	}
	return &gctrpc.GenericResponse{Status: MsgStatusSuccess}, nil
}

// GetExchangeAssets returns the supported asset types
func (s *RPCServer) GetExchangeAssets(_ context.Context, r *gctrpc.GetExchangeAssetsRequest) (*gctrpc.GetExchangeAssetsResponse, error) {
	exch := GetExchangeByName(r.Exchange)
	if exch == nil {
		return nil, errExchangeNotLoaded
	}

	return &gctrpc.GetExchangeAssetsResponse{
		Assets: exch.GetAssetTypes().JoinToString(","),
	}, nil
}

// WebsocketGetInfo returns websocket connection information
func (s *RPCServer) WebsocketGetInfo(_ context.Context, r *gctrpc.WebsocketGetInfoRequest) (*gctrpc.WebsocketGetInfoResponse, error) {
	exch := GetExchangeByName(r.Exchange)
	if exch == nil {
		return nil, errExchangeNotLoaded
	}

	w, err := exch.GetWebsocket()
	if err != nil {
		return nil, err
	}

	return &gctrpc.WebsocketGetInfoResponse{
		Exchange:      exch.GetName(),
		Supported:     exch.SupportsWebsocket(),
		Enabled:       exch.IsWebsocketEnabled(),
		Authenticated: w.CanUseAuthenticatedEndpoints(),
		RunningUrl:    w.GetWebsocketURL(),
		ProxyAddress:  w.GetProxyAddress(),
	}, nil
}

// WebsocketSetEnabled enables or disables the websocket client
func (s *RPCServer) WebsocketSetEnabled(_ context.Context, r *gctrpc.WebsocketSetEnabledRequest) (*gctrpc.GenericResponse, error) {
	exch := GetExchangeByName(r.Exchange)
	if exch == nil {
		return nil, errExchangeNotLoaded
	}

	w, err := exch.GetWebsocket()
	if err != nil {
		return nil, fmt.Errorf("websocket not supported for exchange %s", r.Exchange)
	}

	exchCfg, err := Bot.Config.GetExchangeConfig(r.Exchange)
	if err != nil {
		return nil, err
	}

	if r.Enable {
		err = w.Enable()
		if err != nil {
			return nil, err
		}

		exchCfg.Features.Enabled.Websocket = true
		return &gctrpc.GenericResponse{Status: MsgStatusSuccess, Data: "websocket enabled"}, nil
	}

	err = w.Disable()
	if err != nil {
		return nil, err
	}
	exchCfg.Features.Enabled.Websocket = false
	return &gctrpc.GenericResponse{Status: MsgStatusSuccess, Data: "websocket disabled"}, nil
}

// WebsocketGetSubscriptions returns websocket subscription analysis
func (s *RPCServer) WebsocketGetSubscriptions(_ context.Context, r *gctrpc.WebsocketGetSubscriptionsRequest) (*gctrpc.WebsocketGetSubscriptionsResponse, error) {
	exch := GetExchangeByName(r.Exchange)
	if exch == nil {
		return nil, errExchangeNotLoaded
	}

	w, err := exch.GetWebsocket()
	if err != nil {
		return nil, fmt.Errorf("websocket not supported for exchange %s", r.Exchange)
	}

	payload := new(gctrpc.WebsocketGetSubscriptionsResponse)
	payload.Exchange = exch.GetName()
	subs := w.GetSubscriptions()
	for i := range subs {
		params, err := json.Marshal(subs[i].Params)
		if err != nil {
			return nil, err
		}
		payload.Subscriptions = append(payload.Subscriptions,
			&gctrpc.WebsocketSubscription{
				Channel:  subs[i].Channel,
				Currency: subs[i].Currency.String(),
				Asset:    subs[i].Asset.String(),
				Params:   string(params),
			})
	}
	return payload, nil
}

// WebsocketSetProxy sets client websocket connection proxy
func (s *RPCServer) WebsocketSetProxy(_ context.Context, r *gctrpc.WebsocketSetProxyRequest) (*gctrpc.GenericResponse, error) {
	exch := GetExchangeByName(r.Exchange)
	if exch == nil {
		return nil, errExchangeNotLoaded
	}

	w, err := exch.GetWebsocket()
	if err != nil {
		return nil, fmt.Errorf("websocket not supported for exchange %s", r.Exchange)
	}

	err = w.SetProxyAddress(r.Proxy)
	if err != nil {
		return nil, err
	}
	return &gctrpc.GenericResponse{Status: MsgStatusSuccess,
		Data: fmt.Sprintf("new proxy has been set [%s] for %s websocket connection",
			r.Exchange,
			r.Proxy)}, nil
}

// WebsocketSetURL sets exchange websocket client connection URL
func (s *RPCServer) WebsocketSetURL(_ context.Context, r *gctrpc.WebsocketSetURLRequest) (*gctrpc.GenericResponse, error) {
	exch := GetExchangeByName(r.Exchange)
	if exch == nil {
		return nil, errExchangeNotLoaded
	}

	w, err := exch.GetWebsocket()
	if err != nil {
		return nil, fmt.Errorf("websocket not supported for exchange %s", r.Exchange)
	}

	err = w.SetWebsocketURL(r.Url, false, true)
	if err != nil {
		return nil, err
	}
	return &gctrpc.GenericResponse{Status: MsgStatusSuccess,
		Data: fmt.Sprintf("new URL has been set [%s] for %s websocket connection",
			r.Exchange,
			r.Url)}, nil
}

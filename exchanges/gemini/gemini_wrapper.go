package gemini

import (
	"errors"
	"net/url"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/yurulab/gocryptotrader/common"
	"github.com/yurulab/gocryptotrader/config"
	"github.com/yurulab/gocryptotrader/currency"
	exchange "github.com/yurulab/gocryptotrader/exchanges"
	"github.com/yurulab/gocryptotrader/exchanges/account"
	"github.com/yurulab/gocryptotrader/exchanges/asset"
	"github.com/yurulab/gocryptotrader/exchanges/kline"
	"github.com/yurulab/gocryptotrader/exchanges/order"
	"github.com/yurulab/gocryptotrader/exchanges/orderbook"
	"github.com/yurulab/gocryptotrader/exchanges/protocol"
	"github.com/yurulab/gocryptotrader/exchanges/request"
	"github.com/yurulab/gocryptotrader/exchanges/stream"
	"github.com/yurulab/gocryptotrader/exchanges/ticker"
	"github.com/yurulab/gocryptotrader/log"
	"github.com/yurulab/gocryptotrader/portfolio/withdraw"
)

// GetDefaultConfig returns a default exchange config
func (g *Gemini) GetDefaultConfig() (*config.ExchangeConfig, error) {
	g.SetDefaults()
	exchCfg := new(config.ExchangeConfig)
	exchCfg.Name = g.Name
	exchCfg.HTTPTimeout = exchange.DefaultHTTPTimeout
	exchCfg.BaseCurrencies = g.BaseCurrencies

	err := g.SetupDefaults(exchCfg)
	if err != nil {
		return nil, err
	}

	if g.Features.Supports.RESTCapabilities.AutoPairUpdates {
		err := g.UpdateTradablePairs(true)
		if err != nil {
			return nil, err
		}
	}

	return exchCfg, nil
}

// SetDefaults sets package defaults for gemini exchange
func (g *Gemini) SetDefaults() {
	g.Name = "Gemini"
	g.Enabled = true
	g.Verbose = true
	g.API.CredentialsValidator.RequiresKey = true
	g.API.CredentialsValidator.RequiresSecret = true

	requestFmt := &currency.PairFormat{Uppercase: true}
	configFmt := &currency.PairFormat{Uppercase: true}
	err := g.SetGlobalPairsManager(requestFmt, configFmt, asset.Spot)
	if err != nil {
		log.Errorln(log.ExchangeSys, err)
	}

	g.Features = exchange.Features{
		Supports: exchange.FeaturesSupported{
			REST:      true,
			Websocket: true,
			RESTCapabilities: protocol.Features{
				TickerFetching:      true,
				TradeFetching:       true,
				OrderbookFetching:   true,
				AutoPairUpdates:     true,
				AccountInfo:         true,
				GetOrder:            true,
				CancelOrders:        true,
				CancelOrder:         true,
				SubmitOrder:         true,
				UserTradeHistory:    true,
				CryptoDeposit:       true,
				CryptoWithdrawal:    true,
				TradeFee:            true,
				FiatWithdrawalFee:   true,
				CryptoWithdrawalFee: true,
			},
			WebsocketCapabilities: protocol.Features{
				OrderbookFetching:      true,
				TradeFetching:          true,
				AuthenticatedEndpoints: true,
				MessageSequenceNumbers: true,
				KlineFetching:          true,
			},
			WithdrawPermissions: exchange.AutoWithdrawCryptoWithAPIPermission |
				exchange.AutoWithdrawCryptoWithSetup |
				exchange.WithdrawFiatViaWebsiteOnly,
		},
		Enabled: exchange.FeaturesEnabled{
			AutoPairUpdates: true,
		},
	}

	g.Requester = request.New(g.Name,
		common.NewHTTPClientWithTimeout(exchange.DefaultHTTPTimeout),
		request.WithLimiter(SetRateLimit()))

	g.API.Endpoints.URLDefault = geminiAPIURL
	g.API.Endpoints.URL = g.API.Endpoints.URLDefault
	g.API.Endpoints.WebsocketURL = geminiWebsocketEndpoint
	g.Websocket = stream.New()
	g.WebsocketResponseMaxLimit = exchange.DefaultWebsocketResponseMaxLimit
	g.WebsocketResponseCheckTimeout = exchange.DefaultWebsocketResponseCheckTimeout
	g.WebsocketOrderbookBufferLimit = exchange.DefaultWebsocketOrderbookBufferLimit
}

// Setup sets exchange configuration parameters
func (g *Gemini) Setup(exch *config.ExchangeConfig) error {
	if !exch.Enabled {
		g.SetEnabled(false)
		return nil
	}

	err := g.SetupDefaults(exch)
	if err != nil {
		return err
	}

	if exch.UseSandbox {
		g.API.Endpoints.URL = geminiSandboxAPIURL
	}

	return g.Websocket.Setup(&stream.WebsocketSetup{
		Enabled:                          exch.Features.Enabled.Websocket,
		Verbose:                          exch.Verbose,
		AuthenticatedWebsocketAPISupport: exch.API.AuthenticatedWebsocketSupport,
		WebsocketTimeout:                 exch.WebsocketTrafficTimeout,
		DefaultURL:                       geminiWebsocketEndpoint,
		ExchangeName:                     exch.Name,
		RunningURL:                       exch.API.Endpoints.WebsocketURL,
		Connector:                        g.WsConnect,
		Features:                         &g.Features.Supports.WebsocketCapabilities,
		OrderbookBufferLimit:             exch.WebsocketOrderbookBufferLimit,
		BufferEnabled:                    true,
		SortBuffer:                       true,
	})
}

// Start starts the Gemini go routine
func (g *Gemini) Start(wg *sync.WaitGroup) {
	wg.Add(1)
	go func() {
		g.Run()
		wg.Done()
	}()
}

// Run implements the Gemini wrapper
func (g *Gemini) Run() {
	if g.Verbose {
		g.PrintEnabledPairs()
	}

	if !g.GetEnabledFeatures().AutoPairUpdates {
		return
	}

	err := g.UpdateTradablePairs(false)
	if err != nil {
		log.Errorf(log.ExchangeSys, "%s failed to update tradable pairs. Err: %s", g.Name, err)
	}
}

// FetchTradablePairs returns a list of the exchanges tradable pairs
func (g *Gemini) FetchTradablePairs(asset asset.Item) ([]string, error) {
	return g.GetSymbols()
}

// UpdateTradablePairs updates the exchanges available pairs and stores
// them in the exchanges config
func (g *Gemini) UpdateTradablePairs(forceUpdate bool) error {
	pairs, err := g.GetSymbols()
	if err != nil {
		return err
	}

	p, err := currency.NewPairsFromStrings(pairs)
	if err != nil {
		return err
	}

	return g.UpdatePairs(p, asset.Spot, false, forceUpdate)
}

// UpdateAccountInfo Retrieves balances for all enabled currencies for the
// Gemini exchange
func (g *Gemini) UpdateAccountInfo() (account.Holdings, error) {
	var response account.Holdings
	response.Exchange = g.Name
	accountBalance, err := g.GetBalances()
	if err != nil {
		return response, err
	}

	var currencies []account.Balance
	for i := range accountBalance {
		var exchangeCurrency account.Balance
		exchangeCurrency.CurrencyName = currency.NewCode(accountBalance[i].Currency)
		exchangeCurrency.TotalValue = accountBalance[i].Amount
		exchangeCurrency.Hold = accountBalance[i].Available
		currencies = append(currencies, exchangeCurrency)
	}

	response.Accounts = append(response.Accounts, account.SubAccount{
		Currencies: currencies,
	})

	err = account.Process(&response)
	if err != nil {
		return account.Holdings{}, err
	}

	return response, nil
}

// FetchAccountInfo retrieves balances for all enabled currencies
func (g *Gemini) FetchAccountInfo() (account.Holdings, error) {
	acc, err := account.GetHoldings(g.Name)
	if err != nil {
		return g.UpdateAccountInfo()
	}

	return acc, nil
}

// UpdateTicker updates and returns the ticker for a currency pair
func (g *Gemini) UpdateTicker(p currency.Pair, assetType asset.Item) (*ticker.Price, error) {
	tick, err := g.GetTicker(p.String())
	if err != nil {
		return nil, err
	}

	err = ticker.ProcessTicker(&ticker.Price{
		High:         tick.High,
		Low:          tick.Low,
		Bid:          tick.Bid,
		Ask:          tick.Ask,
		Open:         tick.Open,
		Close:        tick.Close,
		Pair:         p,
		ExchangeName: g.Name,
		AssetType:    assetType})
	if err != nil {
		return nil, err
	}

	return ticker.GetTicker(g.Name, p, assetType)
}

// FetchTicker returns the ticker for a currency pair
func (g *Gemini) FetchTicker(p currency.Pair, assetType asset.Item) (*ticker.Price, error) {
	tickerNew, err := ticker.GetTicker(g.Name, p, assetType)
	if err != nil {
		return g.UpdateTicker(p, assetType)
	}
	return tickerNew, nil
}

// FetchOrderbook returns orderbook base on the currency pair
func (g *Gemini) FetchOrderbook(p currency.Pair, assetType asset.Item) (*orderbook.Base, error) {
	ob, err := orderbook.Get(g.Name, p, assetType)
	if err != nil {
		return g.UpdateOrderbook(p, assetType)
	}
	return ob, nil
}

// UpdateOrderbook updates and returns the orderbook for a currency pair
func (g *Gemini) UpdateOrderbook(p currency.Pair, assetType asset.Item) (*orderbook.Base, error) {
	orderBook := new(orderbook.Base)
	orderbookNew, err := g.GetOrderbook(p.String(), url.Values{})
	if err != nil {
		return orderBook, err
	}

	for x := range orderbookNew.Bids {
		orderBook.Bids = append(orderBook.Bids, orderbook.Item{Amount: orderbookNew.Bids[x].Amount, Price: orderbookNew.Bids[x].Price})
	}

	for x := range orderbookNew.Asks {
		orderBook.Asks = append(orderBook.Asks, orderbook.Item{Amount: orderbookNew.Asks[x].Amount, Price: orderbookNew.Asks[x].Price})
	}

	orderBook.Pair = p
	orderBook.ExchangeName = g.Name
	orderBook.AssetType = assetType

	err = orderBook.Process()
	if err != nil {
		return orderBook, err
	}

	return orderbook.Get(g.Name, p, assetType)
}

// GetFundingHistory returns funding history, deposits and
// withdrawals
func (g *Gemini) GetFundingHistory() ([]exchange.FundHistory, error) {
	return nil, common.ErrFunctionNotSupported
}

// GetExchangeHistory returns historic trade data within the timeframe provided.
func (g *Gemini) GetExchangeHistory(p currency.Pair, assetType asset.Item, timestampStart, timestampEnd time.Time) ([]exchange.TradeHistory, error) {
	return nil, common.ErrNotYetImplemented
}

// SubmitOrder submits a new order
func (g *Gemini) SubmitOrder(s *order.Submit) (order.SubmitResponse, error) {
	var submitOrderResponse order.SubmitResponse
	if err := s.Validate(); err != nil {
		return submitOrderResponse, err
	}

	if s.Type != order.Limit {
		return submitOrderResponse,
			errors.New("only limit orders are enabled through this exchange")
	}

	fpair, err := g.FormatExchangeCurrency(s.Pair, asset.Spot)
	if err != nil {
		return submitOrderResponse, err
	}

	response, err := g.NewOrder(fpair.String(),
		s.Amount,
		s.Price,
		s.Side.String(),
		"exchange limit")
	if err != nil {
		return submitOrderResponse, err
	}
	if response > 0 {
		submitOrderResponse.OrderID = strconv.FormatInt(response, 10)
	}

	submitOrderResponse.IsOrderPlaced = true

	return submitOrderResponse, nil
}

// ModifyOrder will allow of changing orderbook placement and limit to
// market conversion
func (g *Gemini) ModifyOrder(action *order.Modify) (string, error) {
	return "", common.ErrFunctionNotSupported
}

// CancelOrder cancels an order by its corresponding ID number
func (g *Gemini) CancelOrder(order *order.Cancel) error {
	orderIDInt, err := strconv.ParseInt(order.ID, 10, 64)
	if err != nil {
		return err
	}

	_, err = g.CancelExistingOrder(orderIDInt)
	return err
}

// CancelAllOrders cancels all orders associated with a currency pair
func (g *Gemini) CancelAllOrders(_ *order.Cancel) (order.CancelAllResponse, error) {
	cancelAllOrdersResponse := order.CancelAllResponse{
		Status: make(map[string]string),
	}
	resp, err := g.CancelExistingOrders(false)
	if err != nil {
		return cancelAllOrdersResponse, err
	}

	for i := range resp.Details.CancelRejects {
		cancelAllOrdersResponse.Status[resp.Details.CancelRejects[i]] = "Could not cancel order"
	}

	return cancelAllOrdersResponse, nil
}

// GetOrderInfo returns information on a current open order
func (g *Gemini) GetOrderInfo(orderID string) (order.Detail, error) {
	var orderDetail order.Detail
	return orderDetail, common.ErrNotYetImplemented
}

// GetDepositAddress returns a deposit address for a specified currency
func (g *Gemini) GetDepositAddress(cryptocurrency currency.Code, _ string) (string, error) {
	addr, err := g.GetCryptoDepositAddress("", cryptocurrency.String())
	if err != nil {
		return "", err
	}
	return addr.Address, nil
}

// WithdrawCryptocurrencyFunds returns a withdrawal ID when a withdrawal is
// submitted
func (g *Gemini) WithdrawCryptocurrencyFunds(withdrawRequest *withdraw.Request) (*withdraw.ExchangeResponse, error) {
	resp, err := g.WithdrawCrypto(withdrawRequest.Crypto.Address, withdrawRequest.Currency.String(), withdrawRequest.Amount)
	if err != nil {
		return nil, err
	}
	if resp.Result == "error" {
		return nil, errors.New(resp.Message)
	}

	return &withdraw.ExchangeResponse{
		ID: resp.TXHash,
	}, err
}

// WithdrawFiatFunds returns a withdrawal ID when a
// withdrawal is submitted
func (g *Gemini) WithdrawFiatFunds(withdrawRequest *withdraw.Request) (*withdraw.ExchangeResponse, error) {
	return nil, common.ErrFunctionNotSupported
}

// WithdrawFiatFundsToInternationalBank returns a withdrawal ID when a
// withdrawal is submitted
func (g *Gemini) WithdrawFiatFundsToInternationalBank(withdrawRequest *withdraw.Request) (*withdraw.ExchangeResponse, error) {
	return nil, common.ErrFunctionNotSupported
}

// GetFeeByType returns an estimate of fee based on type of transaction
func (g *Gemini) GetFeeByType(feeBuilder *exchange.FeeBuilder) (float64, error) {
	if (!g.AllowAuthenticatedRequest() || g.SkipAuthCheck) && // Todo check connection status
		feeBuilder.FeeType == exchange.CryptocurrencyTradeFee {
		feeBuilder.FeeType = exchange.OfflineTradeFee
	}
	return g.GetFee(feeBuilder)
}

// GetActiveOrders retrieves any orders that are active/open
func (g *Gemini) GetActiveOrders(req *order.GetOrdersRequest) ([]order.Detail, error) {
	resp, err := g.GetOrders()
	if err != nil {
		return nil, err
	}

	format, err := g.GetPairFormat(asset.Spot, false)
	if err != nil {
		return nil, err
	}

	var orders []order.Detail
	for i := range resp {
		var symbol currency.Pair
		symbol, err = currency.NewPairDelimiter(resp[i].Symbol, format.Delimiter)
		if err != nil {
			return nil, err
		}
		var orderType order.Type
		if resp[i].Type == "exchange limit" {
			orderType = order.Limit
		} else if resp[i].Type == "market buy" || resp[i].Type == "market sell" {
			orderType = order.Market
		}

		side := order.Side(strings.ToUpper(resp[i].Type))
		orderDate := time.Unix(resp[i].Timestamp, 0)

		orders = append(orders, order.Detail{
			Amount:          resp[i].OriginalAmount,
			RemainingAmount: resp[i].RemainingAmount,
			ID:              strconv.FormatInt(resp[i].OrderID, 10),
			ExecutedAmount:  resp[i].ExecutedAmount,
			Exchange:        g.Name,
			Type:            orderType,
			Side:            side,
			Price:           resp[i].Price,
			Pair:            symbol,
			Date:            orderDate,
		})
	}

	order.FilterOrdersByTickRange(&orders, req.StartTicks, req.EndTicks)
	order.FilterOrdersBySide(&orders, req.Side)
	order.FilterOrdersByType(&orders, req.Type)
	order.FilterOrdersByCurrencies(&orders, req.Pairs)
	return orders, nil
}

// GetOrderHistory retrieves account order information
// Can Limit response to specific order status
func (g *Gemini) GetOrderHistory(req *order.GetOrdersRequest) ([]order.Detail, error) {
	if len(req.Pairs) == 0 {
		return nil, errors.New("currency must be supplied")
	}

	var trades []TradeHistory
	for j := range req.Pairs {
		fpair, err := g.FormatExchangeCurrency(req.Pairs[j], asset.Spot)
		if err != nil {
			return nil, err
		}

		resp, err := g.GetTradeHistory(fpair.String(), req.StartTicks.Unix())
		if err != nil {
			return nil, err
		}

		for i := range resp {
			resp[i].BaseCurrency = req.Pairs[j].Base.String()
			resp[i].QuoteCurrency = req.Pairs[j].Quote.String()
			trades = append(trades, resp[i])
		}
	}

	format, err := g.GetPairFormat(asset.Spot, false)
	if err != nil {
		return nil, err
	}

	var orders []order.Detail
	for i := range trades {
		side := order.Side(strings.ToUpper(trades[i].Type))
		orderDate := time.Unix(trades[i].Timestamp, 0)

		orders = append(orders, order.Detail{
			Amount:   trades[i].Amount,
			ID:       strconv.FormatInt(trades[i].OrderID, 10),
			Exchange: g.Name,
			Date:     orderDate,
			Side:     side,
			Fee:      trades[i].FeeAmount,
			Price:    trades[i].Price,
			Pair: currency.NewPairWithDelimiter(trades[i].BaseCurrency,
				trades[i].QuoteCurrency,
				format.Delimiter),
		})
	}

	order.FilterOrdersByTickRange(&orders, req.StartTicks, req.EndTicks)
	order.FilterOrdersBySide(&orders, req.Side)
	return orders, nil
}

// ValidateCredentials validates current credentials used for wrapper
// functionality
func (g *Gemini) ValidateCredentials() error {
	_, err := g.UpdateAccountInfo()
	return g.CheckTransientError(err)
}

// GetHistoricCandles returns candles between a time period for a set time interval
func (g *Gemini) GetHistoricCandles(pair currency.Pair, a asset.Item, start, end time.Time, interval kline.Interval) (kline.Item, error) {
	return kline.Item{}, common.ErrFunctionNotSupported
}

// GetHistoricCandlesExtended returns candles between a time period for a set time interval
func (g *Gemini) GetHistoricCandlesExtended(pair currency.Pair, a asset.Item, start, end time.Time, interval kline.Interval) (kline.Item, error) {
	return kline.Item{}, common.ErrFunctionNotSupported
}

package zb

import (
	"errors"
	"fmt"
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
func (z *ZB) GetDefaultConfig() (*config.ExchangeConfig, error) {
	z.SetDefaults()
	exchCfg := new(config.ExchangeConfig)
	exchCfg.Name = z.Name
	exchCfg.HTTPTimeout = exchange.DefaultHTTPTimeout
	exchCfg.BaseCurrencies = z.BaseCurrencies

	err := z.SetupDefaults(exchCfg)
	if err != nil {
		return nil, err
	}

	if z.Features.Supports.RESTCapabilities.AutoPairUpdates {
		err = z.UpdateTradablePairs(true)
		if err != nil {
			return nil, err
		}
	}

	return exchCfg, nil
}

// SetDefaults sets default values for the exchange
func (z *ZB) SetDefaults() {
	z.Name = "ZB"
	z.Enabled = true
	z.Verbose = true
	z.API.CredentialsValidator.RequiresKey = true
	z.API.CredentialsValidator.RequiresSecret = true

	requestFmt := &currency.PairFormat{Delimiter: currency.UnderscoreDelimiter}
	configFmt := &currency.PairFormat{Delimiter: currency.UnderscoreDelimiter, Uppercase: true}
	err := z.SetGlobalPairsManager(requestFmt, configFmt, asset.Spot)
	if err != nil {
		log.Errorln(log.ExchangeSys, err)
	}

	z.Features = exchange.Features{
		Supports: exchange.FeaturesSupported{
			REST:      true,
			Websocket: true,
			RESTCapabilities: protocol.Features{
				TickerBatching:      true,
				TickerFetching:      true,
				KlineFetching:       true,
				OrderbookFetching:   true,
				AutoPairUpdates:     true,
				AccountInfo:         true,
				GetOrder:            true,
				GetOrders:           true,
				CancelOrder:         true,
				CryptoDeposit:       true,
				CryptoWithdrawal:    true,
				TradeFee:            true,
				CryptoDepositFee:    true,
				CryptoWithdrawalFee: true,
			},
			WebsocketCapabilities: protocol.Features{
				TickerFetching:         true,
				TradeFetching:          true,
				OrderbookFetching:      true,
				Subscribe:              true,
				AuthenticatedEndpoints: true,
				AccountInfo:            true,
				CancelOrder:            true,
				SubmitOrder:            true,
				MessageCorrelation:     true,
				GetOrders:              true,
				GetOrder:               true,
			},
			WithdrawPermissions: exchange.AutoWithdrawCrypto |
				exchange.NoFiatWithdrawals,
			Kline: kline.ExchangeCapabilitiesSupported{
				Intervals: true,
			},
		},
		Enabled: exchange.FeaturesEnabled{
			AutoPairUpdates: true,
			Kline: kline.ExchangeCapabilitiesEnabled{
				Intervals: map[string]bool{
					kline.OneMin.Word():     true,
					kline.ThreeMin.Word():   true,
					kline.FiveMin.Word():    true,
					kline.FifteenMin.Word(): true,
					kline.ThirtyMin.Word():  true,
					kline.OneHour.Word():    true,
					kline.TwoHour.Word():    true,
					kline.FourHour.Word():   true,
					kline.SixHour.Word():    true,
					kline.TwelveHour.Word(): true,
					kline.OneDay.Word():     true,
					kline.ThreeDay.Word():   true,
					kline.OneWeek.Word():    true,
				},
				ResultLimit: 1000,
			},
		},
	}

	z.Requester = request.New(z.Name,
		common.NewHTTPClientWithTimeout(exchange.DefaultHTTPTimeout),
		request.WithLimiter(SetRateLimit()))

	z.API.Endpoints.URLDefault = zbTradeURL
	z.API.Endpoints.URL = z.API.Endpoints.URLDefault
	z.API.Endpoints.URLSecondaryDefault = zbMarketURL
	z.API.Endpoints.URLSecondary = z.API.Endpoints.URLSecondaryDefault
	z.API.Endpoints.WebsocketURL = zbWebsocketAPI
	z.Websocket = stream.New()
	z.WebsocketResponseMaxLimit = exchange.DefaultWebsocketResponseMaxLimit
	z.WebsocketResponseCheckTimeout = exchange.DefaultWebsocketResponseCheckTimeout
}

// Setup sets user configuration
func (z *ZB) Setup(exch *config.ExchangeConfig) error {
	if !exch.Enabled {
		z.SetEnabled(false)
		return nil
	}

	err := z.SetupDefaults(exch)
	if err != nil {
		return err
	}

	err = z.Websocket.Setup(&stream.WebsocketSetup{
		Enabled:                          exch.Features.Enabled.Websocket,
		Verbose:                          exch.Verbose,
		AuthenticatedWebsocketAPISupport: exch.API.AuthenticatedWebsocketSupport,
		WebsocketTimeout:                 exch.WebsocketTrafficTimeout,
		DefaultURL:                       zbWebsocketAPI,
		ExchangeName:                     exch.Name,
		RunningURL:                       exch.API.Endpoints.WebsocketURL,
		Connector:                        z.WsConnect,
		GenerateSubscriptions:            z.GenerateDefaultSubscriptions,
		Subscriber:                       z.Subscribe,
		Features:                         &z.Features.Supports.WebsocketCapabilities,
		OrderbookBufferLimit:             exch.WebsocketOrderbookBufferLimit,
	})
	if err != nil {
		return err
	}

	return z.Websocket.SetupNewConnection(stream.ConnectionSetup{
		URL:                  z.Websocket.GetWebsocketURL(),
		RateLimit:            zbWebsocketRateLimit,
		ResponseCheckTimeout: exch.WebsocketResponseCheckTimeout,
		ResponseMaxLimit:     exch.WebsocketResponseMaxLimit,
	})
}

// Start starts the OKEX go routine
func (z *ZB) Start(wg *sync.WaitGroup) {
	wg.Add(1)
	go func() {
		z.Run()
		wg.Done()
	}()
}

// Run implements the OKEX wrapper
func (z *ZB) Run() {
	if z.Verbose {
		z.PrintEnabledPairs()
	}

	if !z.GetEnabledFeatures().AutoPairUpdates {
		return
	}

	err := z.UpdateTradablePairs(false)
	if err != nil {
		log.Errorf(log.ExchangeSys, "%s failed to update tradable pairs. Err: %s", z.Name, err)
	}
}

// FetchTradablePairs returns a list of the exchanges tradable pairs
func (z *ZB) FetchTradablePairs(asset asset.Item) ([]string, error) {
	markets, err := z.GetMarkets()
	if err != nil {
		return nil, err
	}

	var currencies []string
	for x := range markets {
		currencies = append(currencies, x)
	}

	return currencies, nil
}

// UpdateTradablePairs updates the exchanges available pairs and stores
// them in the exchanges config
func (z *ZB) UpdateTradablePairs(forceUpdate bool) error {
	pairs, err := z.FetchTradablePairs(asset.Spot)
	if err != nil {
		return err
	}
	p, err := currency.NewPairsFromStrings(pairs)
	if err != nil {
		return err
	}
	return z.UpdatePairs(p, asset.Spot, false, forceUpdate)
}

// UpdateTicker updates and returns the ticker for a currency pair
func (z *ZB) UpdateTicker(p currency.Pair, assetType asset.Item) (*ticker.Price, error) {
	result, err := z.GetTickers()
	if err != nil {
		return nil, err
	}

	enabledPairs, err := z.GetEnabledPairs(assetType)
	if err != nil {
		return nil, err
	}
	for x := range enabledPairs {
		// We can't use either pair format here, so format it to lower-
		// case and without any delimiter
		curr := enabledPairs[x].Format("", false).String()
		if _, ok := result[curr]; !ok {
			continue
		}

		err = ticker.ProcessTicker(&ticker.Price{
			Pair:         enabledPairs[x],
			High:         result[curr].High,
			Last:         result[curr].Last,
			Ask:          result[curr].Sell,
			Bid:          result[curr].Buy,
			Low:          result[curr].Low,
			Volume:       result[curr].Volume,
			ExchangeName: z.Name,
			AssetType:    assetType})
		if err != nil {
			return nil, err
		}
	}

	return ticker.GetTicker(z.Name, p, assetType)
}

// FetchTicker returns the ticker for a currency pair
func (z *ZB) FetchTicker(p currency.Pair, assetType asset.Item) (*ticker.Price, error) {
	tickerNew, err := ticker.GetTicker(z.Name, p, assetType)
	if err != nil {
		return z.UpdateTicker(p, assetType)
	}
	return tickerNew, nil
}

// FetchOrderbook returns orderbook base on the currency pair
func (z *ZB) FetchOrderbook(p currency.Pair, assetType asset.Item) (*orderbook.Base, error) {
	ob, err := orderbook.Get(z.Name, p, assetType)
	if err != nil {
		return z.UpdateOrderbook(p, assetType)
	}
	return ob, nil
}

// UpdateOrderbook updates and returns the orderbook for a currency pair
func (z *ZB) UpdateOrderbook(p currency.Pair, assetType asset.Item) (*orderbook.Base, error) {
	orderBook := new(orderbook.Base)
	currFormat, err := z.FormatExchangeCurrency(p, assetType)
	if err != nil {
		return nil, err
	}

	orderbookNew, err := z.GetOrderbook(currFormat.String())
	if err != nil {
		return orderBook, err
	}

	for x := range orderbookNew.Bids {
		orderBook.Bids = append(orderBook.Bids, orderbook.Item{
			Amount: orderbookNew.Bids[x][1],
			Price:  orderbookNew.Bids[x][0],
		})
	}

	for x := range orderbookNew.Asks {
		orderBook.Asks = append(orderBook.Asks, orderbook.Item{
			Amount: orderbookNew.Asks[x][1],
			Price:  orderbookNew.Asks[x][0],
		})
	}

	orderBook.Pair = p
	orderBook.AssetType = assetType
	orderBook.ExchangeName = z.Name

	err = orderBook.Process()
	if err != nil {
		return orderBook, err
	}

	return orderbook.Get(z.Name, p, assetType)
}

// UpdateAccountInfo retrieves balances for all enabled currencies for the
// ZB exchange
func (z *ZB) UpdateAccountInfo() (account.Holdings, error) {
	var info account.Holdings
	var balances []account.Balance
	var coins []AccountsResponseCoin
	if z.Websocket.CanUseAuthenticatedWebsocketForWrapper() {
		resp, err := z.wsGetAccountInfoRequest()
		if err != nil {
			return info, err
		}
		coins = resp.Data.Coins
	} else {
		bal, err := z.GetAccountInformation()
		if err != nil {
			return info, err
		}
		coins = bal.Result.Coins
	}

	for i := range coins {
		hold, err := strconv.ParseFloat(coins[i].Freeze, 64)
		if err != nil {
			return info, err
		}

		avail, err := strconv.ParseFloat(coins[i].Available, 64)
		if err != nil {
			return info, err
		}

		balances = append(balances, account.Balance{
			CurrencyName: currency.NewCode(coins[i].EnName),
			TotalValue:   hold + avail,
			Hold:         hold,
		})
	}

	info.Exchange = z.Name
	info.Accounts = append(info.Accounts, account.SubAccount{
		Currencies: balances,
	})

	err := account.Process(&info)
	if err != nil {
		return account.Holdings{}, err
	}

	return info, nil
}

// FetchAccountInfo retrieves balances for all enabled currencies
func (z *ZB) FetchAccountInfo() (account.Holdings, error) {
	acc, err := account.GetHoldings(z.Name)
	if err != nil {
		return z.UpdateAccountInfo()
	}

	return acc, nil
}

// GetFundingHistory returns funding history, deposits and
// withdrawals
func (z *ZB) GetFundingHistory() ([]exchange.FundHistory, error) {
	return nil, common.ErrFunctionNotSupported
}

// GetExchangeHistory returns historic trade data within the timeframe provided.
func (z *ZB) GetExchangeHistory(p currency.Pair, assetType asset.Item, timestampStart, timestampEnd time.Time) ([]exchange.TradeHistory, error) {
	return nil, common.ErrNotYetImplemented
}

// SubmitOrder submits a new order
func (z *ZB) SubmitOrder(o *order.Submit) (order.SubmitResponse, error) {
	var submitOrderResponse order.SubmitResponse
	err := o.Validate()
	if err != nil {
		return submitOrderResponse, err
	}
	if z.Websocket.CanUseAuthenticatedWebsocketForWrapper() {
		var isBuyOrder int64
		if o.Side == order.Buy {
			isBuyOrder = 1
		} else {
			isBuyOrder = 0
		}
		var response *WsSubmitOrderResponse
		response, err = z.wsSubmitOrder(o.Pair, o.Amount, o.Price, isBuyOrder)
		if err != nil {
			return submitOrderResponse, err
		}
		submitOrderResponse.OrderID = strconv.FormatInt(response.Data.EntrustID, 10)
	} else {
		var oT SpotNewOrderRequestParamsType
		if o.Side == order.Buy {
			oT = SpotNewOrderRequestParamsTypeBuy
		} else {
			oT = SpotNewOrderRequestParamsTypeSell
		}

		var params = SpotNewOrderRequestParams{
			Amount: o.Amount,
			Price:  o.Price,
			Symbol: o.Pair.Lower().String(),
			Type:   oT,
		}
		var response int64
		response, err = z.SpotNewOrder(params)
		if err != nil {
			return submitOrderResponse, err
		}
		if response > 0 {
			submitOrderResponse.OrderID = strconv.FormatInt(response, 10)
		}
	}
	submitOrderResponse.IsOrderPlaced = true
	if o.Type == order.Market {
		submitOrderResponse.FullyMatched = true
	}
	return submitOrderResponse, nil
}

// ModifyOrder will allow of changing orderbook placement and limit to
// market conversion
func (z *ZB) ModifyOrder(action *order.Modify) (string, error) {
	return "", common.ErrFunctionNotSupported
}

// CancelOrder cancels an order by its corresponding ID number
func (z *ZB) CancelOrder(o *order.Cancel) error {
	orderIDInt, err := strconv.ParseInt(o.ID, 10, 64)
	if err != nil {
		return err
	}

	if z.Websocket.CanUseAuthenticatedWebsocketForWrapper() {
		var response *WsCancelOrderResponse
		response, err = z.wsCancelOrder(o.Pair, orderIDInt)
		if err != nil {
			return err
		}
		if !response.Success {
			return fmt.Errorf("%v - Could not cancel order %v", z.Name, o.ID)
		}
		return nil
	}
	fpair, err := z.FormatExchangeCurrency(o.Pair, o.AssetType)
	if err != nil {
		return err
	}
	return z.CancelExistingOrder(orderIDInt, fpair.String())
}

// CancelAllOrders cancels all orders associated with a currency pair
func (z *ZB) CancelAllOrders(_ *order.Cancel) (order.CancelAllResponse, error) {
	cancelAllOrdersResponse := order.CancelAllResponse{
		Status: make(map[string]string),
	}
	var allOpenOrders []Order
	enabledPairs, err := z.GetEnabledPairs(asset.Spot)
	if err != nil {
		return cancelAllOrdersResponse, err
	}

	for x := range enabledPairs {
		fPair, err := z.FormatExchangeCurrency(enabledPairs[x], asset.Spot)
		if err != nil {
			return cancelAllOrdersResponse, err
		}
		for y := int64(1); ; y++ {
			openOrders, err := z.GetUnfinishedOrdersIgnoreTradeType(fPair.String(), y, 10)
			if err != nil {
				if strings.Contains(err.Error(), "3001") {
					break
				}
				return cancelAllOrdersResponse, err
			}

			if len(openOrders) == 0 {
				break
			}

			allOpenOrders = append(allOpenOrders, openOrders...)

			if len(openOrders) != 10 {
				break
			}
		}
	}

	for i := range allOpenOrders {
		p, err := currency.NewPairFromString(allOpenOrders[i].Currency)
		if err != nil {
			cancelAllOrdersResponse.Status[strconv.FormatInt(allOpenOrders[i].ID, 10)] = err.Error()
			continue
		}

		err = z.CancelOrder(&order.Cancel{
			ID:   strconv.FormatInt(allOpenOrders[i].ID, 10),
			Pair: p,
		})
		if err != nil {
			cancelAllOrdersResponse.Status[strconv.FormatInt(allOpenOrders[i].ID, 10)] = err.Error()
		}
	}

	return cancelAllOrdersResponse, nil
}

// GetOrderInfo returns information on a current open order
func (z *ZB) GetOrderInfo(orderID string) (order.Detail, error) {
	var orderDetail order.Detail
	return orderDetail, common.ErrNotYetImplemented
}

// GetDepositAddress returns a deposit address for a specified currency
func (z *ZB) GetDepositAddress(cryptocurrency currency.Code, _ string) (string, error) {
	address, err := z.GetCryptoAddress(cryptocurrency)
	if err != nil {
		return "", err
	}

	return address.Message.Data.Key, nil
}

// WithdrawCryptocurrencyFunds returns a withdrawal ID when a withdrawal is
// submitted
func (z *ZB) WithdrawCryptocurrencyFunds(withdrawRequest *withdraw.Request) (*withdraw.ExchangeResponse, error) {
	v, err := z.Withdraw(withdrawRequest.Currency.Lower().String(), withdrawRequest.Crypto.Address, withdrawRequest.TradePassword, withdrawRequest.Amount, withdrawRequest.Crypto.FeeAmount, false)
	if err != nil {
		return nil, err
	}
	return &withdraw.ExchangeResponse{
		ID: v,
	}, nil
}

// WithdrawFiatFunds returns a withdrawal ID when a
// withdrawal is submitted
func (z *ZB) WithdrawFiatFunds(withdrawRequest *withdraw.Request) (*withdraw.ExchangeResponse, error) {
	return nil, common.ErrFunctionNotSupported
}

// WithdrawFiatFundsToInternationalBank returns a withdrawal ID when a
// withdrawal is submitted
func (z *ZB) WithdrawFiatFundsToInternationalBank(withdrawRequest *withdraw.Request) (*withdraw.ExchangeResponse, error) {
	return nil, common.ErrFunctionNotSupported
}

// GetFeeByType returns an estimate of fee based on type of transaction
func (z *ZB) GetFeeByType(feeBuilder *exchange.FeeBuilder) (float64, error) {
	if !z.AllowAuthenticatedRequest() && // Todo check connection status
		feeBuilder.FeeType == exchange.CryptocurrencyTradeFee {
		feeBuilder.FeeType = exchange.OfflineTradeFee
	}
	return z.GetFee(feeBuilder)
}

// GetActiveOrders retrieves any orders that are active/open
// This function is not concurrency safe due to orderSide/orderType maps
func (z *ZB) GetActiveOrders(req *order.GetOrdersRequest) ([]order.Detail, error) {
	var allOrders []Order
	for x := range req.Pairs {
		for i := int64(1); ; i++ {
			fPair, err := z.FormatExchangeCurrency(req.Pairs[x], asset.Spot)
			if err != nil {
				return nil, err
			}
			resp, err := z.GetUnfinishedOrdersIgnoreTradeType(fPair.String(), i, 10)
			if err != nil {
				if strings.Contains(err.Error(), "3001") {
					break
				}
				return nil, err
			}

			if len(resp) == 0 {
				break
			}

			allOrders = append(allOrders, resp...)

			if len(resp) != 10 {
				break
			}
		}
	}

	format, err := z.GetPairFormat(asset.Spot, false)
	if err != nil {
		return nil, err
	}

	var orders []order.Detail
	for i := range allOrders {
		var symbol currency.Pair
		symbol, err = currency.NewPairDelimiter(allOrders[i].Currency,
			format.Delimiter)
		if err != nil {
			return nil, err
		}
		orderDate := time.Unix(int64(allOrders[i].TradeDate), 0)
		orderSide := orderSideMap[allOrders[i].Type]
		orders = append(orders, order.Detail{
			ID:       strconv.FormatInt(allOrders[i].ID, 10),
			Amount:   allOrders[i].TotalAmount,
			Exchange: z.Name,
			Date:     orderDate,
			Price:    allOrders[i].Price,
			Side:     orderSide,
			Pair:     symbol,
		})
	}

	order.FilterOrdersByTickRange(&orders, req.StartTicks, req.EndTicks)
	order.FilterOrdersBySide(&orders, req.Side)
	return orders, nil
}

// GetOrderHistory retrieves account order information
// Can Limit response to specific order status
// This function is not concurrency safe due to orderSide/orderType maps
func (z *ZB) GetOrderHistory(req *order.GetOrdersRequest) ([]order.Detail, error) {
	if req.Side == order.AnySide || req.Side == "" {
		return nil, errors.New("specific order side is required")
	}
	var allOrders []Order
	var orders []order.Detail
	var side int64

	if z.Websocket.CanUseAuthenticatedWebsocketForWrapper() {
		for x := range req.Pairs {
			for y := int64(1); ; y++ {
				resp, err := z.wsGetOrdersIgnoreTradeType(req.Pairs[x], y, 10)
				if err != nil {
					return nil, err
				}
				allOrders = append(allOrders, resp.Data...)
				if len(resp.Data) != 10 {
					break
				}
			}
		}
	} else {
		if req.Side == order.Buy {
			side = 1
		}
		for x := range req.Pairs {
			for y := int64(1); ; y++ {
				fPair, err := z.FormatExchangeCurrency(req.Pairs[x], asset.Spot)
				if err != nil {
					return nil, err
				}
				resp, err := z.GetOrders(fPair.String(), y, side)
				if err != nil {
					return nil, err
				}
				if len(resp) == 0 {
					break
				}
				allOrders = append(allOrders, resp...)
				if len(resp) != 10 {
					break
				}
			}
		}
	}

	format, err := z.GetPairFormat(asset.Spot, false)
	if err != nil {
		return nil, err
	}

	for i := range allOrders {
		var symbol currency.Pair
		symbol, err = currency.NewPairDelimiter(allOrders[i].Currency,
			format.Delimiter)
		if err != nil {
			return nil, err
		}
		orderDate := time.Unix(int64(allOrders[i].TradeDate), 0)
		orderSide := orderSideMap[allOrders[i].Type]
		orders = append(orders, order.Detail{
			ID:       strconv.FormatInt(allOrders[i].ID, 10),
			Amount:   allOrders[i].TotalAmount,
			Exchange: z.Name,
			Date:     orderDate,
			Price:    allOrders[i].Price,
			Side:     orderSide,
			Pair:     symbol,
		})
	}

	order.FilterOrdersByTickRange(&orders, req.StartTicks, req.EndTicks)
	return orders, nil
}

// ValidateCredentials validates current credentials used for wrapper
// functionality
func (z *ZB) ValidateCredentials() error {
	_, err := z.UpdateAccountInfo()
	return z.CheckTransientError(err)
}

// FormatExchangeKlineInterval returns Interval to exchange formatted string
func (z *ZB) FormatExchangeKlineInterval(in kline.Interval) string {
	switch in {
	case kline.OneMin, kline.ThreeMin,
		kline.FiveMin, kline.FifteenMin, kline.ThirtyMin:
		return in.Short() + "in"
	case kline.OneHour, kline.TwoHour, kline.FourHour, kline.SixHour, kline.TwelveHour:
		return in.Short()[:len(in.Short())-1] + "hour"
	case kline.OneDay:
		return "1day"
	case kline.ThreeDay:
		return "3day"
	case kline.OneWeek:
		return "1week"
	}
	return ""
}

// GetHistoricCandles returns candles between a time period for a set time interval
func (z *ZB) GetHistoricCandles(pair currency.Pair, a asset.Item, start, end time.Time, interval kline.Interval) (kline.Item, error) {
	if !z.KlineIntervalEnabled(interval) {
		return kline.Item{}, kline.ErrorKline{
			Interval: interval,
		}
	}

	formattedPair, err := z.FormatExchangeCurrency(pair, a)
	if err != nil {
		return kline.Item{}, err
	}

	klineParams := KlinesRequestParams{
		Type:   z.FormatExchangeKlineInterval(interval),
		Symbol: formattedPair.String(),
	}

	candles, err := z.GetSpotKline(klineParams)
	if err != nil {
		return kline.Item{}, err
	}

	ret := kline.Item{
		Exchange: z.Name,
		Pair:     pair,
		Asset:    a,
		Interval: interval,
	}

	for x := range candles.Data {
		if candles.Data[x].KlineTime.Before(start) || candles.Data[x].KlineTime.After(end) {
			continue
		}
		ret.Candles = append(ret.Candles, kline.Candle{
			Time:   candles.Data[x].KlineTime,
			Open:   candles.Data[x].Open,
			High:   candles.Data[x].Close,
			Low:    candles.Data[x].Low,
			Close:  candles.Data[x].Close,
			Volume: candles.Data[x].Volume,
		})
	}

	ret.SortCandlesByTimestamp(false)
	return ret, nil
}

// GetHistoricCandlesExtended returns candles between a time period for a set time interval
func (z *ZB) GetHistoricCandlesExtended(p currency.Pair, a asset.Item, start, end time.Time, interval kline.Interval) (kline.Item, error) {
	return z.GetHistoricCandles(p, a, start, end, interval)
}

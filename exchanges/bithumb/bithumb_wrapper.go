package bithumb

import (
	"errors"
	"fmt"
	"math"
	"strconv"
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
	"github.com/yurulab/gocryptotrader/exchanges/ticker"
	"github.com/yurulab/gocryptotrader/log"
	"github.com/yurulab/gocryptotrader/portfolio/withdraw"
)

// GetDefaultConfig returns a default exchange config
func (b *Bithumb) GetDefaultConfig() (*config.ExchangeConfig, error) {
	b.SetDefaults()
	exchCfg := new(config.ExchangeConfig)
	exchCfg.Name = b.Name
	exchCfg.HTTPTimeout = exchange.DefaultHTTPTimeout
	exchCfg.BaseCurrencies = b.BaseCurrencies

	err := b.SetupDefaults(exchCfg)
	if err != nil {
		return nil, err
	}

	if b.Features.Supports.RESTCapabilities.AutoPairUpdates {
		err = b.UpdateTradablePairs(true)
		if err != nil {
			return nil, err
		}
	}

	return exchCfg, nil
}

// SetDefaults sets the basic defaults for Bithumb
func (b *Bithumb) SetDefaults() {
	b.Name = "Bithumb"
	b.Enabled = true
	b.Verbose = true
	b.API.CredentialsValidator.RequiresKey = true
	b.API.CredentialsValidator.RequiresSecret = true

	requestFmt := &currency.PairFormat{Uppercase: true, Delimiter: currency.UnderscoreDelimiter}
	configFmt := &currency.PairFormat{Uppercase: true, Index: "KRW"}
	err := b.SetGlobalPairsManager(requestFmt, configFmt, asset.Spot)
	if err != nil {
		log.Errorln(log.ExchangeSys, err)
	}

	b.Features = exchange.Features{
		Supports: exchange.FeaturesSupported{
			REST: true,
			RESTCapabilities: protocol.Features{
				TickerBatching:      true,
				TickerFetching:      true,
				TradeFetching:       true,
				OrderbookFetching:   true,
				AutoPairUpdates:     true,
				AccountInfo:         true,
				CryptoWithdrawal:    true,
				FiatDeposit:         true,
				FiatWithdraw:        true,
				GetOrder:            true,
				CancelOrder:         true,
				SubmitOrder:         true,
				ModifyOrder:         true,
				DepositHistory:      true,
				WithdrawalHistory:   true,
				UserTradeHistory:    true,
				TradeFee:            true,
				FiatWithdrawalFee:   true,
				CryptoDepositFee:    true,
				CryptoWithdrawalFee: true,
				KlineFetching:       true,
			},
			WithdrawPermissions: exchange.AutoWithdrawCrypto |
				exchange.AutoWithdrawFiat,
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
					kline.TenMin.Word():     true,
					kline.ThirtyMin.Word():  true,
					kline.OneHour.Word():    true,
					kline.SixHour.Word():    true,
					kline.TwelveHour.Word(): true,
					kline.OneDay.Word():     true,
				},
			},
		},
	}
	b.Requester = request.New(b.Name,
		common.NewHTTPClientWithTimeout(exchange.DefaultHTTPTimeout),
		request.WithLimiter(SetRateLimit()))

	b.API.Endpoints.URLDefault = apiURL
	b.API.Endpoints.URL = b.API.Endpoints.URLDefault
}

// Setup takes in the supplied exchange configuration details and sets params
func (b *Bithumb) Setup(exch *config.ExchangeConfig) error {
	if !exch.Enabled {
		b.SetEnabled(false)
		return nil
	}
	return b.SetupDefaults(exch)
}

// Start starts the Bithumb go routine
func (b *Bithumb) Start(wg *sync.WaitGroup) {
	wg.Add(1)
	go func() {
		b.Run()
		wg.Done()
	}()
}

// Run implements the Bithumb wrapper
func (b *Bithumb) Run() {
	if b.Verbose {
		b.PrintEnabledPairs()
	}

	if !b.GetEnabledFeatures().AutoPairUpdates {
		return
	}

	err := b.UpdateTradablePairs(false)
	if err != nil {
		log.Errorf(log.ExchangeSys, "%s failed to update tradable pairs. Err: %s", b.Name, err)
	}
}

// FetchTradablePairs returns a list of the exchanges tradable pairs
func (b *Bithumb) FetchTradablePairs(asset asset.Item) ([]string, error) {
	currencies, err := b.GetTradablePairs()
	if err != nil {
		return nil, err
	}

	for x := range currencies {
		currencies[x] += "KRW"
	}

	return currencies, nil
}

// UpdateTradablePairs updates the exchanges available pairs and stores
// them in the exchanges config
func (b *Bithumb) UpdateTradablePairs(forceUpdate bool) error {
	pairs, err := b.FetchTradablePairs(asset.Spot)
	if err != nil {
		return err
	}

	p, err := currency.NewPairsFromStrings(pairs)
	if err != nil {
		return err
	}

	return b.UpdatePairs(p, asset.Spot, false, forceUpdate)
}

// UpdateTicker updates and returns the ticker for a currency pair
func (b *Bithumb) UpdateTicker(p currency.Pair, assetType asset.Item) (*ticker.Price, error) {
	tickers, err := b.GetAllTickers()
	if err != nil {
		return nil, err
	}
	pairs, err := b.GetEnabledPairs(assetType)
	if err != nil {
		return nil, err
	}

	for i := range pairs {
		curr := pairs[i].Base.String()
		t, ok := tickers[curr]
		if !ok {
			return nil,
				fmt.Errorf("enabled pair %s [%s] not found in returned ticker map %v",
					pairs[i], pairs, tickers)
		}

		err = ticker.ProcessTicker(&ticker.Price{
			High:         t.MaxPrice,
			Low:          t.MinPrice,
			Volume:       t.UnitsTraded24Hr,
			Open:         t.OpeningPrice,
			Close:        t.ClosingPrice,
			Pair:         pairs[i],
			ExchangeName: b.Name,
			AssetType:    assetType,
		})
		if err != nil {
			return nil, err
		}
	}
	return ticker.GetTicker(b.Name, p, assetType)
}

// FetchTicker returns the ticker for a currency pair
func (b *Bithumb) FetchTicker(p currency.Pair, assetType asset.Item) (*ticker.Price, error) {
	tickerNew, err := ticker.GetTicker(b.Name, p, assetType)
	if err != nil {
		return b.UpdateTicker(p, assetType)
	}
	return tickerNew, nil
}

// FetchOrderbook returns orderbook base on the currency pair
func (b *Bithumb) FetchOrderbook(p currency.Pair, assetType asset.Item) (*orderbook.Base, error) {
	ob, err := orderbook.Get(b.Name, p, assetType)
	if err != nil {
		return b.UpdateOrderbook(p, assetType)
	}
	return ob, nil
}

// UpdateOrderbook updates and returns the orderbook for a currency pair
func (b *Bithumb) UpdateOrderbook(p currency.Pair, assetType asset.Item) (*orderbook.Base, error) {
	orderBook := new(orderbook.Base)
	curr := p.Base.String()

	orderbookNew, err := b.GetOrderBook(curr)
	if err != nil {
		return orderBook, err
	}

	for i := range orderbookNew.Data.Bids {
		orderBook.Bids = append(orderBook.Bids,
			orderbook.Item{
				Amount: orderbookNew.Data.Bids[i].Quantity,
				Price:  orderbookNew.Data.Bids[i].Price,
			})
	}

	for i := range orderbookNew.Data.Asks {
		orderBook.Asks = append(orderBook.Asks,
			orderbook.Item{
				Amount: orderbookNew.Data.Asks[i].Quantity,
				Price:  orderbookNew.Data.Asks[i].Price,
			})
	}

	orderBook.Pair = p
	orderBook.ExchangeName = b.Name
	orderBook.AssetType = assetType

	err = orderBook.Process()
	if err != nil {
		return orderBook, err
	}

	return orderbook.Get(b.Name, p, assetType)
}

// UpdateAccountInfo retrieves balances for all enabled currencies for the
// Bithumb exchange
func (b *Bithumb) UpdateAccountInfo() (account.Holdings, error) {
	var info account.Holdings
	bal, err := b.GetAccountBalance("ALL")
	if err != nil {
		return info, err
	}

	var exchangeBalances []account.Balance
	for key, totalAmount := range bal.Total {
		hold, ok := bal.InUse[key]
		if !ok {
			return info, fmt.Errorf("getAccountInfo error - in use item not found for currency %s",
				key)
		}

		exchangeBalances = append(exchangeBalances, account.Balance{
			CurrencyName: currency.NewCode(key),
			TotalValue:   totalAmount,
			Hold:         hold,
		})
	}

	info.Accounts = append(info.Accounts, account.SubAccount{
		Currencies: exchangeBalances,
	})

	info.Exchange = b.Name
	err = account.Process(&info)
	if err != nil {
		return account.Holdings{}, err
	}

	return info, nil
}

// FetchAccountInfo retrieves balances for all enabled currencies
func (b *Bithumb) FetchAccountInfo() (account.Holdings, error) {
	acc, err := account.GetHoldings(b.Name)
	if err != nil {
		return b.UpdateAccountInfo()
	}

	return acc, nil
}

// GetFundingHistory returns funding history, deposits and
// withdrawals
func (b *Bithumb) GetFundingHistory() ([]exchange.FundHistory, error) {
	return nil, common.ErrFunctionNotSupported
}

// GetExchangeHistory returns historic trade data within the timeframe provided.
func (b *Bithumb) GetExchangeHistory(p currency.Pair, assetType asset.Item, timestampStart, timestampEnd time.Time) ([]exchange.TradeHistory, error) {
	return nil, common.ErrNotYetImplemented
}

// SubmitOrder submits a new order
// TODO: Fill this out to support limit orders
func (b *Bithumb) SubmitOrder(s *order.Submit) (order.SubmitResponse, error) {
	var submitOrderResponse order.SubmitResponse
	if err := s.Validate(); err != nil {
		return submitOrderResponse, err
	}

	var orderID string
	var err error
	if s.Side == order.Buy {
		var result MarketBuy
		result, err = b.MarketBuyOrder(s.Pair.Base.String(), s.Amount)
		if err != nil {
			return submitOrderResponse, err
		}
		orderID = result.OrderID
	} else if s.Side == order.Sell {
		var result MarketSell
		result, err = b.MarketSellOrder(s.Pair.Base.String(), s.Amount)
		if err != nil {
			return submitOrderResponse, err
		}
		orderID = result.OrderID
	}
	if orderID != "" {
		submitOrderResponse.OrderID = orderID
		submitOrderResponse.FullyMatched = true
	}
	submitOrderResponse.IsOrderPlaced = true

	return submitOrderResponse, nil
}

// ModifyOrder will allow of changing orderbook placement and limit to
// market conversion
func (b *Bithumb) ModifyOrder(action *order.Modify) (string, error) {
	order, err := b.ModifyTrade(action.ID,
		action.Pair.Base.String(),
		action.Side.Lower(),
		action.Amount,
		int64(action.Price))

	if err != nil {
		return "", err
	}

	return order.Data[0].ContID, nil
}

// CancelOrder cancels an order by its corresponding ID number
func (b *Bithumb) CancelOrder(order *order.Cancel) error {
	_, err := b.CancelTrade(order.Side.String(),
		order.ID,
		order.Pair.Base.String())
	return err
}

// CancelAllOrders cancels all orders associated with a currency pair
func (b *Bithumb) CancelAllOrders(orderCancellation *order.Cancel) (order.CancelAllResponse, error) {
	cancelAllOrdersResponse := order.CancelAllResponse{
		Status: make(map[string]string),
	}

	var allOrders []OrderData
	currs, err := b.GetEnabledPairs(asset.Spot)
	if err != nil {
		return cancelAllOrdersResponse, err
	}

	for i := range currs {
		orders, err := b.GetOrders("",
			orderCancellation.Side.String(),
			"100",
			"",
			currs[i].Base.String())
		if err != nil {
			return cancelAllOrdersResponse, err
		}
		allOrders = append(allOrders, orders.Data...)
	}

	for i := range allOrders {
		_, err := b.CancelTrade(orderCancellation.Side.String(),
			allOrders[i].OrderID,
			orderCancellation.Pair.Base.String())
		if err != nil {
			cancelAllOrdersResponse.Status[allOrders[i].OrderID] = err.Error()
		}
	}

	return cancelAllOrdersResponse, nil
}

// GetOrderInfo returns information on a current open order
func (b *Bithumb) GetOrderInfo(orderID string) (order.Detail, error) {
	var orderDetail order.Detail
	return orderDetail, common.ErrNotYetImplemented
}

// GetDepositAddress returns a deposit address for a specified currency
func (b *Bithumb) GetDepositAddress(cryptocurrency currency.Code, _ string) (string, error) {
	addr, err := b.GetWalletAddress(cryptocurrency.String())
	if err != nil {
		return "", err
	}

	return addr.Data.WalletAddress, nil
}

// WithdrawCryptocurrencyFunds returns a withdrawal ID when a withdrawal is
// submitted
func (b *Bithumb) WithdrawCryptocurrencyFunds(withdrawRequest *withdraw.Request) (*withdraw.ExchangeResponse, error) {
	v, err := b.WithdrawCrypto(withdrawRequest.Crypto.Address,
		withdrawRequest.Crypto.AddressTag,
		withdrawRequest.Currency.String(),
		withdrawRequest.Amount)
	if err != nil {
		return nil, err
	}
	return &withdraw.ExchangeResponse{
		ID:     v.Message,
		Status: v.Status,
	}, err
}

// WithdrawFiatFunds returns a withdrawal ID when a
// withdrawal is submitted
func (b *Bithumb) WithdrawFiatFunds(withdrawRequest *withdraw.Request) (*withdraw.ExchangeResponse, error) {
	if math.Mod(withdrawRequest.Amount, 1) != 0 {
		return nil, errors.New("currency KRW does not support decimal places")
	}
	if withdrawRequest.Currency != currency.KRW {
		return nil, errors.New("only KRW is supported")
	}
	bankDetails := strconv.FormatFloat(withdrawRequest.Fiat.Bank.BankCode, 'f', -1, 64) +
		"_" + withdrawRequest.Fiat.Bank.BankName
	resp, err := b.RequestKRWWithdraw(bankDetails, withdrawRequest.Fiat.Bank.AccountNumber, int64(withdrawRequest.Amount))
	if err != nil {
		return nil, err
	}
	if resp.Status != "0000" {
		return nil, errors.New(resp.Message)
	}

	return &withdraw.ExchangeResponse{
		Status: resp.Status,
	}, nil
}

// WithdrawFiatFundsToInternationalBank is not supported as Bithumb only withdraws KRW to South Korean banks
func (b *Bithumb) WithdrawFiatFundsToInternationalBank(withdrawRequest *withdraw.Request) (*withdraw.ExchangeResponse, error) {
	return nil, common.ErrFunctionNotSupported
}

// GetFeeByType returns an estimate of fee based on type of transaction
func (b *Bithumb) GetFeeByType(feeBuilder *exchange.FeeBuilder) (float64, error) {
	if !b.AllowAuthenticatedRequest() && // Todo check connection status
		feeBuilder.FeeType == exchange.CryptocurrencyTradeFee {
		feeBuilder.FeeType = exchange.OfflineTradeFee
	}
	return b.GetFee(feeBuilder)
}

// GetActiveOrders retrieves any orders that are active/open
func (b *Bithumb) GetActiveOrders(req *order.GetOrdersRequest) ([]order.Detail, error) {
	var orders []order.Detail
	resp, err := b.GetOrders("", "", "1000", "", "")
	if err != nil {
		return nil, err
	}

	format, err := b.GetPairFormat(asset.Spot, false)
	if err != nil {
		return nil, err
	}

	for i := range resp.Data {
		if resp.Data[i].Status != "placed" {
			continue
		}

		orderDate := time.Unix(resp.Data[i].OrderDate, 0)
		orderDetail := order.Detail{
			Amount:          resp.Data[i].Units,
			Exchange:        b.Name,
			ID:              resp.Data[i].OrderID,
			Date:            orderDate,
			Price:           resp.Data[i].Price,
			RemainingAmount: resp.Data[i].UnitsRemaining,
			Status:          order.Active,
			Pair: currency.NewPairWithDelimiter(resp.Data[i].OrderCurrency,
				resp.Data[i].PaymentCurrency,
				format.Delimiter),
		}

		if resp.Data[i].Type == "bid" {
			orderDetail.Side = order.Buy
		} else if resp.Data[i].Type == "ask" {
			orderDetail.Side = order.Sell
		}

		orders = append(orders, orderDetail)
	}

	order.FilterOrdersBySide(&orders, req.Side)
	order.FilterOrdersByTickRange(&orders, req.StartTicks, req.EndTicks)
	order.FilterOrdersByCurrencies(&orders, req.Pairs)
	return orders, nil
}

// GetOrderHistory retrieves account order information
// Can Limit response to specific order status
func (b *Bithumb) GetOrderHistory(req *order.GetOrdersRequest) ([]order.Detail, error) {
	var orders []order.Detail
	resp, err := b.GetOrders("", "", "1000", "", "")
	if err != nil {
		return nil, err
	}

	format, err := b.GetPairFormat(asset.Spot, false)
	if err != nil {
		return nil, err
	}

	for i := range resp.Data {
		if resp.Data[i].Status == "placed" {
			continue
		}

		orderDate := time.Unix(resp.Data[i].OrderDate, 0)
		orderDetail := order.Detail{
			Amount:          resp.Data[i].Units,
			Exchange:        b.Name,
			ID:              resp.Data[i].OrderID,
			Date:            orderDate,
			Price:           resp.Data[i].Price,
			RemainingAmount: resp.Data[i].UnitsRemaining,
			Pair: currency.NewPairWithDelimiter(resp.Data[i].OrderCurrency,
				resp.Data[i].PaymentCurrency,
				format.Delimiter),
		}

		if resp.Data[i].Type == "bid" {
			orderDetail.Side = order.Buy
		} else if resp.Data[i].Type == "ask" {
			orderDetail.Side = order.Sell
		}

		orders = append(orders, orderDetail)
	}

	order.FilterOrdersBySide(&orders, req.Side)
	order.FilterOrdersByTickRange(&orders, req.StartTicks, req.EndTicks)
	order.FilterOrdersByCurrencies(&orders, req.Pairs)
	return orders, nil
}

// ValidateCredentials validates current credentials used for wrapper
// functionality
func (b *Bithumb) ValidateCredentials() error {
	_, err := b.UpdateAccountInfo()
	return b.CheckTransientError(err)
}

// FormatExchangeKlineInterval returns Interval to exchange formatted string
func (b *Bithumb) FormatExchangeKlineInterval(in kline.Interval) string {
	return in.Short()
}

// GetHistoricCandles returns candles between a time period for a set time interval
func (b *Bithumb) GetHistoricCandles(pair currency.Pair, a asset.Item, start, end time.Time, interval kline.Interval) (kline.Item, error) {
	if !b.KlineIntervalEnabled(interval) {
		return kline.Item{}, kline.ErrorKline{
			Interval: interval,
		}
	}

	formattedPair, err := b.FormatExchangeCurrency(pair, a)
	if err != nil {
		return kline.Item{}, err
	}

	candle, err := b.GetCandleStick(formattedPair.String(),
		b.FormatExchangeKlineInterval(interval))
	if err != nil {
		return kline.Item{}, err
	}

	ret := kline.Item{
		Exchange: b.Name,
		Pair:     pair,
		Interval: interval,
	}

	for x := range candle.Data {
		var tempCandle kline.Candle

		tempTime := candle.Data[x][0].(float64)
		timestamp := time.Unix(0, int64(tempTime)*int64(time.Millisecond))
		if timestamp.Before(start) {
			continue
		}
		if timestamp.After(end) {
			break
		}
		tempCandle.Time = timestamp

		open, ok := candle.Data[x][1].(string)
		if !ok {
			return kline.Item{}, errors.New("open conversion failed")
		}
		tempCandle.Open, err = strconv.ParseFloat(open, 64)
		if err != nil {
			return kline.Item{}, err
		}
		high, ok := candle.Data[x][2].(string)
		if !ok {
			return kline.Item{}, errors.New("high conversion failed")
		}
		tempCandle.High, err = strconv.ParseFloat(high, 64)
		if err != nil {
			return kline.Item{}, err
		}

		low, ok := candle.Data[x][3].(string)
		if !ok {
			return kline.Item{}, errors.New("low conversion failed")
		}
		tempCandle.Low, err = strconv.ParseFloat(low, 64)
		if err != nil {
			return kline.Item{}, err
		}

		closeTemp, ok := candle.Data[x][4].(string)
		if !ok {
			return kline.Item{}, errors.New("close conversion failed")
		}
		tempCandle.Close, err = strconv.ParseFloat(closeTemp, 64)
		if err != nil {
			return kline.Item{}, err
		}

		vol, ok := candle.Data[x][5].(string)
		if !ok {
			return kline.Item{}, errors.New("vol conversion failed")
		}
		tempCandle.Volume, err = strconv.ParseFloat(vol, 64)
		if err != nil {
			return kline.Item{}, err
		}
		ret.Candles = append(ret.Candles, tempCandle)
	}

	ret.SortCandlesByTimestamp(false)
	return ret, nil
}

// GetHistoricCandlesExtended returns candles between a time period for a set time interval
func (b *Bithumb) GetHistoricCandlesExtended(pair currency.Pair, a asset.Item, start, end time.Time, interval kline.Interval) (kline.Item, error) {
	return b.GetHistoricCandles(pair, a, start, end, interval)
}

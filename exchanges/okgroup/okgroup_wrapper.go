package okgroup

import (
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/yurulab/gocryptotrader/common"
	"github.com/yurulab/gocryptotrader/config"
	"github.com/yurulab/gocryptotrader/currency"
	exchange "github.com/yurulab/gocryptotrader/exchanges"
	"github.com/yurulab/gocryptotrader/exchanges/account"
	"github.com/yurulab/gocryptotrader/exchanges/asset"
	"github.com/yurulab/gocryptotrader/exchanges/order"
	"github.com/yurulab/gocryptotrader/exchanges/orderbook"
	"github.com/yurulab/gocryptotrader/exchanges/stream"
	"github.com/yurulab/gocryptotrader/portfolio/withdraw"
)

// Note: GoCryptoTrader wrapper funcs currently only support SPOT trades.
// Therefore this OKGroup_Wrapper can be shared between OKEX and OKCoin.
// When circumstances change, wrapper funcs can be split appropriately

// Setup sets user exchange configuration settings
func (o *OKGroup) Setup(exch *config.ExchangeConfig) error {
	if !exch.Enabled {
		o.SetEnabled(false)
		return nil
	}

	err := o.SetupDefaults(exch)
	if err != nil {
		return err
	}

	err = o.Websocket.Setup(&stream.WebsocketSetup{
		Enabled:                          exch.Features.Enabled.Websocket,
		Verbose:                          exch.Verbose,
		AuthenticatedWebsocketAPISupport: exch.API.AuthenticatedWebsocketSupport,
		WebsocketTimeout:                 exch.WebsocketTrafficTimeout,
		DefaultURL:                       o.API.Endpoints.WebsocketURL,
		ExchangeName:                     exch.Name,
		RunningURL:                       exch.API.Endpoints.WebsocketURL,
		Connector:                        o.WsConnect,
		Subscriber:                       o.Subscribe,
		UnSubscriber:                     o.Unsubscribe,
		GenerateSubscriptions:            o.GenerateDefaultSubscriptions,
		Features:                         &o.Features.Supports.WebsocketCapabilities,
		OrderbookBufferLimit:             exch.WebsocketOrderbookBufferLimit,
	})
	if err != nil {
		return err
	}

	return o.Websocket.SetupNewConnection(stream.ConnectionSetup{
		RateLimit:            okGroupWsRateLimit,
		ResponseCheckTimeout: exch.WebsocketResponseCheckTimeout,
		ResponseMaxLimit:     exch.WebsocketResponseMaxLimit,
	})
}

// FetchOrderbook returns orderbook base on the currency pair
func (o *OKGroup) FetchOrderbook(p currency.Pair, assetType asset.Item) (*orderbook.Base, error) {
	ob, err := orderbook.Get(o.Name, p, assetType)
	if err != nil {
		return o.UpdateOrderbook(p, assetType)
	}
	return ob, nil
}

// UpdateOrderbook updates and returns the orderbook for a currency pair
func (o *OKGroup) UpdateOrderbook(p currency.Pair, a asset.Item) (*orderbook.Base, error) {
	orderBook := new(orderbook.Base)
	if a == asset.Index {
		return orderBook, errors.New("no orderbooks for index")
	}

	fpair, err := o.FormatExchangeCurrency(p, a)
	if err != nil {
		return nil, err
	}

	orderbookNew, err := o.GetOrderBook(GetOrderBookRequest{
		InstrumentID: fpair.String(),
	}, a)
	if err != nil {
		return orderBook, err
	}

	for x := range orderbookNew.Bids {
		amount, convErr := strconv.ParseFloat(orderbookNew.Bids[x][1], 64)
		if convErr != nil {
			return orderBook, err
		}
		price, convErr := strconv.ParseFloat(orderbookNew.Bids[x][0], 64)
		if convErr != nil {
			return orderBook, err
		}

		var liquidationOrders, orderCount int64
		// Contract specific variables
		if len(orderbookNew.Bids[x]) == 4 {
			liquidationOrders, convErr = strconv.ParseInt(orderbookNew.Bids[x][2], 10, 64)
			if convErr != nil {
				return orderBook, err
			}

			orderCount, convErr = strconv.ParseInt(orderbookNew.Bids[x][3], 10, 64)
			if convErr != nil {
				return orderBook, err
			}
		}

		orderBook.Bids = append(orderBook.Bids, orderbook.Item{
			Amount:            amount,
			Price:             price,
			LiquidationOrders: liquidationOrders,
			OrderCount:        orderCount,
		})
	}

	for x := range orderbookNew.Asks {
		amount, convErr := strconv.ParseFloat(orderbookNew.Asks[x][1], 64)
		if convErr != nil {
			return orderBook, err
		}
		price, convErr := strconv.ParseFloat(orderbookNew.Asks[x][0], 64)
		if convErr != nil {
			return orderBook, err
		}

		var liquidationOrders, orderCount int64
		// Contract specific variables
		if len(orderbookNew.Asks[x]) == 4 {
			liquidationOrders, convErr = strconv.ParseInt(orderbookNew.Asks[x][2], 10, 64)
			if convErr != nil {
				return orderBook, err
			}

			orderCount, convErr = strconv.ParseInt(orderbookNew.Asks[x][3], 10, 64)
			if convErr != nil {
				return orderBook, err
			}
		}

		orderBook.Asks = append(orderBook.Asks, orderbook.Item{
			Amount:            amount,
			Price:             price,
			LiquidationOrders: liquidationOrders,
			OrderCount:        orderCount,
		})
	}

	orderBook.Pair = p
	orderBook.AssetType = a
	orderBook.ExchangeName = o.Name

	err = orderBook.Process()
	if err != nil {
		return orderBook, err
	}

	return orderbook.Get(o.Name, p, a)
}

// UpdateAccountInfo retrieves balances for all enabled currencies
func (o *OKGroup) UpdateAccountInfo() (account.Holdings, error) {
	currencies, err := o.GetSpotTradingAccounts()
	if err != nil {
		return account.Holdings{}, err
	}

	var resp account.Holdings
	resp.Exchange = o.Name
	currencyAccount := account.SubAccount{}

	for i := range currencies {
		hold, parseErr := strconv.ParseFloat(currencies[i].Hold, 64)
		if parseErr != nil {
			return resp, parseErr
		}
		totalValue, parseErr := strconv.ParseFloat(currencies[i].Balance, 64)
		if parseErr != nil {
			return resp, parseErr
		}
		currencyAccount.Currencies = append(currencyAccount.Currencies,
			account.Balance{
				CurrencyName: currency.NewCode(currencies[i].Currency),
				Hold:         hold,
				TotalValue:   totalValue,
			})
	}

	resp.Accounts = append(resp.Accounts, currencyAccount)

	err = account.Process(&resp)
	if err != nil {
		return resp, err
	}

	return resp, nil
}

// FetchAccountInfo retrieves balances for all enabled currencies
func (o *OKGroup) FetchAccountInfo() (account.Holdings, error) {
	acc, err := account.GetHoldings(o.Name)
	if err != nil {
		return o.UpdateAccountInfo()
	}

	return acc, nil
}

// GetFundingHistory returns funding history, deposits and
// withdrawals
func (o *OKGroup) GetFundingHistory() (resp []exchange.FundHistory, err error) {
	accountDepositHistory, err := o.GetAccountDepositHistory("")
	if err != nil {
		return
	}
	for x := range accountDepositHistory {
		orderStatus := ""
		switch accountDepositHistory[x].Status {
		case 0:
			orderStatus = "waiting"
		case 1:
			orderStatus = "confirmation account"
		case 2:
			orderStatus = "recharge success"
		}

		resp = append(resp, exchange.FundHistory{
			Amount:       accountDepositHistory[x].Amount,
			Currency:     accountDepositHistory[x].Currency,
			ExchangeName: o.Name,
			Status:       orderStatus,
			Timestamp:    accountDepositHistory[x].Timestamp,
			TransferID:   accountDepositHistory[x].TransactionID,
			TransferType: "deposit",
		})
	}
	accountWithdrawlHistory, err := o.GetAccountWithdrawalHistory("")
	for i := range accountWithdrawlHistory {
		resp = append(resp, exchange.FundHistory{
			Amount:       accountWithdrawlHistory[i].Amount,
			Currency:     accountWithdrawlHistory[i].Currency,
			ExchangeName: o.Name,
			Status:       OrderStatus[accountWithdrawlHistory[i].Status],
			Timestamp:    accountWithdrawlHistory[i].Timestamp,
			TransferID:   accountWithdrawlHistory[i].TransactionID,
			TransferType: "withdrawal",
		})
	}
	return resp, err
}

// GetExchangeHistory returns historic trade data within the timeframe provided.
func (o *OKGroup) GetExchangeHistory(p currency.Pair, assetType asset.Item, timestampStart, timestampEnd time.Time) ([]exchange.TradeHistory, error) {
	return nil, common.ErrNotYetImplemented
}

// SubmitOrder submits a new order
func (o *OKGroup) SubmitOrder(s *order.Submit) (order.SubmitResponse, error) {
	err := s.Validate()
	if err != nil {
		return order.SubmitResponse{}, err
	}

	fpair, err := o.FormatExchangeCurrency(s.Pair, s.AssetType)
	if err != nil {
		return order.SubmitResponse{}, err
	}

	request := PlaceOrderRequest{
		ClientOID:    s.ClientID,
		InstrumentID: fpair.String(),
		Side:         s.Side.Lower(),
		Type:         s.Type.Lower(),
		Size:         strconv.FormatFloat(s.Amount, 'f', -1, 64),
	}
	if s.Type == order.Limit {
		request.Price = strconv.FormatFloat(s.Price, 'f', -1, 64)
	}

	orderResponse, err := o.PlaceSpotOrder(&request)
	if err != nil {
		return order.SubmitResponse{}, err
	}

	var resp order.SubmitResponse
	resp.IsOrderPlaced = orderResponse.Result
	resp.OrderID = orderResponse.OrderID
	if s.Type == order.Market {
		resp.FullyMatched = true
	}

	return resp, nil
}

// ModifyOrder will allow of changing orderbook placement and limit to
// market conversion
func (o *OKGroup) ModifyOrder(action *order.Modify) (string, error) {
	return "", common.ErrFunctionNotSupported
}

// CancelOrder cancels an order by its corresponding ID number
func (o *OKGroup) CancelOrder(orderCancellation *order.Cancel) (err error) {
	orderID, err := strconv.ParseInt(orderCancellation.ID, 10, 64)
	if err != nil {
		return
	}

	fpair, err := o.FormatExchangeCurrency(orderCancellation.Pair,
		orderCancellation.AssetType)
	if err != nil {
		return
	}

	orderCancellationResponse, err := o.CancelSpotOrder(CancelSpotOrderRequest{
		InstrumentID: fpair.String(),
		OrderID:      orderID,
	})

	if !orderCancellationResponse.Result {
		err = fmt.Errorf("order %d failed to be cancelled",
			orderCancellationResponse.OrderID)
	}

	return
}

// CancelAllOrders cancels all orders associated with a currency pair
func (o *OKGroup) CancelAllOrders(orderCancellation *order.Cancel) (order.CancelAllResponse, error) {
	orderIDs := strings.Split(orderCancellation.ID, ",")
	resp := order.CancelAllResponse{}
	resp.Status = make(map[string]string)
	var orderIDNumbers []int64
	for i := range orderIDs {
		orderIDNumber, err := strconv.ParseInt(orderIDs[i], 10, 64)
		if err != nil {
			resp.Status[orderIDs[i]] = err.Error()
			continue
		}
		orderIDNumbers = append(orderIDNumbers, orderIDNumber)
	}

	fpair, err := o.FormatExchangeCurrency(orderCancellation.Pair,
		orderCancellation.AssetType)
	if err != nil {
		return resp, err
	}

	cancelOrdersResponse, err := o.CancelMultipleSpotOrders(CancelMultipleSpotOrdersRequest{
		InstrumentID: fpair.String(),
		OrderIDs:     orderIDNumbers,
	})
	if err != nil {
		return resp, err
	}

	for x := range cancelOrdersResponse {
		for y := range cancelOrdersResponse[x] {
			resp.Status[strconv.FormatInt(cancelOrdersResponse[x][y].OrderID, 10)] = strconv.FormatBool(cancelOrdersResponse[x][y].Result)
		}
	}

	return resp, err
}

// GetOrderInfo returns information on a current open order
func (o *OKGroup) GetOrderInfo(orderID string) (resp order.Detail, err error) {
	mOrder, err := o.GetSpotOrder(GetSpotOrderRequest{OrderID: orderID})
	if err != nil {
		return
	}

	format, err := o.GetPairFormat(asset.Spot, false)
	if err != nil {
		return resp, err
	}

	p, err := currency.NewPairDelimiter(mOrder.InstrumentID, format.Delimiter)
	if err != nil {
		return resp, err
	}

	resp = order.Detail{
		Amount:         mOrder.Size,
		Pair:           p,
		Exchange:       o.Name,
		Date:           mOrder.Timestamp,
		ExecutedAmount: mOrder.FilledSize,
		Status:         order.Status(mOrder.Status),
		Side:           order.Side(mOrder.Side),
	}
	return
}

// GetDepositAddress returns a deposit address for a specified currency
func (o *OKGroup) GetDepositAddress(p currency.Code, accountID string) (string, error) {
	wallet, err := o.GetAccountDepositAddressForCurrency(p.Lower().String())
	if err != nil || len(wallet) == 0 {
		return "", err
	}
	return wallet[0].Address, nil
}

// WithdrawCryptocurrencyFunds returns a withdrawal ID when a withdrawal is
// submitted
func (o *OKGroup) WithdrawCryptocurrencyFunds(withdrawRequest *withdraw.Request) (*withdraw.ExchangeResponse, error) {
	withdrawal, err := o.AccountWithdraw(AccountWithdrawRequest{
		Amount:      withdrawRequest.Amount,
		Currency:    withdrawRequest.Currency.Lower().String(),
		Destination: 4, // 1, 2, 3 are all internal
		Fee:         withdrawRequest.Crypto.FeeAmount,
		ToAddress:   withdrawRequest.Crypto.Address,
		TradePwd:    withdrawRequest.TradePassword,
	})
	if err != nil {
		return nil, err
	}
	if !withdrawal.Result {
		return nil,
			fmt.Errorf("could not withdraw currency %s to %s, no error specified",
				withdrawRequest.Currency,
				withdrawRequest.Crypto.Address)
	}

	return &withdraw.ExchangeResponse{
		ID: strconv.FormatInt(withdrawal.WithdrawalID, 10),
	}, nil
}

// WithdrawFiatFunds returns a withdrawal ID when a
// withdrawal is submitted
func (o *OKGroup) WithdrawFiatFunds(withdrawRequest *withdraw.Request) (*withdraw.ExchangeResponse, error) {
	return nil, common.ErrFunctionNotSupported
}

// WithdrawFiatFundsToInternationalBank returns a withdrawal ID when a
// withdrawal is submitted
func (o *OKGroup) WithdrawFiatFundsToInternationalBank(withdrawRequest *withdraw.Request) (*withdraw.ExchangeResponse, error) {
	return nil, common.ErrFunctionNotSupported
}

// GetActiveOrders retrieves any orders that are active/open
func (o *OKGroup) GetActiveOrders(req *order.GetOrdersRequest) (resp []order.Detail, err error) {
	for x := range req.Pairs {
		fpair, err := o.FormatExchangeCurrency(req.Pairs[x], asset.Spot)
		if err != nil {
			return nil, err
		}
		spotOpenOrders, err := o.GetSpotOpenOrders(GetSpotOpenOrdersRequest{
			InstrumentID: fpair.String(),
		})
		if err != nil {
			return resp, err
		}
		for i := range spotOpenOrders {
			resp = append(resp, order.Detail{
				ID:             spotOpenOrders[i].OrderID,
				Price:          spotOpenOrders[i].Price,
				Amount:         spotOpenOrders[i].Size,
				Pair:           req.Pairs[x],
				Exchange:       o.Name,
				Side:           order.Side(spotOpenOrders[i].Side),
				Type:           order.Type(spotOpenOrders[i].Type),
				ExecutedAmount: spotOpenOrders[i].FilledSize,
				Date:           spotOpenOrders[i].Timestamp,
				Status:         order.Status(spotOpenOrders[i].Status),
			})
		}
	}

	return
}

// GetOrderHistory retrieves account order information
// Can Limit response to specific order status
func (o *OKGroup) GetOrderHistory(req *order.GetOrdersRequest) (resp []order.Detail, err error) {
	for x := range req.Pairs {
		fpair, err := o.FormatExchangeCurrency(req.Pairs[x], asset.Spot)
		if err != nil {
			return nil, err
		}
		spotOpenOrders, err := o.GetSpotOrders(GetSpotOrdersRequest{
			Status:       strings.Join([]string{"filled", "cancelled", "failure"}, "|"),
			InstrumentID: fpair.String(),
		})
		if err != nil {
			return resp, err
		}
		for i := range spotOpenOrders {
			resp = append(resp, order.Detail{
				ID:             spotOpenOrders[i].OrderID,
				Price:          spotOpenOrders[i].Price,
				Amount:         spotOpenOrders[i].Size,
				Pair:           req.Pairs[x],
				Exchange:       o.Name,
				Side:           order.Side(spotOpenOrders[i].Side),
				Type:           order.Type(spotOpenOrders[i].Type),
				ExecutedAmount: spotOpenOrders[i].FilledSize,
				Date:           spotOpenOrders[i].Timestamp,
				Status:         order.Status(spotOpenOrders[i].Status),
			})
		}
	}

	return
}

// GetFeeByType returns an estimate of fee based on type of transaction
func (o *OKGroup) GetFeeByType(feeBuilder *exchange.FeeBuilder) (float64, error) {
	if !o.AllowAuthenticatedRequest() && // Todo check connection status
		feeBuilder.FeeType == exchange.CryptocurrencyTradeFee {
		feeBuilder.FeeType = exchange.OfflineTradeFee
	}
	return o.GetFee(feeBuilder)
}

// GetWithdrawCapabilities returns the types of withdrawal methods permitted by the exchange
func (o *OKGroup) GetWithdrawCapabilities() uint32 {
	return o.GetWithdrawPermissions()
}

// AuthenticateWebsocket sends an authentication message to the websocket
func (o *OKGroup) AuthenticateWebsocket() error {
	return o.WsLogin()
}

// ValidateCredentials validates current credentials used for wrapper
// functionality
func (o *OKGroup) ValidateCredentials() error {
	_, err := o.UpdateAccountInfo()
	return o.CheckTransientError(err)
}

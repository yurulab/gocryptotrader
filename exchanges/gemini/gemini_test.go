package gemini

import (
	"net/url"
	"testing"
	"time"

	"github.com/gorilla/websocket"
	"github.com/yurulab/gocryptotrader/common"
	"github.com/yurulab/gocryptotrader/core"
	"github.com/yurulab/gocryptotrader/currency"
	exchange "github.com/yurulab/gocryptotrader/exchanges"
	"github.com/yurulab/gocryptotrader/exchanges/order"
	"github.com/yurulab/gocryptotrader/exchanges/sharedtestvalues"
	"github.com/yurulab/gocryptotrader/exchanges/stream"
	"github.com/yurulab/gocryptotrader/portfolio/withdraw"
)

// Please enter sandbox API keys & assigned roles for better testing procedures
const (
	apiKey                  = ""
	apiSecret               = ""
	apiKeyRole              = ""
	sessionHeartBeat        = false
	canManipulateRealOrders = false
)

const testCurrency = "btcusd"

var g Gemini

func TestGetSymbols(t *testing.T) {
	t.Parallel()
	_, err := g.GetSymbols()
	if err != nil {
		t.Error("GetSymbols() error", err)
	}
}

func TestGetTicker(t *testing.T) {
	t.Parallel()
	_, err := g.GetTicker("BTCUSD")
	if err != nil {
		t.Error("GetTicker() error", err)
	}
	_, err = g.GetTicker("bla")
	if err == nil {
		t.Error("GetTicker() Expected error")
	}
}

func TestGetOrderbook(t *testing.T) {
	t.Parallel()
	_, err := g.GetOrderbook(testCurrency, url.Values{})
	if err != nil {
		t.Error("GetOrderbook() error", err)
	}
}

func TestGetTrades(t *testing.T) {
	t.Parallel()
	_, err := g.GetTrades(testCurrency, url.Values{})
	if err != nil {
		t.Error("GetTrades() error", err)
	}
}

func TestGetNotionalVolume(t *testing.T) {
	t.Parallel()
	_, err := g.GetNotionalVolume()
	if err != nil && mockTests {
		t.Error("GetNotionalVolume() error", err)
	} else if err == nil && !mockTests {
		t.Error("GetNotionalVolume() error cannot be nil")
	}
}

func TestGetAuction(t *testing.T) {
	t.Parallel()
	_, err := g.GetAuction(testCurrency)
	if err != nil {
		t.Error("GetAuction() error", err)
	}
}

func TestGetAuctionHistory(t *testing.T) {
	t.Parallel()
	_, err := g.GetAuctionHistory(testCurrency, url.Values{})
	if err != nil {
		t.Error("GetAuctionHistory() error", err)
	}
}

func TestNewOrder(t *testing.T) {
	t.Parallel()
	_, err := g.NewOrder(testCurrency,
		1,
		9000000,
		order.Sell.Lower(),
		"exchange limit")
	if err != nil && mockTests {
		t.Error("NewOrder() error", err)
	} else if err == nil && !mockTests {
		t.Error("NewOrder() error cannot be nil")
	}
}

func TestCancelExistingOrder(t *testing.T) {
	t.Parallel()
	_, err := g.CancelExistingOrder(265555413)
	if err != nil && mockTests {
		t.Error("CancelExistingOrder() error", err)
	} else if err == nil && !mockTests {
		t.Error("CancelExistingOrder() error cannot be nil")
	}
}

func TestCancelExistingOrders(t *testing.T) {
	t.Parallel()
	_, err := g.CancelExistingOrders(false)
	if err != nil && mockTests {
		t.Error("CancelExistingOrders() error", err)
	} else if err == nil && !mockTests {
		t.Error("CancelExistingOrders() error cannot be nil")
	}
}

func TestGetOrderStatus(t *testing.T) {
	t.Parallel()
	_, err := g.GetOrderStatus(265563260)
	if err != nil && mockTests {
		t.Error("GetOrderStatus() error", err)
	} else if err == nil && !mockTests {
		t.Error("GetOrderStatus() error cannot be nil")
	}
}

func TestGetOrders(t *testing.T) {
	t.Parallel()
	_, err := g.GetOrders()
	if err != nil && mockTests {
		t.Error("GetOrders() error", err)
	} else if err == nil && !mockTests {
		t.Error("GetOrders() error cannot be nil")
	}
}

func TestGetTradeHistory(t *testing.T) {
	t.Parallel()
	_, err := g.GetTradeHistory(testCurrency, 0)
	if err != nil && mockTests {
		t.Error("GetTradeHistory() error", err)
	} else if err == nil && !mockTests {
		t.Error("GetTradeHistory() error cannot be nil")
	}
}

func TestGetTradeVolume(t *testing.T) {
	t.Parallel()
	_, err := g.GetTradeVolume()
	if err != nil && mockTests {
		t.Error("GetTradeVolume() error", err)
	} else if err == nil && !mockTests {
		t.Error("GetTradeVolume() error cannot be nil")
	}
}

func TestGetBalances(t *testing.T) {
	t.Parallel()
	_, err := g.GetBalances()
	if err != nil && mockTests {
		t.Error("GetBalances() error", err)
	} else if err == nil && !mockTests {
		t.Error("GetBalances() error cannot be nil")
	}
}

func TestGetCryptoDepositAddress(t *testing.T) {
	t.Parallel()
	_, err := g.GetCryptoDepositAddress("LOL123", "btc")
	if err == nil {
		t.Error("GetCryptoDepositAddress() Expected error")
	}
}

func TestWithdrawCrypto(t *testing.T) {
	t.Parallel()
	_, err := g.WithdrawCrypto("LOL123", "btc", 1)
	if err == nil {
		t.Error("WithdrawCrypto() Expected error")
	}
}

func TestPostHeartbeat(t *testing.T) {
	t.Parallel()
	_, err := g.PostHeartbeat()
	if err != nil && mockTests {
		t.Error("PostHeartbeat() error", err)
	} else if err == nil && !mockTests {
		t.Error("PostHeartbeat() error cannot be nil")
	}
}

func setFeeBuilder() *exchange.FeeBuilder {
	return &exchange.FeeBuilder{
		Amount:  1,
		FeeType: exchange.CryptocurrencyTradeFee,
		Pair: currency.NewPairWithDelimiter(currency.BTC.String(),
			currency.LTC.String(),
			"_"),
		PurchasePrice:       1,
		FiatCurrency:        currency.USD,
		BankTransactionType: exchange.WireTransfer,
	}
}

// TestGetFeeByTypeOfflineTradeFee logic test
func TestGetFeeByTypeOfflineTradeFee(t *testing.T) {
	t.Parallel()
	var feeBuilder = setFeeBuilder()
	g.GetFeeByType(feeBuilder)

	if !areTestAPIKeysSet() {
		if feeBuilder.FeeType != exchange.OfflineTradeFee {
			t.Errorf("Expected %v, received %v",
				exchange.OfflineTradeFee,
				feeBuilder.FeeType)
		}
	} else {
		if feeBuilder.FeeType != exchange.CryptocurrencyTradeFee {
			t.Errorf("Expected %v, received %v",
				exchange.CryptocurrencyTradeFee,
				feeBuilder.FeeType)
		}
	}
}

func TestGetFee(t *testing.T) {
	t.Parallel()
	var feeBuilder = setFeeBuilder()
	if areTestAPIKeysSet() || mockTests {
		// CryptocurrencyTradeFee Basic
		if resp, err := g.GetFee(feeBuilder); resp != float64(0.0035) || err != nil {
			t.Errorf("GetFee() error. Expected: %f, Received: %f",
				float64(0.0035),
				resp)
			t.Error(err)
		}

		// CryptocurrencyTradeFee High quantity
		feeBuilder = setFeeBuilder()
		feeBuilder.Amount = 1000
		feeBuilder.PurchasePrice = 1000
		if resp, err := g.GetFee(feeBuilder); resp != float64(3500) || err != nil {
			t.Errorf("GetFee() error. Expected: %f, Received: %f",
				float64(3500),
				resp)
			t.Error(err)
		}

		// CryptocurrencyTradeFee IsMaker
		feeBuilder = setFeeBuilder()
		feeBuilder.IsMaker = true
		if resp, err := g.GetFee(feeBuilder); resp != float64(0.001) || err != nil {
			t.Errorf("GetFee() error. Expected: %f, Received: %f",
				float64(0.001),
				resp)
			t.Error(err)
		}

		// CryptocurrencyTradeFee Negative purchase price
		feeBuilder = setFeeBuilder()
		feeBuilder.PurchasePrice = -1000
		if resp, err := g.GetFee(feeBuilder); resp != float64(0) || err != nil {
			t.Errorf("GetFee() error. Expected: %f, Received: %f",
				float64(0),
				resp)
			t.Error(err)
		}
	}
	// CryptocurrencyWithdrawalFee Basic
	feeBuilder = setFeeBuilder()
	feeBuilder.FeeType = exchange.CryptocurrencyWithdrawalFee
	if resp, err := g.GetFee(feeBuilder); resp != float64(0) || err != nil {
		t.Errorf("GetFee() error. Expected: %f, Received: %f",
			float64(0),
			resp)
		t.Error(err)
	}

	// CryptocurrencyWithdrawalFee Invalid currency
	feeBuilder = setFeeBuilder()
	feeBuilder.Pair.Base = currency.NewCode("hello")
	feeBuilder.FeeType = exchange.CryptocurrencyWithdrawalFee
	if resp, err := g.GetFee(feeBuilder); resp != float64(0) || err != nil {
		t.Errorf("GetFee() error. Expected: %f, Received: %f",
			float64(0),
			resp)
		t.Error(err)
	}

	// CyptocurrencyDepositFee Basic
	feeBuilder = setFeeBuilder()
	feeBuilder.FeeType = exchange.CyptocurrencyDepositFee
	if resp, err := g.GetFee(feeBuilder); resp != float64(0) || err != nil {
		t.Errorf("GetFee() error. Expected: %f, Received: %f",
			float64(0),
			resp)
		t.Error(err)
	}

	// InternationalBankDepositFee Basic
	feeBuilder = setFeeBuilder()
	feeBuilder.FeeType = exchange.InternationalBankDepositFee
	if resp, err := g.GetFee(feeBuilder); resp != float64(0) || err != nil {
		t.Errorf("GetFee() error. Expected: %f, Received: %f",
			float64(0),
			resp)
		t.Error(err)
	}

	// InternationalBankWithdrawalFee Basic
	feeBuilder = setFeeBuilder()
	feeBuilder.FeeType = exchange.InternationalBankWithdrawalFee
	feeBuilder.FiatCurrency = currency.USD
	if resp, err := g.GetFee(feeBuilder); resp != float64(0) || err != nil {
		t.Errorf("GetFee() error. Expected: %f, Received: %f",
			float64(0),
			resp)
		t.Error(err)
	}
}

func TestFormatWithdrawPermissions(t *testing.T) {
	t.Parallel()
	expectedResult := exchange.AutoWithdrawCryptoWithAPIPermissionText +
		" & " +
		exchange.AutoWithdrawCryptoWithSetupText +
		" & " +
		exchange.WithdrawFiatViaWebsiteOnlyText
	withdrawPermissions := g.FormatWithdrawPermissions()
	if withdrawPermissions != expectedResult {
		t.Errorf("Expected: %s, Received: %s",
			expectedResult,
			withdrawPermissions)
	}
}

func TestGetActiveOrders(t *testing.T) {
	t.Parallel()
	var getOrdersRequest = order.GetOrdersRequest{
		Type: order.AnyType,
		Pairs: []currency.Pair{
			currency.NewPair(currency.LTC, currency.BTC),
		},
	}

	_, err := g.GetActiveOrders(&getOrdersRequest)
	switch {
	case areTestAPIKeysSet() && err != nil && !mockTests:
		t.Errorf("Could not get open orders: %s", err)
	case !areTestAPIKeysSet() && err == nil && !mockTests:
		t.Error("Expecting an error when no keys are set")
	case mockTests && err != nil:
		t.Errorf("Could not get open orders: %s", err)
	}
}

func TestGetOrderHistory(t *testing.T) {
	t.Parallel()
	var getOrdersRequest = order.GetOrdersRequest{
		Type:  order.AnyType,
		Pairs: []currency.Pair{currency.NewPair(currency.LTC, currency.BTC)},
	}

	_, err := g.GetOrderHistory(&getOrdersRequest)
	switch {
	case areTestAPIKeysSet() && err != nil:
		t.Errorf("Could not get order history: %s", err)
	case !areTestAPIKeysSet() && err == nil && !mockTests:
		t.Error("Expecting an error when no keys are set")
	case err != nil && mockTests:
		t.Errorf("Could not get order history: %s", err)
	}
}

// Any tests below this line have the ability to impact your orders on the exchange. Enable canManipulateRealOrders to run them
// ----------------------------------------------------------------------------------------------------------------------------
func areTestAPIKeysSet() bool {
	return g.ValidateAPICredentials()
}

func TestSubmitOrder(t *testing.T) {
	t.Parallel()
	if areTestAPIKeysSet() && !canManipulateRealOrders && !mockTests {
		t.Skip("API keys set, canManipulateRealOrders false, skipping test")
	}

	var orderSubmission = &order.Submit{
		Pair: currency.Pair{
			Delimiter: "_",
			Base:      currency.LTC,
			Quote:     currency.BTC,
		},
		Side:     order.Buy,
		Type:     order.Limit,
		Price:    10,
		Amount:   1,
		ClientID: "1234234",
	}

	response, err := g.SubmitOrder(orderSubmission)
	switch {
	case areTestAPIKeysSet() && (err != nil || !response.IsOrderPlaced):
		t.Errorf("Order failed to be placed: %v", err)
	case !areTestAPIKeysSet() && err == nil && !mockTests:
		t.Error("Expecting an error when no keys are set")
	case mockTests && err != nil:
		t.Errorf("Order failed to be placed: %v", err)
	}
}

func TestCancelExchangeOrder(t *testing.T) {
	t.Parallel()
	if areTestAPIKeysSet() && !canManipulateRealOrders && !mockTests {
		t.Skip("API keys set, canManipulateRealOrders false, skipping test")
	}
	var orderCancellation = &order.Cancel{
		ID: "266029865",
	}

	err := g.CancelOrder(orderCancellation)
	switch {
	case !areTestAPIKeysSet() && err == nil && !mockTests:
		t.Error("Expecting an error when no keys are set")
	case areTestAPIKeysSet() && err != nil:
		t.Errorf("Could not cancel orders: %v", err)
	case err != nil && mockTests:
		t.Errorf("Could not cancel orders: %v", err)
	}
}

func TestCancelAllExchangeOrders(t *testing.T) {
	t.Parallel()
	if areTestAPIKeysSet() && !canManipulateRealOrders && !mockTests {
		t.Skip("API keys set, canManipulateRealOrders false, skipping test")
	}

	currencyPair := currency.NewPair(currency.LTC, currency.BTC)
	var orderCancellation = &order.Cancel{
		ID:            "1",
		WalletAddress: core.BitcoinDonationAddress,
		AccountID:     "1",
		Pair:          currencyPair,
	}

	resp, err := g.CancelAllOrders(orderCancellation)
	switch {
	case !areTestAPIKeysSet() && err == nil && !mockTests:
		t.Error("Expecting an error when no keys are set")
	case areTestAPIKeysSet() && err != nil:
		t.Errorf("Could not cancel orders: %v", err)
	case mockTests && err != nil:
		t.Errorf("Could not cancel orders: %v", err)
	}

	if len(resp.Status) > 0 {
		t.Errorf("%v orders failed to cancel", len(resp.Status))
	}
}

func TestModifyOrder(t *testing.T) {
	t.Parallel()
	_, err := g.ModifyOrder(&order.Modify{})
	if err == nil {
		t.Error("ModifyOrder() Expected error")
	}
}

func TestWithdraw(t *testing.T) {
	t.Parallel()
	withdrawCryptoRequest := withdraw.Request{
		Amount:      -1,
		Currency:    currency.BTC,
		Description: "WITHDRAW IT ALL",
		Crypto: &withdraw.CryptoRequest{
			Address: core.BitcoinDonationAddress,
		},
	}

	if areTestAPIKeysSet() && !canManipulateRealOrders && !mockTests {
		t.Skip("API keys set, canManipulateRealOrders false, skipping test")
	}

	_, err := g.WithdrawCryptocurrencyFunds(&withdrawCryptoRequest)
	if !areTestAPIKeysSet() && err == nil {
		t.Error("Expecting an error when no keys are set")
	}
	if areTestAPIKeysSet() && err != nil && !mockTests {
		t.Errorf("Withdraw failed to be placed: %v", err)
	}
	if areTestAPIKeysSet() && err == nil && mockTests {
		t.Errorf("Withdraw failed to be placed: %v", err)
	}
}

func TestWithdrawFiat(t *testing.T) {
	t.Parallel()
	if areTestAPIKeysSet() && !canManipulateRealOrders && !mockTests {
		t.Skip("API keys set, canManipulateRealOrders false, skipping test")
	}

	var withdrawFiatRequest = withdraw.Request{}
	_, err := g.WithdrawFiatFunds(&withdrawFiatRequest)
	if err != common.ErrFunctionNotSupported {
		t.Errorf("Expected '%v', received: '%v'",
			common.ErrFunctionNotSupported,
			err)
	}
}

func TestWithdrawInternationalBank(t *testing.T) {
	t.Parallel()
	if areTestAPIKeysSet() && !canManipulateRealOrders && !mockTests {
		t.Skip("API keys set, canManipulateRealOrders false, skipping test")
	}

	var withdrawFiatRequest = withdraw.Request{}
	_, err := g.WithdrawFiatFundsToInternationalBank(&withdrawFiatRequest)
	if err != common.ErrFunctionNotSupported {
		t.Errorf("Expected '%v', received: '%v'",
			common.ErrFunctionNotSupported,
			err)
	}
}

func TestGetDepositAddress(t *testing.T) {
	t.Parallel()
	_, err := g.GetDepositAddress(currency.BTC, "")
	if err == nil {
		t.Error("GetDepositAddress error cannot be nil")
	}
}

// TestWsAuth dials websocket, sends login request.
func TestWsAuth(t *testing.T) {
	t.Parallel()
	g.API.Endpoints.WebsocketURL = geminiWebsocketSandboxEndpoint

	if !g.Websocket.IsEnabled() &&
		!g.API.AuthenticatedWebsocketSupport ||
		!areTestAPIKeysSet() {
		t.Skip(stream.WebsocketNotEnabled)
	}
	var dialer websocket.Dialer
	go g.wsReadData()
	err := g.WsSecureSubscribe(&dialer, geminiWsOrderEvents)
	if err != nil {
		t.Error(err)
	}
	timer := time.NewTimer(sharedtestvalues.WebsocketResponseDefaultTimeout)
	select {
	case resp := <-g.Websocket.DataHandler:
		if resp.(WsSubscriptionAcknowledgementResponse).Type != "subscription_ack" {
			t.Error("Login failed")
		}
	case <-timer.C:
		t.Error("Expected response")
	}
	timer.Stop()
}

func TestWsMissingRole(t *testing.T) {
	pair, err := currency.NewPairFromString("BTCUSD")
	if err != nil {
		t.Fatal(err)
	}

	pressXToJSON := []byte(`{
		"result":"error",
		"reason":"MissingRole",
		"message":"To access this endpoint, you need to log in to the website and go to the settings page to assign one of these roles [FundManager] to API key wujB3szN54gtJ4QDhqRJ which currently has roles [Trader]"
	}`)
	err = g.wsHandleData(pressXToJSON, pair)
	if err == nil {
		t.Error("Expected error")
	}
}

func TestWsOrderEventSubscriptionResponse(t *testing.T) {
	pair, err := currency.NewPairFromString("BTCUSD")
	if err != nil {
		t.Fatal(err)
	}
	pressXToJSON := []byte(`[ {
  "type" : "accepted",
  "order_id" : "372456298",
  "event_id" : "372456299",
  "client_order_id": "20170208_example", 
  "api_session" : "AeRLptFXoYEqLaNiRwv8",
  "symbol" : "btcusd",
  "side" : "buy",
  "order_type" : "exchange limit",
  "timestamp" : "1478203017",
  "timestampms" : 1478203017455,
  "is_live" : true,
  "is_cancelled" : false,
  "is_hidden" : false,
  "avg_execution_price" : "0", 
  "original_amount" : "14.0296",
  "price" : "1059.54"
} ]`)
	err = g.wsHandleData(pressXToJSON, pair)
	if err != nil {
		t.Error(err)
	}

	pressXToJSON = []byte(`[{
    "type": "accepted",
    "order_id": "109535951",
    "event_id": "109535952",
    "api_session": "UI",
    "symbol": "btcusd",
    "side": "buy",
    "order_type": "exchange limit",
    "timestamp": "1547742904",
    "timestampms": 1547742904989,
    "is_live": true,
    "is_cancelled": false,
    "is_hidden": false,
    "original_amount": "1",
    "price": "3592.00",
    "socket_sequence": 13
}]`)
	err = g.wsHandleData(pressXToJSON, pair)
	if err != nil {
		t.Error(err)
	}

	pressXToJSON = []byte(`[{
    "type": "accepted",
    "order_id": "109964529",
    "event_id": "109964530",
    "api_session": "UI",
    "symbol": "btcusd",
    "side": "buy",
    "order_type": "market buy",
    "timestamp": "1547756076",
    "timestampms": 1547756076644,
    "is_live": false,
    "is_cancelled": false,
    "is_hidden": false,
    "total_spend": "200.00",
    "socket_sequence": 29
}]`)
	err = g.wsHandleData(pressXToJSON, pair)
	if err != nil {
		t.Error(err)
	}

	pressXToJSON = []byte(`[{
    "type": "accepted",
    "order_id": "109964616",
    "event_id": "109964617",
    "api_session": "UI",
    "symbol": "btcusd",
    "side": "sell",
    "order_type": "market sell",
    "timestamp": "1547756893",
    "timestampms": 1547756893937,
    "is_live": true,
    "is_cancelled": false,
    "is_hidden": false,
    "original_amount": "25",
    "socket_sequence": 26
}]`)
	err = g.wsHandleData(pressXToJSON, pair)
	if err != nil {
		t.Error(err)
	}

	pressXToJSON = []byte(`[ {
  "type" : "accepted",
  "order_id" : "6321",
  "event_id" : "6322",
  "api_session" : "UI",
  "symbol" : "btcusd",
  "side" : "sell",
  "order_type" : "block_trade",
  "timestamp" : "1478204198",
  "timestampms" : 1478204198989,
  "is_live" : true,
  "is_cancelled" : false,
  "is_hidden" : true,
  "avg_execution_price" : "0",
  "original_amount" : "500",
  "socket_sequence" : 32307
} ]`)
	err = g.wsHandleData(pressXToJSON, pair)
	if err != nil {
		t.Error(err)
	}
}

func TestWsSubAck(t *testing.T) {
	pair, err := currency.NewPairFromString("BTCUSD")
	if err != nil {
		t.Fatal(err)
	}
	pressXToJSON := []byte(`{
  "type": "subscription_ack",
  "accountId": 5365,
  "subscriptionId": "ws-order-events-5365-b8bk32clqeb13g9tk8p0",
  "symbolFilter": [
    "btcusd"
  ],
  "apiSessionFilter": [
    "UI"
  ],
  "eventTypeFilter": [
    "fill",
    "closed"
  ]
}`)
	err = g.wsHandleData(pressXToJSON, pair)
	if err != nil {
		t.Error(err)
	}
}

func TestWsHeartbeat(t *testing.T) {
	pair, err := currency.NewPairFromString("BTCUSD")
	if err != nil {
		t.Fatal(err)
	}
	pressXToJSON := []byte(`{
  "type": "heartbeat",
  "timestampms": 1547742998508,
  "sequence": 31,
  "trace_id": "b8biknoqppr32kc7gfgg",
  "socket_sequence": 37
}`)
	err = g.wsHandleData(pressXToJSON, pair)
	if err != nil {
		t.Error(err)
	}
}

func TestWsUnsubscribe(t *testing.T) {
	pair, err := currency.NewPairFromString("BTCUSD")
	if err != nil {
		t.Fatal(err)
	}
	pressXToJSON := []byte(`{
    "type": "unsubscribe",
    "subscriptions": [{
        "name": "l2",
        "symbols": [
            "BTCUSD",
            "ETHBTC"
        ]},
        {"name": "candles_1m",
        "symbols": [
            "BTCUSD",
            "ETHBTC"
        ]}
    ]
}`)
	err = g.wsHandleData(pressXToJSON, pair)
	if err != nil {
		t.Error(err)
	}
}

func TestWsTradeData(t *testing.T) {
	pair, err := currency.NewPairFromString("BTCUSD")
	if err != nil {
		t.Fatal(err)
	}
	pressXToJSON := []byte(`{
  "type": "update",
  "eventId": 5375547515,
  "timestamp": 1547760288,
  "timestampms": 1547760288001,
  "socket_sequence": 15,
  "events": [
    {
      "type": "trade",
      "tid": 5375547515,
      "price": "3632.54",
      "amount": "0.1362819142",
      "makerSide": "ask"
    }
  ]
}`)
	err = g.wsHandleData(pressXToJSON, pair)
	if err != nil {
		t.Error(err)
	}
}

func TestWsAuctionData(t *testing.T) {
	pair, err := currency.NewPairFromString("BTCUSD")
	if err != nil {
		t.Fatal(err)
	}
	pressXToJSON := []byte(`{
    "eventId": 371469414,
    "socket_sequence":4009, 
    "timestamp":1486501200,
    "timestampms":1486501200000,
    "events": [
        {
            "amount": "1406",
            "makerSide": "auction",
            "price": "1048.75",
            "tid": 371469414,
            "type": "trade"
        },
        {
            "auction_price": "1048.75",
            "auction_quantity": "1406",
            "eid": 371469414,
            "highest_bid_price": "1050.98",
            "lowest_ask_price": "1050.99",
            "result": "success",
            "time_ms": 1486501200000,
            "type": "auction_result"
        }
    ],
    "type": "update"
}`)
	err = g.wsHandleData(pressXToJSON, pair)
	if err != nil {
		t.Error(err)
	}
}

func TestWsBlockTrade(t *testing.T) {
	pair, err := currency.NewPairFromString("BTCUSD")
	if err != nil {
		t.Fatal(err)
	}
	pressXToJSON := []byte(`{
   "type":"update",
   "eventId":1111597035,
   "socket_sequence":8,
   "timestamp":1501175027,
   "timestampms":1501175027304,
   "events":[
      {
         "type":"block_trade",
         "tid":1111597035,
         "price":"10100.00",
         "amount":"1000"
      }
   ]
}`)
	err = g.wsHandleData(pressXToJSON, pair)
	if err != nil {
		t.Error(err)
	}
}

func TestWsCandles(t *testing.T) {
	pair, err := currency.NewPairFromString("BTCUSD")
	if err != nil {
		t.Fatal(err)
	}
	pressXToJSON := []byte(`{
  "type": "candles_15m_updates",
  "symbol": "BTCUSD",
  "changes": [
    [
        1561054500000,
        9350.18,
        9358.35,
        9350.18,
        9355.51,
        2.07
    ],
    [
        1561053600000,
        9357.33,
        9357.33,
        9350.18,
        9350.18,
        1.5900161
    ]
  ]
}`)
	err = g.wsHandleData(pressXToJSON, pair)
	if err != nil {
		t.Error(err)
	}
}

func TestWsAuctions(t *testing.T) {
	pair, err := currency.NewPairFromString("BTCUSD")
	if err != nil {
		t.Fatal(err)
	}
	pressXToJSON := []byte(`{
    "eventId": 372481811,
    "socket_sequence":23,
    "timestamp": 1486591200,
    "timestampms": 1486591200000,  
    "events": [
        {
            "auction_open_ms": 1486591200000,
            "auction_time_ms": 1486674000000,
            "first_indicative_ms": 1486673400000,
            "last_cancel_time_ms": 1486673985000,
            "type": "auction_open"
        }
    ],
    "type": "update"
}`)
	err = g.wsHandleData(pressXToJSON, pair)
	if err != nil {
		t.Error(err)
	}

	pressXToJSON = []byte(`{
    "type": "update",
    "eventId": 2248762586,
    "timestamp": 1510865640,
    "timestampms": 1510865640122,
    "socket_sequence": 177,
    "events": [
        {
            "type": "auction_indicative",
            "eid": 2248762586,
            "result": "success",
            "time_ms": 1510865640000,
            "highest_bid_price": "7730.69",
            "lowest_ask_price": "7730.7",
            "collar_price": "7730.695",
            "indicative_price": "7750",
            "indicative_quantity": "45.43325086"
        }
    ]
}`)
	err = g.wsHandleData(pressXToJSON, pair)
	if err != nil {
		t.Error(err)
	}

	pressXToJSON = []byte(`{
    "type": "update",
    "eventId": 2248795680,
    "timestamp": 1510866000,
    "timestampms": 1510866000095,
    "socket_sequence": 2920,
    "events": [
        {
            "type": "trade",
            "tid": 2248795680,
            "price": "7763.23",
            "amount": "55.95",
            "makerSide": "auction"
        },
        {
            "type": "auction_result",
            "eid": 2248795680,
            "result": "success",
            "time_ms": 1510866000000,
            "highest_bid_price": "7769",
            "lowest_ask_price": "7769.01",
            "collar_price": "7769.005",
            "auction_price": "7763.23",
            "auction_quantity": "55.95"
        }
    ]
}`)
	err = g.wsHandleData(pressXToJSON, pair)
	if err != nil {
		t.Error(err)
	}
}

func TestWsMarketData(t *testing.T) {
	pair, err := currency.NewPairFromString("BTCUSD")
	if err != nil {
		t.Fatal(err)
	}
	pressXToJSON := []byte(`{
  "type": "update",
  "eventId": 5375461993,
  "socket_sequence": 0,
  "events": [
    {
      "type": "change",
      "reason": "initial",
      "price": "3641.61",
      "delta": "0.83372051",
      "remaining": "0.83372051",
      "side": "bid"
    },
    {
      "type": "change",
      "reason": "initial",
      "price": "3641.62",
      "delta": "4.072",
      "remaining": "4.072",
      "side": "ask"
    }
  ]
}    `)
	err = g.wsHandleData(pressXToJSON, pair)
	if err != nil {
		t.Error(err)
	}

	pressXToJSON = []byte(`{
  "type": "update",
  "eventId": 5375461993,
  "socket_sequence": 0,
  "events": [
    {
      "type": "change",
      "reason": "initial",
      "price": "3641.61",
      "delta": "0.83372051",
      "remaining": "0.83372051",
      "side": "bid"
    },
    {
      "type": "change",
      "reason": "initial",
      "price": "3641.62",
      "delta": "4.072",
      "remaining": "4.072",
      "side": "ask"
    }
  ]
}    `)
	err = g.wsHandleData(pressXToJSON, pair)
	if err != nil {
		t.Error(err)
	}

	pressXToJSON = []byte(`{
  "type": "update",
  "eventId": 5375503736,
  "timestamp": 1547759964,
  "timestampms": 1547759964051,
  "socket_sequence": 2,
  "events": [
    {
      "type": "change",
      "side": "bid",
      "price": "3628.01",
      "remaining": "0",
      "delta": "-2",
      "reason": "cancel"
    }
  ]
}  `)
	err = g.wsHandleData(pressXToJSON, pair)
	if err != nil {
		t.Error(err)
	}
}

func TestResponseToStatus(t *testing.T) {
	type TestCases struct {
		Case   string
		Result order.Status
	}
	testCases := []TestCases{
		{Case: "accepted", Result: order.New},
		{Case: "booked", Result: order.Active},
		{Case: "fill", Result: order.Filled},
		{Case: "cancelled", Result: order.Cancelled},
		{Case: "cancel_rejected", Result: order.Rejected},
		{Case: "closed", Result: order.Filled},
		{Case: "LOL", Result: order.UnknownStatus},
	}
	for i := range testCases {
		result, _ := stringToOrderStatus(testCases[i].Case)
		if result != testCases[i].Result {
			t.Errorf("Exepcted: %v, received: %v", testCases[i].Result, result)
		}
	}
}

func TestResponseToOrderType(t *testing.T) {
	type TestCases struct {
		Case   string
		Result order.Type
	}
	testCases := []TestCases{
		{Case: "exchange limit", Result: order.Limit},
		{Case: "auction-only limit", Result: order.Limit},
		{Case: "indication-of-interest limit", Result: order.Limit},
		{Case: "market buy", Result: order.Market},
		{Case: "market sell", Result: order.Market},
		{Case: "block_trade", Result: order.Market},
		{Case: "LOL", Result: order.UnknownType},
	}
	for i := range testCases {
		result, _ := stringToOrderType(testCases[i].Case)
		if result != testCases[i].Result {
			t.Errorf("Exepcted: %v, received: %v", testCases[i].Result, result)
		}
	}
}

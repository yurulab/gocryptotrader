package kraken

import (
	"log"
	"net/http"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/gorilla/websocket"
	"github.com/yurulab/gocryptotrader/common/convert"
	"github.com/yurulab/gocryptotrader/config"
	"github.com/yurulab/gocryptotrader/core"
	"github.com/yurulab/gocryptotrader/currency"
	exchange "github.com/yurulab/gocryptotrader/exchanges"
	"github.com/yurulab/gocryptotrader/exchanges/asset"
	"github.com/yurulab/gocryptotrader/exchanges/kline"
	"github.com/yurulab/gocryptotrader/exchanges/order"
	"github.com/yurulab/gocryptotrader/exchanges/sharedtestvalues"
	"github.com/yurulab/gocryptotrader/exchanges/stream"
	"github.com/yurulab/gocryptotrader/portfolio/withdraw"
)

var k Kraken
var wsSetupRan bool

// Please add your own APIkeys to do correct due diligence testing.
const (
	apiKey                  = ""
	apiSecret               = ""
	canManipulateRealOrders = false
)

// TestSetup setup func
func TestMain(m *testing.M) {
	k.SetDefaults()
	cfg := config.GetConfig()
	err := cfg.LoadConfig("../../testdata/configtest.json", true)
	if err != nil {
		log.Fatal(err)
	}
	krakenConfig, err := cfg.GetExchangeConfig("Kraken")
	if err != nil {
		log.Fatal(err)
	}
	krakenConfig.API.AuthenticatedSupport = true
	krakenConfig.API.Credentials.Key = apiKey
	krakenConfig.API.Credentials.Secret = apiSecret
	krakenConfig.API.Endpoints.WebsocketURL = k.API.Endpoints.WebsocketURL
	k.Websocket = sharedtestvalues.NewTestWebsocket()
	err = k.Setup(krakenConfig)
	if err != nil {
		log.Fatal(err)
	}
	os.Exit(m.Run())
}

// TestGetServerTime API endpoint test
func TestGetServerTime(t *testing.T) {
	t.Parallel()
	_, err := k.GetServerTime()
	if err != nil {
		t.Error("GetServerTime() error", err)
	}
}

// TestGetAssets API endpoint test
func TestGetAssets(t *testing.T) {
	t.Parallel()
	_, err := k.GetAssets()
	if err != nil {
		t.Error("GetAssets() error", err)
	}
}

func TestSeedAssetTranslator(t *testing.T) {
	t.Parallel()
	// Test currency pair
	if r := assetTranslator.LookupAltname("XXBTZUSD"); r != "XBTUSD" {
		t.Error("unexpected result")
	}
	if r := assetTranslator.LookupCurrency("XBTUSD"); r != "XXBTZUSD" {
		t.Error("unexpected result")
	}

	// Test fiat currency
	if r := assetTranslator.LookupAltname("ZUSD"); r != "USD" {
		t.Error("unexpected result")
	}
	if r := assetTranslator.LookupCurrency("USD"); r != "ZUSD" {
		t.Error("unexpected result")
	}

	// Test cryptocurrency
	if r := assetTranslator.LookupAltname("XXBT"); r != "XBT" {
		t.Error("unexpected result")
	}
	if r := assetTranslator.LookupCurrency("XBT"); r != "XXBT" {
		t.Error("unexpected result")
	}
}

func TestSeedAssets(t *testing.T) {
	t.Parallel()
	var a assetTranslatorStore
	if r := a.LookupAltname("ZUSD"); r != "" {
		t.Error("unexpected result")
	}
	a.Seed("ZUSD", "USD")
	if r := a.LookupAltname("ZUSD"); r != "USD" {
		t.Error("unexpected result")
	}
	a.Seed("ZUSD", "BLA")
	if r := a.LookupAltname("ZUSD"); r != "USD" {
		t.Error("unexpected result")
	}
}

func TestLookupCurrency(t *testing.T) {
	t.Parallel()
	var a assetTranslatorStore
	if r := a.LookupCurrency("USD"); r != "" {
		t.Error("unexpected result")
	}
	a.Seed("ZUSD", "USD")
	if r := a.LookupCurrency("USD"); r != "ZUSD" {
		t.Error("unexpected result")
	}
	if r := a.LookupCurrency("EUR"); r != "" {
		t.Error("unexpected result")
	}
}

// TestGetAssetPairs API endpoint test
func TestGetAssetPairs(t *testing.T) {
	t.Parallel()
	_, err := k.GetAssetPairs()
	if err != nil {
		t.Error("GetAssetPairs() error", err)
	}
}

// TestGetTicker API endpoint test
func TestGetTicker(t *testing.T) {
	t.Parallel()
	_, err := k.GetTicker("BCHEUR")
	if err != nil {
		t.Error("GetTicker() error", err)
	}
}

// TestGetTickers API endpoint test
func TestGetTickers(t *testing.T) {
	t.Parallel()
	_, err := k.GetTickers("LTCUSD,ETCUSD")
	if err != nil {
		t.Error("GetTickers() error", err)
	}
}

// TestGetOHLC API endpoint test
func TestGetOHLC(t *testing.T) {
	t.Parallel()
	_, err := k.GetOHLC("XXBTZUSD", "1440")
	if err != nil {
		t.Error("GetOHLC() error", err)
	}
}

// TestGetDepth API endpoint test
func TestGetDepth(t *testing.T) {
	t.Parallel()
	_, err := k.GetDepth("BCHEUR")
	if err != nil {
		t.Error("GetDepth() error", err)
	}
}

// TestGetTrades API endpoint test
func TestGetTrades(t *testing.T) {
	t.Parallel()
	_, err := k.GetTrades("BCHEUR")
	if err != nil {
		t.Error("GetTrades() error", err)
	}
}

// TestGetSpread API endpoint test
func TestGetSpread(t *testing.T) {
	t.Parallel()
	_, err := k.GetSpread("BCHEUR")
	if err != nil {
		t.Error("GetSpread() error", err)
	}
}

// TestGetBalance API endpoint test
func TestGetBalance(t *testing.T) {
	t.Parallel()
	_, err := k.GetBalance()
	if err == nil {
		t.Error("GetBalance() Expected error")
	}
}

// TestGetTradeBalance API endpoint test
func TestGetTradeBalance(t *testing.T) {
	t.Parallel()
	args := TradeBalanceOptions{Asset: "ZEUR"}
	_, err := k.GetTradeBalance(args)
	if err == nil {
		t.Error("GetTradeBalance() Expected error")
	}
}

// TestGetOpenOrders API endpoint test
func TestGetOpenOrders(t *testing.T) {
	t.Parallel()
	args := OrderInfoOptions{Trades: true}
	_, err := k.GetOpenOrders(args)
	if err == nil {
		t.Error("GetOpenOrders() Expected error")
	}
}

// TestGetClosedOrders API endpoint test
func TestGetClosedOrders(t *testing.T) {
	t.Parallel()
	args := GetClosedOrdersOptions{Trades: true, Start: "OE4KV4-4FVQ5-V7XGPU"}
	_, err := k.GetClosedOrders(args)
	if err == nil {
		t.Error("GetClosedOrders() Expected error")
	}
}

// TestQueryOrdersInfo API endpoint test
func TestQueryOrdersInfo(t *testing.T) {
	t.Parallel()
	args := OrderInfoOptions{Trades: true}
	_, err := k.QueryOrdersInfo(args, "OR6ZFV-AA6TT-CKFFIW", "OAMUAJ-HLVKG-D3QJ5F")
	if err == nil {
		t.Error("QueryOrdersInfo() Expected error")
	}
}

// TestGetTradesHistory API endpoint test
func TestGetTradesHistory(t *testing.T) {
	t.Parallel()
	args := GetTradesHistoryOptions{Trades: true, Start: "TMZEDR-VBJN2-NGY6DX", End: "TVRXG2-R62VE-RWP3UW"}
	_, err := k.GetTradesHistory(args)
	if err == nil {
		t.Error("GetTradesHistory() Expected error")
	}
}

// TestQueryTrades API endpoint test
func TestQueryTrades(t *testing.T) {
	t.Parallel()
	_, err := k.QueryTrades(true, "TMZEDR-VBJN2-NGY6DX", "TFLWIB-KTT7L-4TWR3L", "TDVRAH-2H6OS-SLSXRX")
	if err == nil {
		t.Error("QueryTrades() Expected error")
	}
}

// TestOpenPositions API endpoint test
func TestOpenPositions(t *testing.T) {
	t.Parallel()
	_, err := k.OpenPositions(false)
	if err == nil {
		t.Error("OpenPositions() Expected error")
	}
}

// TestGetLedgers API endpoint test
func TestGetLedgers(t *testing.T) {
	t.Parallel()
	args := GetLedgersOptions{Start: "LRUHXI-IWECY-K4JYGO", End: "L5NIY7-JZQJD-3J4M2V", Ofs: 15}
	_, err := k.GetLedgers(args)
	if err == nil {
		t.Error("GetLedgers() Expected error")
	}
}

// TestQueryLedgers API endpoint test
func TestQueryLedgers(t *testing.T) {
	t.Parallel()
	_, err := k.QueryLedgers("LVTSFS-NHZVM-EXNZ5M")
	if err == nil {
		t.Error("QueryLedgers() Expected error")
	}
}

// TestGetTradeVolume API endpoint test
func TestGetTradeVolume(t *testing.T) {
	t.Parallel()
	_, err := k.GetTradeVolume(true, "OAVY7T-MV5VK-KHDF5X")
	if err == nil {
		t.Error("GetTradeVolume() Expected error")
	}
}

// TestAddOrder API endpoint test
func TestAddOrder(t *testing.T) {
	t.Parallel()
	args := AddOrderOptions{OrderFlags: "fcib"}
	_, err := k.AddOrder("XXBTZUSD",
		order.Sell.Lower(), order.Limit.Lower(),
		0.00000001, 0, 0, 0, &args)
	if err == nil {
		t.Error("AddOrder() Expected error")
	}
}

// TestCancelExistingOrder API endpoint test
func TestCancelExistingOrder(t *testing.T) {
	t.Parallel()
	_, err := k.CancelExistingOrder("OAVY7T-MV5VK-KHDF5X")
	if err == nil {
		t.Error("CancelExistingOrder() Expected error")
	}
}

func setFeeBuilder() *exchange.FeeBuilder {
	return &exchange.FeeBuilder{
		Amount:              1,
		FeeType:             exchange.CryptocurrencyTradeFee,
		Pair:                currency.NewPair(currency.XXBT, currency.ZUSD),
		PurchasePrice:       1,
		FiatCurrency:        currency.USD,
		BankTransactionType: exchange.WireTransfer,
	}
}

// TestGetFee logic test

// TestGetFeeByTypeOfflineTradeFee logic test
func TestGetFeeByTypeOfflineTradeFee(t *testing.T) {
	var feeBuilder = setFeeBuilder()
	k.GetFeeByType(feeBuilder)
	if !areTestAPIKeysSet() {
		if feeBuilder.FeeType != exchange.OfflineTradeFee {
			t.Errorf("Expected %v, received %v", exchange.OfflineTradeFee, feeBuilder.FeeType)
		}
	} else {
		if feeBuilder.FeeType != exchange.CryptocurrencyTradeFee {
			t.Errorf("Expected %v, received %v", exchange.CryptocurrencyTradeFee, feeBuilder.FeeType)
		}
	}
}

func TestGetFee(t *testing.T) {
	var feeBuilder = setFeeBuilder()

	if areTestAPIKeysSet() {
		// CryptocurrencyTradeFee Basic
		if resp, err := k.GetFee(feeBuilder); resp != float64(0.0026) || err != nil {
			t.Error(err)
			t.Errorf("GetFee() error. Expected: %f, Received: %f", float64(0.0026), resp)
		}

		// CryptocurrencyTradeFee High quantity
		feeBuilder = setFeeBuilder()
		feeBuilder.Amount = 1000
		feeBuilder.PurchasePrice = 1000
		if resp, err := k.GetFee(feeBuilder); resp != float64(2600) || err != nil {
			t.Errorf("GetFee() error. Expected: %f, Received: %f", float64(2600), resp)
			t.Error(err)
		}

		// CryptocurrencyTradeFee IsMaker
		feeBuilder = setFeeBuilder()
		feeBuilder.IsMaker = true
		if resp, err := k.GetFee(feeBuilder); resp != float64(0.0016) || err != nil {
			t.Errorf("GetFee() error. Expected: %f, Received: %f", float64(0.0016), resp)
			t.Error(err)
		}

		// CryptocurrencyTradeFee Negative purchase price
		feeBuilder = setFeeBuilder()
		feeBuilder.PurchasePrice = -1000
		if resp, err := k.GetFee(feeBuilder); resp != float64(0) || err != nil {
			t.Errorf("GetFee() error. Expected: %f, Received: %f", float64(0), resp)
			t.Error(err)
		}

		// InternationalBankDepositFee Basic
		feeBuilder = setFeeBuilder()
		feeBuilder.FeeType = exchange.InternationalBankDepositFee
		if resp, err := k.GetFee(feeBuilder); resp != float64(5) || err != nil {
			t.Errorf("GetFee() error. Expected: %f, Received: %f", float64(5), resp)
			t.Error(err)
		}
	}

	// CyptocurrencyDepositFee Basic
	feeBuilder = setFeeBuilder()
	feeBuilder.FeeType = exchange.CyptocurrencyDepositFee
	feeBuilder.Pair.Base = currency.XXBT
	if resp, err := k.GetFee(feeBuilder); resp != float64(0) || err != nil {
		t.Errorf("GetFee() error. Expected: %f, Received: %f", float64(5), resp)
		t.Error(err)
	}

	// CryptocurrencyWithdrawalFee Basic
	feeBuilder = setFeeBuilder()
	feeBuilder.FeeType = exchange.CryptocurrencyWithdrawalFee
	if resp, err := k.GetFee(feeBuilder); resp != float64(0.0005) || err != nil {
		t.Errorf("GetFee() error. Expected: %f, Received: %f", float64(0.0005), resp)
		t.Error(err)
	}

	// CryptocurrencyWithdrawalFee Invalid currency
	feeBuilder = setFeeBuilder()
	feeBuilder.Pair.Base = currency.NewCode("hello")
	feeBuilder.FeeType = exchange.CryptocurrencyWithdrawalFee
	if resp, err := k.GetFee(feeBuilder); resp != float64(0) || err != nil {
		t.Errorf("GetFee() error. Expected: %f, Received: %f", float64(0), resp)
		t.Error(err)
	}

	// InternationalBankWithdrawalFee Basic
	feeBuilder = setFeeBuilder()
	feeBuilder.FeeType = exchange.InternationalBankWithdrawalFee
	feeBuilder.FiatCurrency = currency.USD
	if resp, err := k.GetFee(feeBuilder); resp != float64(5) || err != nil {
		t.Errorf("GetFee() error. Expected: %f, Received: %f", float64(5), resp)
		t.Error(err)
	}
}

// TestFormatWithdrawPermissions logic test
func TestFormatWithdrawPermissions(t *testing.T) {
	expectedResult := exchange.AutoWithdrawCryptoWithSetupText + " & " + exchange.WithdrawCryptoWith2FAText + " & " + exchange.AutoWithdrawFiatWithSetupText + " & " + exchange.WithdrawFiatWith2FAText
	withdrawPermissions := k.FormatWithdrawPermissions()
	if withdrawPermissions != expectedResult {
		t.Errorf("Expected: %s, Received: %s", expectedResult, withdrawPermissions)
	}
}

// TestGetActiveOrders wrapper test
func TestGetActiveOrders(t *testing.T) {
	var getOrdersRequest = order.GetOrdersRequest{
		Type: order.AnyType,
	}

	_, err := k.GetActiveOrders(&getOrdersRequest)
	if areTestAPIKeysSet() && err != nil {
		t.Errorf("Could not get open orders: %s", err)
	} else if !areTestAPIKeysSet() && err == nil {
		t.Error("Expecting an error when no keys are set")
	}
}

// TestGetOrderHistory wrapper test
func TestGetOrderHistory(t *testing.T) {
	var getOrdersRequest = order.GetOrdersRequest{
		Type: order.AnyType,
	}

	_, err := k.GetOrderHistory(&getOrdersRequest)
	if areTestAPIKeysSet() && err != nil {
		t.Errorf("Could not get order history: %s", err)
	} else if !areTestAPIKeysSet() && err == nil {
		t.Error("Expecting an error when no keys are set")
	}
}

// TestGetOrderHistory wrapper test
func TestGetOrderInfo(t *testing.T) {
	if areTestAPIKeysSet() && !canManipulateRealOrders {
		t.Skip("API keys set, canManipulateRealOrders false, skipping test")
	}

	_, err := k.GetOrderInfo("OZPTPJ-HVYHF-EDIGXS")
	if !areTestAPIKeysSet() && err == nil {
		t.Error("Expecting error")
	}
	if areTestAPIKeysSet() && err != nil {
		if !strings.Contains(err.Error(), "- Order ID not found:") {
			t.Error("Expected Order ID not found error")
		} else {
			t.Error(err)
		}
	}
}

// Any tests below this line have the ability to impact your orders on the exchange. Enable canManipulateRealOrders to run them
// ----------------------------------------------------------------------------------------------------------------------------
func areTestAPIKeysSet() bool {
	return k.ValidateAPICredentials()
}

// TestSubmitOrder wrapper test
func TestSubmitOrder(t *testing.T) {
	if areTestAPIKeysSet() && !canManipulateRealOrders {
		t.Skip("API keys set, canManipulateRealOrders false, skipping test")
	}

	var orderSubmission = &order.Submit{
		Pair: currency.Pair{
			Base:  currency.XBT,
			Quote: currency.USD,
		},
		Side:     order.Buy,
		Type:     order.Limit,
		Price:    1,
		Amount:   1,
		ClientID: "meowOrder",
	}
	response, err := k.SubmitOrder(orderSubmission)
	if areTestAPIKeysSet() && (err != nil || !response.IsOrderPlaced) {
		t.Errorf("Order failed to be placed: %v", err)
	} else if !areTestAPIKeysSet() && err == nil {
		t.Error("Expecting an error when no keys are set")
	}
}

// TestCancelExchangeOrder wrapper test
func TestCancelExchangeOrder(t *testing.T) {
	if areTestAPIKeysSet() && !canManipulateRealOrders {
		t.Skip("API keys set, canManipulateRealOrders false, skipping test")
	}

	var orderCancellation = &order.Cancel{
		ID: "OGEX6P-B5Q74-IGZ72R",
	}

	err := k.CancelOrder(orderCancellation)
	if !areTestAPIKeysSet() && err == nil {
		t.Error("Expecting an error when no keys are set")
	}
	if areTestAPIKeysSet() && err != nil {
		t.Errorf("Could not cancel orders: %v", err)
	}
}

// TestCancelAllExchangeOrders wrapper test
func TestCancelAllExchangeOrders(t *testing.T) {
	if areTestAPIKeysSet() && !canManipulateRealOrders {
		t.Skip("API keys set, canManipulateRealOrders false, skipping test")
	}

	resp, err := k.CancelAllOrders(&order.Cancel{})
	if !areTestAPIKeysSet() && err == nil {
		t.Error("Expecting an error when no keys are set")
	}
	if areTestAPIKeysSet() && err != nil {
		t.Errorf("Could not cancel orders: %v", err)
	}

	if len(resp.Status) > 0 {
		t.Errorf("%v orders failed to cancel", len(resp.Status))
	}
}

// TestGetAccountInfo wrapper test
func TestGetAccountInfo(t *testing.T) {
	if areTestAPIKeysSet() {
		_, err := k.UpdateAccountInfo()
		if err != nil {
			t.Error("GetAccountInfo() error", err)
		}
	} else {
		_, err := k.UpdateAccountInfo()
		if err == nil {
			t.Error("GetAccountInfo() Expected error")
		}
	}
}

// TestModifyOrder wrapper test
func TestModifyOrder(t *testing.T) {
	if areTestAPIKeysSet() && !canManipulateRealOrders {
		t.Skip("API keys set, canManipulateRealOrders false, skipping test")
	}
	_, err := k.ModifyOrder(&order.Modify{})
	if err == nil {
		t.Error("ModifyOrder() Expected error")
	}
}

// TestWithdraw wrapper test
func TestWithdraw(t *testing.T) {
	withdrawCryptoRequest := withdraw.Request{
		Crypto: &withdraw.CryptoRequest{
			Address: core.BitcoinDonationAddress,
		},
		Amount:        -1,
		Currency:      currency.XXBT,
		Description:   "WITHDRAW IT ALL",
		TradePassword: "Key",
	}

	if areTestAPIKeysSet() && !canManipulateRealOrders {
		t.Skip("API keys set, canManipulateRealOrders false, skipping test")
	}

	_, err := k.WithdrawCryptocurrencyFunds(&withdrawCryptoRequest)
	if !areTestAPIKeysSet() && err == nil {
		t.Error("Expecting an error when no keys are set")
	}
	if areTestAPIKeysSet() && err != nil {
		t.Errorf("Withdraw failed to be placed: %v", err)
	}
}

// TestWithdrawFiat wrapper test
func TestWithdrawFiat(t *testing.T) {
	if areTestAPIKeysSet() && !canManipulateRealOrders {
		t.Skip("API keys set, canManipulateRealOrders false, skipping test")
	}

	var withdrawFiatRequest = withdraw.Request{
		Amount:        -1,
		Currency:      currency.EUR,
		Description:   "WITHDRAW IT ALL",
		TradePassword: "someBank",
	}

	_, err := k.WithdrawFiatFunds(&withdrawFiatRequest)
	if !areTestAPIKeysSet() && err == nil {
		t.Error("Expecting an error when no keys are set")
	}
	if areTestAPIKeysSet() && err != nil {
		t.Errorf("Withdraw failed to be placed: %v", err)
	}
}

// TestWithdrawInternationalBank wrapper test
func TestWithdrawInternationalBank(t *testing.T) {
	if areTestAPIKeysSet() && !canManipulateRealOrders {
		t.Skip("API keys set, canManipulateRealOrders false, skipping test")
	}

	var withdrawFiatRequest = withdraw.Request{
		Amount:        -1,
		Currency:      currency.EUR,
		Description:   "WITHDRAW IT ALL",
		TradePassword: "someBank",
	}

	_, err := k.WithdrawFiatFundsToInternationalBank(&withdrawFiatRequest)
	if !areTestAPIKeysSet() && err == nil {
		t.Error("Expecting an error when no keys are set")
	}
	if areTestAPIKeysSet() && err != nil {
		t.Errorf("Withdraw failed to be placed: %v", err)
	}
}

// TestGetDepositAddress wrapper test
func TestGetDepositAddress(t *testing.T) {
	if areTestAPIKeysSet() {
		_, err := k.GetDepositAddress(currency.BTC, "")
		if err != nil {
			t.Error("GetDepositAddress() error", err)
		}
	} else {
		_, err := k.GetDepositAddress(currency.BTC, "")
		if err == nil {
			t.Error("GetDepositAddress() error can not be nil")
		}
	}
}

// TestWithdrawStatus wrapper test
func TestWithdrawStatus(t *testing.T) {
	if areTestAPIKeysSet() {
		_, err := k.WithdrawStatus(currency.BTC, "")
		if err != nil {
			t.Error("WithdrawStatus() error", err)
		}
	} else {
		_, err := k.WithdrawStatus(currency.BTC, "")
		if err == nil {
			t.Error("GetDepositAddress() error can not be nil")
		}
	}
}

// TestWithdrawCancel wrapper test
func TestWithdrawCancel(t *testing.T) {
	_, err := k.WithdrawCancel(currency.BTC, "")
	if areTestAPIKeysSet() && err == nil {
		t.Error("WithdrawCancel() error cannot be nil")
	} else if !areTestAPIKeysSet() && err == nil {
		t.Errorf("WithdrawCancel() error - expecting an error when no keys are set but received nil")
	}
}

// ---------------------------- Websocket tests -----------------------------------------

func setupWsTests(t *testing.T) {
	if wsSetupRan {
		return
	}
	if !k.Websocket.IsEnabled() && !k.API.AuthenticatedWebsocketSupport || !areTestAPIKeysSet() {
		t.Skip(stream.WebsocketNotEnabled)
	}
	var dialer websocket.Dialer
	err := k.Websocket.Conn.Dial(&dialer, http.Header{})
	if err != nil {
		t.Fatal(err)
	}
	err = k.Websocket.AuthConn.Dial(&dialer, http.Header{})
	if err != nil {
		t.Fatal(err)
	}

	token, err := k.GetWebsocketToken()
	if err != nil {
		t.Error(err)
	}
	authToken = token
	comms := make(chan stream.Response)
	go k.wsFunnelConnectionData(k.Websocket.Conn, comms)
	go k.wsFunnelConnectionData(k.Websocket.AuthConn, comms)
	go k.wsReadData(comms)
	go k.wsPingHandler()
	wsSetupRan = true
}

// TestWebsocketSubscribe tests returning a message with an id
func TestWebsocketSubscribe(t *testing.T) {
	setupWsTests(t)
	err := k.Subscribe([]stream.ChannelSubscription{
		{
			Channel:  defaultSubscribedChannels[0],
			Currency: currency.NewPairWithDelimiter("XBT", "USD", "/"),
		},
	})
	if err != nil {
		t.Error(err)
	}
}

func TestGetWSToken(t *testing.T) {
	t.Parallel()
	if !areTestAPIKeysSet() {
		t.Skip("API keys required, skipping")
	}
	resp, err := k.GetWebsocketToken()
	if err != nil {
		t.Error(err)
	}
	if resp == "" {
		t.Error("Token not returned")
	}
}

func TestWsAddOrder(t *testing.T) {
	setupWsTests(t)
	_, err := k.wsAddOrder(&WsAddOrderRequest{
		OrderType: order.Limit.Lower(),
		OrderSide: order.Buy.Lower(),
		Pair:      "XBT/USD",
		Price:     -100,
	})
	if err != nil {
		t.Error(err)
	}
}

func TestWsCancelOrder(t *testing.T) {
	setupWsTests(t)
	err := k.wsCancelOrders([]string{"1337"})
	if err != nil {
		t.Error(err)
	}
}

func TestWsPong(t *testing.T) {
	pressXToJSON := []byte(`{
  "event": "pong",
  "reqid": 42
}`)
	err := k.wsHandleData(pressXToJSON)
	if err != nil {
		t.Error(err)
	}
}

func TestWsSystemStatus(t *testing.T) {
	pressXToJSON := []byte(`{
  "connectionID": 8628615390848610000,
  "event": "systemStatus",
  "status": "online",
  "version": "1.0.0"
}`)
	err := k.wsHandleData(pressXToJSON)
	if err != nil {
		t.Error(err)
	}
}

func TestWsSubscriptionStatus(t *testing.T) {
	pressXToJSON := []byte(`{
  "channelID": 10001,
  "channelName": "ticker",
  "event": "subscriptionStatus",
  "pair": "XBT/EUR",
  "status": "subscribed",
  "subscription": {
    "name": "ticker"
  }
}`)
	err := k.wsHandleData(pressXToJSON)
	if err != nil {
		t.Error(err)
	}

	pressXToJSON = []byte(`{
  "channelID": 10001,
  "channelName": "ohlc-5",
  "event": "subscriptionStatus",
  "pair": "XBT/EUR",
  "reqid": 42,
  "status": "unsubscribed",
  "subscription": {
    "interval": 5,
    "name": "ohlc"
  }
}`)
	err = k.wsHandleData(pressXToJSON)
	if err != nil {
		t.Error(err)
	}

	pressXToJSON = []byte(`{
  "channelName": "ownTrades",
  "event": "subscriptionStatus",
  "status": "subscribed",
  "subscription": {
    "name": "ownTrades"
  }
}`)
	err = k.wsHandleData(pressXToJSON)
	if err != nil {
		t.Error(err)
	}
	pressXToJSON = []byte(`{
  "errorMessage": "Subscription depth not supported",
  "event": "subscriptionStatus",
  "pair": "XBT/USD",
  "status": "error",
  "subscription": {
    "depth": 42,
    "name": "book"
  }
}`)
	err = k.wsHandleData(pressXToJSON)
	if err == nil {
		t.Error("Expected error")
	}
}

func TestWsTicker(t *testing.T) {
	pressXToJSON := []byte(`{
  "channelID": 1337,
  "channelName": "ticker",
  "event": "subscriptionStatus",
  "pair": "XBT/EUR",
  "status": "subscribed",
  "subscription": {
    "name": "ticker"
  }
}`)
	err := k.wsHandleData(pressXToJSON)
	if err != nil {
		t.Error(err)
	}
	pressXToJSON = []byte(`[
  1337,
  {
    "a": [
      "5525.40000",
      1,
      "1.000"
    ],
    "b": [
      "5525.10000",
      1,
      "1.000"
    ],
    "c": [
      "5525.10000",
      "0.00398963"
    ],
    "h": [
      "5783.00000",
      "5783.00000"
    ],
    "l": [
      "5505.00000",
      "5505.00000"
    ],
    "o": [
      "5760.70000",
      "5763.40000"
    ],
    "p": [
      "5631.44067",
      "5653.78939"
    ],
    "t": [
      11493,
      16267
    ],
    "v": [
      "2634.11501494",
      "3591.17907851"
    ]
  },
  "ticker",
  "XBT/USD"
]`)
	err = k.wsHandleData(pressXToJSON)
	if err != nil {
		t.Error(err)
	}
}

func TestWsOHLC(t *testing.T) {
	pressXToJSON := []byte(`{
  "channelID": 13337,
  "channelName": "ohlc",
  "event": "subscriptionStatus",
  "pair": "XBT/EUR",
  "status": "subscribed",
  "subscription": {
    "name": "ohlc"
  }
}`)
	err := k.wsHandleData(pressXToJSON)
	if err != nil {
		t.Error(err)
	}
	pressXToJSON = []byte(`[
  13337,
  [
    "1542057314.748456",
    "1542057360.435743",
    "3586.70000",
    "3586.70000",
    "3586.60000",
    "3586.60000",
    "3586.68894",
    "0.03373000",
    2
  ],
  "ohlc-5",
  "XBT/USD"
]`)
	err = k.wsHandleData(pressXToJSON)
	if err != nil {
		t.Error(err)
	}
}

func TestWsTrade(t *testing.T) {
	pressXToJSON := []byte(`{
  "channelID": 133337,
  "channelName": "trade",
  "event": "subscriptionStatus",
  "pair": "XBT/EUR",
  "status": "subscribed",
  "subscription": {
    "name": "trade"
  }
}`)
	err := k.wsHandleData(pressXToJSON)
	if err != nil {
		t.Error(err)
	}
	pressXToJSON = []byte(`[
  133337,
  [
    [
      "5541.20000",
      "0.15850568",
      "1534614057.321597",
      "s",
      "l",
      ""
    ],
    [
      "6060.00000",
      "0.02455000",
      "1534614057.324998",
      "b",
      "l",
      ""
    ]
  ],
  "trade",
  "XBT/USD"
]`)
	err = k.wsHandleData(pressXToJSON)
	if err != nil {
		t.Error(err)
	}
}

func TestWsSpread(t *testing.T) {
	pressXToJSON := []byte(`{
  "channelID": 1333337,
  "channelName": "spread",
  "event": "subscriptionStatus",
  "pair": "XBT/EUR",
  "status": "subscribed",
  "subscription": {
    "name": "spread"
  }
}`)
	err := k.wsHandleData(pressXToJSON)
	if err != nil {
		t.Error(err)
	}
	pressXToJSON = []byte(`[
  1333337,
  [
    "5698.40000",
    "5700.00000",
    "1542057299.545897",
    "1.01234567",
    "0.98765432"
  ],
  "spread",
  "XBT/USD"
]`)
	err = k.wsHandleData(pressXToJSON)
	if err != nil {
		t.Error(err)
	}
}

func TestWsOrdrbook(t *testing.T) {
	pressXToJSON := []byte(`{
  "channelID": 13333337,
  "channelName": "book",
  "event": "subscriptionStatus",
  "pair": "XBT/EUR",
  "status": "subscribed",
  "subscription": {
    "name": "book"
  }
}`)
	err := k.wsHandleData(pressXToJSON)
	if err != nil {
		t.Error(err)
	}
	pressXToJSON = []byte(`[
  13333337,
  {
    "as": [
      [
        "5541.30000",
        "2.50700000",
        "1534614248.123678"
      ],
      [
        "5541.80000",
        "0.33000000",
        "1534614098.345543"
      ],
      [
        "5542.70000",
        "0.64700000",
        "1534614244.654432"
      ]
    ],
    "bs": [
      [
        "5541.20000",
        "1.52900000",
        "1534614248.765567"
      ],
      [
        "5539.90000",
        "0.30000000",
        "1534614241.769870"
      ],
      [
        "5539.50000",
        "5.00000000",
        "1534613831.243486"
      ]
    ]
  },
  "book-100",
  "XBT/USD"
]`)
	err = k.wsHandleData(pressXToJSON)
	if err != nil {
		t.Error(err)
	}
	pressXToJSON = []byte(`[
  13333337,
  {
    "a": [
      [
        "5541.30000",
        "2.50700000",
        "1534614248.456738"
      ],
      [
        "5542.50000",
        "0.40100000",
        "1534614248.456738"
      ]
    ]
  },
  "book-10",
  "XBT/USD"
]`)
	err = k.wsHandleData(pressXToJSON)
	if err != nil {
		t.Error(err)
	}
	pressXToJSON = []byte(`[
  13333337,
  {
    "b": [
      [
        "5541.30000",
        "0.00000000",
        "1534614335.345903"
      ]
    ]
  },
  "book-10",
  "XBT/USD"
]`)
	err = k.wsHandleData(pressXToJSON)
	if err != nil {
		t.Error(err)
	}
}

func TestWsOwnTrades(t *testing.T) {
	pressXToJSON := []byte(`[
  [
    {
      "TDLH43-DVQXD-2KHVYY": {
        "cost": "1000000.00000",
        "fee": "1600.00000",
        "margin": "0.00000",
        "ordertxid": "TDLH43-DVQXD-2KHVYY",
        "ordertype": "limit",
        "pair": "XBT/USD",
        "postxid": "OGTT3Y-C6I3P-XRI6HX",
        "price": "100000.00000",
        "time": "1560516023.070651",
        "type": "sell",
        "vol": "1000000000.00000000"
      }
    },
    {
      "TDLH43-DVQXD-2KHVYY": {
        "cost": "1000000.00000",
        "fee": "600.00000",
        "margin": "0.00000",
        "ordertxid": "TDLH43-DVQXD-2KHVYY",
        "ordertype": "limit",
        "pair": "XBT/USD",
        "postxid": "OGTT3Y-C6I3P-XRI6HX",
        "price": "100000.00000",
        "time": "1560516023.070658",
        "type": "buy",
        "vol": "1000000000.00000000"
      }
    },
    {
      "TDLH43-DVQXD-2KHVYY": {
        "cost": "1000000.00000",
        "fee": "1600.00000",
        "margin": "0.00000",
        "ordertxid": "TDLH43-DVQXD-2KHVYY",
        "ordertype": "limit",
        "pair": "XBT/USD",
        "postxid": "OGTT3Y-C6I3P-XRI6HX",
        "price": "100000.00000",
        "time": "1560520332.914657",
        "type": "sell",
        "vol": "1000000000.00000000"
      }
    },
    {
      "TDLH43-DVQXD-2KHVYY": {
        "cost": "1000000.00000",
        "fee": "600.00000",
        "margin": "0.00000",
        "ordertxid": "TDLH43-DVQXD-2KHVYY",
        "ordertype": "limit",
        "pair": "XBT/USD",
        "postxid": "OGTT3Y-C6I3P-XRI6HX",
        "price": "100000.00000",
        "time": "1560520332.914664",
        "type": "buy",
        "vol": "1000000000.00000000"
      }
    }
  ],
  "ownTrades"
]`)
	err := k.wsHandleData(pressXToJSON)
	if err != nil {
		t.Error(err)
	}
}

func TestWsOpenOrders(t *testing.T) {
	pressXToJSON := []byte(`[
  [
    {
      "OGTT3Y-C6I3P-XRI6HX": {
        "cost": "0.00000",
        "descr": {
          "close": "",
          "leverage": "0:1",
          "order": "sell 10.00345345 XBT/USD @ limit 34.50000 with 0:1 leverage",
          "ordertype": "limit",
          "pair": "XBT/USD",
          "price": "34.50000",
          "price2": "0.00000",
          "type": "sell"
        },
        "expiretm": "0.000000",
        "fee": "0.00000",
        "limitprice": "34.50000",
        "misc": "",
        "oflags": "fcib",
        "opentm": "0.000000",
        "price": "34.50000",
        "refid": "OKIVMP-5GVZN-Z2D2UA",
        "starttm": "0.000000",
        "status": "open",
        "stopprice": "0.000000",
        "userref": 0,
        "vol": "10.00345345",
        "vol_exec": "0.00000000"
      }
    },
    {
      "OGTT3Y-C6I3P-XRI6HX": {
        "cost": "0.00000",
        "descr": {
          "close": "",
          "leverage": "0:1",
          "order": "sell 0.00000010 XBT/USD @ limit 5334.60000 with 0:1 leverage",
          "ordertype": "limit",
          "pair": "XBT/USD",
          "price": "5334.60000",
          "price2": "0.00000",
          "type": "sell"
        },
        "expiretm": "0.000000",
        "fee": "0.00000",
        "limitprice": "5334.60000",
        "misc": "",
        "oflags": "fcib",
        "opentm": "0.000000",
        "price": "5334.60000",
        "refid": "OKIVMP-5GVZN-Z2D2UA",
        "starttm": "0.000000",
        "status": "open",
        "stopprice": "0.000000",
        "userref": 0,
        "vol": "0.00000010",
        "vol_exec": "0.00000000"
      }
    },
    {
      "OGTT3Y-C6I3P-XRI6HX": {
        "cost": "0.00000",
        "descr": {
          "close": "",
          "leverage": "0:1",
          "order": "sell 0.00001000 XBT/USD @ limit 90.40000 with 0:1 leverage",
          "ordertype": "limit",
          "pair": "XBT/USD",
          "price": "90.40000",
          "price2": "0.00000",
          "type": "sell"
        },
        "expiretm": "0.000000",
        "fee": "0.00000",
        "limitprice": "90.40000",
        "misc": "",
        "oflags": "fcib",
        "opentm": "0.000000",
        "price": "90.40000",
        "refid": "OKIVMP-5GVZN-Z2D2UA",
        "starttm": "0.000000",
        "status": "open",
        "stopprice": "0.000000",
        "userref": 0,
        "vol": "0.00001000",
        "vol_exec": "0.00000000"
      }
    },
    {
      "OGTT3Y-C6I3P-XRI6HX": {
        "cost": "0.00000",
        "descr": {
          "close": "",
          "leverage": "0:1",
          "order": "sell 0.00001000 XBT/USD @ limit 9.00000 with 0:1 leverage",
          "ordertype": "limit",
          "pair": "XBT/USD",
          "price": "9.00000",
          "price2": "0.00000",
          "type": "sell"
        },
        "expiretm": "0.000000",
        "fee": "0.00000",
        "limitprice": "9.00000",
        "misc": "",
        "oflags": "fcib",
        "opentm": "0.000000",
        "price": "9.00000",
        "refid": "OKIVMP-5GVZN-Z2D2UA",
        "starttm": "0.000000",
        "status": "open",
        "stopprice": "0.000000",
        "userref": 0,
        "vol": "0.00001000",
        "vol_exec": "0.00000000"
      }
    }
  ],
  "openOrders"
]`)
	err := k.wsHandleData(pressXToJSON)
	if err != nil {
		t.Error(err)
	}
	pressXToJSON = []byte(`[
  [
    {
      "OGTT3Y-C6I3P-XRI6HX": {
        "status": "closed"
      }
    },
    {
      "OGTT3Y-C6I3P-XRI6HX": {
        "status": "closed"
      }
    }
  ],
  "openOrders"
]`)
	err = k.wsHandleData(pressXToJSON)
	if err != nil {
		t.Error(err)
	}
}

func TestWsAddOrderJSON(t *testing.T) {
	pressXToJSON := []byte(`{
  "descr": "buy 0.01770000 XBTUSD @ limit 4000",
  "event": "addOrderStatus",
  "status": "ok",
  "txid": "ONPNXH-KMKMU-F4MR5V"
}`)
	err := k.wsHandleData(pressXToJSON)
	if err != nil {
		t.Error(err)
	}

	pressXToJSON = []byte(`{
  "errorMessage": "EOrder:Order minimum not met",
  "event": "addOrderStatus",
  "status": "error"
}`)
	err = k.wsHandleData(pressXToJSON)
	if err == nil {
		t.Error("Expected error")
	}
}

func TestWsCancelOrderJSON(t *testing.T) {
	pressXToJSON := []byte(`{
  "event": "cancelOrderStatus",
  "status": "ok"
}`)
	err := k.wsHandleData(pressXToJSON)
	if err != nil {
		t.Error(err)
	}
}

func TestParseTime(t *testing.T) {
	// Test REST example
	r := convert.TimeFromUnixTimestampDecimal(1373750306.9819).UTC()
	if r.Year() != 2013 ||
		r.Month().String() != "July" ||
		r.Day() != 13 {
		t.Error("unexpected result")
	}

	// Test Websocket time example
	r = convert.TimeFromUnixTimestampDecimal(1534614098.345543).UTC()
	if r.Year() != 2018 ||
		r.Month().String() != "August" ||
		r.Day() != 18 {
		t.Error("unexpected result")
	}
}

func TestGetHistoricCandles(t *testing.T) {
	currencyPair, err := currency.NewPairFromString("XBTUSD")
	if err != nil {
		t.Fatal(err)
	}
	_, err = k.GetHistoricCandles(currencyPair, asset.Spot, time.Now().AddDate(0, 0, -1), time.Now(), kline.OneMin)
	if err != nil {
		t.Fatal(err)
	}

	_, err = k.GetHistoricCandles(currencyPair, asset.Spot, time.Now(), time.Now(), kline.Interval(time.Hour*7))
	if err == nil {
		t.Fatal("unexpected result")
	}
}

func TestGetHistoricCandlesExtended(t *testing.T) {
	currencyPair, err := currency.NewPairFromString("XBTUSD")
	if err != nil {
		t.Fatal(err)
	}
	_, err = k.GetHistoricCandlesExtended(currencyPair, asset.Spot, time.Now().AddDate(0, -6, 0), time.Now(), kline.OneDay)
	if err != nil {
		t.Fatal(err)
	}

	_, err = k.GetHistoricCandlesExtended(currencyPair, asset.Spot, time.Now(), time.Now(), kline.Interval(time.Hour*7))
	if err == nil {
		t.Fatal("unexpected result")
	}
}

func Test_FormatExchangeKlineInterval(t *testing.T) {
	testCases := []struct {
		name     string
		interval kline.Interval
		output   string
	}{
		{
			"OneMin",
			kline.OneMin,
			"1",
		},
		{
			"OneDay",
			kline.OneDay,
			"1440",
		},
	}

	for x := range testCases {
		test := testCases[x]

		t.Run(test.name, func(t *testing.T) {
			ret := k.FormatExchangeKlineInterval(test.interval)

			if ret != test.output {
				t.Fatalf("unexpected result return expected: %v received: %v", test.output, ret)
			}
		})
	}
}

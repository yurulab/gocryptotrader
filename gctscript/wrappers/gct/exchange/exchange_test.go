package exchange

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/yurulab/gocryptotrader/currency"
	"github.com/yurulab/gocryptotrader/engine"
	"github.com/yurulab/gocryptotrader/exchanges/asset"
	"github.com/yurulab/gocryptotrader/exchanges/order"
)

// change these if you wish to test another exchange and/or currency pair
const (
	exchName      = "BTC Markets" // change to test on another exchange
	exchAPIKEY    = ""
	exchAPISECRET = ""
	exchClientID  = ""
	pairs         = "BTC-AUD" // change to test another currency pair
	delimiter     = "-"
	assetType     = asset.Spot
	orderID       = "1234"
	orderType     = order.Limit
	orderSide     = order.Buy
	orderClientID = ""
	orderPrice    = 1
	orderAmount   = 1
)

var (
	settings = engine.Settings{
		ConfigFile:          filepath.Join("..", "..", "..", "..", "testdata", "configtest.json"),
		EnableDryRun:        true,
		DataDir:             filepath.Join("..", "..", "..", "..", "testdata", "gocryptotrader"),
		Verbose:             false,
		EnableGRPC:          false,
		EnableDeprecatedRPC: false,
		EnableWebsocketRPC:  false,
	}
	exchangeTest = Exchange{}
)

func TestMain(m *testing.M) {
	var t int
	err := setupEngine()
	if err != nil {
		fmt.Printf("Failed to configure exchange test cannot continue: %v", err)
		os.Exit(1)
	}
	t = m.Run()
	cleanup()
	os.Exit(t)
}

func TestExchange_Exchanges(t *testing.T) {
	t.Parallel()
	x := exchangeTest.Exchanges(false)
	y := len(x)
	if y != 28 {
		t.Fatalf("expected 28 received %v", y)
	}
}

func TestExchange_GetExchange(t *testing.T) {
	t.Parallel()
	_, err := exchangeTest.GetExchange(exchName)
	if err != nil {
		t.Fatal(err)
	}
	_, err = exchangeTest.GetExchange("hello world")
	if err == nil {
		t.Fatal("unexpected error message received nil")
	}
}

func TestExchange_IsEnabled(t *testing.T) {
	t.Parallel()
	x := exchangeTest.IsEnabled(exchName)
	if !x {
		t.Fatal("expected return to be true")
	}
	x = exchangeTest.IsEnabled("fake_exchange")
	if x {
		t.Fatal("expected return to be false")
	}
}

func TestExchange_Ticker(t *testing.T) {
	t.Parallel()
	c, err := currency.NewPairDelimiter(pairs, delimiter)
	if err != nil {
		t.Fatal(err)
	}
	_, err = exchangeTest.Ticker(exchName, c, assetType)
	if err != nil {
		t.Fatal(err)
	}
}

func TestExchange_Orderbook(t *testing.T) {
	t.Parallel()
	c, err := currency.NewPairDelimiter(pairs, delimiter)
	if err != nil {
		t.Fatal(err)
	}
	_, err = exchangeTest.Orderbook(exchName, c, assetType)
	if err != nil {
		t.Fatal(err)
	}
}

func TestExchange_Pairs(t *testing.T) {
	t.Parallel()
	_, err := exchangeTest.Pairs(exchName, false, assetType)
	if err != nil {
		t.Fatal(err)
	}
	_, err = exchangeTest.Pairs(exchName, true, assetType)
	if err != nil {
		t.Fatal(err)
	}
}

func TestExchange_AccountInformation(t *testing.T) {
	if !configureExchangeKeys() {
		t.Skip("no exchange configured test skipped")
	}
	_, err := exchangeTest.AccountInformation(exchName)
	if err != nil {
		t.Fatal(err)
	}
}

func TestExchange_QueryOrder(t *testing.T) {
	if !configureExchangeKeys() {
		t.Skip("no exchange configured test skipped")
	}
	_, err := exchangeTest.QueryOrder(exchName, orderID)
	if err != nil {
		t.Fatal(err)
	}
}

func TestExchange_SubmitOrder(t *testing.T) {
	if !configureExchangeKeys() {
		t.Skip("no exchange configured test skipped")
	}

	c, err := currency.NewPairDelimiter(pairs, delimiter)
	if err != nil {
		t.Fatal(err)
	}
	tempOrder := &order.Submit{
		Pair:         c,
		Type:         orderType,
		Side:         orderSide,
		TriggerPrice: 0,
		TargetAmount: 0,
		Price:        orderPrice,
		Amount:       orderAmount,
		ClientID:     orderClientID,
		Exchange:     exchName,
	}
	_, err = exchangeTest.SubmitOrder(tempOrder)
	if err != nil {
		t.Fatal(err)
	}
}

func TestExchange_CancelOrder(t *testing.T) {
	if !configureExchangeKeys() {
		t.Skip("no exchange configured test skipped")
	}
	_, err := exchangeTest.CancelOrder(exchName, orderID)
	if err != nil {
		t.Fatal(err)
	}
}

func setupEngine() (err error) {
	engine.Bot, err = engine.NewFromSettings(&settings)
	if err != nil {
		return err
	}
	return engine.Bot.Start()
}

func cleanup() {
	err := os.RemoveAll(settings.DataDir)
	if err != nil {
		fmt.Printf("Clean up failed to remove file: %v manual removal may be required", err)
	}
}

func configureExchangeKeys() bool {
	ex := engine.GetExchangeByName(exchName).GetBase()
	ex.SetAPIKeys(exchAPIKEY, exchAPISECRET, exchClientID)
	ex.SkipAuthCheck = true
	return ex.ValidateAPICredentials()
}

//+build !mock_test_off

// This will build if build tag mock_test_off is not parsed and will try to mock
// all tests in _test.go
package localbitcoins

import (
	"log"
	"os"
	"testing"

	"github.com/yurulab/gocryptotrader/config"
	"github.com/yurulab/gocryptotrader/exchanges/mock"
	"github.com/yurulab/gocryptotrader/exchanges/sharedtestvalues"
)

const mockfile = "../../testdata/http_mock/localbitcoins/localbitcoins.json"

var mockTests = true

func TestMain(m *testing.M) {
	cfg := config.GetConfig()
	err := cfg.LoadConfig("../../testdata/configtest.json", true)
	if err != nil {
		log.Fatal("Localbitcoins load config error", err)
	}
	localbitcoinsConfig, err := cfg.GetExchangeConfig("LocalBitcoins")
	if err != nil {
		log.Fatal("Localbitcoins Setup() init error", err)
	}
	l.SkipAuthCheck = true
	localbitcoinsConfig.API.AuthenticatedSupport = true
	localbitcoinsConfig.API.Credentials.Key = apiKey
	localbitcoinsConfig.API.Credentials.Secret = apiSecret
	l.SetDefaults()
	err = l.Setup(localbitcoinsConfig)
	if err != nil {
		log.Fatal("Localbitcoins setup error", err)
	}

	serverDetails, newClient, err := mock.NewVCRServer(mockfile)
	if err != nil {
		log.Fatalf("Mock server error %s", err)
	}

	l.HTTPClient = newClient
	l.API.Endpoints.URL = serverDetails

	log.Printf(sharedtestvalues.MockTesting, l.Name, l.API.Endpoints.URL)
	os.Exit(m.Run())
}

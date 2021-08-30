//+build !mock_test_off

// This will build if build tag mock_test_off is not parsed and will try to mock
// all tests in _test.go
package gemini

import (
	"log"
	"os"
	"testing"

	"github.com/yurulab/gocryptotrader/config"
	"github.com/yurulab/gocryptotrader/exchanges/mock"
	"github.com/yurulab/gocryptotrader/exchanges/sharedtestvalues"
)

const mockFile = "../../testdata/http_mock/gemini/gemini.json"

var mockTests = true

func TestMain(m *testing.M) {
	cfg := config.GetConfig()
	err := cfg.LoadConfig("../../testdata/configtest.json", true)
	if err != nil {
		log.Fatal("Gemini load config error", err)
	}
	geminiConfig, err := cfg.GetExchangeConfig("Gemini")
	if err != nil {
		log.Fatal("Mock server error", err)
	}
	g.SkipAuthCheck = true
	geminiConfig.API.AuthenticatedSupport = true
	geminiConfig.API.Credentials.Key = apiKey
	geminiConfig.API.Credentials.Secret = apiSecret
	g.SetDefaults()
	g.Websocket = sharedtestvalues.NewTestWebsocket()
	err = g.Setup(geminiConfig)
	if err != nil {
		log.Fatal("Gemini setup error", err)
	}

	serverDetails, newClient, err := mock.NewVCRServer(mockFile)
	if err != nil {
		log.Fatalf("Mock server error %s", err)
	}

	g.HTTPClient = newClient
	g.API.Endpoints.URL = serverDetails
	log.Printf(sharedtestvalues.MockTesting, g.Name, g.API.Endpoints.URL)
	os.Exit(m.Run())
}

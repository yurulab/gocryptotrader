package engine

import (
	"net/http"

	"github.com/yurulab/gocryptotrader/exchanges/account"
	"github.com/yurulab/gocryptotrader/exchanges/orderbook"
	"github.com/yurulab/gocryptotrader/exchanges/ticker"
)

// Route is a sub type that holds the request routes
type Route struct {
	Name        string
	Method      string
	Pattern     string
	HandlerFunc http.HandlerFunc
}

// AllEnabledExchangeOrderbooks holds the enabled exchange orderbooks
type AllEnabledExchangeOrderbooks struct {
	Data []EnabledExchangeOrderbooks `json:"data"`
}

// EnabledExchangeOrderbooks is a sub type for singular exchanges and respective
// orderbooks
type EnabledExchangeOrderbooks struct {
	ExchangeName   string           `json:"exchangeName"`
	ExchangeValues []orderbook.Base `json:"exchangeValues"`
}

// AllEnabledExchangeCurrencies holds the enabled exchange currencies
type AllEnabledExchangeCurrencies struct {
	Data []EnabledExchangeCurrencies `json:"data"`
}

// EnabledExchangeCurrencies is a sub type for singular exchanges and respective
// currencies
type EnabledExchangeCurrencies struct {
	ExchangeName   string         `json:"exchangeName"`
	ExchangeValues []ticker.Price `json:"exchangeValues"`
}

// AllEnabledExchangeAccounts holds all enabled accounts info
type AllEnabledExchangeAccounts struct {
	Data []account.Holdings `json:"data"`
}

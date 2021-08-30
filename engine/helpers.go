package engine

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"errors"
	"fmt"
	"io/ioutil"
	"math/big"
	"net"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/pquerna/otp/totp"
	"github.com/yurulab/gocryptotrader/common"
	"github.com/yurulab/gocryptotrader/common/file"
	"github.com/yurulab/gocryptotrader/currency"
	"github.com/yurulab/gocryptotrader/dispatch"
	exchange "github.com/yurulab/gocryptotrader/exchanges"
	"github.com/yurulab/gocryptotrader/exchanges/account"
	"github.com/yurulab/gocryptotrader/exchanges/asset"
	"github.com/yurulab/gocryptotrader/exchanges/orderbook"
	"github.com/yurulab/gocryptotrader/exchanges/stats"
	"github.com/yurulab/gocryptotrader/exchanges/ticker"
	"github.com/yurulab/gocryptotrader/gctscript/vm"
	"github.com/yurulab/gocryptotrader/log"
	"github.com/yurulab/gocryptotrader/portfolio"
)

var (
	errCertExpired     = errors.New("gRPC TLS certificate has expired")
	errCertDataIsNil   = errors.New("gRPC TLS certificate PEM data is nil")
	errCertTypeInvalid = errors.New("gRPC TLS certificate type is invalid")
)

// GetSubsystemsStatus returns the status of various subsystems
func GetSubsystemsStatus() map[string]bool {
	systems := make(map[string]bool)
	systems["communications"] = Bot.CommsManager.Started()
	systems["internet_monitor"] = Bot.ConnectionManager.Started()
	systems["orders"] = Bot.OrderManager.Started()
	systems["portfolio"] = Bot.PortfolioManager.Started()
	systems["ntp_timekeeper"] = Bot.NTPManager.Started()
	systems["database"] = Bot.DatabaseManager.Started()
	systems["exchange_syncer"] = Bot.Settings.EnableExchangeSyncManager
	systems["grpc"] = Bot.Settings.EnableGRPC
	systems["grpc_proxy"] = Bot.Settings.EnableGRPCProxy
	systems["gctscript"] = Bot.GctScriptManager.Started()
	systems["deprecated_rpc"] = Bot.Settings.EnableDeprecatedRPC
	systems["websocket_rpc"] = Bot.Settings.EnableWebsocketRPC
	systems["dispatch"] = dispatch.IsRunning()
	return systems
}

// RPCEndpoint stores an RPC endpoint status and addr
type RPCEndpoint struct {
	Started    bool
	ListenAddr string
}

// GetRPCEndpoints returns a list of RPC endpoints and their listen addrs
func GetRPCEndpoints() map[string]RPCEndpoint {
	endpoints := make(map[string]RPCEndpoint)
	endpoints["grpc"] = RPCEndpoint{
		Started:    Bot.Settings.EnableGRPC,
		ListenAddr: "grpc://" + Bot.Config.RemoteControl.GRPC.ListenAddress,
	}
	endpoints["grpc_proxy"] = RPCEndpoint{
		Started:    Bot.Settings.EnableGRPCProxy,
		ListenAddr: "http://" + Bot.Config.RemoteControl.GRPC.GRPCProxyListenAddress,
	}
	endpoints["deprecated_rpc"] = RPCEndpoint{
		Started:    Bot.Settings.EnableDeprecatedRPC,
		ListenAddr: "http://" + Bot.Config.RemoteControl.DeprecatedRPC.ListenAddress,
	}
	endpoints["websocket_rpc"] = RPCEndpoint{
		Started:    Bot.Settings.EnableWebsocketRPC,
		ListenAddr: "ws://" + Bot.Config.RemoteControl.WebsocketRPC.ListenAddress,
	}
	return endpoints
}

// SetSubsystem enables or disables an engine subsystem
func SetSubsystem(subsys string, enable bool) error {
	switch strings.ToLower(subsys) {
	case "communications":
		if enable {
			return Bot.CommsManager.Start()
		}
		return Bot.CommsManager.Stop()
	case "internet_monitor":
		if enable {
			return Bot.ConnectionManager.Start()
		}
		return Bot.CommsManager.Stop()
	case "orders":
		if enable {
			return Bot.OrderManager.Start()
		}
		return Bot.OrderManager.Stop()
	case "portfolio":
		if enable {
			return Bot.PortfolioManager.Start()
		}
		return Bot.OrderManager.Stop()
	case "ntp_timekeeper":
		if enable {
			return Bot.NTPManager.Start()
		}
		return Bot.NTPManager.Stop()
	case "database":
		if enable {
			return Bot.DatabaseManager.Start()
		}
		return Bot.DatabaseManager.Stop()
	case "exchange_syncer":
		if enable {
			Bot.ExchangeCurrencyPairManager.Start()
		}
		Bot.ExchangeCurrencyPairManager.Stop()
	case "dispatch":
		if enable {
			return dispatch.Start(Bot.Settings.DispatchMaxWorkerAmount, Bot.Settings.DispatchJobsLimit)
		}
		return dispatch.Stop()
	case "gctscript":
		if enable {
			vm.GCTScriptConfig.Enabled = true
			return Bot.GctScriptManager.Start()
		}
		vm.GCTScriptConfig.Enabled = false
		return Bot.GctScriptManager.Stop()
	}

	return errors.New("subsystem not found")
}

// GetExchangeOTPs returns OTP codes for all exchanges which have a otpsecret
// stored
func GetExchangeOTPs() (map[string]string, error) {
	otpCodes := make(map[string]string)
	for x := range Bot.Config.Exchanges {
		if otpSecret := Bot.Config.Exchanges[x].API.Credentials.OTPSecret; otpSecret != "" {
			exchName := Bot.Config.Exchanges[x].Name
			o, err := totp.GenerateCode(otpSecret, time.Now())
			if err != nil {
				log.Errorf(log.Global, "Unable to generate OTP code for exchange %s. Err: %s\n",
					exchName, err)
				continue
			}
			otpCodes[exchName] = o
		}
	}

	if len(otpCodes) == 0 {
		return nil, errors.New("no exchanges found which have a OTP secret stored")
	}

	return otpCodes, nil
}

// GetExchangeoOTPByName returns a OTP code for the desired exchange
// if it exists
func GetExchangeoOTPByName(exchName string) (string, error) {
	for x := range Bot.Config.Exchanges {
		if !strings.EqualFold(Bot.Config.Exchanges[x].Name, exchName) {
			continue
		}

		if otpSecret := Bot.Config.Exchanges[x].API.Credentials.OTPSecret; otpSecret != "" {
			return totp.GenerateCode(otpSecret, time.Now())
		}
	}
	return "", errors.New("exchange does not have a OTP secret stored")
}

// GetAuthAPISupportedExchanges returns a list of auth api enabled exchanges
func GetAuthAPISupportedExchanges() []string {
	var exchangeNames []string
	exchanges := GetExchanges()
	for x := range exchanges {
		if !exchanges[x].GetAuthenticatedAPISupport(exchange.RestAuthentication) &&
			!exchanges[x].GetAuthenticatedAPISupport(exchange.WebsocketAuthentication) {
			continue
		}
		exchangeNames = append(exchangeNames, exchanges[x].GetName())
	}
	return exchangeNames
}

// IsOnline returns whether or not the engine has Internet connectivity
func IsOnline() bool {
	return Bot.ConnectionManager.IsOnline()
}

// GetAvailableExchanges returns a list of enabled exchanges
func GetAvailableExchanges() []string {
	var enExchanges []string
	for x := range Bot.Config.Exchanges {
		if Bot.Config.Exchanges[x].Enabled {
			enExchanges = append(enExchanges, Bot.Config.Exchanges[x].Name)
		}
	}
	return enExchanges
}

// GetAllAvailablePairs returns a list of all available pairs on either enabled
// or disabled exchanges
func GetAllAvailablePairs(enabledExchangesOnly bool, assetType asset.Item) currency.Pairs {
	var pairList currency.Pairs
	for x := range Bot.Config.Exchanges {
		if enabledExchangesOnly && !Bot.Config.Exchanges[x].Enabled {
			continue
		}

		exchName := Bot.Config.Exchanges[x].Name
		pairs, err := Bot.Config.GetAvailablePairs(exchName, assetType)
		if err != nil {
			continue
		}

		for y := range pairs {
			if pairList.Contains(pairs[y], false) {
				continue
			}
			pairList = append(pairList, pairs[y])
		}
	}
	return pairList
}

// GetSpecificAvailablePairs returns a list of supported pairs based on specific
// parameters
func GetSpecificAvailablePairs(enabledExchangesOnly, fiatPairs, includeUSDT, cryptoPairs bool, assetType asset.Item) currency.Pairs {
	var pairList currency.Pairs
	supportedPairs := GetAllAvailablePairs(enabledExchangesOnly, assetType)

	for x := range supportedPairs {
		if fiatPairs {
			if supportedPairs[x].IsCryptoFiatPair() &&
				!supportedPairs[x].ContainsCurrency(currency.USDT) ||
				(includeUSDT &&
					supportedPairs[x].ContainsCurrency(currency.USDT) &&
					supportedPairs[x].IsCryptoPair()) {
				if pairList.Contains(supportedPairs[x], false) {
					continue
				}
				pairList = append(pairList, supportedPairs[x])
			}
		}
		if cryptoPairs {
			if supportedPairs[x].IsCryptoPair() {
				if pairList.Contains(supportedPairs[x], false) {
					continue
				}
				pairList = append(pairList, supportedPairs[x])
			}
		}
	}
	return pairList
}

// IsRelatablePairs checks to see if the two pairs are relatable
func IsRelatablePairs(p1, p2 currency.Pair, includeUSDT bool) bool {
	if p1.EqualIncludeReciprocal(p2) {
		return true
	}

	var relatablePairs = GetRelatableCurrencies(p1, true, includeUSDT)
	if p1.IsCryptoFiatPair() {
		for x := range relatablePairs {
			relatablePairs = append(relatablePairs,
				GetRelatableFiatCurrencies(relatablePairs[x])...)
		}
	}
	return relatablePairs.Contains(p2, false)
}

// MapCurrenciesByExchange returns a list of currency pairs mapped to an
// exchange
func MapCurrenciesByExchange(p currency.Pairs, enabledExchangesOnly bool, assetType asset.Item) map[string]currency.Pairs {
	currencyExchange := make(map[string]currency.Pairs)
	for x := range p {
		for y := range Bot.Config.Exchanges {
			if enabledExchangesOnly && !Bot.Config.Exchanges[y].Enabled {
				continue
			}
			exchName := Bot.Config.Exchanges[y].Name
			if !Bot.Config.SupportsPair(exchName, p[x], assetType) {
				continue
			}

			result, ok := currencyExchange[exchName]
			if !ok {
				var pairs []currency.Pair
				pairs = append(pairs, p[x])
				currencyExchange[exchName] = pairs
			} else {
				if result.Contains(p[x], false) {
					continue
				}
				result = append(result, p[x])
				currencyExchange[exchName] = result
			}
		}
	}
	return currencyExchange
}

// GetExchangeNamesByCurrency returns a list of exchanges supporting
// a currency pair based on whether the exchange is enabled or not
func GetExchangeNamesByCurrency(p currency.Pair, enabled bool, assetType asset.Item) []string {
	var exchanges []string
	for x := range Bot.Config.Exchanges {
		if enabled != Bot.Config.Exchanges[x].Enabled {
			continue
		}

		exchName := Bot.Config.Exchanges[x].Name
		if !Bot.Config.SupportsPair(exchName, p, assetType) {
			continue
		}
		exchanges = append(exchanges, exchName)
	}
	return exchanges
}

// GetRelatableCryptocurrencies returns a list of currency pairs if it can find
// any relatable currencies (e.g ETHBTC -> ETHLTC -> ETHUSDT -> ETHREP)
func GetRelatableCryptocurrencies(p currency.Pair) currency.Pairs {
	var pairs currency.Pairs
	cryptocurrencies := currency.GetCryptocurrencies()

	for x := range cryptocurrencies {
		newPair := currency.NewPair(p.Base, cryptocurrencies[x])
		if newPair.IsInvalid() {
			continue
		}

		if newPair.Base.Upper() == p.Base.Upper() &&
			newPair.Quote.Upper() == p.Quote.Upper() {
			continue
		}

		if pairs.Contains(newPair, false) {
			continue
		}
		pairs = append(pairs, newPair)
	}
	return pairs
}

// GetRelatableFiatCurrencies returns a list of currency pairs if it can find
// any relatable currencies (e.g ETHUSD -> ETHAUD -> ETHGBP -> ETHJPY)
func GetRelatableFiatCurrencies(p currency.Pair) currency.Pairs {
	var pairs currency.Pairs
	fiatCurrencies := currency.GetFiatCurrencies()

	for x := range fiatCurrencies {
		newPair := currency.NewPair(p.Base, fiatCurrencies[x])
		if newPair.Base.Upper() == newPair.Quote.Upper() {
			continue
		}

		if newPair.Base.Upper() == p.Base.Upper() &&
			newPair.Quote.Upper() == p.Quote.Upper() {
			continue
		}

		if pairs.Contains(newPair, false) {
			continue
		}
		pairs = append(pairs, newPair)
	}
	return pairs
}

// GetRelatableCurrencies returns a list of currency pairs if it can find
// any relatable currencies (e.g BTCUSD -> BTC USDT -> XBT USDT -> XBT USD)
// incOrig includes the supplied pair if desired
func GetRelatableCurrencies(p currency.Pair, incOrig, incUSDT bool) currency.Pairs {
	var pairs currency.Pairs

	addPair := func(p currency.Pair) {
		if pairs.Contains(p, true) {
			return
		}
		pairs = append(pairs, p)
	}

	buildPairs := func(p currency.Pair, incOrig bool) {
		if incOrig {
			addPair(p)
		}

		first := currency.GetTranslation(p.Base)
		if first != p.Base {
			addPair(currency.NewPair(first, p.Quote))

			second := currency.GetTranslation(p.Quote)
			if second != p.Quote {
				addPair(currency.NewPair(first, second))
			}
		}

		second := currency.GetTranslation(p.Quote)
		if second != p.Quote {
			addPair(currency.NewPair(p.Base, second))
		}
	}

	buildPairs(p, incOrig)
	buildPairs(p.Swap(), incOrig)

	if !incUSDT {
		pairs = pairs.RemovePairsByFilter(currency.USDT)
	}

	return pairs
}

// GetSpecificOrderbook returns a specific orderbook given the currency,
// exchangeName and assetType
func GetSpecificOrderbook(p currency.Pair, exchangeName string, assetType asset.Item) (*orderbook.Base, error) {
	exch := GetExchangeByName(exchangeName)
	if exch == nil {
		return nil, ErrExchangeNotFound
	}
	return exch.FetchOrderbook(p, assetType)
}

// GetSpecificTicker returns a specific ticker given the currency,
// exchangeName and assetType
func GetSpecificTicker(p currency.Pair, exchangeName string, assetType asset.Item) (*ticker.Price, error) {
	exch := GetExchangeByName(exchangeName)
	if exch == nil {
		return nil, ErrExchangeNotFound
	}
	return exch.FetchTicker(p, assetType)
}

// GetCollatedExchangeAccountInfoByCoin collates individual exchange account
// information and turns into into a map string of
// exchange.AccountCurrencyInfo
func GetCollatedExchangeAccountInfoByCoin(accounts []account.Holdings) map[currency.Code]account.Balance {
	result := make(map[currency.Code]account.Balance)
	for x := range accounts {
		for y := range accounts[x].Accounts {
			for z := range accounts[x].Accounts[y].Currencies {
				currencyName := accounts[x].Accounts[y].Currencies[z].CurrencyName
				avail := accounts[x].Accounts[y].Currencies[z].TotalValue
				onHold := accounts[x].Accounts[y].Currencies[z].Hold
				info, ok := result[currencyName]
				if !ok {
					accountInfo := account.Balance{
						CurrencyName: currencyName,
						Hold:         onHold,
						TotalValue:   avail,
					}
					result[currencyName] = accountInfo
				} else {
					info.Hold += onHold
					info.TotalValue += avail
					result[currencyName] = info
				}
			}
		}
	}
	return result
}

// GetExchangeHighestPriceByCurrencyPair returns the exchange with the highest
// price for a given currency pair and asset type
func GetExchangeHighestPriceByCurrencyPair(p currency.Pair, a asset.Item) (string, error) {
	result := stats.SortExchangesByPrice(p, a, true)
	if len(result) == 0 {
		return "", fmt.Errorf("no stats for supplied currency pair and asset type")
	}

	return result[0].Exchange, nil
}

// GetExchangeLowestPriceByCurrencyPair returns the exchange with the lowest
// price for a given currency pair and asset type
func GetExchangeLowestPriceByCurrencyPair(p currency.Pair, assetType asset.Item) (string, error) {
	result := stats.SortExchangesByPrice(p, assetType, false)
	if len(result) == 0 {
		return "", fmt.Errorf("no stats for supplied currency pair and asset type")
	}

	return result[0].Exchange, nil
}

// SeedExchangeAccountInfo seeds account info
func SeedExchangeAccountInfo(accounts []account.Holdings) {
	if len(accounts) == 0 {
		return
	}

	port := portfolio.GetPortfolio()

	for x := range accounts {
		exchangeName := accounts[x].Exchange
		var currencies []account.Balance
		for y := range accounts[x].Accounts {
			for z := range accounts[x].Accounts[y].Currencies {
				var update bool
				for i := range currencies {
					if accounts[x].Accounts[y].Currencies[z].CurrencyName == currencies[i].CurrencyName {
						currencies[i].Hold += accounts[x].Accounts[y].Currencies[z].Hold
						currencies[i].TotalValue += accounts[x].Accounts[y].Currencies[z].TotalValue
						update = true
					}
				}

				if update {
					continue
				}

				currencies = append(currencies, account.Balance{
					CurrencyName: accounts[x].Accounts[y].Currencies[z].CurrencyName,
					TotalValue:   accounts[x].Accounts[y].Currencies[z].TotalValue,
					Hold:         accounts[x].Accounts[y].Currencies[z].Hold,
				})
			}
		}

		for x := range currencies {
			currencyName := currencies[x].CurrencyName
			total := currencies[x].TotalValue

			if !port.ExchangeAddressExists(exchangeName, currencyName) {
				if total <= 0 {
					continue
				}

				log.Debugf(log.PortfolioMgr, "Portfolio: Adding new exchange address: %s, %s, %f, %s\n",
					exchangeName,
					currencyName,
					total,
					portfolio.PortfolioAddressExchange)

				port.Addresses = append(
					port.Addresses,
					portfolio.Address{Address: exchangeName,
						CoinType:    currencyName,
						Balance:     total,
						Description: portfolio.PortfolioAddressExchange})
			} else {
				if total <= 0 {
					log.Debugf(log.PortfolioMgr, "Portfolio: Removing %s %s entry.\n",
						exchangeName,
						currencyName)
					port.RemoveExchangeAddress(exchangeName, currencyName)
				} else {
					balance, ok := port.GetAddressBalance(exchangeName,
						portfolio.PortfolioAddressExchange,
						currencyName)
					if !ok {
						continue
					}

					if balance != total {
						log.Debugf(log.PortfolioMgr, "Portfolio: Updating %s %s entry with balance %f.\n",
							exchangeName,
							currencyName,
							total)
						port.UpdateExchangeAddressBalance(exchangeName,
							currencyName,
							total)
					}
				}
			}
		}
	}
}

// GetCryptocurrenciesByExchange returns a list of cryptocurrencies the exchange supports
func GetCryptocurrenciesByExchange(exchangeName string, enabledExchangesOnly, enabledPairs bool, assetType asset.Item) ([]string, error) {
	var cryptocurrencies []string
	for x := range Bot.Config.Exchanges {
		if !strings.EqualFold(Bot.Config.Exchanges[x].Name, exchangeName) {
			continue
		}
		if enabledExchangesOnly && !Bot.Config.Exchanges[x].Enabled {
			continue
		}

		var err error
		var pairs []currency.Pair
		if enabledPairs {
			pairs, err = Bot.Config.GetEnabledPairs(exchangeName, assetType)
			if err != nil {
				return nil, err
			}
		} else {
			pairs, err = Bot.Config.GetAvailablePairs(exchangeName, assetType)
			if err != nil {
				return nil, err
			}
		}

		for y := range pairs {
			if pairs[y].Base.IsCryptocurrency() &&
				!common.StringDataCompareInsensitive(cryptocurrencies, pairs[y].Base.String()) {
				cryptocurrencies = append(cryptocurrencies, pairs[y].Base.String())
			}

			if pairs[y].Quote.IsCryptocurrency() &&
				!common.StringDataCompareInsensitive(cryptocurrencies, pairs[y].Quote.String()) {
				cryptocurrencies = append(cryptocurrencies, pairs[y].Quote.String())
			}
		}
	}
	return cryptocurrencies, nil
}

// GetCryptocurrencyDepositAddressesByExchange returns the cryptocurrency deposit addresses for a particular exchange
func GetCryptocurrencyDepositAddressesByExchange(exchName string) (map[string]string, error) {
	if Bot.DepositAddressManager != nil {
		return Bot.DepositAddressManager.GetDepositAddressesByExchange(exchName)
	}

	result := GetExchangeCryptocurrencyDepositAddresses()
	r, ok := result[exchName]
	if !ok {
		return nil, ErrExchangeNotFound
	}
	return r, nil
}

// GetExchangeCryptocurrencyDepositAddress returns the cryptocurrency deposit address for a particular
// exchange
func GetExchangeCryptocurrencyDepositAddress(exchName, accountID string, item currency.Code) (string, error) {
	if Bot.DepositAddressManager != nil {
		return Bot.DepositAddressManager.GetDepositAddressByExchange(exchName, item)
	}

	exch := GetExchangeByName(exchName)
	if exch == nil {
		return "", ErrExchangeNotFound
	}
	return exch.GetDepositAddress(item, accountID)
}

// GetExchangeCryptocurrencyDepositAddresses obtains an exchanges deposit cryptocurrency list
func GetExchangeCryptocurrencyDepositAddresses() map[string]map[string]string {
	result := make(map[string]map[string]string)
	exchanges := GetExchanges()
	for x := range exchanges {
		exchName := exchanges[x].GetName()
		if !exchanges[x].GetAuthenticatedAPISupport(exchange.RestAuthentication) {
			if Bot.Settings.Verbose {
				log.Debugf(log.ExchangeSys, "GetExchangeCryptocurrencyDepositAddresses: Skippping %s due to disabled authenticated API support.\n", exchName)
			}
			continue
		}

		cryptoCurrencies, err := GetCryptocurrenciesByExchange(exchName, true, true, asset.Spot)
		if err != nil {
			log.Debugf(log.ExchangeSys, "%s failed to get cryptocurrency deposit addresses. Err: %s\n", exchName, err)
			continue
		}

		cryptoAddr := make(map[string]string)
		for y := range cryptoCurrencies {
			cryptocurrency := cryptoCurrencies[y]
			depositAddr, err := exchanges[x].GetDepositAddress(currency.NewCode(cryptocurrency), "")
			if err != nil {
				log.Errorf(log.Global, "%s failed to get cryptocurrency deposit addresses. Err: %s\n", exchName, err)
				continue
			}
			cryptoAddr[cryptocurrency] = depositAddr
		}
		result[exchName] = cryptoAddr
	}
	return result
}

// FormatCurrency is a method that formats and returns a currency pair
// based on the user currency display preferences
func FormatCurrency(p currency.Pair) currency.Pair {
	return p.Format(Bot.Config.Currency.CurrencyPairFormat.Delimiter,
		Bot.Config.Currency.CurrencyPairFormat.Uppercase)
}

// GetExchangeNames returns a list of enabled or disabled exchanges
func GetExchangeNames(enabledOnly bool) []string {
	exchangeNames := GetAvailableExchanges()
	if enabledOnly {
		return exchangeNames
	}
	exchangeNames = append(exchangeNames, Bot.Config.GetDisabledExchanges()...)
	return exchangeNames
}

// GetAllActiveTickers returns all enabled exchange tickers
func GetAllActiveTickers() []EnabledExchangeCurrencies {
	var tickerData []EnabledExchangeCurrencies
	exchanges := GetExchanges()
	for x := range exchanges {
		assets := exchanges[x].GetAssetTypes()
		exchName := exchanges[x].GetName()
		var exchangeTicker EnabledExchangeCurrencies
		exchangeTicker.ExchangeName = exchName

		for y := range assets {
			currencies, err := exchanges[x].GetEnabledPairs(assets[y])
			if err != nil {
				log.Errorf(log.ExchangeSys,
					"Exchange %s could not retrieve enabled currencies. Err: %s\n",
					exchName,
					err)
				continue
			}
			for z := range currencies {
				tp, err := exchanges[x].FetchTicker(currencies[z], assets[y])
				if err != nil {
					log.Errorf(log.ExchangeSys, "Exchange %s failed to retrieve %s ticker. Err: %s\n", exchName,
						currencies[z].String(),
						err)
					continue
				}
				exchangeTicker.ExchangeValues = append(exchangeTicker.ExchangeValues, *tp)
			}
			tickerData = append(tickerData, exchangeTicker)
		}
	}
	return tickerData
}

// GetAllEnabledExchangeAccountInfo returns all the current enabled exchanges
func GetAllEnabledExchangeAccountInfo() AllEnabledExchangeAccounts {
	var response AllEnabledExchangeAccounts
	exchanges := GetExchanges()
	for x := range exchanges {
		if exchanges[x] != nil && exchanges[x].IsEnabled() {
			if !exchanges[x].GetAuthenticatedAPISupport(exchange.RestAuthentication) {
				if Bot.Settings.Verbose {
					log.Debugf(log.ExchangeSys,
						"GetAllEnabledExchangeAccountInfo: Skippping %s due to disabled authenticated API support.\n",
						exchanges[x].GetName())
				}
				continue
			}
			accountInfo, err := exchanges[x].FetchAccountInfo()
			if err != nil {
				log.Errorf(log.ExchangeSys,
					"Error encountered retrieving exchange account info for %s. Error %s\n",
					exchanges[x].GetName(),
					err)
				continue
			}
			response.Data = append(response.Data, accountInfo)
		}
	}
	return response
}

func verifyCert(pemData []byte) error {
	var pemBlock *pem.Block
	pemBlock, _ = pem.Decode(pemData)
	if pemBlock == nil {
		return errCertDataIsNil
	}

	if pemBlock.Type != "CERTIFICATE" {
		return errCertTypeInvalid
	}

	cert, err := x509.ParseCertificate(pemBlock.Bytes)
	if err != nil {
		return err
	}

	if time.Now().After(cert.NotAfter) {
		return errCertExpired
	}
	return nil
}

func checkCerts(certDir string) error {
	certFile := filepath.Join(certDir, "cert.pem")
	keyFile := filepath.Join(certDir, "key.pem")

	if !file.Exists(certFile) || !file.Exists(keyFile) {
		log.Warnln(log.Global, "gRPC certificate/key file missing, recreating...")
		return genCert(certDir)
	}

	pemData, err := ioutil.ReadFile(certFile)
	if err != nil {
		return fmt.Errorf("unable to open TLS cert file: %s", err)
	}

	if err = verifyCert(pemData); err != nil {
		if err != errCertExpired {
			return err
		}
		log.Warnln(log.Global, "gRPC certificate has expired, regenerating...")
		return genCert(certDir)
	}

	log.Infoln(log.Global, "gRPC TLS certificate and key files exist, will use them.")
	return nil
}

func genCert(targetDir string) error {
	privKey, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		return fmt.Errorf("failed to generate ecdsa private key: %s", err)
	}

	serialNumberLimit := new(big.Int).Lsh(big.NewInt(1), 128)
	serialNumber, err := rand.Int(rand.Reader, serialNumberLimit)
	if err != nil {
		return fmt.Errorf("failed to generate serial number: %s", err)
	}

	host, err := os.Hostname()
	if err != nil {
		return fmt.Errorf("failed to get hostname: %s", err)
	}

	dnsNames := []string{host}
	if host != "localhost" {
		dnsNames = append(dnsNames, "localhost")
	}

	template := x509.Certificate{
		SerialNumber: serialNumber,
		Subject: pkix.Name{
			Organization: []string{"gocryptotrader"},
			CommonName:   host,
		},
		NotBefore:             time.Now(),
		NotAfter:              time.Now().Add(time.Hour * 24 * 365),
		IsCA:                  true,
		BasicConstraintsValid: true,
		KeyUsage:              x509.KeyUsageKeyEncipherment | x509.KeyUsageDigitalSignature | x509.KeyUsageCertSign,
		ExtKeyUsage:           []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth},
		IPAddresses: []net.IP{
			net.ParseIP("127.0.0.1"),
			net.ParseIP("::1"),
		},
		DNSNames: dnsNames,
	}

	derBytes, err := x509.CreateCertificate(rand.Reader, &template, &template, &privKey.PublicKey, privKey)
	if err != nil {
		return fmt.Errorf("failed to create certificate: %s", err)
	}

	certData := pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: derBytes})
	if certData == nil {
		return fmt.Errorf("cert data is nil")
	}

	b, err := x509.MarshalECPrivateKey(privKey)
	if err != nil {
		return fmt.Errorf("failed to marshal ECDSA private key: %s", err)
	}

	keyData := pem.EncodeToMemory(&pem.Block{Type: "EC PRIVATE KEY", Bytes: b})
	if keyData == nil {
		return fmt.Errorf("key pem data is nil")
	}

	err = file.Write(filepath.Join(targetDir, "key.pem"), keyData)
	if err != nil {
		return fmt.Errorf("failed to write key.pem file %s", err)
	}

	err = file.Write(filepath.Join(targetDir, "cert.pem"), certData)
	if err != nil {
		return fmt.Errorf("failed to write cert.pem file %s", err)
	}

	log.Infof(log.Global, "gRPC TLS key.pem and cert.pem files written to %s\n", targetDir)
	return nil
}

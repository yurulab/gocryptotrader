package okcoin

import (
	"errors"
	"sync"
	"time"

	"github.com/yurulab/gocryptotrader/common"
	"github.com/yurulab/gocryptotrader/common/convert"
	"github.com/yurulab/gocryptotrader/config"
	"github.com/yurulab/gocryptotrader/currency"
	exchange "github.com/yurulab/gocryptotrader/exchanges"
	"github.com/yurulab/gocryptotrader/exchanges/asset"
	"github.com/yurulab/gocryptotrader/exchanges/kline"
	"github.com/yurulab/gocryptotrader/exchanges/okgroup"
	"github.com/yurulab/gocryptotrader/exchanges/protocol"
	"github.com/yurulab/gocryptotrader/exchanges/request"
	"github.com/yurulab/gocryptotrader/exchanges/stream"
	"github.com/yurulab/gocryptotrader/exchanges/ticker"
	"github.com/yurulab/gocryptotrader/log"
)

// GetDefaultConfig returns a default exchange config
func (o *OKCoin) GetDefaultConfig() (*config.ExchangeConfig, error) {
	o.SetDefaults()
	exchCfg := new(config.ExchangeConfig)
	exchCfg.Name = o.Name
	exchCfg.HTTPTimeout = exchange.DefaultHTTPTimeout
	exchCfg.BaseCurrencies = o.BaseCurrencies

	err := o.SetupDefaults(exchCfg)
	if err != nil {
		return nil, err
	}

	if o.Features.Supports.RESTCapabilities.AutoPairUpdates {
		err = o.UpdateTradablePairs(true)
		if err != nil {
			return nil, err
		}
	}

	return exchCfg, nil
}

// SetDefaults method assignes the default values for OKEX
func (o *OKCoin) SetDefaults() {
	o.SetErrorDefaults()
	o.SetCheckVarDefaults()
	o.Name = okCoinExchangeName
	o.Enabled = true
	o.Verbose = true

	o.API.CredentialsValidator.RequiresKey = true
	o.API.CredentialsValidator.RequiresSecret = true
	o.API.CredentialsValidator.RequiresClientID = true

	requestFmt := &currency.PairFormat{Uppercase: true, Delimiter: currency.DashDelimiter}
	configFmt := &currency.PairFormat{Uppercase: true, Delimiter: currency.DashDelimiter}
	o.SetGlobalPairsManager(requestFmt, configFmt, asset.Spot, asset.Margin)

	o.Features = exchange.Features{
		Supports: exchange.FeaturesSupported{
			REST:      true,
			Websocket: true,
			RESTCapabilities: protocol.Features{
				TickerBatching:      true,
				TickerFetching:      true,
				KlineFetching:       true,
				TradeFetching:       true,
				OrderbookFetching:   true,
				AutoPairUpdates:     true,
				AccountInfo:         true,
				GetOrder:            true,
				GetOrders:           true,
				CancelOrder:         true,
				CancelOrders:        true,
				SubmitOrder:         true,
				SubmitOrders:        true,
				DepositHistory:      true,
				WithdrawalHistory:   true,
				UserTradeHistory:    true,
				CryptoDeposit:       true,
				CryptoWithdrawal:    true,
				TradeFee:            true,
				CryptoWithdrawalFee: true,
			},
			WebsocketCapabilities: protocol.Features{
				TickerFetching:         true,
				TradeFetching:          true,
				KlineFetching:          true,
				OrderbookFetching:      true,
				Subscribe:              true,
				Unsubscribe:            true,
				AuthenticatedEndpoints: true,
				MessageCorrelation:     true,
				GetOrders:              true,
				GetOrder:               true,
				AccountBalance:         true,
			},
			WithdrawPermissions: exchange.AutoWithdrawCrypto |
				exchange.NoFiatWithdrawals,
			Kline: kline.ExchangeCapabilitiesSupported{
				DateRanges: true,
				Intervals:  true,
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
				ResultLimit: 1440,
			},
		},
	}

	o.Requester = request.New(o.Name,
		common.NewHTTPClientWithTimeout(exchange.DefaultHTTPTimeout),
		// TODO: Specify each individual endpoint rate limits as per docs
		request.WithLimiter(request.NewBasicRateLimit(okCoinRateInterval, okCoinStandardRequestRate)),
	)

	o.API.Endpoints.URLDefault = okCoinAPIURL
	o.API.Endpoints.URL = okCoinAPIURL
	o.API.Endpoints.WebsocketURL = okCoinWebsocketURL
	o.APIVersion = okCoinAPIVersion
	o.Websocket = stream.New()
	o.WebsocketResponseMaxLimit = exchange.DefaultWebsocketResponseMaxLimit
	o.WebsocketResponseCheckTimeout = exchange.DefaultWebsocketResponseCheckTimeout
	o.WebsocketOrderbookBufferLimit = exchange.DefaultWebsocketOrderbookBufferLimit
}

// Start starts the OKGroup go routine
func (o *OKCoin) Start(wg *sync.WaitGroup) {
	wg.Add(1)
	go func() {
		o.Run()
		wg.Done()
	}()
}

// Run implements the OKEX wrapper
func (o *OKCoin) Run() {
	if o.Verbose {
		log.Debugf(log.ExchangeSys,
			"%s Websocket: %s. (url: %s).\n",
			o.Name,
			common.IsEnabled(o.Websocket.IsEnabled()),
			o.WebsocketURL)
	}

	forceUpdate := false
	format, err := o.GetPairFormat(asset.Spot, false)
	if err != nil {
		log.Errorf(log.ExchangeSys,
			"%s failed to update currencies. Err: %s\n",
			o.Name,
			err)
		return
	}
	enabled, err := o.CurrencyPairs.GetPairs(asset.Spot, true)
	if err != nil {
		log.Errorf(log.ExchangeSys,
			"%s failed to update currencies. Err: %s\n",
			o.Name,
			err)
		return
	}

	avail, err := o.CurrencyPairs.GetPairs(asset.Spot, false)
	if err != nil {
		log.Errorf(log.ExchangeSys,
			"%s failed to update currencies. Err: %s\n",
			o.Name,
			err)
		return
	}

	if !common.StringDataContains(enabled.Strings(), format.Delimiter) ||
		!common.StringDataContains(avail.Strings(), format.Delimiter) {
		var p currency.Pairs
		p, err = currency.NewPairsFromStrings([]string{currency.BTC.String() +
			format.Delimiter +
			currency.USD.String()})
		if err != nil {
			log.Errorf(log.ExchangeSys,
				"%s failed to update currencies.\n",
				o.Name)
		} else {
			log.Warnf(log.ExchangeSys,
				"Enabled pairs for %v reset due to config upgrade, please enable the ones you would like again.\n",
				o.Name)
			forceUpdate = true

			err = o.UpdatePairs(p, asset.Spot, true, true)
			if err != nil {
				log.Errorf(log.ExchangeSys,
					"%s failed to update currencies. Err: %s\n",
					o.Name,
					err)
				return
			}
		}
	}

	if !o.GetEnabledFeatures().AutoPairUpdates && !forceUpdate {
		return
	}

	err = o.UpdateTradablePairs(forceUpdate)
	if err != nil {
		log.Errorf(log.ExchangeSys,
			"%s failed to update tradable pairs. Err: %s",
			o.Name,
			err)
	}
}

// FetchTradablePairs returns a list of the exchanges tradable pairs
func (o *OKCoin) FetchTradablePairs(asset asset.Item) ([]string, error) {
	prods, err := o.GetSpotTokenPairDetails()
	if err != nil {
		return nil, err
	}

	format, err := o.GetPairFormat(asset, false)
	if err != nil {
		return nil, err
	}

	var pairs []string
	for x := range prods {
		pairs = append(pairs, prods[x].BaseCurrency+
			format.Delimiter+
			prods[x].QuoteCurrency)
	}

	return pairs, nil
}

// UpdateTradablePairs updates the exchanges available pairs and stores
// them in the exchanges config
func (o *OKCoin) UpdateTradablePairs(forceUpdate bool) error {
	pairs, err := o.FetchTradablePairs(asset.Spot)
	if err != nil {
		return err
	}
	p, err := currency.NewPairsFromStrings(pairs)
	if err != nil {
		return err
	}
	return o.UpdatePairs(p, asset.Spot, false, forceUpdate)
}

// UpdateTicker updates and returns the ticker for a currency pair
func (o *OKCoin) UpdateTicker(p currency.Pair, assetType asset.Item) (*ticker.Price, error) {
	if assetType == asset.Spot {
		resp, err := o.GetSpotAllTokenPairsInformation()
		if err != nil {
			return nil, err
		}
		pairs, err := o.GetEnabledPairs(assetType)
		if err != nil {
			return nil, err
		}
		for i := range pairs {
			for j := range resp {
				if !pairs[i].Equal(resp[j].InstrumentID) {
					continue
				}

				err = ticker.ProcessTicker(&ticker.Price{
					Last:         resp[j].Last,
					High:         resp[j].High24h,
					Low:          resp[j].Low24h,
					Bid:          resp[j].BestBid,
					Ask:          resp[j].BestAsk,
					Volume:       resp[j].BaseVolume24h,
					QuoteVolume:  resp[j].QuoteVolume24h,
					Open:         resp[j].Open24h,
					Pair:         pairs[i],
					LastUpdated:  resp[j].Timestamp,
					ExchangeName: o.Name,
					AssetType:    assetType})
				if err != nil {
					return nil, err
				}
			}
		}
	}
	return ticker.GetTicker(o.Name, p, assetType)
}

// FetchTicker returns the ticker for a currency pair
func (o *OKCoin) FetchTicker(p currency.Pair, assetType asset.Item) (tickerData *ticker.Price, err error) {
	tickerData, err = ticker.GetTicker(o.Name, p, assetType)
	if err != nil {
		return o.UpdateTicker(p, assetType)
	}
	return
}

// GetHistoricCandles returns candles between a time period for a set time interval
func (o *OKCoin) GetHistoricCandles(pair currency.Pair, a asset.Item, start, end time.Time, interval kline.Interval) (kline.Item, error) {
	if !o.KlineIntervalEnabled(interval) {
		return kline.Item{}, kline.ErrorKline{
			Interval: interval,
		}
	}

	formattedPair, err := o.FormatExchangeCurrency(pair, a)
	if err != nil {
		return kline.Item{}, err
	}

	req := &okgroup.GetMarketDataRequest{
		Asset:        a,
		Start:        start.UTC().Format(time.RFC3339),
		End:          end.UTC().Format(time.RFC3339),
		Granularity:  o.FormatExchangeKlineInterval(interval),
		InstrumentID: formattedPair.String(),
	}

	candles, err := o.GetMarketData(req)
	if err != nil {
		return kline.Item{}, err
	}

	ret := kline.Item{
		Exchange: o.Name,
		Pair:     pair,
		Asset:    a,
		Interval: interval,
	}

	for x := range candles {
		t := candles[x].([]interface{})
		tempCandle := kline.Candle{}
		v, ok := t[0].(string)
		if !ok {
			return kline.Item{}, errors.New("unexpected value received")
		}
		tempCandle.Time, err = time.Parse(time.RFC3339, v)
		if err != nil {
			return kline.Item{}, err
		}
		tempCandle.Open, err = convert.FloatFromString(t[1])
		if err != nil {
			return kline.Item{}, err
		}
		tempCandle.High, err = convert.FloatFromString(t[2])
		if err != nil {
			return kline.Item{}, err
		}

		tempCandle.Low, err = convert.FloatFromString(t[3])
		if err != nil {
			return kline.Item{}, err
		}

		tempCandle.Close, err = convert.FloatFromString(t[4])
		if err != nil {
			return kline.Item{}, err
		}

		tempCandle.Volume, err = convert.FloatFromString(t[5])
		if err != nil {
			return kline.Item{}, err
		}
		ret.Candles = append(ret.Candles, tempCandle)
	}
	ret.SortCandlesByTimestamp(false)
	return ret, nil
}

// GetHistoricCandlesExtended returns candles between a time period for a set time interval
func (o *OKCoin) GetHistoricCandlesExtended(pair currency.Pair, a asset.Item, start, end time.Time, interval kline.Interval) (kline.Item, error) {
	if !o.KlineIntervalEnabled(interval) {
		return kline.Item{}, kline.ErrorKline{
			Interval: interval,
		}
	}

	ret := kline.Item{
		Exchange: o.Name,
		Pair:     pair,
		Asset:    a,
		Interval: interval,
	}

	dates := kline.CalcDateRanges(start, end, interval, o.Features.Enabled.Kline.ResultLimit)
	formattedPair, err := o.FormatExchangeCurrency(pair, a)
	if err != nil {
		return kline.Item{}, err
	}

	for x := range dates {
		req := &okgroup.GetMarketDataRequest{
			Asset:        a,
			Start:        dates[x].Start.UTC().Format(time.RFC3339),
			End:          dates[x].End.UTC().Format(time.RFC3339),
			Granularity:  o.FormatExchangeKlineInterval(interval),
			InstrumentID: formattedPair.String(),
		}

		candles, err := o.GetMarketData(req)
		if err != nil {
			return kline.Item{}, err
		}

		for i := range candles {
			t := candles[i].([]interface{})
			tempCandle := kline.Candle{}
			v, ok := t[0].(string)
			if !ok {
				return kline.Item{}, errors.New("unexpected value received")
			}
			tempCandle.Time, err = time.Parse(time.RFC3339, v)
			if err != nil {
				return kline.Item{}, err
			}
			tempCandle.Open, err = convert.FloatFromString(t[1])
			if err != nil {
				return kline.Item{}, err
			}
			tempCandle.High, err = convert.FloatFromString(t[2])
			if err != nil {
				return kline.Item{}, err
			}

			tempCandle.Low, err = convert.FloatFromString(t[3])
			if err != nil {
				return kline.Item{}, err
			}

			tempCandle.Close, err = convert.FloatFromString(t[4])
			if err != nil {
				return kline.Item{}, err
			}

			tempCandle.Volume, err = convert.FloatFromString(t[5])
			if err != nil {
				return kline.Item{}, err
			}
			ret.Candles = append(ret.Candles, tempCandle)
		}
	}

	ret.SortCandlesByTimestamp(false)
	return ret, nil
}

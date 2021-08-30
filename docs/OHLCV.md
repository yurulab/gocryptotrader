# GoCryptoTrader OHLCV support

<img src="https://github.com/yurulab/gocryptotrader/blob/master/web/src/assets/page-logo.png?raw=true" width="350px" height="350px" hspace="70">

[![Build Status](https://travis-ci.org/yurulab/gocryptotrader.svg?branch=master)](https://travis-ci.org/yurulab/gocryptotrader)
[![Software License](https://img.shields.io/badge/License-MIT-orange.svg?style=flat-square)](https://github.com/yurulab/gocryptotrader/blob/master/LICENSE)
[![GoDoc](https://godoc.org/github.com/yurulab/gocryptotrader?status.svg)](https://godoc.org/github.com/yurulab/gocryptotrader/exchanges)
[![Coverage Status](http://codecov.io/github/yurulab/gocryptotrader/coverage.svg?branch=master)](http://codecov.io/github/yurulab/gocryptotrader?branch=master)
[![Go Report Card](https://goreportcard.com/badge/github.com/yurulab/gocryptotrader)](https://goreportcard.com/report/github.com/yurulab/gocryptotrader)

This exchanges package is part of the GoCryptoTrader codebase.

## This is still in active development

You can track ideas, planned features and what's in progress on this Trello board: [https://trello.com/b/ZAhMhpOy/gocryptotrader](https://trello.com/b/ZAhMhpOy/gocryptotrader).

Join our slack to discuss all things related to GoCryptoTrader! [GoCryptoTrader Slack](https://join.slack.com/t/gocryptotrader/shared_invite/enQtNTQ5NDAxMjA2Mjc5LTc5ZDE1ZTNiOGM3ZGMyMmY1NTAxYWZhODE0MWM5N2JlZDk1NDU0YTViYzk4NTk3OTRiMDQzNGQ1YTc4YmRlMTk)

## Wrapper Methods

Candle retrieval is handled by two methods 


GetHistoricCandles which makes a single request to the exchange and follows all exchange limitations
```go
func (b *base) GetHistoricCandles(pair currency.Pair, a asset.Item, start, end time.Time, interval kline.Interval) (kline.Item, error) {
	return kline.Item{}, common.ErrFunctionNotSupported
}
```

GetHistoricCandlesExtended that will make multiple requests to an exchange if the requested periods are outside exchange limits
```go
func (b *base) GetHistoricCandlesExtended(pair currency.Pair, a asset.Item, start, end time.Time, interval kline.Interval) (kline.Item, error) {
	return kline.Item{}, common.ErrFunctionNotSupported
}
```

both methods return kline.Item{} 

```go
// Item holds all the relevant information for internal kline elements
type Item struct {
	Exchange string
	Pair     currency.Pair
	Asset    asset.Item
	Interval Interval
	Candles  []Candle
}

// Candle holds historic rate information.
type Candle struct {
	Time   time.Time
	Open   float64
	High   float64
	Low    float64
	Close  float64
	Volume float64
}
```

## Exchange status
| Exchange       | Supported   | 
|----------------|-------------|
| Binance        | Y           | 
| Bitfinex       | Y           | 
| Bitflyer       |             | 
| Bithumb        | Y           | 
| Bitmex         |             |        
| Bitstamp       | Y           | 
| BTC Markets    | Y           | 
| Bittrex        |             | 
| BTSE           |             |      
| Coinbase Pro   | Y           |
| Coinbene       | Y           | 
| Coinut         |             |         
| Exmo           |             |
| GateIO         | Y           |
| Gemini         |             |
| HitBTC         | Y           |     
| Huobi          | Y           |      
| FTX            | Y           | 
| itBIT          |             |          
| Kraken         | Y           |           
| LakeBTC        |             |           
| lBank          | Y           |          
| Localbitcoins  |             |          
| Okcoin         | Y           |           
| Okex           | Y           |    
| Poloniex       | Y           |          
| Yobit          |            |    
| ZB             | Y           |          

package kline

import (
	"math/rand"
	"strings"
	"testing"
	"time"

	"github.com/yurulab/gocryptotrader/common/crypto"
	"github.com/yurulab/gocryptotrader/currency"
	"github.com/yurulab/gocryptotrader/exchanges/asset"
	"github.com/yurulab/gocryptotrader/exchanges/order"
)

func TestValidateData(t *testing.T) {
	err := validateData(nil)
	if err == nil {
		t.Error("error cannot be nil")
	}

	var empty []order.TradeHistory
	err = validateData(empty)
	if err == nil {
		t.Error("error cannot be nil")
	}

	tn := time.Now()
	trade1 := []order.TradeHistory{
		{Timestamp: tn.Add(2 * time.Minute), TID: "2"},
		{Timestamp: tn.Add(time.Minute), TID: "1"},
		{Timestamp: tn.Add(3 * time.Minute), TID: "3"},
	}

	err = validateData(trade1)
	if err == nil {
		t.Error("error cannot be nil")
	}

	trade2 := []order.TradeHistory{
		{Timestamp: tn.Add(2 * time.Minute), TID: "2", Amount: 1, Price: 0},
	}

	err = validateData(trade2)
	if err == nil {
		t.Error("error cannot be nil")
	}

	trade3 := []order.TradeHistory{
		{TID: "2", Amount: 1, Price: 0},
	}

	err = validateData(trade3)
	if err == nil {
		t.Error("error cannot be nil")
	}

	trade4 := []order.TradeHistory{
		{Timestamp: tn.Add(2 * time.Minute), TID: "2", Amount: 1, Price: 1000},
		{Timestamp: tn.Add(time.Minute), TID: "1", Amount: 1, Price: 1001},
		{Timestamp: tn.Add(3 * time.Minute), TID: "3", Amount: 1, Price: 1001.5},
	}

	err = validateData(trade4)
	if err != nil {
		t.Error(err)
	}

	if trade4[0].TID != "1" || trade4[1].TID != "2" || trade4[2].TID != "3" {
		t.Error("trade history sorted incorrectly")
	}
}

func TestCreateKline(t *testing.T) {
	c, err := CreateKline(nil,
		OneMin,
		currency.NewPair(currency.BTC, currency.USD),
		asset.Spot,
		"Binance")
	if err == nil {
		t.Fatal("error cannot be nil")
	}

	var trades []order.TradeHistory
	rand.Seed(time.Now().Unix())
	for i := 0; i < 24000; i++ {
		trades = append(trades, order.TradeHistory{
			Timestamp: time.Now().Add((time.Duration(rand.Intn(10)) * time.Minute) +
				(time.Duration(rand.Intn(10)) * time.Second)),
			TID:    crypto.HexEncodeToString([]byte(string(i))),
			Amount: float64(rand.Intn(20)) + 1,
			Price:  1000 + float64(rand.Intn(1000)),
		})
	}

	c, err = CreateKline(trades,
		0,
		currency.NewPair(currency.BTC, currency.USD),
		asset.Spot,
		"Binance")
	if err == nil {
		t.Fatal("error cannot be nil")
	}

	c, err = CreateKline(trades,
		OneMin,
		currency.NewPair(currency.BTC, currency.USD),
		asset.Spot,
		"Binance")
	if err != nil {
		t.Fatal(err)
	}

	if len(c.Candles) == 0 {
		t.Fatal("no data returned, expecting a lot.")
	}
}

func TestKlineWord(t *testing.T) {
	if OneDay.Word() != "oneday" {
		t.Fatalf("unexpected result: %v", OneDay.Word())
	}
}

func TestKlineDuration(t *testing.T) {
	if OneDay.Duration() != time.Hour*24 {
		t.Fatalf("unexpected result: %v", OneDay.Duration())
	}
}

func TestKlineShort(t *testing.T) {
	if OneDay.Short() != "24h" {
		t.Fatalf("unexpected result: %v", OneDay.Short())
	}
}

func TestDurationToWord(t *testing.T) {
	testCases := []struct {
		name     string
		interval Interval
	}{
		{
			"FifteenSecond",
			FifteenSecond,
		},
		{
			"OneMin",
			OneMin,
		},
		{
			"ThreeMin",
			ThreeMin,
		},
		{
			"FiveMin",
			FiveMin,
		},
		{
			"TenMin",
			TenMin,
		},
		{
			"FifteenMin",
			FifteenMin,
		},
		{
			"ThirtyMin",
			ThirtyMin,
		},
		{
			"OneHour",
			OneHour,
		},
		{
			"TwoHour",
			TwoHour,
		},
		{
			"FourHour",
			FourHour,
		},
		{
			"SixHour",
			SixHour,
		},
		{
			"EightHour",
			OneHour * 8,
		},
		{
			"TwelveHour",
			TwelveHour,
		},
		{
			"OneDay",
			OneDay,
		},
		{
			"ThreeDay",
			ThreeDay,
		},
		{
			"FifteenDay",
			FifteenDay,
		},
		{
			"OneWeek",
			OneWeek,
		},
		{
			"TwoWeek",
			TwoWeek,
		},
		{
			"OneMonth",
			OneMonth,
		},
		{
			"OneYear",
			OneYear,
		},
		{
			"notfound",
			Interval(time.Hour * 1337),
		},
	}
	for x := range testCases {
		test := testCases[x]
		t.Run(test.name, func(t *testing.T) {
			v := durationToWord(test.interval)
			if !strings.EqualFold(v, test.name) {
				t.Fatalf("%v: received %v expected %v", test.name, v, test.name)
			}
		})
	}
}

func TestKlineErrors(t *testing.T) {
	v := ErrorKline{
		Interval: OneYear,
	}

	if v.Error() != "oneyear interval unsupported by exchange" {
		t.Fatal("unexpected error returned")
	}

	if v.Unwrap().Error() != "8760h0m0s interval unsupported by exchange" {
		t.Fatal("unexpected error returned")
	}
}

func TestTotalCandlesPerInterval(t *testing.T) {
	start := time.Date(2019, 1, 1, 0, 0, 0, 0, time.UTC)
	end := time.Date(2020, 1, 1, 0, 0, 0, 0, time.UTC)

	testCases := []struct {
		name     string
		interval Interval
		expected uint32
	}{
		{
			"FifteenSecond",
			FifteenSecond,
			2102400,
		},
		{
			"OneMin",
			OneMin,
			525600,
		},
		{
			"ThreeMin",
			ThreeMin,
			175200,
		},
		{
			"FiveMin",
			FiveMin,
			105120,
		},
		{
			"TenMin",
			TenMin,
			52560,
		},
		{
			"FifteenMin",
			FifteenMin,
			35040,
		},
		{
			"ThirtyMin",
			ThirtyMin,
			17520,
		},
		{
			"OneHour",
			OneHour,
			8760,
		},
		{
			"TwoHour",
			TwoHour,
			4380,
		},
		{
			"FourHour",
			FourHour,
			2190,
		},
		{
			"SixHour",
			SixHour,
			1460,
		},
		{
			"EightHour",
			OneHour * 8,
			1095,
		},
		{
			"TwelveHour",
			TwelveHour,
			730,
		},
		{
			"OneDay",
			OneDay,
			365,
		},
		{
			"ThreeDay",
			ThreeDay,
			121,
		},
		{
			"FifteenDay",
			FifteenDay,
			24,
		},
		{
			"OneWeek",
			OneWeek,
			52,
		},
		{
			"TwoWeek",
			TwoWeek,
			26,
		},
		{
			"OneMonth",
			OneMonth,
			12,
		},
		{
			"OneYear",
			OneYear,
			1,
		},
	}
	for x := range testCases {
		test := testCases[x]
		t.Run(test.name, func(t *testing.T) {
			v := TotalCandlesPerInterval(start, end, test.interval)
			if v != test.expected {
				t.Fatalf("%v: received %v expected %v", test.name, v, test.expected)
			}
		})
	}
}

func TestCalcDateRanges(t *testing.T) {
	start := time.Unix(1546300800, 0)
	end := time.Unix(1577836799, 0)

	v := CalcDateRanges(start, end, OneMin, 300)

	if v[0].Start.Unix() != time.Unix(1546300800, 0).Unix() {
		t.Fatalf("unexpected result received %v", v[0].Start.Unix())
	}

	v = CalcDateRanges(time.Now(), time.Now().AddDate(0, 0, 1), OneDay, 100)
	if len(v) != 1 {
		t.Fatal("expected CalcDateRanges() with a Candle count lower than limit to return 1 result")
	}
}

func TestItem_SortCandlesByTimestamp(t *testing.T) {
	var tempKline = Item{
		Exchange: "testExchange",
		Pair:     currency.NewPair(currency.BTC, currency.USDT),
		Asset:    asset.Spot,
		Interval: OneDay,
	}

	for x := 0; x < 100; x++ {
		y := rand.Float64()
		tempKline.Candles = append(tempKline.Candles,
			Candle{
				Time:   time.Now().AddDate(0, 0, -x),
				Open:   y,
				High:   y + float64(x),
				Low:    y - float64(x),
				Close:  y,
				Volume: y,
			})
	}

	tempKline.SortCandlesByTimestamp(false)
	if tempKline.Candles[0].Time.After(tempKline.Candles[1].Time) {
		t.Fatal("expected kline.Candles to be in descending order")
	}

	tempKline.SortCandlesByTimestamp(true)
	if tempKline.Candles[0].Time.Before(tempKline.Candles[1].Time) {
		t.Fatal("expected kline.Candles to be in ascending order")
	}
}

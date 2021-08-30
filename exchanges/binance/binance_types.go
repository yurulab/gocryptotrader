package binance

import (
	"time"

	"github.com/yurulab/gocryptotrader/currency"
)

// Response holds basic binance api response data
type Response struct {
	Code int    `json:"code"`
	Msg  string `json:"msg"`
}

// ExchangeInfo holds the full exchange information type
type ExchangeInfo struct {
	Code       int    `json:"code"`
	Msg        string `json:"msg"`
	Timezone   string `json:"timezone"`
	Servertime int64  `json:"serverTime"`
	RateLimits []struct {
		RateLimitType string `json:"rateLimitType"`
		Interval      string `json:"interval"`
		Limit         int    `json:"limit"`
	} `json:"rateLimits"`
	ExchangeFilters interface{} `json:"exchangeFilters"`
	Symbols         []struct {
		Symbol                     string   `json:"symbol"`
		Status                     string   `json:"status"`
		BaseAsset                  string   `json:"baseAsset"`
		BaseAssetPrecision         int      `json:"baseAssetPrecision"`
		QuoteAsset                 string   `json:"quoteAsset"`
		QuotePrecision             int      `json:"quotePrecision"`
		OrderTypes                 []string `json:"orderTypes"`
		IcebergAllowed             bool     `json:"icebergAllowed"`
		OCOAllowed                 bool     `json:"ocoAllowed"`
		QuoteOrderQtyMarketAllowed bool     `json:"quoteOrderQtyMarketAllowed"`
		IsSpotTradingAllowed       bool     `json:"isSpotTradingAllowed"`
		IsMarginTradingAllowed     bool     `json:"isMarginTradingAllowed"`
		Filters                    []struct {
			FilterType          string  `json:"filterType"`
			MinPrice            float64 `json:"minPrice,string"`
			MaxPrice            float64 `json:"maxPrice,string"`
			TickSize            float64 `json:"tickSize,string"`
			MultiplierUp        float64 `json:"multiplierUp,string"`
			MultiplierDown      float64 `json:"multiplierDown,string"`
			AvgPriceMins        int64   `json:"avgPriceMins"`
			MinQty              float64 `json:"minQty,string"`
			MaxQty              float64 `json:"maxQty,string"`
			StepSize            float64 `json:"stepSize,string"`
			MinNotional         float64 `json:"minNotional,string"`
			ApplyToMarket       bool    `json:"applyToMarket"`
			Limit               int64   `json:"limit"`
			MaxNumAlgoOrders    int64   `json:"maxNumAlgoOrders"`
			MaxNumIcebergOrders int64   `json:"maxNumIcebergOrders"`
		} `json:"filters"`
	} `json:"symbols"`
}

// OrderBookDataRequestParams represents Klines request data.
type OrderBookDataRequestParams struct {
	Symbol string `json:"symbol"` // Required field; example LTCBTC,BTCUSDT
	Limit  int    `json:"limit"`  // Default 100; max 1000. Valid limits:[5, 10, 20, 50, 100, 500, 1000]
}

// OrderbookItem stores an individual orderbook item
type OrderbookItem struct {
	Price    float64
	Quantity float64
}

// OrderBookData is resp data from orderbook endpoint
type OrderBookData struct {
	Code         int         `json:"code"`
	Msg          string      `json:"msg"`
	LastUpdateID int64       `json:"lastUpdateId"`
	Bids         [][2]string `json:"bids"`
	Asks         [][2]string `json:"asks"`
}

// OrderBook actual structured data that can be used for orderbook
type OrderBook struct {
	LastUpdateID int64
	Code         int
	Msg          string
	Bids         []OrderbookItem
	Asks         []OrderbookItem
}

// DepthUpdateParams is used as an embedded type for WebsocketDepthStream
type DepthUpdateParams []struct {
	PriceLevel float64
	Quantity   float64
	ingnore    []interface{}
}

// WebsocketDepthStream is the difference for the update depth stream
type WebsocketDepthStream struct {
	Event         string          `json:"e"`
	Timestamp     int64           `json:"E"`
	Pair          string          `json:"s"`
	FirstUpdateID int64           `json:"U"`
	LastUpdateID  int64           `json:"u"`
	UpdateBids    [][]interface{} `json:"b"`
	UpdateAsks    [][]interface{} `json:"a"`
}

// RecentTradeRequestParams represents Klines request data.
type RecentTradeRequestParams struct {
	Symbol string `json:"symbol"` // Required field. example LTCBTC, BTCUSDT
	Limit  int    `json:"limit"`  // Default 500; max 500.
}

// RecentTrade holds recent trade data
type RecentTrade struct {
	ID           int64   `json:"id"`
	Price        float64 `json:"price,string"`
	Quantity     float64 `json:"qty,string"`
	Time         float64 `json:"time"`
	IsBuyerMaker bool    `json:"isBuyerMaker"`
	IsBestMatch  bool    `json:"isBestMatch"`
}

// TradeStream holds the trade stream data
type TradeStream struct {
	EventType      string `json:"e"`
	EventTime      int64  `json:"E"`
	Symbol         string `json:"s"`
	TradeID        int64  `json:"t"`
	Price          string `json:"p"`
	Quantity       string `json:"q"`
	BuyerOrderID   int64  `json:"b"`
	SellerOrderID  int64  `json:"a"`
	TimeStamp      int64  `json:"T"`
	Maker          bool   `json:"m"`
	BestMatchPrice bool   `json:"M"`
}

// KlineStream holds the kline stream data
type KlineStream struct {
	EventType string `json:"e"`
	EventTime int64  `json:"E"`
	Symbol    string `json:"s"`
	Kline     struct {
		StartTime                int64   `json:"t"`
		CloseTime                int64   `json:"T"`
		Symbol                   string  `json:"s"`
		Interval                 string  `json:"i"`
		FirstTradeID             int64   `json:"f"`
		LastTradeID              int64   `json:"L"`
		OpenPrice                float64 `json:"o,string"`
		ClosePrice               float64 `json:"c,string"`
		HighPrice                float64 `json:"h,string"`
		LowPrice                 float64 `json:"l,string"`
		Volume                   float64 `json:"v,string"`
		NumberOfTrades           int64   `json:"n"`
		KlineClosed              bool    `json:"x"`
		Quote                    float64 `json:"q,string"`
		TakerBuyBaseAssetVolume  float64 `json:"V,string"`
		TakerBuyQuoteAssetVolume float64 `json:"Q,string"`
	} `json:"k"`
}

// TickerStream holds the ticker stream data
type TickerStream struct {
	EventType              string  `json:"e"`
	EventTime              int64   `json:"E"`
	Symbol                 string  `json:"s"`
	PriceChange            float64 `json:"p,string"`
	PriceChangePercent     float64 `json:"P,string"`
	WeightedAvgPrice       float64 `json:"w,string"`
	ClosePrice             float64 `json:"x,string"`
	LastPrice              float64 `json:"c,string"`
	LastPriceQuantity      float64 `json:"Q,string"`
	BestBidPrice           float64 `json:"b,string"`
	BestBidQuantity        float64 `json:"B,string"`
	BestAskPrice           float64 `json:"a,string"`
	BestAskQuantity        float64 `json:"A,string"`
	OpenPrice              float64 `json:"o,string"`
	HighPrice              float64 `json:"h,string"`
	LowPrice               float64 `json:"l,string"`
	TotalTradedVolume      float64 `json:"v,string"`
	TotalTradedQuoteVolume float64 `json:"q,string"`
	OpenTime               int64   `json:"O"`
	CloseTime              int64   `json:"C"`
	FirstTradeID           int64   `json:"F"`
	LastTradeID            int64   `json:"L"`
	NumberOfTrades         int64   `json:"n"`
}

// HistoricalTrade holds recent trade data
type HistoricalTrade struct {
	Code         int     `json:"code"`
	Msg          string  `json:"msg"`
	ID           int64   `json:"id"`
	Price        float64 `json:"price,string"`
	Quantity     float64 `json:"qty,string"`
	Time         int64   `json:"time"`
	IsBuyerMaker bool    `json:"isBuyerMaker"`
	IsBestMatch  bool    `json:"isBestMatch"`
}

// AggregatedTrade holds aggregated trade information
type AggregatedTrade struct {
	ATradeID       int64   `json:"a"`
	Price          float64 `json:"p,string"`
	Quantity       float64 `json:"q,string"`
	FirstTradeID   int64   `json:"f"`
	LastTradeID    int64   `json:"l"`
	TimeStamp      int64   `json:"T"`
	Maker          bool    `json:"m"`
	BestMatchPrice bool    `json:"M"`
}

// CandleStick holds kline data
type CandleStick struct {
	OpenTime                 time.Time
	Open                     float64
	High                     float64
	Low                      float64
	Close                    float64
	Volume                   float64
	CloseTime                time.Time
	QuoteAssetVolume         float64
	TradeCount               float64
	TakerBuyAssetVolume      float64
	TakerBuyQuoteAssetVolume float64
}

// AveragePrice holds current average symbol price
type AveragePrice struct {
	Mins  int64   `json:"mins"`
	Price float64 `json:"price,string"`
}

// PriceChangeStats contains statistics for the last 24 hours trade
type PriceChangeStats struct {
	Symbol             string  `json:"symbol"`
	PriceChange        float64 `json:"priceChange,string"`
	PriceChangePercent float64 `json:"priceChangePercent,string"`
	WeightedAvgPrice   float64 `json:"weightedAvgPrice,string"`
	PrevClosePrice     float64 `json:"prevClosePrice,string"`
	LastPrice          float64 `json:"lastPrice,string"`
	LastQty            float64 `json:"lastQty,string"`
	BidPrice           float64 `json:"bidPrice,string"`
	AskPrice           float64 `json:"askPrice,string"`
	OpenPrice          float64 `json:"openPrice,string"`
	HighPrice          float64 `json:"highPrice,string"`
	LowPrice           float64 `json:"lowPrice,string"`
	Volume             float64 `json:"volume,string"`
	QuoteVolume        float64 `json:"quoteVolume,string"`
	OpenTime           int64   `json:"openTime"`
	CloseTime          int64   `json:"closeTime"`
	FirstID            int64   `json:"firstId"`
	LastID             int64   `json:"lastId"`
	Count              int64   `json:"count"`
}

// SymbolPrice holds basic symbol price
type SymbolPrice struct {
	Symbol string  `json:"symbol"`
	Price  float64 `json:"price,string"`
}

// BestPrice holds best price data
type BestPrice struct {
	Symbol   string  `json:"symbol"`
	BidPrice float64 `json:"bidPrice,string"`
	BidQty   float64 `json:"bidQty,string"`
	AskPrice float64 `json:"askPrice,string"`
	AskQty   float64 `json:"askQty,string"`
}

// NewOrderRequest request type
type NewOrderRequest struct {
	// Symbol (currency pair to trade)
	Symbol string
	// Side Buy or Sell
	Side string
	// TradeType (market or limit order)
	TradeType RequestParamsOrderType
	// TimeInForce specifies how long the order remains in effect.
	// Examples are (Good Till Cancel (GTC), Immediate or Cancel (IOC) and Fill Or Kill (FOK))
	TimeInForce RequestParamsTimeForceType
	// Quantity is the total base qty spent or received in an order.
	Quantity float64
	// QuoteOrderQty is the total quote qty spent or received in a MARKET order.
	QuoteOrderQty    float64
	Price            float64
	NewClientOrderID string
	StopPrice        float64 // Used with STOP_LOSS, STOP_LOSS_LIMIT, TAKE_PROFIT, and TAKE_PROFIT_LIMIT orders.
	IcebergQty       float64 // Used with LIMIT, STOP_LOSS_LIMIT, and TAKE_PROFIT_LIMIT to create an iceberg order.
	NewOrderRespType string
}

// NewOrderResponse is the return structured response from the exchange
type NewOrderResponse struct {
	Code            int     `json:"code"`
	Msg             string  `json:"msg"`
	Symbol          string  `json:"symbol"`
	OrderID         int64   `json:"orderId"`
	ClientOrderID   string  `json:"clientOrderId"`
	TransactionTime int64   `json:"transactTime"`
	Price           float64 `json:"price,string"`
	OrigQty         float64 `json:"origQty,string"`
	ExecutedQty     float64 `json:"executedQty,string"`
	// The cumulative amount of the quote that has been spent (with a BUY order) or received (with a SELL order).
	CumulativeQuoteQty float64 `json:"cummulativeQuoteQty,string"`
	Status             string  `json:"status"`
	TimeInForce        string  `json:"timeInForce"`
	Type               string  `json:"type"`
	Side               string  `json:"side"`
	Fills              []struct {
		Price           float64 `json:"price,string"`
		Qty             float64 `json:"qty,string"`
		Commission      float64 `json:"commission,string"`
		CommissionAsset string  `json:"commissionAsset"`
	} `json:"fills"`
}

// CancelOrderResponse is the return structured response from the exchange
type CancelOrderResponse struct {
	Symbol            string `json:"symbol"`
	OrigClientOrderID string `json:"origClientOrderId"`
	OrderID           int64  `json:"orderId"`
	ClientOrderID     string `json:"clientOrderId"`
}

// QueryOrderData holds query order data
type QueryOrderData struct {
	Code          int     `json:"code"`
	Msg           string  `json:"msg"`
	Symbol        string  `json:"symbol"`
	OrderID       int64   `json:"orderId"`
	ClientOrderID string  `json:"clientOrderId"`
	Price         float64 `json:"price,string"`
	OrigQty       float64 `json:"origQty,string"`
	ExecutedQty   float64 `json:"executedQty,string"`
	Status        string  `json:"status"`
	TimeInForce   string  `json:"timeInForce"`
	Type          string  `json:"type"`
	Side          string  `json:"side"`
	StopPrice     float64 `json:"stopPrice,string"`
	IcebergQty    float64 `json:"icebergQty,string"`
	Time          float64 `json:"time"`
	IsWorking     bool    `json:"isWorking"`
}

// Balance holds query order data
type Balance struct {
	Asset  string `json:"asset"`
	Free   string `json:"free"`
	Locked string `json:"locked"`
}

// Account holds the account data
type Account struct {
	MakerCommission  int       `json:"makerCommission"`
	TakerCommission  int       `json:"takerCommission"`
	BuyerCommission  int       `json:"buyerCommission"`
	SellerCommission int       `json:"sellerCommission"`
	CanTrade         bool      `json:"canTrade"`
	CanWithdraw      bool      `json:"canWithdraw"`
	CanDeposit       bool      `json:"canDeposit"`
	UpdateTime       int64     `json:"updateTime"`
	Balances         []Balance `json:"balances"`
}

// RequestParamsTimeForceType Time in force
type RequestParamsTimeForceType string

var (
	// BinanceRequestParamsTimeGTC GTC
	BinanceRequestParamsTimeGTC = RequestParamsTimeForceType("GTC")

	// BinanceRequestParamsTimeIOC IOC
	BinanceRequestParamsTimeIOC = RequestParamsTimeForceType("IOC")

	// BinanceRequestParamsTimeFOK FOK
	BinanceRequestParamsTimeFOK = RequestParamsTimeForceType("FOK")
)

// RequestParamsOrderType trade order type
type RequestParamsOrderType string

var (
	// BinanceRequestParamsOrderLimit Limit order
	BinanceRequestParamsOrderLimit = RequestParamsOrderType("LIMIT")

	// BinanceRequestParamsOrderMarket Market order
	BinanceRequestParamsOrderMarket = RequestParamsOrderType("MARKET")

	// BinanceRequestParamsOrderStopLoss STOP_LOSS
	BinanceRequestParamsOrderStopLoss = RequestParamsOrderType("STOP_LOSS")

	// BinanceRequestParamsOrderStopLossLimit STOP_LOSS_LIMIT
	BinanceRequestParamsOrderStopLossLimit = RequestParamsOrderType("STOP_LOSS_LIMIT")

	// BinanceRequestParamsOrderTakeProfit TAKE_PROFIT
	BinanceRequestParamsOrderTakeProfit = RequestParamsOrderType("TAKE_PROFIT")

	// BinanceRequestParamsOrderTakeProfitLimit TAKE_PROFIT_LIMIT
	BinanceRequestParamsOrderTakeProfitLimit = RequestParamsOrderType("TAKE_PROFIT_LIMIT")

	// BinanceRequestParamsOrderLimitMarker LIMIT_MAKER
	BinanceRequestParamsOrderLimitMarker = RequestParamsOrderType("LIMIT_MAKER")
)

// KlinesRequestParams represents Klines request data.
type KlinesRequestParams struct {
	Symbol    string // Required field; example LTCBTC, BTCUSDT
	Interval  string // Time interval period
	Limit     int    // Default 500; max 500.
	StartTime int64
	EndTime   int64
}

// WithdrawalFees the large list of predefined withdrawal fees
// Prone to change
var WithdrawalFees = map[currency.Code]float64{
	currency.BNB:     0.13,
	currency.BTC:     0.0005,
	currency.NEO:     0,
	currency.ETH:     0.01,
	currency.LTC:     0.001,
	currency.QTUM:    0.01,
	currency.EOS:     0.1,
	currency.SNT:     35,
	currency.BNT:     1,
	currency.GAS:     0,
	currency.BCC:     0.001,
	currency.BTM:     5,
	currency.USDT:    3.4,
	currency.HCC:     0.0005,
	currency.OAX:     6.5,
	currency.DNT:     54,
	currency.MCO:     0.31,
	currency.ICN:     3.5,
	currency.ZRX:     1.9,
	currency.OMG:     0.4,
	currency.WTC:     0.5,
	currency.LRC:     12.3,
	currency.LLT:     67.8,
	currency.YOYO:    1,
	currency.TRX:     1,
	currency.STRAT:   0.1,
	currency.SNGLS:   54,
	currency.BQX:     3.9,
	currency.KNC:     3.5,
	currency.SNM:     25,
	currency.FUN:     86,
	currency.LINK:    4,
	currency.XVG:     0.1,
	currency.CTR:     35,
	currency.SALT:    2.3,
	currency.MDA:     2.3,
	currency.IOTA:    0.5,
	currency.SUB:     11.4,
	currency.ETC:     0.01,
	currency.MTL:     2,
	currency.MTH:     45,
	currency.ENG:     2.2,
	currency.AST:     14.4,
	currency.DASH:    0.002,
	currency.BTG:     0.001,
	currency.EVX:     2.8,
	currency.REQ:     29.9,
	currency.VIB:     30,
	currency.POWR:    8.2,
	currency.ARK:     0.2,
	currency.XRP:     0.25,
	currency.MOD:     2,
	currency.ENJ:     26,
	currency.STORJ:   5.1,
	currency.KMD:     0.002,
	currency.RCN:     47,
	currency.NULS:    0.01,
	currency.RDN:     2.5,
	currency.XMR:     0.04,
	currency.DLT:     19.8,
	currency.AMB:     8.9,
	currency.BAT:     8,
	currency.ZEC:     0.005,
	currency.BCPT:    14.5,
	currency.ARN:     3,
	currency.GVT:     0.13,
	currency.CDT:     81,
	currency.GXS:     0.3,
	currency.POE:     134,
	currency.QSP:     36,
	currency.BTS:     1,
	currency.XZC:     0.02,
	currency.LSK:     0.1,
	currency.TNT:     47,
	currency.FUEL:    79,
	currency.MANA:    18,
	currency.BCD:     0.01,
	currency.DGD:     0.04,
	currency.ADX:     6.3,
	currency.ADA:     1,
	currency.PPT:     0.41,
	currency.CMT:     12,
	currency.XLM:     0.01,
	currency.CND:     58,
	currency.LEND:    84,
	currency.WABI:    6.6,
	currency.SBTC:    0.0005,
	currency.BCX:     0.5,
	currency.WAVES:   0.002,
	currency.TNB:     139,
	currency.GTO:     20,
	currency.ICX:     0.02,
	currency.OST:     32,
	currency.ELF:     3.9,
	currency.AION:    3.2,
	currency.CVC:     10.9,
	currency.REP:     0.2,
	currency.GNT:     8.9,
	currency.DATA:    37,
	currency.ETF:     1,
	currency.BRD:     3.8,
	currency.NEBL:    0.01,
	currency.VIBE:    17.3,
	currency.LUN:     0.36,
	currency.CHAT:    60.7,
	currency.RLC:     3.4,
	currency.INS:     3.5,
	currency.IOST:    105.6,
	currency.STEEM:   0.01,
	currency.NANO:    0.01,
	currency.AE:      1.3,
	currency.VIA:     0.01,
	currency.BLZ:     10.3,
	currency.SYS:     1,
	currency.NCASH:   247.6,
	currency.POA:     0.01,
	currency.ONT:     1,
	currency.ZIL:     37.2,
	currency.STORM:   152,
	currency.XEM:     4,
	currency.WAN:     0.1,
	currency.WPR:     43.4,
	currency.QLC:     1,
	currency.GRS:     0.2,
	currency.CLOAK:   0.02,
	currency.LOOM:    11.9,
	currency.BCN:     1,
	currency.TUSD:    1.35,
	currency.ZEN:     0.002,
	currency.SKY:     0.01,
	currency.THETA:   24,
	currency.IOTX:    90.5,
	currency.QKC:     24.6,
	currency.AGI:     29.81,
	currency.NXS:     0.02,
	currency.SC:      0.1,
	currency.EON:     10,
	currency.NPXS:    897,
	currency.KEY:     223,
	currency.NAS:     0.1,
	currency.ADD:     100,
	currency.MEETONE: 300,
	currency.ATD:     100,
	currency.MFT:     175,
	currency.EOP:     5,
	currency.DENT:    596,
	currency.IQ:      50,
	currency.ARDR:    2,
	currency.HOT:     1210,
	currency.VET:     100,
	currency.DOCK:    68,
	currency.POLY:    7,
	currency.VTHO:    21,
	currency.ONG:     0.1,
	currency.PHX:     1,
	currency.HC:      0.005,
	currency.GO:      0.01,
	currency.PAX:     1.4,
	currency.EDO:     1.3,
	currency.WINGS:   8.9,
	currency.NAV:     0.2,
	currency.TRIG:    49.1,
	currency.APPC:    12.4,
	currency.PIVX:    0.02,
}

// WithdrawResponse contains status of withdrawal request
type WithdrawResponse struct {
	Success bool   `json:"success"`
	Msg     string `json:"msg"`
	ID      string `json:"id"`
}

// UserAccountStream contains a key to maintain an authorised
// websocket connection
type UserAccountStream struct {
	ListenKey string `json:"listenKey"`
}

type wsAccountInfo struct {
	Stream string `json:"stream"`
	Data   struct {
		CanDeposit       bool    `json:"D"`
		CanTrade         bool    `json:"T"`
		CanWithdraw      bool    `json:"W"`
		EventTime        int64   `json:"E"`
		LastUpdated      int64   `json:"u"`
		BuyerCommission  float64 `json:"b"`
		MakerCommission  float64 `json:"m"`
		SellerCommission float64 `json:"s"`
		TakerCommission  float64 `json:"t"`
		EventType        string  `json:"e"`
		Currencies       []struct {
			Asset     string  `json:"a"`
			Available float64 `json:"f,string"`
			Locked    float64 `json:"l,string"`
		} `json:"B"`
	} `json:"data"`
}

type wsAccountPosition struct {
	Stream string `json:"stream"`
	Data   struct {
		Currencies []struct {
			Asset     string  `json:"a"`
			Available float64 `json:"f,string"`
			Locked    float64 `json:"l,string"`
		} `json:"B"`
		EventTime   int64  `json:"E"`
		LastUpdated int64  `json:"u"`
		EventType   string `json:"e"`
	} `json:"data"`
}

type wsBalanceUpdate struct {
	Stream string `json:"stream"`
	Data   struct {
		EventTime    int64   `json:"E"`
		ClearTime    int64   `json:"T"`
		BalanceDelta float64 `json:"d,string"`
		Asset        string  `json:"a"`
		EventType    string  `json:"e"`
	} `json:"data"`
}

type wsOrderUpdate struct {
	Stream string `json:"stream"`
	Data   struct {
		ClientOrderID                     string  `json:"C"`
		EventTime                         int64   `json:"E"`
		IcebergQuantity                   float64 `json:"F,string"`
		LastExecutedPrice                 float64 `json:"L,string"`
		CommissionAsset                   float64 `json:"N"`
		OrderCreationTime                 int64   `json:"O"`
		StopPrice                         float64 `json:"P,string"`
		QuoteOrderQuantity                float64 `json:"Q,string"`
		Side                              string  `json:"S"`
		TransactionTime                   int64   `json:"T"`
		OrderStatus                       string  `json:"X"`
		LastQuoteAssetTransactedQuantity  float64 `json:"Y,string"`
		CumulativeQuoteTransactedQuantity float64 `json:"Z,string"`
		CancelledClientOrderID            string  `json:"c"`
		EventType                         string  `json:"e"`
		TimeInForce                       string  `json:"f"`
		OrderListID                       int64   `json:"g"`
		OrderID                           int64   `json:"i"`
		LastExecutedQuantity              float64 `json:"l,string"`
		IsMaker                           bool    `json:"m"`
		Commission                        float64 `json:"n,string"`
		OrderType                         string  `json:"o"`
		Price                             float64 `json:"p,string"`
		Quantity                          float64 `json:"q,string"`
		RejectionReason                   string  `json:"r"`
		Symbol                            string  `json:"s"`
		TradeID                           int64   `json:"t"`
		IsOnOrderBook                     bool    `json:"w"`
		CurrentExecutionType              string  `json:"x"`
		CumulativeFilledQuantity          float64 `json:"z,string"`
	} `json:"data"`
}

type wsListStatus struct {
	Stream string `json:"stream"`
	Data   struct {
		ListClientOrderID string `json:"C"`
		EventTime         int64  `json:"E"`
		ListOrderStatus   string `json:"L"`
		Orders            []struct {
			ClientOrderID string `json:"c"`
			OrderID       int64  `json:"i"`
			Symbol        string `json:"s"`
		} `json:"O"`
		TransactionTime int64  `json:"T"`
		ContingencyType string `json:"c"`
		EventType       string `json:"e"`
		OrderListID     int64  `json:"g"`
		ListStatusType  string `json:"l"`
		RejectionReason string `json:"r"`
		Symbol          string `json:"s"`
	} `json:"data"`
}

// WsPayload defines the payload through the websocket connection
type WsPayload struct {
	Method string   `json:"method"`
	Params []string `json:"params"`
	ID     int64    `json:"id"`
}

package zb

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/gorilla/websocket"
	"github.com/yurulab/gocryptotrader/common"
	"github.com/yurulab/gocryptotrader/common/crypto"
	"github.com/yurulab/gocryptotrader/currency"
	exchange "github.com/yurulab/gocryptotrader/exchanges"
	"github.com/yurulab/gocryptotrader/exchanges/asset"
	"github.com/yurulab/gocryptotrader/exchanges/order"
	"github.com/yurulab/gocryptotrader/exchanges/orderbook"
	"github.com/yurulab/gocryptotrader/exchanges/stream"
	"github.com/yurulab/gocryptotrader/exchanges/ticker"
	"github.com/yurulab/gocryptotrader/log"
)

const (
	zbWebsocketAPI       = "wss://api.zb.live/websocket"
	zWebsocketAddChannel = "addChannel"
	zbWebsocketRateLimit = 20
)

// WsConnect initiates a websocket connection
func (z *ZB) WsConnect() error {
	if !z.Websocket.IsEnabled() || !z.IsEnabled() {
		return errors.New(stream.WebsocketNotEnabled)
	}
	var dialer websocket.Dialer
	err := z.Websocket.Conn.Dial(&dialer, http.Header{})
	if err != nil {
		return err
	}

	subs, err := z.GenerateDefaultSubscriptions()
	if err != nil {
		return err
	}
	go z.wsReadData()
	return z.Websocket.SubscribeToChannels(subs)
}

// wsReadData handles all the websocket data coming from the websocket
// connection
func (z *ZB) wsReadData() {
	z.Websocket.Wg.Add(1)
	defer z.Websocket.Wg.Done()

	for {
		resp := z.Websocket.Conn.ReadMessage()
		if resp.Raw == nil {
			return
		}
		err := z.wsHandleData(resp.Raw)
		if err != nil {
			z.Websocket.DataHandler <- err
		}
	}
}

func (z *ZB) wsHandleData(respRaw []byte) error {
	fixedJSON := z.wsFixInvalidJSON(respRaw)
	var result Generic
	err := json.Unmarshal(fixedJSON, &result)
	if err != nil {
		return err
	}
	if result.No > 0 {
		if z.Websocket.Match.IncomingWithData(result.No, fixedJSON) {
			return nil
		}
	}
	if result.Code > 0 && result.Code != 1000 {
		return fmt.Errorf("%v request failed, message: %v, error code: %v",
			z.Name,
			result.Message,
			wsErrCodes[result.Code])
	}

	switch {
	case strings.Contains(result.Channel, "markets"):
		var markets Markets
		err := json.Unmarshal(result.Data, &markets)
		if err != nil {
			return err
		}
	case strings.Contains(result.Channel, "ticker"):
		cPair := strings.Split(result.Channel, "_")
		var wsTicker WsTicker
		err := json.Unmarshal(fixedJSON, &wsTicker)
		if err != nil {
			return err
		}

		p, err := currency.NewPairFromString(cPair[0])
		if err != nil {
			return err
		}

		z.Websocket.DataHandler <- &ticker.Price{
			ExchangeName: z.Name,
			Close:        wsTicker.Data.Last,
			Volume:       wsTicker.Data.Volume24Hr,
			High:         wsTicker.Data.High,
			Low:          wsTicker.Data.Low,
			Last:         wsTicker.Data.Last,
			Bid:          wsTicker.Data.Buy,
			Ask:          wsTicker.Data.Sell,
			LastUpdated:  time.Unix(0, wsTicker.Date*int64(time.Millisecond)),
			AssetType:    asset.Spot,
			Pair:         p,
		}
	case strings.Contains(result.Channel, "depth"):
		var depth WsDepth
		err := json.Unmarshal(fixedJSON, &depth)
		if err != nil {
			return err
		}

		var asks []orderbook.Item
		for i := range depth.Asks {
			asks = append(asks, orderbook.Item{
				Amount: depth.Asks[i][1].(float64),
				Price:  depth.Asks[i][0].(float64),
			})
		}

		var bids []orderbook.Item
		for i := range depth.Bids {
			bids = append(bids, orderbook.Item{
				Amount: depth.Bids[i][1].(float64),
				Price:  depth.Bids[i][0].(float64),
			})
		}

		channelInfo := strings.Split(result.Channel, "_")
		cPair, err := currency.NewPairFromString(channelInfo[0])
		if err != nil {
			return err
		}

		var newOrderBook orderbook.Base
		newOrderBook.Asks = asks
		newOrderBook.Bids = bids
		newOrderBook.AssetType = asset.Spot
		newOrderBook.Pair = cPair
		newOrderBook.ExchangeName = z.Name

		err = z.Websocket.Orderbook.LoadSnapshot(&newOrderBook)
		if err != nil {
			return err
		}
	case strings.Contains(result.Channel, "_order"):
		cPair := strings.Split(result.Channel, "_")
		var o WsSubmitOrderResponse
		err := json.Unmarshal(fixedJSON, &o)
		if err != nil {
			return err
		}
		if !o.Success {
			return fmt.Errorf("%s - Order %v failed to be placed. %s",
				z.Name,
				o.Data.EntrustID,
				respRaw)
		}

		p, err := currency.NewPairFromString(cPair[0])
		if err != nil {
			return err
		}

		var a asset.Item
		a, err = z.GetPairAssetType(p)
		if err != nil {
			return err
		}
		z.Websocket.DataHandler <- &order.Detail{
			Exchange:  z.Name,
			ID:        strconv.FormatInt(o.Data.EntrustID, 10),
			Pair:      p,
			AssetType: a,
		}
	case strings.Contains(result.Channel, "_cancelorder"):
		cPair := strings.Split(result.Channel, "_")
		var o WsSubmitOrderResponse
		err := json.Unmarshal(fixedJSON, &o)
		if err != nil {
			return err
		}
		if !o.Success {
			return fmt.Errorf("%s - Order %v failed to be cancelled. %s",
				z.Name,
				o.Data.EntrustID,
				respRaw)
		}

		p, err := currency.NewPairFromString(cPair[0])
		if err != nil {
			return err
		}

		z.Websocket.DataHandler <- &order.Modify{
			Exchange: z.Name,
			ID:       strconv.FormatInt(o.Data.EntrustID, 10),
			Pair:     p,
			Status:   order.Cancelled,
		}
	case strings.Contains(result.Channel, "trades"):
		var trades WsTrades
		err := json.Unmarshal(fixedJSON, &trades)
		if err != nil {
			return err
		}

		for i := range trades.Data {
			channelInfo := strings.Split(result.Channel, "_")
			cPair, err := currency.NewPairFromString(channelInfo[0])
			if err != nil {
				return err
			}

			tSide, err := order.StringToOrderSide(trades.Data[i].TradeType)
			if err != nil {
				z.Websocket.DataHandler <- order.ClassificationError{
					Exchange: z.Name,
					Err:      err,
				}
			}

			z.Websocket.DataHandler <- stream.TradeData{
				Timestamp:    time.Unix(trades.Data[i].Date, 0),
				CurrencyPair: cPair,
				AssetType:    asset.Spot,
				Exchange:     z.Name,
				Price:        trades.Data[i].Price,
				Amount:       trades.Data[i].Amount,
				Side:         tSide,
			}
		}
	default:
		z.Websocket.DataHandler <- stream.UnhandledMessageWarning{
			Message: z.Name +
				stream.UnhandledMessage +
				string(respRaw)}
	}
	return nil
}

// GenerateDefaultSubscriptions Adds default subscriptions to websocket to be handled by ManageSubscriptions()
func (z *ZB) GenerateDefaultSubscriptions() ([]stream.ChannelSubscription, error) {
	var subscriptions []stream.ChannelSubscription
	// market configuration is its own channel
	subscriptions = append(subscriptions, stream.ChannelSubscription{
		Channel: "markets",
	})
	channels := []string{"%s_ticker", "%s_depth", "%s_trades"}
	enabledCurrencies, err := z.GetEnabledPairs(asset.Spot)
	if err != nil {
		return nil, err
	}

	for i := range channels {
		for j := range enabledCurrencies {
			enabledCurrencies[j].Delimiter = ""
			subscriptions = append(subscriptions, stream.ChannelSubscription{
				Channel:  fmt.Sprintf(channels[i], enabledCurrencies[j].Lower().String()),
				Currency: enabledCurrencies[j].Lower(),
				Asset:    asset.Spot,
			})
		}
	}
	return subscriptions, nil
}

// Subscribe sends a websocket message to receive data from the channel
func (z *ZB) Subscribe(channelsToSubscribe []stream.ChannelSubscription) error {
	var errs common.Errors
	for i := range channelsToSubscribe {
		subscriptionRequest := Subscription{
			Event:   zWebsocketAddChannel,
			Channel: channelsToSubscribe[i].Channel,
		}
		err := z.Websocket.Conn.SendJSONMessage(subscriptionRequest)
		if err != nil {
			errs = append(errs, err)
			continue
		}
		z.Websocket.AddSuccessfulSubscriptions(channelsToSubscribe[i])
	}
	if errs != nil {
		return errs
	}
	return nil
}

func (z *ZB) wsGenerateSignature(request interface{}) string {
	jsonResponse, err := json.Marshal(request)
	if err != nil {
		log.Error(log.ExchangeSys, err)
		return ""
	}
	hmac := crypto.GetHMAC(crypto.HashMD5,
		jsonResponse,
		[]byte(crypto.Sha1ToHex(z.API.Credentials.Secret)))
	return fmt.Sprintf("%x", hmac)
}

func (z *ZB) wsFixInvalidJSON(json []byte) []byte {
	invalidZbJSONRegex := `(\"\[|\"\{)(.*)(\]\"|\}\")`
	regexChecker := regexp.MustCompile(invalidZbJSONRegex)
	matchingResults := regexChecker.Find(json)
	if matchingResults == nil {
		return json
	}
	// Remove first quote character
	capturedInvalidZBJSON := strings.Replace(string(matchingResults), "\"", "", 1)
	// Remove last quote character
	fixedJSON := capturedInvalidZBJSON[:len(capturedInvalidZBJSON)-1]
	return []byte(strings.Replace(string(json), string(matchingResults), fixedJSON, 1))
}

func (z *ZB) wsAddSubUser(username, password string) (*WsGetSubUserListResponse, error) {
	if !z.GetAuthenticatedAPISupport(exchange.WebsocketAuthentication) {
		return nil, fmt.Errorf("%v AuthenticatedWebsocketAPISupport not enabled", z.Name)
	}
	request := WsAddSubUserRequest{
		Memo:        "memo",
		Password:    password,
		SubUserName: username,
	}
	request.Channel = "addSubUser"
	request.Event = zWebsocketAddChannel
	request.Accesskey = z.API.Credentials.Key
	request.No = z.Websocket.Conn.GenerateMessageID(true)
	request.Sign = z.wsGenerateSignature(request)
	resp, err := z.Websocket.Conn.SendMessageReturnResponse(request.No, request)
	if err != nil {
		return nil, err
	}
	var genericResponse Generic
	err = json.Unmarshal(resp, &genericResponse)
	if err != nil {
		return nil, err
	}
	if genericResponse.Code > 0 && genericResponse.Code != 1000 {
		return nil,
			fmt.Errorf("%v request failed, message: %v, error code: %v",
				z.Name,
				genericResponse.Message,
				wsErrCodes[genericResponse.Code])
	}
	var response WsGetSubUserListResponse
	err = json.Unmarshal(resp, &response)
	if err != nil {
		return nil, err
	}
	return &response, nil
}

func (z *ZB) wsGetSubUserList() (*WsGetSubUserListResponse, error) {
	if !z.GetAuthenticatedAPISupport(exchange.WebsocketAuthentication) {
		return nil,
			fmt.Errorf("%v AuthenticatedWebsocketAPISupport not enabled", z.Name)
	}
	request := WsAuthenticatedRequest{}
	request.Channel = "getSubUserList"
	request.Event = zWebsocketAddChannel
	request.Accesskey = z.API.Credentials.Key
	request.No = z.Websocket.Conn.GenerateMessageID(true)
	request.Sign = z.wsGenerateSignature(request)

	resp, err := z.Websocket.Conn.SendMessageReturnResponse(request.No, request)
	if err != nil {
		return nil, err
	}
	var response WsGetSubUserListResponse
	err = json.Unmarshal(resp, &response)
	if err != nil {
		return nil, err
	}
	if response.Code > 0 && response.Code != 1000 {
		return &response,
			fmt.Errorf("%v request failed, message: %v, error code: %v",
				z.Name,
				response.Message,
				wsErrCodes[response.Code])
	}
	return &response, nil
}

func (z *ZB) wsDoTransferFunds(pair currency.Code, amount float64, fromUserName, toUserName string) (*WsRequestResponse, error) {
	if !z.GetAuthenticatedAPISupport(exchange.WebsocketAuthentication) {
		return nil,
			fmt.Errorf("%v AuthenticatedWebsocketAPISupport not enabled", z.Name)
	}
	request := WsDoTransferFundsRequest{
		Amount:       amount,
		Currency:     pair,
		FromUserName: fromUserName,
		ToUserName:   toUserName,
		No:           z.Websocket.Conn.GenerateMessageID(true),
	}
	request.Channel = "doTransferFunds"
	request.Event = zWebsocketAddChannel
	request.Accesskey = z.API.Credentials.Key
	request.Sign = z.wsGenerateSignature(request)

	resp, err := z.Websocket.Conn.SendMessageReturnResponse(request.No, request)
	if err != nil {
		return nil, err
	}
	var response WsRequestResponse
	err = json.Unmarshal(resp, &response)
	if err != nil {
		return nil, err
	}
	if response.Code > 0 && response.Code != 1000 {
		return &response,
			fmt.Errorf("%v request failed, message: %v, error code: %v",
				z.Name,
				response.Message,
				wsErrCodes[response.Code])
	}
	return &response, nil
}

func (z *ZB) wsCreateSubUserKey(assetPerm, entrustPerm, leverPerm, moneyPerm bool, keyName, toUserID string) (*WsRequestResponse, error) {
	if !z.GetAuthenticatedAPISupport(exchange.WebsocketAuthentication) {
		return nil,
			fmt.Errorf("%v AuthenticatedWebsocketAPISupport not enabled", z.Name)
	}
	request := WsCreateSubUserKeyRequest{
		AssetPerm:   assetPerm,
		EntrustPerm: entrustPerm,
		KeyName:     keyName,
		LeverPerm:   leverPerm,
		MoneyPerm:   moneyPerm,
		No:          z.Websocket.Conn.GenerateMessageID(true),
		ToUserID:    toUserID,
	}
	request.Channel = "createSubUserKey"
	request.Event = zWebsocketAddChannel
	request.Accesskey = z.API.Credentials.Key
	request.Sign = z.wsGenerateSignature(request)

	resp, err := z.Websocket.Conn.SendMessageReturnResponse(request.No, request)
	if err != nil {
		return nil, err
	}
	var response WsRequestResponse
	err = json.Unmarshal(resp, &response)
	if err != nil {
		return nil, err
	}
	if response.Code > 0 && response.Code != 1000 {
		return &response,
			fmt.Errorf("%v request failed, message: %v, error code: %v",
				z.Name,
				response.Message,
				wsErrCodes[response.Code])
	}
	return &response, nil
}

func (z *ZB) wsSubmitOrder(pair currency.Pair, amount, price float64, tradeType int64) (*WsSubmitOrderResponse, error) {
	if !z.GetAuthenticatedAPISupport(exchange.WebsocketAuthentication) {
		return nil,
			fmt.Errorf("%v AuthenticatedWebsocketAPISupport not enabled", z.Name)
	}
	request := WsSubmitOrderRequest{
		Amount:    amount,
		Price:     price,
		TradeType: tradeType,
		No:        z.Websocket.Conn.GenerateMessageID(true),
	}
	request.Channel = pair.String() + "_order"
	request.Event = zWebsocketAddChannel
	request.Accesskey = z.API.Credentials.Key
	request.Sign = z.wsGenerateSignature(request)

	resp, err := z.Websocket.Conn.SendMessageReturnResponse(request.No, request)
	if err != nil {
		return nil, err
	}
	var response WsSubmitOrderResponse
	err = json.Unmarshal(resp, &response)
	if err != nil {
		return nil, err
	}
	if response.Code > 0 && response.Code != 1000 {
		return &response,
			fmt.Errorf("%v request failed, message: %v, error code: %v",
				z.Name,
				response.Message,
				wsErrCodes[response.Code])
	}
	return &response, nil
}

func (z *ZB) wsCancelOrder(pair currency.Pair, orderID int64) (*WsCancelOrderResponse, error) {
	if !z.GetAuthenticatedAPISupport(exchange.WebsocketAuthentication) {
		return nil,
			fmt.Errorf("%v AuthenticatedWebsocketAPISupport not enabled", z.Name)
	}
	request := WsCancelOrderRequest{
		ID: orderID,
		No: z.Websocket.Conn.GenerateMessageID(true),
	}
	request.Channel = pair.String() + "_cancelorder"
	request.Event = zWebsocketAddChannel
	request.Accesskey = z.API.Credentials.Key
	request.Sign = z.wsGenerateSignature(request)

	resp, err := z.Websocket.Conn.SendMessageReturnResponse(request.No, request)
	if err != nil {
		return nil, err
	}
	var response WsCancelOrderResponse
	err = json.Unmarshal(resp, &response)
	if err != nil {
		return nil, err
	}
	if response.Code > 0 && response.Code != 1000 {
		return &response,
			fmt.Errorf("%v request failed, message: %v, error code: %v",
				z.Name,
				response.Message,
				wsErrCodes[response.Code])
	}
	return &response, nil
}

func (z *ZB) wsGetOrder(pair currency.Pair, orderID int64) (*WsGetOrderResponse, error) {
	if !z.GetAuthenticatedAPISupport(exchange.WebsocketAuthentication) {
		return nil,
			fmt.Errorf("%v AuthenticatedWebsocketAPISupport not enabled", z.Name)
	}
	request := WsGetOrderRequest{
		ID: orderID,
		No: z.Websocket.Conn.GenerateMessageID(true),
	}
	request.Channel = pair.String() + "_getorder"
	request.Event = zWebsocketAddChannel
	request.Accesskey = z.API.Credentials.Key
	request.Sign = z.wsGenerateSignature(request)

	resp, err := z.Websocket.Conn.SendMessageReturnResponse(request.No, request)
	if err != nil {
		return nil, err
	}
	var response WsGetOrderResponse
	err = json.Unmarshal(resp, &response)
	if err != nil {
		return nil, err
	}
	if response.Code > 0 && response.Code != 1000 {
		return &response,
			fmt.Errorf("%v request failed, message: %v, error code: %v",
				z.Name,
				response.Message,
				wsErrCodes[response.Code])
	}
	return &response, nil
}

func (z *ZB) wsGetOrders(pair currency.Pair, pageIndex, tradeType int64) (*WsGetOrdersResponse, error) {
	if !z.GetAuthenticatedAPISupport(exchange.WebsocketAuthentication) {
		return nil,
			fmt.Errorf("%v AuthenticatedWebsocketAPISupport not enabled", z.Name)
	}
	request := WsGetOrdersRequest{
		PageIndex: pageIndex,
		TradeType: tradeType,
		No:        z.Websocket.Conn.GenerateMessageID(true),
	}
	request.Channel = pair.String() + "_getorders"
	request.Event = zWebsocketAddChannel
	request.Accesskey = z.API.Credentials.Key
	request.Sign = z.wsGenerateSignature(request)
	resp, err := z.Websocket.Conn.SendMessageReturnResponse(request.No, request)
	if err != nil {
		return nil, err
	}
	var response WsGetOrdersResponse
	err = json.Unmarshal(resp, &response)
	if err != nil {
		return nil, err
	}
	if response.Code > 0 && response.Code != 1000 {
		return &response,
			fmt.Errorf("%v request failed, message: %v, error code: %v",
				z.Name,
				response.Message,
				wsErrCodes[response.Code])
	}
	return &response, nil
}

func (z *ZB) wsGetOrdersIgnoreTradeType(pair currency.Pair, pageIndex, pageSize int64) (*WsGetOrdersIgnoreTradeTypeResponse, error) {
	if !z.GetAuthenticatedAPISupport(exchange.WebsocketAuthentication) {
		return nil,
			fmt.Errorf("%v AuthenticatedWebsocketAPISupport not enabled", z.Name)
	}
	request := WsGetOrdersIgnoreTradeTypeRequest{
		PageIndex: pageIndex,
		PageSize:  pageSize,
		No:        z.Websocket.Conn.GenerateMessageID(true),
	}
	request.Channel = pair.String() + "_getordersignoretradetype"
	request.Event = zWebsocketAddChannel
	request.Accesskey = z.API.Credentials.Key
	request.Sign = z.wsGenerateSignature(request)

	resp, err := z.Websocket.Conn.SendMessageReturnResponse(request.No, request)
	if err != nil {
		return nil, err
	}
	var response WsGetOrdersIgnoreTradeTypeResponse
	err = json.Unmarshal(resp, &response)
	if err != nil {
		return nil, err
	}
	if response.Code > 0 && response.Code != 1000 {
		return &response,
			fmt.Errorf("%v request failed, message: %v, error code: %v",
				z.Name,
				response.Message,
				wsErrCodes[response.Code])
	}
	return &response, nil
}

func (z *ZB) wsGetAccountInfoRequest() (*WsGetAccountInfoResponse, error) {
	if !z.GetAuthenticatedAPISupport(exchange.WebsocketAuthentication) {
		return nil,
			fmt.Errorf("%v AuthenticatedWebsocketAPISupport not enabled", z.Name)
	}
	request := WsAuthenticatedRequest{
		Channel:   "getaccountinfo",
		Event:     zWebsocketAddChannel,
		Accesskey: z.API.Credentials.Key,
		No:        z.Websocket.Conn.GenerateMessageID(true),
	}
	request.Sign = z.wsGenerateSignature(request)

	resp, err := z.Websocket.Conn.SendMessageReturnResponse(request.No, request)
	if err != nil {
		return nil, err
	}
	var response WsGetAccountInfoResponse
	err = json.Unmarshal(resp, &response)
	if err != nil {
		return nil, err
	}
	if response.Code > 0 && response.Code != 1000 {
		return &response,
			fmt.Errorf("%v request failed, message: %v, error code: %v",
				z.Name,
				response.Message,
				wsErrCodes[response.Code])
	}
	return &response, nil
}

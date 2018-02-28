package okcoin

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/gorilla/websocket"
	"github.com/pkg/errors"
	. "github.com/thbourlove/GoEx"
)

const (
	API_BASE_URL     = "https://www.okex.com/api/v1/"
	EXCHANGE_NAME_CN = "okcoin.cn"
	url_ticker       = "ticker.do"
	url_depth        = "depth.do"
	url_trades       = "trades.do"
	url_kline        = "kline.do?symbol=%s&type=%s&size=%d&since=%d"

	url_userinfo      = "userinfo.do"
	url_trade         = "trade.do"
	url_cancel_order  = "cancel_order.do"
	url_order_info    = "order_info.do"
	url_orders_info   = "orders_info.do"
	order_history_uri = "order_history.do"
	trade_uri         = "trade_history.do"
)

type OKCoinCN_API struct {
	client       *http.Client
	api_key      string
	secret_key   string
	api_base_url string
}

var _INERNAL_KLINE_PERIOD_CONVERTER = map[int]string{
	KLINE_PERIOD_1MIN:  "1min",
	KLINE_PERIOD_5MIN:  "5min",
	KLINE_PERIOD_15MIN: "15min",
	KLINE_PERIOD_30MIN: "30min",
	KLINE_PERIOD_60MIN: "1hour",
	KLINE_PERIOD_4H:    "4hour",
	KLINE_PERIOD_1DAY:  "1day",
	KLINE_PERIOD_1WEEK: "1week",
}

//func currencyPair2String(currency CurrencyPair) string {
//	switch currency {
//	case BTC_CNY:
//		return "btc_cny"
//	case LTC_CNY:
//		return "ltc_cny"
//	case BTC_USD:
//		return "btc_usd"
//	case LTC_USD:
//		return "ltc_usd"
//	default:
//		return ""
//	}
//}

func New(client *http.Client, api_key, secret_key string) *OKCoinCN_API {
	return &OKCoinCN_API{client, api_key, secret_key, API_BASE_URL}
}

func (ctx *OKCoinCN_API) buildPostForm(postForm *url.Values) error {
	postForm.Set("api_key", ctx.api_key)
	//postForm.Set("secret_key", ctx.secret_key);

	payload := postForm.Encode()
	payload = payload + "&secret_key=" + ctx.secret_key

	sign, err := GetParamMD5Sign(ctx.secret_key, payload)
	if err != nil {
		return err
	}

	postForm.Set("sign", strings.ToUpper(sign))
	//postForm.Del("secret_key")
	return nil
}

func (ctx *OKCoinCN_API) placeOrder(side, amount, price string, currency CurrencyPair) (*Order, error) {
	postData := url.Values{}
	postData.Set("type", side)

	if side != "buy_market" {
		postData.Set("amount", amount)
	}
	if side != "sell_market" {
		postData.Set("price", price)
	}
	postData.Set("symbol", strings.ToLower(currency.ToSymbol("_")))

	err := ctx.buildPostForm(&postData)
	if err != nil {
		return nil, err
	}

	body, err := HttpPostForm(ctx.client, ctx.api_base_url+url_trade, postData)
	if err != nil {
		return nil, err
	}

	//println(string(body));

	var respMap map[string]interface{}

	err = json.Unmarshal(body, &respMap)
	if err != nil {
		return nil, err
	}

	if err, isok := respMap["error_code"].(float64); isok {
		return nil, errors.New(fmt.Sprint(err))
	}

	order := new(Order)
	order.OrderID = int(respMap["order_id"].(float64))
	order.Price, _ = strconv.ParseFloat(price, 64)
	order.Amount, _ = strconv.ParseFloat(amount, 64)
	order.Currency = currency
	order.Status = ORDER_UNFINISH

	switch side {
	case "buy":
		order.Side = BUY
	case "sell":
		order.Side = SELL
	}

	return order, nil
}

func (ctx *OKCoinCN_API) LimitBuy(amount, price string, currency CurrencyPair) (*Order, error) {
	return ctx.placeOrder("buy", amount, price, currency)
}

func (ctx *OKCoinCN_API) LimitSell(amount, price string, currency CurrencyPair) (*Order, error) {
	return ctx.placeOrder("sell", amount, price, currency)
}

func (ctx *OKCoinCN_API) MarketBuy(amount, price string, currency CurrencyPair) (*Order, error) {
	return ctx.placeOrder("buy_market", amount, price, currency)
}

func (ctx *OKCoinCN_API) MarketSell(amount, price string, currency CurrencyPair) (*Order, error) {
	return ctx.placeOrder("sell_market", amount, price, currency)
}

func (ctx *OKCoinCN_API) CancelOrder(orderId string, currency CurrencyPair) (bool, error) {
	postData := url.Values{}
	postData.Set("order_id", orderId)
	postData.Set("symbol", strings.ToLower(currency.ToSymbol("_")))

	ctx.buildPostForm(&postData)

	body, err := HttpPostForm(ctx.client, ctx.api_base_url+url_cancel_order, postData)

	if err != nil {
		return false, err
	}

	var respMap map[string]interface{}

	err = json.Unmarshal(body, &respMap)
	if err != nil {
		return false, err
	}

	if err, isok := respMap["error_code"].(float64); isok {
		return false, errors.New(fmt.Sprint(err))
	}

	return true, nil
}

func (ctx *OKCoinCN_API) getOrders(orderId string, currency CurrencyPair) ([]Order, error) {
	postData := url.Values{}
	postData.Set("order_id", orderId)
	postData.Set("symbol", strings.ToLower(currency.ToSymbol("_")))

	ctx.buildPostForm(&postData)

	body, err := HttpPostForm(ctx.client, ctx.api_base_url+url_order_info, postData)
	//println(string(body))
	if err != nil {
		return nil, err
	}

	var respMap map[string]interface{}

	err = json.Unmarshal(body, &respMap)
	if err != nil {
		return nil, err
	}

	if err, isok := respMap["error_code"].(float64); isok {
		return nil, errors.New(fmt.Sprint(err))
	}

	orders := respMap["orders"].([]interface{})

	var orderAr []Order
	for _, v := range orders {
		orderMap := v.(map[string]interface{})

		var order Order
		order.Currency = currency
		order.OrderID = int(orderMap["order_id"].(float64))
		order.Amount = orderMap["amount"].(float64)
		order.Price = orderMap["price"].(float64)
		order.DealAmount = orderMap["deal_amount"].(float64)
		order.AvgPrice = orderMap["avg_price"].(float64)
		order.OrderTime = int(orderMap["create_date"].(float64))

		//status:-1:已撤销  0:未成交  1:部分成交  2:完全成交 4:撤单处理中
		switch int(orderMap["status"].(float64)) {
		case -1:
			order.Status = ORDER_CANCEL
		case 0:
			order.Status = ORDER_UNFINISH
		case 1:
			order.Status = ORDER_PART_FINISH
		case 2:
			order.Status = ORDER_FINISH
		case 4:
			order.Status = ORDER_CANCEL_ING
		}

		switch orderMap["type"].(string) {
		case "buy":
			order.Side = BUY
		case "sell":
			order.Side = SELL
		case "buy_market":
			order.Side = BUY_MARKET
		case "sell_market":
			order.Side = SELL_MARKET
		}

		orderAr = append(orderAr, order)
	}

	//fmt.Println(orders);
	return orderAr, nil
}

func (ctx *OKCoinCN_API) GetOneOrder(orderId string, currency CurrencyPair) (*Order, error) {
	orderAr, err := ctx.getOrders(orderId, currency)
	if err != nil {
		return nil, err
	}

	if len(orderAr) == 0 {
		return nil, nil
	}

	return &orderAr[0], nil
}

func (ctx *OKCoinCN_API) GetUnfinishOrders(currency CurrencyPair) ([]Order, error) {
	return ctx.getOrders("-1", currency)
}

func (ctx *OKCoinCN_API) GetAccount() (*Account, error) {
	postData := url.Values{}
	err := ctx.buildPostForm(&postData)
	if err != nil {
		return nil, err
	}

	body, err := HttpPostForm(ctx.client, ctx.api_base_url+url_userinfo, postData)
	if err != nil {
		return nil, err
	}

	var respMap map[string]interface{}

	err = json.Unmarshal(body, &respMap)
	if err != nil {
		return nil, err
	}

	if err, isok := respMap["error_code"].(float64); isok {
		return nil, errors.New(fmt.Sprint(err))
	}

	info, ok := respMap["info"].(map[string]interface{})
	if !ok {
		return nil, errors.New(string(body))
	}

	funds := info["funds"].(map[string]interface{})
	asset := funds["asset"].(map[string]interface{})
	free := funds["free"].(map[string]interface{})
	freezed := funds["freezed"].(map[string]interface{})

	account := new(Account)
	account.Exchange = ctx.GetExchangeName()
	account.Asset, _ = strconv.ParseFloat(asset["total"].(string), 64)
	account.NetAsset, _ = strconv.ParseFloat(asset["net"].(string), 64)

	var btcSubAccount SubAccount
	var ltcSubAccount SubAccount
	var cnySubAccount SubAccount
	var ethSubAccount SubAccount
	var etcSubAccount SubAccount
	var bccSubAccount SubAccount

	btcSubAccount.Currency = BTC
	btcSubAccount.Amount, _ = strconv.ParseFloat(free["btc"].(string), 64)
	btcSubAccount.LoanAmount = 0
	btcSubAccount.ForzenAmount, _ = strconv.ParseFloat(freezed["btc"].(string), 64)

	ltcSubAccount.Currency = LTC
	ltcSubAccount.Amount, _ = strconv.ParseFloat(free["ltc"].(string), 64)
	ltcSubAccount.LoanAmount = 0
	ltcSubAccount.ForzenAmount, _ = strconv.ParseFloat(freezed["ltc"].(string), 64)

	ethSubAccount.Currency = ETH
	ethSubAccount.Amount, _ = strconv.ParseFloat(free["eth"].(string), 64)
	ethSubAccount.LoanAmount = 0
	ethSubAccount.ForzenAmount, _ = strconv.ParseFloat(freezed["eth"].(string), 64)

	etcSubAccount.Currency = ETC
	etcSubAccount.Amount = ToFloat64(free["etc"])
	etcSubAccount.LoanAmount = 0
	etcSubAccount.ForzenAmount = ToFloat64(freezed["etc"])

	bccSubAccount.Currency = BCC
	bccSubAccount.Amount = ToFloat64(free["bcc"])
	bccSubAccount.LoanAmount = 0
	bccSubAccount.ForzenAmount = ToFloat64(freezed["bcc"])

	cnySubAccount.Currency = CNY
	cnySubAccount.Amount, _ = strconv.ParseFloat(free["cny"].(string), 64)
	cnySubAccount.LoanAmount = 0
	cnySubAccount.ForzenAmount, _ = strconv.ParseFloat(freezed["cny"].(string), 64)

	account.SubAccounts = make(map[Currency]SubAccount, 3)
	account.SubAccounts[BTC] = btcSubAccount
	account.SubAccounts[LTC] = ltcSubAccount
	account.SubAccounts[CNY] = cnySubAccount
	account.SubAccounts[ETH] = ethSubAccount
	account.SubAccounts[ETC] = etcSubAccount
	account.SubAccounts[BCC] = bccSubAccount

	return account, nil
}

func (ctx *OKCoinCN_API) GetTicker(currency CurrencyPair) (*Ticker, error) {
	var tickerMap map[string]interface{}
	var ticker Ticker

	url := ctx.api_base_url + url_ticker + "?symbol=" + strings.ToLower(currency.ToSymbol("_"))
	bodyDataMap, err := HttpGet(ctx.client, url)
	if err != nil {
		return nil, err
	}

	tickerMap = bodyDataMap["ticker"].(map[string]interface{})
	ticker.Date, _ = strconv.ParseUint(bodyDataMap["date"].(string), 10, 64)
	ticker.Last, _ = strconv.ParseFloat(tickerMap["last"].(string), 64)
	ticker.Buy, _ = strconv.ParseFloat(tickerMap["buy"].(string), 64)
	ticker.Sell, _ = strconv.ParseFloat(tickerMap["sell"].(string), 64)
	ticker.Low, _ = strconv.ParseFloat(tickerMap["low"].(string), 64)
	ticker.High, _ = strconv.ParseFloat(tickerMap["high"].(string), 64)
	ticker.Vol, _ = strconv.ParseFloat(tickerMap["vol"].(string), 64)

	return &ticker, nil
}

func (ctx *OKCoinCN_API) GetDepth(size int, currency CurrencyPair) (*Depth, error) {
	var depth Depth

	url := ctx.api_base_url + url_depth + "?symbol=" + strings.ToLower(currency.ToSymbol("_")) + "&size=" + strconv.Itoa(size)
	//fmt.Println(url)
	bodyDataMap, err := HttpGet(ctx.client, url)
	if err != nil {
		return nil, err
	}

	if err, isok := bodyDataMap["error_code"].(float64); isok {
		return nil, errors.New(fmt.Sprint(err))
	}

	if dep, isok := bodyDataMap["asks"].([]interface{}); isok {
		for _, v := range dep {
			var dr DepthRecord
			for i, vv := range v.([]interface{}) {
				switch i {
				case 0:
					dr.Price = vv.(float64)
				case 1:
					dr.Amount = vv.(float64)
				}
			}
			depth.AskList = append(depth.AskList, dr)
		}
	}

	if dep, isok := bodyDataMap["bids"].([]interface{}); isok {
		for _, v := range dep {
			var dr DepthRecord
			for i, vv := range v.([]interface{}) {
				switch i {
				case 0:
					dr.Price = vv.(float64)
				case 1:
					dr.Amount = vv.(float64)
				}
			}
			depth.BidList = append(depth.BidList, dr)
		}
	}

	return &depth, nil
}

func (ctx *OKCoinCN_API) GetExchangeName() string {
	return EXCHANGE_NAME_CN
}

func (ctx *OKCoinCN_API) GetKlineRecords(currency CurrencyPair, period, size, since int) ([]Kline, error) {

	klineUrl := ctx.api_base_url + fmt.Sprintf(url_kline,
		strings.ToLower(currency.ToSymbol("_")),
		_INERNAL_KLINE_PERIOD_CONVERTER[period], size, since)

	resp, err := http.Get(klineUrl)
	if err != nil {
		return nil, err
	}

	defer resp.Body.Close()

	body, err := ioutil.ReadAll(resp.Body)

	var klines [][]interface{}

	err = json.Unmarshal(body, &klines)
	if err != nil {
		return nil, err
	}

	var klineRecords []Kline

	for _, record := range klines {
		r := Kline{}
		for i, e := range record {
			switch i {
			case 0:
				r.Timestamp = int64(e.(float64)) / 1000 //to unix timestramp
			case 1:
				r.Open = e.(float64)
			case 2:
				r.High = e.(float64)
			case 3:
				r.Low = e.(float64)
			case 4:
				r.Close = e.(float64)
			case 5:
				r.Vol = e.(float64)
			}
		}
		klineRecords = append(klineRecords, r)
	}

	return klineRecords, nil
}

func (ctx *OKCoinCN_API) GetOrderHistorys(currency CurrencyPair, currentPage, pageSize int) ([]Order, error) {
	orderHistoryUrl := ctx.api_base_url + order_history_uri

	postData := url.Values{}
	postData.Set("status", "1")
	postData.Set("symbol", strings.ToLower(currency.ToSymbol("_")))
	postData.Set("current_page", fmt.Sprintf("%d", currentPage))
	postData.Set("page_length", fmt.Sprintf("%d", pageSize))

	err := ctx.buildPostForm(&postData)
	if err != nil {
		return nil, err
	}

	body, err := HttpPostForm(ctx.client, orderHistoryUrl, postData)
	if err != nil {
		return nil, err
	}

	var respMap map[string]interface{}

	err = json.Unmarshal(body, &respMap)
	if err != nil {
		return nil, err
	}

	if err, isok := respMap["error_code"].(float64); isok {
		return nil, errors.New(fmt.Sprint(err))
	}

	orders := respMap["orders"].([]interface{})

	var orderAr []Order
	for _, v := range orders {
		orderMap := v.(map[string]interface{})

		var order Order
		order.Currency = currency
		order.OrderID = int(orderMap["order_id"].(float64))
		order.Amount = orderMap["amount"].(float64)
		order.Price = orderMap["price"].(float64)
		order.DealAmount = orderMap["deal_amount"].(float64)
		order.AvgPrice = orderMap["avg_price"].(float64)
		order.OrderTime = int(orderMap["create_date"].(float64))

		//status:-1:已撤销  0:未成交  1:部分成交  2:完全成交 4:撤单处理中
		switch int(orderMap["status"].(float64)) {
		case -1:
			order.Status = ORDER_CANCEL
		case 0:
			order.Status = ORDER_UNFINISH
		case 1:
			order.Status = ORDER_PART_FINISH
		case 2:
			order.Status = ORDER_FINISH
		case 4:
			order.Status = ORDER_CANCEL_ING
		}

		switch orderMap["type"].(string) {
		case "buy":
			order.Side = BUY
		case "sell":
			order.Side = SELL
		}

		orderAr = append(orderAr, order)
	}

	return orderAr, nil
}

func (ok *OKCoinCN_API) GetTrades(currencyPair CurrencyPair, since int64) ([]Trade, error) {
	tradeUrl := ok.api_base_url + trade_uri
	postData := url.Values{}
	postData.Set("symbol", strings.ToLower(currencyPair.ToSymbol("_")))
	postData.Set("since", fmt.Sprintf("%d", since))

	err := ok.buildPostForm(&postData)
	if err != nil {
		return nil, err
	}

	body, err := HttpPostForm(ok.client, tradeUrl, postData)
	if err != nil {
		return nil, err
	}
	//println(string(body))

	var trades []Trade
	err = json.Unmarshal(body, &trades)
	if err != nil {
		return nil, err
	}

	return trades, nil
}

func (ok *OKEx) GetDepthChan(pair CurrencyPair) (chan *Depth, chan struct{}, error) {
	c, resp, err := websocket.DefaultDialer.Dial("wss://real.okex.com:10440/websocket/okexapi", nil)
	if err != nil {
		return nil, nil, errors.Wrap(err, "dial websocket")
	}
	defer resp.Body.Close()
	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return nil, nil, errors.Wrap(err, "read body")
	}
	fmt.Println(body)
	c.WriteJSON(map[string]interface{}{
		"event":   "addChannel",
		"channel": fmt.Sprintf("ok_sub_spot_%s_depth", strings.ToLower(pair.ToSymbol("_"))),
	})

	done := make(chan struct{})
	depth := make(chan *Depth)

	go func() {
		defer c.Close()
		defer close(done)
		ticker := time.NewTicker(time.Duration(1) * time.Second)
		for {
			select {
			case <-ticker.C:
				log.Printf("depth next ticker\n")
				err := c.WriteJSON(map[string]string{
					"event": "ping",
				})
				if err != nil {
					log.Printf("write ping: %v\n", err)
				} else {
					log.Printf("write ping success.")
				}
			default:
				log.Printf("depth next reader\n")
				_, reader, err := c.NextReader()
				if err != nil {
					log.Printf("get next reader: %v\n", err)
					return
					continue
				}
				jsonReader := json.NewDecoder(reader)

				var msgmap []map[string]interface{}
				if err := jsonReader.Decode(&msgmap); err != nil {
					log.Printf("decode json: %v\n", err)
					continue
				}

				tick, ok := msgmap[0]["data"].(map[string]interface{})
				if !ok {
					log.Printf("msg: %v\n", msgmap)
					continue
				}

				bids, _ := tick["bids"].([]interface{})
				asks, _ := tick["asks"].([]interface{})

				d := new(Depth)
				for _, r := range asks {
					var dr DepthRecord
					rr := r.([]interface{})
					dr.Price = ToFloat64(rr[0])
					dr.Amount = ToFloat64(rr[1])
					d.AskList = append(d.AskList, dr)
				}

				for _, r := range bids {
					var dr DepthRecord
					rr := r.([]interface{})
					dr.Price = ToFloat64(rr[0])
					dr.Amount = ToFloat64(rr[1])
					d.BidList = append(d.BidList, dr)
				}

				depth <- d
			}
		}
	}()

	return depth, done, nil
}

func (ok *OKEx) GetTradeChan(pair CurrencyPair) (chan []Trade, chan struct{}, error) {
	c, resp, err := websocket.DefaultDialer.Dial("wss://real.okex.com:10440/websocket/okexapi", nil)
	if err != nil {
		return nil, nil, errors.Wrap(err, "dial websocket")
	}
	defer resp.Body.Close()
	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return nil, nil, errors.Wrap(err, "read body")
	}
	fmt.Println(body)
	c.WriteJSON(map[string]interface{}{
		"event":   "addChannel",
		"channel": fmt.Sprintf("ok_sub_spot_%s_deals", strings.ToLower(pair.ToSymbol("_"))),
	})

	done := make(chan struct{})
	tradeChan := make(chan []Trade)

	go func() {
		defer c.Close()
		defer close(done)
		for {
			select {
			default:
				_, reader, err := c.NextReader()
				if err != nil {
					log.Printf("get next reader: %v\n", err)
					return
					continue
				}
				jsonReader := json.NewDecoder(reader)

				var msgmap []map[string]interface{}
				if err := jsonReader.Decode(&msgmap); err != nil {
					log.Printf("decode json: %v\n", err)
					continue
				}

				data, ok := msgmap[0]["data"].([]interface{})
				if !ok {
					log.Printf("get trade msg: %v\n", msgmap[0]["data"])
					continue
				}

				trades := []Trade{}

				for _, d := range data {
					dd := d.([]interface{})
					t := Trade{}
					t.Tid = ToInt64(dd[0])
					t.Price = ToFloat64(dd[1])
					t.Amount = ToFloat64(dd[2])

					now := time.Now()
					timeStr := dd[3].(string)
					timeMeta := strings.Split(timeStr, ":")
					h, _ := strconv.Atoi(timeMeta[0])
					m, _ := strconv.Atoi(timeMeta[1])
					s, _ := strconv.Atoi(timeMeta[2])
					//临界点处理
					if now.Hour() == 0 {
						if h <= 23 && h >= 20 {
							pre := now.AddDate(0, 0, -1)
							t.Date = time.Date(pre.Year(), pre.Month(), pre.Day(), h, m, s, 0, time.Local).Unix() * 1000
						} else if h == 0 {
							t.Date = time.Date(now.Year(), now.Month(), now.Day(), h, m, s, 0, time.Local).Unix() * 1000
						}
					} else {
						t.Date = time.Date(now.Year(), now.Month(), now.Day(), h, m, s, 0, time.Local).Unix() * 1000
					}

					t.Type = dd[4].(string)
					trades = append(trades, t)
				}

				tradeChan <- trades
			}
		}
	}()

	return tradeChan, done, nil
}

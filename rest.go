/*
	Бот создан в рамках проекта Bablofil, подробнее тут
	https://bablofil.ru/vnutrenniy-arbitraj-chast-2/
	или на форуме https://forum.bablofil.ru/
*/

package main

import (
	"bytes"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/json"
	"flag"
	"fmt"
	"io/ioutil"
	"math"
	"net/http"
	"net/url"
	"os"
	"os/signal"
	"strconv"
	"sync"
	"time"
)

// Получить информацию по всем парам - список, лимиты, ограничения и т.п.
func get_exchange_info() exchangeInfo_struct {
	//запросить страницу
	url := "https://api.binance.com/api/v1/exchangeInfo"
	resp, err := http.Get(url)
	if err != nil {
		logger.Print(err)
	}
	defer resp.Body.Close()

	//Получить тело ответа
	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		logger.Print(err)
	}

	//структура для хранения
	var exch_local exchangeInfo_struct

	//парсим JSON в структуру
	err = json.Unmarshal(body, &exch_local)
	if err != nil {
		logger.Print(err)
	}

	return exch_local
}

// На старте программы получить текущие стаканы по каждой паре
func get_depth(pair string, wg *sync.WaitGroup) {

	defer wg.Done()

	//запросить страницу
	url := "https://api.binance.com/api/v1/depth?symbol=" + pair
	resp, err := http.Get(url)
	if err != nil {
		logger.Print(err)
	}
	defer resp.Body.Close()

	//Получить тело ответа
	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		logger.Print(err)
	}

	//структура для хранения
	var depth_local Depth

	//парсим JSON в структуру
	err = json.Unmarshal(body, &depth_local)
	if err != nil {
		//logger.Print(err)
	}

	curr_s := symbols[pair]

	//fill bids tree
	root := update_tree(depth_local.Bids, curr_s.BTree)
	if curr_s.BTree == nil {
		curr_s.BTree = root
	}
	curr_s.CurrB, curr_s.CurrBVol = get_highest_price(curr_s.BTree)

	//fill asks tree
	root = update_tree(depth_local.Asks, curr_s.ATree)
	if curr_s.ATree == nil {
		curr_s.ATree = root
	}
	curr_s.CurrA, curr_s.CurrAVol = get_lowest_price(curr_s.ATree)

	currentTime := time.Now()
	curr_s.LastUpdated = currentTime.Format("2006-01-02 15:04:05.000000000")
}

// Подписаться на обновления по состоянию своих ордеров через сокеты
func get_listen_key() {

	ds_url := "https://api.binance.com/api/v1/userDataStream"

	client := &http.Client{}
	req, _ := http.NewRequest("POST", ds_url, nil)
	req.Header.Set("X-MBX-APIKEY", API_KEY)

	resp, err := client.Do(req)

	if nil != err {
		logger.Panic("errorination happened getting the response", err)
	}

	defer resp.Body.Close()
	body, err := ioutil.ReadAll(resp.Body)

	if nil != err {
		logger.Panic("errorination happened reading the body", err)

	}

	//структура для хранения
	var lk_local ListenKeyObj

	//парсим JSON в структуру
	err = json.Unmarshal(body, &lk_local)
	if err != nil {
		logger.Panic(err)
	}

	listen_key = lk_local.ListenKey
	//logger.Println("LK", listen_key)
}

// Время от времени нужно переподключаться к сокету своих ордеров
func reload_listen_key(wg *sync.WaitGroup) {
	defer wg.Done()

	done := make(chan struct{})

	flag.Parse()

	interrupt := make(chan os.Signal, 1)
	signal.Notify(interrupt, os.Interrupt)

	//cnt := 0
	go func() {
		defer close(done)
		for {
			time.Sleep(1 * time.Minute)

			ds_url := "https://api.binance.com/api/v1/userDataStream"

			buffer := new(bytes.Buffer)
			params := url.Values{}
			params.Set("listenKey", listen_key)

			buffer.WriteString(params.Encode())

			client := &http.Client{}
			req, _ := http.NewRequest("PUT", ds_url, buffer)
			req.Header.Set("X-MBX-APIKEY", API_KEY)

			resp, err := client.Do(req)

			if nil != err {
				logger.Panic("errorination happened getting the response", err)
			}

			defer resp.Body.Close()
			_, err = ioutil.ReadAll(resp.Body)

			if nil != err {
				logger.Panic("errorination happened reading the body", err)
			}
		}
	}()

	select {
	case <-done:
		return

	case <-interrupt:
		logger.Println("interrupt LK")
		wg.Done()
		return
	}

}

// Закрыть поток получения данных по ордерам
func close_listen_key() {
	ds_url := "https://api.binance.com/api/v1/userDataStream"

	buffer := new(bytes.Buffer)
	params := url.Values{}
	params.Set("listenKey", listen_key)

	buffer.WriteString(params.Encode())

	client := &http.Client{}
	req, _ := http.NewRequest("DELETE", ds_url, buffer)
	req.Header.Set("X-MBX-APIKEY", API_KEY)

	resp, err := client.Do(req)

	if nil != err {
		logger.Panic("errorination happened getting the response", err)
	}

	defer resp.Body.Close()
	body, err := ioutil.ReadAll(resp.Body)

	if nil != err {
		logger.Panic("errorination happened reading the body", err)
	}

	fmt.Println("LK %s", string(body[:]))

}

//Генерация криптографической подписи для rest запросов
func get_signature(message string) string {
	mac := hmac.New(sha256.New, []byte(API_SECRET))
	mac.Write([]byte(message))
	return fmt.Sprintf("%x", (mac.Sum(nil)))
}

//Текущее время
func currentTimestamp() int64 {
	return int64(time.Nanosecond)*time.Now().UnixNano()/int64(time.Millisecond) - 2000
}

//Непосредственно создание ордера
func create_order(pair string, quantity float64, side string) bool {

	logger.Println(fmt.Sprintf("Going to make %s order: symbol %s, quantity:%0.8f", side, pair, quantity))
	currentProfitObj.mx.Lock()
	defer currentProfitObj.mx.Unlock()

	var order_created = false

	curr_ts := currentTimestamp()

	buffer := new(bytes.Buffer)

	params := url.Values{}
	params.Set("symbol", pair)
	params.Set("side", side)

	params.Set("type", "MARKET")

	params.Set("newOrderRespType", "RESULT")
	params.Set("quantity", fmt.Sprintf("%0.8f", quantity))
	params.Set("timestamp", fmt.Sprintf("%d", curr_ts))

	signature := get_signature(params.Encode())
	params.Set("signature", signature)

	order_url := fmt.Sprintf("https://api.binance.com/api/v3/order")

	logger.Println(params)
	buffer.WriteString(params.Encode())

	client := &http.Client{}
	req, _ := http.NewRequest("POST", order_url, buffer)
	req.Header.Set("X-MBX-APIKEY", API_KEY)

	resp, err := client.Do(req)

	if nil != err {
		logger.Panic("errorination happened getting the response", err)
	}

	defer resp.Body.Close()
	body, err := ioutil.ReadAll(resp.Body)

	if nil != err {
		logger.Panic("errorination happened reading the body", err)
	}

	fmt.Println("LK %s", string(body[:]))

	var order_obj OrderResult
	err = json.Unmarshal(body, &order_obj)
	if err != nil {
		logger.Println("Error at creation", err, body)
	}

	logger.Println("OrderObj", order_obj.OrderId, order_obj.Status, order_obj)

	if order_obj.OrderId != 0 {
		executed, _ := strconv.ParseFloat(order_obj.ExecutedQty, 32)
		spent, _ := strconv.ParseFloat(order_obj.CummulativeQuoteQty, 32)

		currentProfitObj.OrderId = order_obj.OrderId

		logger.Println("Order created", executed, order_obj)
		//Может получиться так, что ордер исполнится до того, как вернется REST ответ, поэтому можно сразу загнать его в словарь
		//НО! Может получиться так же так, что информация по исполненному ордеру прилетит по сокету до ответа через REST и к моменту получения ответа ордер уже будет в словаре
		if order_obj.Status == "FILLED" {
			if _, ok := orders[order_obj.OrderId]; !ok {
				var order_info = OrderInfo{OrderId: order_obj.OrderId, Symbol: order_obj.Symbol, Side: order_obj.Side, SpentQty: spent, GotQty: executed}
				orders[order_obj.OrderId] = order_info
				logger.Println("OrderInfo", order_info)
				if currentProfitObj.Phase+1 < CHAIN_LEN {
					if currentProfitObj.Chain[currentProfitObj.Phase+1].Type == "SELL" {
						currentProfitObj.Chain[currentProfitObj.Phase+1].MinCoins = executed
					} else {

						interf := currentProfitObj.Chain[currentProfitObj.Phase+1].S.Symbol.Filters[2]
						filter_map := interf.(map[string]interface{})
						stepSize, _ := strconv.ParseFloat(filter_map["stepSize"].(string), 64)

						coins := executed / currentProfitObj.Chain[currentProfitObj.Phase+1].S.CurrA
						coins = math.Floor(coins*(1/stepSize)) / (1 / stepSize)
						currentProfitObj.Chain[currentProfitObj.Phase+1].MinCoins = coins

					}
				}
				currentProfitObj.Phase += 1
			}

		}

		order_created = true
	} else {
		logger.Println("Failed to create order")
	}

	return order_created
}

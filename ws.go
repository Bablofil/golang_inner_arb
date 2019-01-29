/*
	Бот создан в рамках проекта Bablofil, подробнее тут
	https://bablofil.ru/vnutrenniy-arbitraj-chast-2/
	или на форуме https://forum.bablofil.ru/
*/

package main

import (
	"encoding/json"
	"flag"
	"math"
	"net/url"
	"os"
	"os/signal"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/gorilla/websocket"
)

var addr = flag.String("addr", "stream.binance.com:9443", "http service address")

/*
	Подписаться на обновления стаканов сразу по всем парам, обновления заносятся в деревья соответствующих объектов
*/
func ws_listener(wg *sync.WaitGroup) {
	defer wg.Done()

	flag.Parse()

	interrupt := make(chan os.Signal, 1)
	signal.Notify(interrupt, os.Interrupt)

	pairs := "/ws/"
	for _, v := range exchangeInfo.Symbols {
		pairs = pairs + strings.ToLower(v.Symbol) + "@depth/"
	}

	u := url.URL{Scheme: "wss", Host: *addr, Path: pairs}

	c, _, err := websocket.DefaultDialer.Dial(u.String(), nil)
	if err != nil {
		logger.Fatal("dial:", err)
	}
	defer c.Close()

	done := make(chan struct{})

	go func() {

		defer close(done)
		for {
			_, message, err := c.ReadMessage()

			if err != nil {
				logger.Println("read:", err)
				break
			}

			//структура для хранения
			var depth partial_depth

			//парсим JSON в структуру
			err = json.Unmarshal(message, &depth)
			if err != nil {
				//logger.Print(err)
			}

			curr_s := symbols[depth.Symbol]

			//fill bids tree
			root := update_tree(depth.Bids, curr_s.BTree)
			if curr_s.BTree == nil {
				curr_s.BTree = root
			}
			curr_s.CurrB, curr_s.CurrBVol = get_highest_price(curr_s.BTree)

			//fill asks tree
			root = update_tree(depth.Asks, curr_s.ATree)
			if curr_s.ATree == nil {
				curr_s.ATree = root
			}
			curr_s.CurrA, curr_s.CurrAVol = get_lowest_price(curr_s.ATree)
			currentTime := time.Now()
			curr_s.LastUpdated = currentTime.Format("2006-01-02 15:04:05.000000000")

		}
	}()

	for {
		select {
		case <-done:
			return

		case <-interrupt:
			logger.Println("interrupt WS")

			return
		}
	}
}

/*
	Подписка на user-data-stream, когда на бирже происходит обновление пользовательских
	ордеров или иная информация по аккаунту, приходит обновление по сокету.
	Используем только то, что касается ордеров (executionReport), остальное отбрасываем
	Учитывается, что информация по ордеру могла быть получена через rest в момент создания,
	так же это мог быть последний ордер в цепочке, и другой процесс мог занулить указатель на текущую структуру, если получил информацию раньше.
*/
func orders_listener(wg *sync.WaitGroup) {
	defer wg.Done()

	flag.Parse()
	logger.SetFlags(0)

	interrupt := make(chan os.Signal, 1)
	signal.Notify(interrupt, os.Interrupt)

	params := "/ws/" + listen_key

	u := url.URL{Scheme: "wss", Host: *addr, Path: params}

	c, _, err := websocket.DefaultDialer.Dial(u.String(), nil)
	if err != nil {
		logger.Fatal("dial:", err)
	}
	defer c.Close()

	done := make(chan struct{})

	go func() {

		defer close(done)
		for {
			_, message, err := c.ReadMessage()

			if err != nil {
				logger.Println("read:", err)
				break
			}

			var execution_report ExecutionReport

			if strings.Contains(string(message), "executionReport") {
				logger.Printf("recv: %s", message)

				err = json.Unmarshal(message, &execution_report)
				if err != nil {
					//logger.Print(err)
				}

				logger.Println("Exec rep", execution_report.OrderId, execution_report.Symbol, execution_report.CumulativeFilledQuantity, execution_report.CumulativeQuoteQuantity, execution_report)

				if execution_report.Status == "FILLED" {
					if currentProfitObj != nil {
						currentProfitObj.mx.Lock()
					}
					if _, ok := orders[execution_report.OrderId]; !ok {

						spent, _ := strconv.ParseFloat(execution_report.CumulativeFilledQuantity, 32)
						executed, _ := strconv.ParseFloat(execution_report.CumulativeQuoteQuantity, 32)

						var order_info = OrderInfo{OrderId: execution_report.OrderId, Symbol: execution_report.Symbol, Side: execution_report.Side, SpentQty: spent, GotQty: executed}
						orders[execution_report.OrderId] = order_info

						logger.Println("OrderInfo", order_info)

						if currentProfitObj != nil && currentProfitObj.Phase+1 < CHAIN_LEN {
							if currentProfitObj != nil && currentProfitObj.Chain[currentProfitObj.Phase+1].Type == "SELL" {
								if currentProfitObj != nil {
									currentProfitObj.Chain[currentProfitObj.Phase+1].MinCoins = executed
								}
							} else {
								if currentProfitObj != nil {
									interf := currentProfitObj.Chain[currentProfitObj.Phase+1].S.Symbol.Filters[2]
									filter_map := interf.(map[string]interface{})
									stepSize, _ := strconv.ParseFloat(filter_map["stepSize"].(string), 64)

									coins := executed / currentProfitObj.Chain[currentProfitObj.Phase+1].S.CurrA
									coins = math.Floor(coins*(1/stepSize)) / (1 / stepSize)
									currentProfitObj.Chain[currentProfitObj.Phase+1].MinCoins = coins
								}
							}
						}

						currentProfitObj.Phase += 1
					}
					if currentProfitObj != nil {
						currentProfitObj.mx.Unlock()
					}
				}

			}

		}
	}()

	for {
		select {
		case <-done:
			return

		case <-interrupt:
			logger.Println("interrupt OL")
			wg.Done()

			return
		}
	}
}

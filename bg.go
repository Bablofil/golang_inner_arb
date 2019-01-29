/*
	Бот создан в рамках проекта Bablofil, подробнее тут
	https://bablofil.ru/vnutrenniy-arbitraj-chast-2/
	или на форуме https://forum.bablofil.ru/
*/

package main

import (
	"flag"
	"fmt"
	"math"
	"os"
	"os/signal"
	"sort"
	"strconv"
	//"time"
)

// Эта функция проверяет имеющиеся цепи на профитность с учетом текущих курсов
// полученных в других потоках, создает ордера, если это нужно
func process_chains() {

	done := make(chan struct{})

	flag.Parse()

	interrupt := make(chan os.Signal, 1)
	signal.Notify(interrupt, os.Interrupt)

	go func() {
		defer close(done)
		for {

			// Если в данный момент нет открытых ордеров, то проверить, нет ли профитной ситуации
			if currentProfitObj == nil {
				// Проходим в цикле по каждой возможной цепи
				for _, po := range chains {

					s := po.Chain

					var coins float64 = 0

					str := ""
					fits := 0
					var multy_factor float64 = 0

					// Теперь цикл по каждому звену выбрынной цепи
					for i := 0; i < CHAIN_LEN; i++ {
						// Берем действущие ограничения для пары
						interf := s[i].S.Symbol.Filters[2]

						filter_map := interf.(map[string]interface{})
						stepSize, _ := strconv.ParseFloat(filter_map["stepSize"].(string), 64)
						// Для первой сделки считаем кол-во монет к покупке
						if i == 0 && s[i].Type == "BUY" {

							coins = BASE_AMOUNT / s[i].S.CurrA

							if s[i].S.CurrAVol >= coins {
								fits += 1
								multy_factor = s[i].S.CurrAVol / coins
							}
							coins = math.Floor(coins*(1/stepSize)) / (1 / stepSize)
							s[i].MinCoins = coins

						} else {
							/*
								для каждой последующей сделки предварительно считаем,
								сколько монет нужно купить или продать, исходя из ранее полученного кол-ва
							*/
							if s[i].Type == "SELL" {

								coins = math.Floor(coins*(1/stepSize)) / (1 / stepSize)
								s[i].MinCoins = coins
								tmp_multy_factor := s[i].S.CurrBVol / coins

								if s[i].S.CurrBVol >= coins {
									fits += 1
									if tmp_multy_factor < multy_factor {
										multy_factor = tmp_multy_factor
									}
								}

								coins = coins * s[i].S.CurrB

							} else {

								coins = coins / s[i].S.CurrA

								tmp_multy_factor := s[i].S.CurrAVol / coins

								if s[i].S.CurrAVol >= coins {
									fits += 1

									if tmp_multy_factor < multy_factor {
										multy_factor = tmp_multy_factor
									}
								}
								coins = math.Floor(coins*(1/stepSize)) / (1 / stepSize)
								s[i].MinCoins = coins
							}
						}

						// Формируем строку, которая будет отображена в логе
						str += s[i].S.Symbol.Symbol + " " + s[i].Type + " " + fmt.Sprintf("ASK %0.8f BID %0.8f (NEED COINS %0.8f, ASK VOL %0.8f, BID VOL %0.8f) MF (%0.8f) LastUpdated %s\n", s[i].S.CurrA, s[i].S.CurrB, coins, s[i].S.CurrAVol, s[i].S.CurrBVol, multy_factor, s[i].S.LastUpdated) + " "

					}

					// Минимальное кол-во монет, которое отдадим на комисии на первом шаге
					min_fee := BASE_AMOUNT * FEE_SIZE
					/*
						Если денег в стакане больше, чем минимальная ставка,
						то умножить мин ставку это значение - должно получиться
						число в диапазоне минимальной и максимальной ставок
					*/
					po.MultyFactor = multy_factor

					invest_diff := MAX_AMOUNT / BASE_AMOUNT
					if po.MultyFactor > invest_diff {
						po.MultyFactor = invest_diff
					}
					/*
							Предварительный подсчет прибыли за вычетом комиссий
						 	Т.к. на каждом шаге мы платим комиссии от разных объемов разных монет,
							приводим все к живым деньгам в валюте начала торгов
					*/
					coins_prof := coins - coins*FEE_SIZE - min_fee*(CHAIN_LEN-1) - BASE_AMOUNT
					po.Profit = (coins_prof / BASE_AMOUNT) * 100
					// Дополняем строку, которая пойдет в лог
					str += " !!! " + fmt.Sprintf("%0.8f%%", po.Profit) + " (" + fmt.Sprintf("  MF %0.8f,  %0.8f", po.MultyFactor, coins_prof) + ") !!! "
					// Если объемы торгов на каждом шаге подходят под наши объемы торгов, то эта цепь подходит
					po.Fits = fits == CHAIN_LEN-1
					po.Description = str

				}
				// После прохода по всем цепям сортируем все цепи по профитности
				sort.Slice(chains[:], func(i, j int) bool {
					if !math.IsInf(chains[i].Profit, 0) && !math.IsInf(chains[j].Profit, 0) && chains[i].Profit > 0 && chains[j].Profit > 0 {
						return chains[i].Profit > chains[j].Profit
					} else {
						return true
					}
				})
				// Т.к. положительная бесконечность это тоже значение, берем все пары, но оставляем только одну, без бесконечностей
				// TODO: ужасный кусок кода, надо переделать
				for i, po := range chains {
                    if i == 0 {
                        logger.Println(po.Description);
                    }
					if po.Profit >= 0.1 && po.Fits && !math.IsInf(po.Profit, 0) {
						/*
							Если пара подходит по объемам и профит больше, чем 0.1%,
							выводим информацию в лог и создаем первый ордер
						*/
						logger.Println(po.Description)

						currentProfitObj = po
						if !create_order(po.Chain[0].S.Symbol.Symbol, po.Chain[0].MinCoins, "BUY") {
							currentProfitObj = nil
						} else {
							// Если удалось создать ордер, выходим из цикла
							break
						}
					}

				}

			} else { // Ордер был создан, не проверяем цепи, обслуживаем ордера

				currOrderId := currentProfitObj.OrderId
				if order_info, ok := orders[currOrderId]; ok {
					//Если информация по ордеру присутствует в словаре, значит он был исполнен на бирже и мы получили эту информацию
					if currentProfitObj.Phase >= CHAIN_LEN {
						// Все ордера во всей цепи были исполнены
						currentProfitObj.mx.Lock()
						currentProfitObj.Phase = 0
						currentProfitObj = nil
						//Если есть желание убивать бота после исполнения цепочки ордеров, можно раскомментировать следующую строку
						//logger.Panic("Things done")

					} else {
						// Исполнен промежуточный ордер, нужно создать следующий
						symbol := currentProfitObj.Chain[currentProfitObj.Phase].S.Symbol.Symbol
						quantity := order_info.GotQty

						side := currentProfitObj.Chain[currentProfitObj.Phase].Type
						if side == "BUY" {
							quantity = quantity / symbols[symbol].CurrA
						}

						interf := currentProfitObj.Chain[currentProfitObj.Phase].S.Symbol.Filters[2]
						filter_map := interf.(map[string]interface{})
						stepSize, _ := strconv.ParseFloat(filter_map["stepSize"].(string), 64)
						quantity = math.Floor(quantity*(1/stepSize)) / (1 / stepSize)

						create_order(symbol, quantity, side)
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
			logger.Println("interrupt CHAINS")
			wg.Done()
			return
		}
	}

}

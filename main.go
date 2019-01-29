/*
	Бот создан в рамках проекта Bablofil, подробнее тут
	https://bablofil.ru/vnutrenniy-arbitraj-chast-2/
	или на форуме https://forum.bablofil.ru/
*/

package main

import (
	"io"
	"log"
	"os"
	"path/filepath"
	"strings"
	"sync"
)

const CHAIN_LEN = 3       //	Длина цепи, по которой работаем (не менять)
const BASE_COIN = "BTC"   //	С этой монеты начинается и заканчивается цепь
const BASE_AMOUNT = 0.003 //	Сколько минимально денег вкладывать на первую покупку
const MAX_AMOUNT = 0.005  // 	Сколько максимально вкладывать (не
const FEE_SIZE = 0.00075  // 	Размер комиссии

// Ключи API
const API_KEY = ""
const API_SECRET = ""

var logger *log.Logger               // Логирует (в консоль и в файл)
var symbols map[string]*SymbolObj    // Хранит информацию о паре
var exchangeInfo exchangeInfo_struct // Репрезентует структуру ExchangeInfo binance
var wg sync.WaitGroup                // Для группировки потоков (ждем окончание каждого)

var listen_key string // При запуске подпишемся на обновление истории своих торгов через сокеты

var chains []*ProfitObj        // Тут будем хранить всевозможные цепи, и сопутствующую информацию
var orders map[int64]OrderInfo // Тут храним открытые непроверенные ордера

var currentProfitObj *ProfitObj //Текущая открытая цепь ордеров

// Точка входа в программу
func main() {

	// В текущей директории создаем файл log.txt
	dir, err := filepath.Abs(filepath.Dir(os.Args[0]))
	if err != nil {
		log.Fatal(err)
	}

	f, err := os.OpenFile(dir+"/log.txt", os.O_RDWR|os.O_CREATE|os.O_APPEND, 0666)
	if err != nil {
		log.Fatalf("error opening file: %v", err)
	}
	defer f.Close()

	// Лог будем писать одновременно и в файл, и выводить на консоль
	mw := io.MultiWriter(os.Stdout, f)
	log.SetOutput(mw)

	// При логировании будем выводить подробную информацию, включая время, название файла, строку и т.п.
	logger = log.New(mw, "", -1)
	logger.SetFlags(-1)

	// Инициализация начальных объектов
	symbols = make(map[string]*SymbolObj)
	orders = make(map[int64]OrderInfo)
	exchangeInfo = get_exchange_info()

	// Создаем локальное представление пар со всеми свойствами (лимиты, округления и т.п.)
	for _, symbol := range exchangeInfo.Symbols {
		tmp_s := SymbolObj{CurrB: 0, CurrA: 0, Symbol: symbol, BTree: nil, ATree: nil}
		symbols[symbol.Symbol] = &tmp_s
	}

	// На старте программы строим цепи для торговли, в дальнейшем будем проверять именно их
	// Первой операцией всегда будет покупка, последней продажа, в промежутке тип зависит от пар
	for _, v := range exchangeInfo.Symbols {
		if v.Status == "TRADING" {

			if strings.HasSuffix(v.Symbol, BASE_COIN) {
				pref1 := v.Symbol[:len(v.Symbol)-len(BASE_COIN)]
				for _, v2 := range exchangeInfo.Symbols {
					if (strings.HasSuffix(v2.Symbol, pref1) || strings.HasPrefix(v2.Symbol, pref1)) && v2.Status == "TRADING" && !strings.HasSuffix(v2.Symbol, BASE_COIN) {
						var pref2 string

						middle_type := "SELL"
						if strings.HasSuffix(v2.Symbol, pref1) {
							pref2 = v2.Symbol[:len(v2.Symbol)-len(pref1)]
							middle_type = "BUY"
						} else {
							if v2.Symbol[:len(v2.Symbol)-len(pref1)] == "BTC" || v2.Symbol[:len(v2.Symbol)-len(pref1)] == "ETH" || v2.Symbol[:len(v2.Symbol)-len(pref1)] == "BNB" || v2.Symbol[:len(v2.Symbol)-len(pref1)] == "USDT" {
								pref2 = v2.Symbol[len(pref1):]
							} else {
								continue
							}
						}

						for _, v3 := range exchangeInfo.Symbols {

							if pref2+BASE_COIN == v3.Symbol && v3.Status == "TRADING" {

								var ch [CHAIN_LEN]*DealObj
								d1 := DealObj{S: symbols[v.Symbol], Type: "BUY", MinCoins: 0}
								d2 := DealObj{S: symbols[v2.Symbol], Type: middle_type, MinCoins: 0}
								d3 := DealObj{S: symbols[v3.Symbol], Type: "SELL", MinCoins: 0}

								ch[0] = &d1
								ch[1] = &d2
								ch[2] = &d3
								var po ProfitObj = ProfitObj{Chain: ch, Profit: 0, Phase: 0}
								chains = append(chains, &po)
							}
						}
					}
				}
			}
		}

	}

	// Получаем ключ, с которым будем подписываться на веб-сокеты изменения ордеров пользователя и аккаунта
	get_listen_key()

	// Binance просит ключ продлевать, так что отдельным потоком обновляем полученный ключ с определенным интервалом
	go reload_listen_key(&wg)
	// Это бесконечный процесс, и основной поток не должен заканчивать работу раньше него
	wg.Add(1)

	// Непосредственно подписка на веб-сокеты аккаунта и ордеров
	go orders_listener(&wg)
	wg.Add(1)

	/*
		Для построения локального стакана мы запросим с биржи текущее состояние
		по каждой паре, и потом будем обновлять через сокеты
	*/
	for _, link := range symbols {
		go get_depth(link.Symbol.Symbol, &wg)
		wg.Add(1)
	}

	/*
		 	Это фоновый поток, который работает с полученными стаканами,
			проверяет цепи на профитность, создает ордера, если пришло время,
			проверяет их состояние, создает новые, если цепь требует
	*/
	go process_chains()
	wg.Add(1)

	// Через сокеты подписываемся на все пары, обновляем стаканы
	go ws_listener(&wg)
	wg.Add(1)

	wg.Wait()
}

/*
	Бот создан в рамках проекта Bablofil, подробнее тут
	https://bablofil.ru/vnutrenniy-arbitraj-chast-2/
	или на форуме https://forum.bablofil.ru/
*/

package main

import (
	"sync"
)

/* Структуры для деревьев хранения стаканов */
type leaf struct {
	left  *leaf
	price float64 // Depth level price
	vol   float64 // Depth level volume
	right *leaf
}

/*  По каждой паре храним два стакана, текущие лучшие цены из каждого,
объемы текущих лучших цен, пару и время последнего обновления
*/
type SymbolObj struct {
	BTree *leaf   // Bid depth
	ATree *leaf   // Ask depth
	CurrB float64 //Best bid price
	CurrA float64 // Best ask price

	CurrBVol float64 // Best bid position volume
	CurrAVol float64 //Best ask position volume

	Symbol      symbol_struct // Pair
	LastUpdated string        // Last update DT
}

/*
	Ссылка на потенциальную сделку
*/
type DealObj struct {
	S        *SymbolObj // Pointer to SymbolObj - depth, rates etc.
	Type     string     // Deal type (buy, sell)
	MinCoins float64    // Min coins amount to buy|sell (calculates on demand)
}

/*
	Эта структура данных олицетворяет цепь сделок, прибыль по ним и прочее.
	Когда приходит время торговать, специальный указатель ссылается на экзмепляр такой структуры, и код создает/проверяет ордера в соответствии с ней.
*/
type ProfitObj struct {
	Chain       [CHAIN_LEN]*DealObj // Цепь сделок, которые нужно провести
	Profit      float64             // Потенциальная прибыль после выполнения сделок
	Description string              // Сформированная строка, которая выводится в лог для отладки и информации
	Fits        bool                // Признак того, что объем каждой сделки в цепи содержит минимально нужный объем
	MultyFactor float64             // Если объем сделок выше нашего минимального, то увеличить объем покупок до нашего максимального
	Phase       int32               // Какое звено chain сейчас исполняется
	OrderId     int64               // Какой ордер нужно отслеживать (если есть)
	mx          sync.Mutex          // Для блокировки параллельного доступа
}

/* exchange info - структуры, репрезентующие exchangeInfo binance */
type limit_struct struct {
	RateLimitType string `json:rateLimitType`
	Interval      string `json:interval`
	Limit         int32  `json:limit`
}

type symbol_struct struct {
	Symbol             string        `json:symbol`
	Status             string        `json:status`
	BaseAsset          string        `json:baseAsset`
	BaseAssetPrecision int32         `json:baseAssetPrecision`
	QuoteAsset         string        `json:quoteAsset`
	QuotePrecision     int32         `json:quotePrecision`
	OrderTypes         []string      `json:orderTypes`
	IcebergAllowed     bool          `json:icebergAllowed`
	Filters            []interface{} `json:filters`
}

type exchangeInfo_struct struct {
	Timezone        string          `json:timezone`
	ServerTime      int64           `json:serverTime`
	RateLimits      []limit_struct  `json:rateLimits`
	ExchangeFilters []string        `json:exchangeFilters`
	Symbols         []symbol_struct `json:symbols`
}

/* Для парсинга данных о стаканах из сокетов*/
type partial_depth struct {
	e      string     `json:"e"`
	E      int64      `json:"E"`
	Symbol string     `json:"s"`
	U      int64      `json:"U"`
	Bids   [][]string `json:"b"`
	Asks   [][]string `json:"a"`
}

/* Для парсинга rest depth*/
type Depth struct {
	LastUpdateId int32      `json:"lastUpdateId"`
	Bids         [][]string `json:"bids"`
	Asks         [][]string `json:"asks"`
}

/* Получение ключа для подписки на user-data-stream */
type ListenKeyObj struct {
	ListenKey string `json:"listenKey"`
}

/* Информация об созданном ордере, полученная через rest*/
type OrderResult struct {
	Symbol              string `json: "symbol"`
	OrderId             int64  `json: "orderId"`
	ClientOrderId       string `json: "clientOrderId"`
	TransactTime        int64  `json:"transactTime"`
	Price               string `json:"price"`
	OrigQty             string `json:"origQty"`
	ExecutedQty         string `json: "executedQty"`
	CummulativeQuoteQty string `json: "cummulativeQuoteQty"`
	Status              string `json:"status"`
	TimeInForce         string `json:"timeInForce"`
	Type                string `json:"type"`
	Side                string `json:"side"`
}

/* Информация об созданном ордере, полученная через сокеты*/
type ExecutionReport struct {
	eventType                string `json:"e"`  // Event type
	EventTime                int64  `json:"E"`  // Event time
	Symbol                   string `json:"s"`  // Symbol
	ClientOrderId            string `json:"c"`  // Client order ID
	Side                     string `json:"S"`  // Side
	OrderType                string `json:"o"`  // Order type
	TimeInForce              string `json:"f"`  // Time in force
	Quantity                 string `json: "q"` // Order quantity
	Price                    string `json: "p"` // Order price
	StopPrice                string `json:"P"`  // Stop price
	IcebergQuantity          string `json:"F"`  // Iceberg quantity
	Dummy                    int64  `json:"g"`  // Ignore
	OriginalClientOrderId    string `json:"C"`  // Original client order ID; This is the ID of the order being canceled
	ExecutionType            string `json:"x"`  // Current execution type
	Status                   string `json:"X"`  // Current order status
	RejectReason             string `json: "r"` // Order reject reason; will be an error code.
	OrderId                  int64  `json:"i"`  // Order ID
	LastExecutedQuantity     string `json: "l"` // Last executed quantity
	CumulativeFilledQuantity string `json:"z"`  // Cumulative filled quantity
	LastExecutedPrice        string `json:"L"`  // Last executed price
	ComissionAmount          string `json:"n"`  // Commission amount
	ComissionAsset           string `json: "N"` // Commission asset
	TransactionTime          int64  `json:"T"`  // Transaction time
	TradeId                  int64  `json:"t"`  // Trade ID
	Dummy2                   int64  `json:"I"`  // Ignore
	IsWorking                bool   `json: "w"` // Is the order working? Stops will have
	IsMaker                  bool   `json:"m"`  // Is this trade the maker side?
	Dummy3                   bool   `json:"M"`  // Ignore
	OrderCreated             int64  `json:"O"`  // Order creation time
	CumulativeQuoteQuantity  string `json:"Z"`  // Cumulative quote asset transacted quantity
	LastQuoteQuantity        string `json:"Y"`  // Last quote asset transacted quantity (i.e. lastPrice * lastQty)
}

/* Внутренняя структура для хранения некоторых полей ордера */
type OrderInfo struct {
	OrderId  int64
	Symbol   string
	Side     string
	SpentQty float64
	GotQty   float64
}

Бот для Binance
Написан на Golang, использует сокеты для синхронизации стаканов и обновления информации об ордерах пользователя.
Подробнее описано тут https://bablofil.ru/vnutrenniy-arbitraj-chast-2/

Требования
go.1.11.1 and later
go get -u github.com/gorilla/websocket

Как пользоваться
1. Установить Go (Проверяалсь работа под Windows и Linux)
2. Установить зависимости (github.com/gorilla/websocket)
3. В файле main.go указать ключи API

// Ключи API
const API_KEY = ""
const API_SECRET = ""

4. go build -o arbitrage.exe
5. ./arbitrage.exe

Возможны ситуации, когда например рвется коннект или биржа откзывает в обслуживании - в этом случае программа экстренно прекращает работу. Для этого можно запускать его во внешнем watchdog - см. приложенный .bat файл для примере. 
В linux это будет watch или другой, привычный вам инструмент.


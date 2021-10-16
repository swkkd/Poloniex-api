package main

import (
	"encoding/json"
	"fmt"
	"log"
	"net/url"
	"os"
	"os/signal"
	"strconv"
	"time"

	"github.com/gorilla/websocket"
)

var addr = "api2.poloniex.com"

type RecentTrade struct {
	Id        string
	Side      string
	Pair      string
	Price     float64
	Amount    float64
	Timestamp time.Time
}

type PoloniexMessage []interface{}

func bidToSide(bid float64) string {
	if bid == 0 {
		return "SELL"
	}
	return "BUY"
}

func pairId(chanId int) string {
	switch chanId {
	case 121:
		return "USDT/BTC"
	case 265:
		return "USDT/TRX"
	case 149:
		return "USDT/ETH"
	}
	return "Unknown pair"
}

func priceToFloat(price string) float64 {
	if s, err := strconv.ParseFloat(price, 64); err != nil {
		panic(fmt.Sprintf("Cannot convert priceToFloat!: %v", err))
	} else {
		return s
	}
}

func main() {

	interrupt := make(chan os.Signal, 1)
	signal.Notify(interrupt, os.Interrupt)

	u := url.URL{Scheme: "wss", Host: addr}
	log.Printf("connecting to %s", u.String())

	c, _, err := websocket.DefaultDialer.Dial(u.String(), nil)
	if err != nil {
		log.Fatal("dial:", err)
	}
	defer c.Close()

	done := make(chan struct{})

	go func() {
		defer close(done)
		_, _, _ = c.ReadMessage()
		for {
			_, message, err := c.ReadMessage()
			if err != nil {
				log.Println("read:", err)
				return
			}
			var rtrade PoloniexMessage
			err = json.Unmarshal(message, &rtrade)
			if err != nil {
				log.Println(err)
			}
			if len(rtrade) < 2 {
				continue
			}
			pair := pairId(int(rtrade[0].(float64)))
			rtradeMessages := rtrade[2].([]interface{})

			for _, msg := range rtradeMessages {

				tradeMsg := msg.([]interface{})
				if tradeMsg[0].(string) != "t" {
					continue
				}

				trade := RecentTrade{
					Id:        tradeMsg[1].(string),
					Side:      bidToSide(tradeMsg[2].(float64)),
					Pair:      pair,
					Price:     priceToFloat(tradeMsg[3].(string)),
					Amount:    priceToFloat(tradeMsg[4].(string)),
					Timestamp: time.Unix(int64(tradeMsg[5].(float64)), 0),
				}
				log.Printf("%+v", trade)
			}

		}
	}()

	type commandStruct struct {
		Command string `json:"command"`
		Channel string `json:"channel"`
	} // Json request struct

	command := &commandStruct{
		Command: "subscribe",
		Channel: "121,265,149"}
	json, _ := json.Marshal(command)
	stringJSON := string(json)
	fmt.Println("JSON:", stringJSON)
	connectionErr := c.WriteJSON(command)
	if connectionErr != nil {
		log.Println("write:", connectionErr)
	}

	ticker := time.NewTicker(time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-done:
			return
		case <-interrupt:
			log.Println("interrupt")

			err := c.WriteMessage(websocket.CloseMessage,
				websocket.FormatCloseMessage(websocket.CloseNormalClosure, ""))
			if err != nil {
				log.Println("write close:", err)
				return
			}
			select {
			case <-done:
			case <-time.After(time.Second):
			}
			return
		}
	}
}

// INVERTED CURRENCY PAIRS !!!!!
// Why does the inverse of USDEUR not equal to EURUSD (or any 2 currency pairs)?
// The "inverse" of a currency pair is not always equal to the exchange rate of
// the opposite pair because we receive spot rates directly from contributors and
// market makers for each currency pair.
// For example: USDEUR has a value of 1.3050. The inverse of this rate is 1 / 1.3050 = 0.7763.
// You may expect that the exchange rate of EURUSD should equal 0.7763 at the same time.
// While the two numbers should be very close, there is no guarantee they will match exactly,
// as the spot rate of EURUSD comes directly from market makers and are not calculated on our
// end through the inverse of USDEUR.

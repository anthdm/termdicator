package main

import (
	"fmt"
	"log"
	"strconv"
	"time"

	"github.com/VictorLowther/btree"
	"github.com/gorilla/websocket"
	"github.com/mattn/go-runewidth"
	"github.com/nsf/termbox-go"
)

const wsendpoint = "wss://fstream.binance.com/stream?streams=btcusdt@depth"

func byBestBid(a, b *OrderbookEntry) bool {
	return a.Price >= b.Price
}

func byBestAsk(a, b *OrderbookEntry) bool {
	return a.Price < b.Price
}

type OrderbookEntry struct {
	Price  float64
	Volume float64
}

type Orderbook struct {
	Asks *btree.Tree[*OrderbookEntry]
	Bids *btree.Tree[*OrderbookEntry]
}

func NewOrderbook() *Orderbook {
	return &Orderbook{
		Asks: btree.New(byBestAsk),
		Bids: btree.New(byBestBid),
	}
}

func getBidByPrice(price float64) btree.CompareAgainst[*OrderbookEntry] {
	return func(e *OrderbookEntry) int {
		switch {
		case e.Price > price:
			return -1
		case e.Price < price:
			return 1
		default:
			return 0
		}
	}
}

func getAskByPrice(price float64) btree.CompareAgainst[*OrderbookEntry] {
	return func(e *OrderbookEntry) int {
		switch {
		case e.Price < price:
			return -1
		case e.Price > price:
			return 1
		default:
			return 0
		}
	}
}

func (ob *Orderbook) handleDepthResponse(res BinanceDepthResult) {
	for _, ask := range res.Asks {
		price, _ := strconv.ParseFloat(ask[0], 64)
		volume, _ := strconv.ParseFloat(ask[1], 64)
		if volume == 0 {
			if entry, ok := ob.Asks.Get(getAskByPrice(price)); ok {
				// fmt.Printf("-- deleting level %.2f", price)
				ob.Asks.Delete(entry)
			}
			return
		}
		entry := &OrderbookEntry{
			Price:  price,
			Volume: volume,
		}
		ob.Asks.Insert(entry)
	}
	for _, bid := range res.Bids {
		price, _ := strconv.ParseFloat(bid[0], 64)
		volume, _ := strconv.ParseFloat(bid[1], 64)
		if volume == 0 {
			if entry, ok := ob.Bids.Get(getBidByPrice(price)); ok {
				// fmt.Printf("-- deleting level %.2f", price)
				ob.Bids.Delete(entry)
			}
			return
		}
		entry := &OrderbookEntry{
			Price:  price,
			Volume: volume,
		}
		ob.Bids.Insert(entry)
	}
}

func (ob *Orderbook) render(x, y int) {
	it := ob.Asks.Iterator(nil, nil)
	i := 0
	for it.Next() {
		item := it.Item()
		priceStr := fmt.Sprintf("%.2f", item.Price)
		renderText(x, y+i, priceStr, termbox.ColorRed)
		i++
	}

	it = ob.Bids.Iterator(nil, nil)
	i = 0
	x = x + 10
	for it.Next() {
		item := it.Item()
		priceStr := fmt.Sprintf("%.2f", item.Price)
		renderText(x, y+i, priceStr, termbox.ColorGreen)
		i++
	}
}

type BinanceDepthResult struct {
	// price | size (volume)
	Asks [][]string `json:"a"`
	Bids [][]string `json:"b"`
}

type BinanceDepthResponse struct {
	Stream string             `json:"stream"`
	Data   BinanceDepthResult `json:"data"`
}

func main() {
	termbox.Init()

	conn, _, err := websocket.DefaultDialer.Dial(wsendpoint, nil)
	if err != nil {
		log.Fatal(err)
	}

	var (
		ob     = NewOrderbook()
		result BinanceDepthResponse
	)
	go func() {
		for {
			if err := conn.ReadJSON(&result); err != nil {
				log.Fatal(err)
			}
			ob.handleDepthResponse(result.Data)

		}
	}()

	isrunning := true
	go func() {
		time.Sleep(time.Second * 60)
		isrunning = false
	}()

	for isrunning {
		// switch ev := termbox.PollEvent(); ev.Type {
		// case termbox.EventKey:
		// 	switch ev.Key {
		// 	case termbox.KeySpace:
		// 	case termbox.KeyEsc:
		// 		break loop
		// 	}
		// }
		termbox.Clear(termbox.ColorDefault, termbox.ColorDefault)
		ob.render(0, 0)
		termbox.Flush()
	}
}

func renderText(x int, y int, msg string, color termbox.Attribute) {
	for _, ch := range msg {
		termbox.SetCell(x, y, ch, color, termbox.ColorDefault)
		w := runewidth.RuneWidth(ch)
		x += w
	}
}

func _main() {
	conn, _, err := websocket.DefaultDialer.Dial(wsendpoint, nil)
	if err != nil {
		log.Fatal(err)
	}

	var (
		ob     = NewOrderbook()
		result BinanceDepthResponse
	)
	for {
		if err := conn.ReadJSON(&result); err != nil {
			log.Fatal(err)
		}
		ob.handleDepthResponse(result.Data)
		it := ob.Asks.Iterator(nil, nil)
		for it.Next() {
			fmt.Printf("%+v\n", it.Item())
		}
	}
}

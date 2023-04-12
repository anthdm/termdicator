package main

import (
	"fmt"
	"log"
	"sort"
	"strconv"
	"time"

	"github.com/gorilla/websocket"
	"github.com/mattn/go-runewidth"
	"github.com/nsf/termbox-go"
)

const wsendpoint = "wss://fstream.binance.com/stream?streams=btcusdt@markPrice/btcusdt@depth"

var (
	WIDTH         = 0
	HEIGHT        = 0
	currMarkPrice = 0.0
	prevMarkPrice = 0.0
	ARROW_UP      = '↑'
	ARROW_DOWN    = '↓'
)

type OrderbookEntry struct {
	Price  float64
	Volume float64
}

type byBestAsk []OrderbookEntry

func (a byBestAsk) Len() int           { return len(a) }
func (a byBestAsk) Swap(i, j int)      { a[i], a[j] = a[j], a[i] }
func (a byBestAsk) Less(i, j int) bool { return a[i].Price < a[j].Price }

type byBestBid []OrderbookEntry

func (a byBestBid) Len() int           { return len(a) }
func (a byBestBid) Swap(i, j int)      { a[i], a[j] = a[j], a[i] }
func (a byBestBid) Less(i, j int) bool { return a[i].Price > a[j].Price }

type Orderbook struct {
	Asks map[float64]float64
	Bids map[float64]float64
}

func NewOrderbook() *Orderbook {
	return &Orderbook{
		Asks: make(map[float64]float64),
		Bids: make(map[float64]float64),
	}
}

func (ob *Orderbook) handleDepthResponse(asks, bids []any) {
	for _, v := range asks {
		ask := v.([]any)
		price, _ := strconv.ParseFloat(ask[0].(string), 64)
		volume, _ := strconv.ParseFloat(ask[1].(string), 64)
		ob.addAsk(price, volume)
	}
	for _, v := range bids {
		ask := v.([]any)
		price, _ := strconv.ParseFloat(ask[0].(string), 64)
		volume, _ := strconv.ParseFloat(ask[1].(string), 64)
		ob.addBid(price, volume)
	}
}

func (ob *Orderbook) addBid(price, volume float64) {
	if _, ok := ob.Bids[price]; ok {
		if volume == 0.0 {
			delete(ob.Bids, price)
			return
		}
	}
	ob.Bids[price] = volume
}

func (ob *Orderbook) addAsk(price, volume float64) {
	if volume == 0 {
		delete(ob.Asks, price)
		return
	}
	ob.Asks[price] = volume
}

func (ob *Orderbook) getBids() []OrderbookEntry {
	depth := 10
	entries := make(byBestBid, len(ob.Bids))
	i := 0
	for price, volume := range ob.Bids {
		entries[i] = OrderbookEntry{
			Price:  price,
			Volume: volume,
		}
		i++
	}
	sort.Sort(entries)
	var want byBestAsk
	if len(entries) >= depth {
		want = byBestAsk(entries[:depth])
	} else {
		want = byBestAsk(entries)
	}
	sort.Sort(want)
	return want
}

func (ob *Orderbook) getAsks() []OrderbookEntry {
	depth := 10
	entries := make(byBestAsk, len(ob.Asks))
	i := 0
	for price, volume := range ob.Asks {
		entries[i] = OrderbookEntry{
			Price:  price,
			Volume: volume,
		}
		i++
	}
	sort.Sort(entries)
	if len(entries) >= depth {
		return entries[:depth]
	}
	return entries
}

func (ob *Orderbook) render(x, y int) {
	// render the orderbook left frame border
	for i := 0; i < HEIGHT; i++ {
		termbox.SetCell(WIDTH-22, i, '|', termbox.ColorWhite, termbox.ColorDefault)
	}
	for i, ask := range ob.getAsks() {
		price := fmt.Sprintf("%.2f", ask.Price)
		volume := fmt.Sprintf("%.3f", ask.Volume)
		renderText(x, y+i, price, termbox.ColorRed)
		renderText(x+10, y+i, volume, termbox.ColorCyan)
	}
	for i, ask := range ob.getBids() {
		price := fmt.Sprintf("%.2f", ask.Price)
		volume := fmt.Sprintf("%.3f", ask.Volume)
		renderText(x, 10+i, price, termbox.ColorGreen)
		renderText(x+10, 10+i, volume, termbox.ColorCyan)
	}
}

type BinanceTradeResult struct {
	Data struct {
		Price string `json:"p"`
	} `json:"data"`
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
	WIDTH, HEIGHT = termbox.Size()

	conn, _, err := websocket.DefaultDialer.Dial(wsendpoint, nil)
	if err != nil {
		log.Fatal(err)
	}

	var (
		ob = NewOrderbook()
		// result BinanceDepthResponse
		result map[string]any
	)
	go func() {
		for {
			if err := conn.ReadJSON(&result); err != nil {
				log.Fatal(err)
			}
			stream := result["stream"]
			if stream == "btcusdt@depth" {
				data := result["data"].(map[string]any)
				asks := data["a"].([]any)
				bids := data["b"].([]any)
				ob.handleDepthResponse(asks, bids)
			}
			// each 3s
			if stream == "btcusdt@markPrice" {
				prevMarkPrice = currMarkPrice
				data := result["data"].(map[string]any)
				priceStr := data["p"].(string)
				currMarkPrice, _ = strconv.ParseFloat(priceStr, 64)
			}
		}
	}()

	isrunning := true
	eventch := make(chan termbox.Event, 1)
	go func() {
		for {
			eventch <- termbox.PollEvent()
		}
	}()
	for isrunning {
		select {
		case event := <-eventch:
			switch event.Key {
			case termbox.KeyEsc:
				isrunning = false
			}
		default:
		}
		termbox.Clear(termbox.ColorDefault, termbox.ColorDefault)
		render()
		ob.render(WIDTH-18, 2)
		time.Sleep(time.Millisecond * 16)
		termbox.Flush()
	}
}

func renderMarketPrice() {
	color := termbox.ColorRed
	arrow := ARROW_DOWN
	if currMarkPrice > prevMarkPrice {
		color = termbox.ColorGreen
		arrow = ARROW_UP
	}
	renderText(2, 1, fmt.Sprintf("%.2f", currMarkPrice), color)
	renderText(10, 1, string(arrow), color)
}

func render() {
	// render the window frame border
	for i := 0; i < WIDTH; i++ {
		termbox.SetCell(i, 0, '-', termbox.ColorWhite, termbox.ColorDefault)
		termbox.SetCell(i, HEIGHT-1, '-', termbox.ColorWhite, termbox.ColorDefault)
	}
	for i := 0; i < HEIGHT; i++ {
		termbox.SetCell(0, i, '|', termbox.ColorWhite, termbox.ColorDefault)
		termbox.SetCell(WIDTH-1, i, '|', termbox.ColorWhite, termbox.ColorDefault)
	}

	// render the misc border
	for i := 0; i < WIDTH; i++ {
		termbox.SetCell(i, 2, '-', termbox.ColorWhite, termbox.ColorDefault)
	}
	renderMarketPrice()
}

// U+2191 -> arrow
func renderLine(start, end int, color termbox.Attribute) {
	// for i := 0; i < end-start; i++ {
	// 	termbox.SetCell(0, i, '|', termbox.ColorWhite, termbox.ColorDefault)
	// 	termbox.SetCell(WIDTH-1, i, '|', termbox.ColorWhite, termbox.ColorDefault)
	// }
}

func renderText(x int, y int, msg string, color termbox.Attribute) {
	for _, ch := range msg {
		termbox.SetCell(x, y, ch, color, termbox.ColorDefault)
		w := runewidth.RuneWidth(ch)
		x += w
	}
}

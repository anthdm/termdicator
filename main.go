package main

import (
	"fmt"
	"log"
	"sort"
	"strconv"
	"time"

	ui "github.com/gizak/termui/v3"
	"github.com/gizak/termui/v3/widgets"
	"github.com/gorilla/websocket"
)

const wsendpoint = "wss://fstream.binance.com/stream?streams=btcusdt@markPrice/btcusdt@depth"

var (
	WIDTH         = 0
	HEIGHT        = 0
	currMarkPrice = 0.0
	prevMarkPrice = 0.0
	fundingRate   = "n/a"
	ARROW_UP      = "↑"
	ARROW_DOWN    = "↓"
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
		if volume == 0 {
			continue
		}
		entries[i] = OrderbookEntry{
			Price:  price,
			Volume: volume,
		}
		i++
	}
	sort.Sort(entries)
	// var want byBestAsk
	// if len(entries) >= depth {
	// 	want = byBestAsk(entries[:depth])
	// } else {
	// 	want = byBestAsk(entries)
	// }
	// sort.Sort(want)
	if len(entries) >= depth {
		return entries[:depth]
	}
	return entries
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
	if err := ui.Init(); err != nil {
		log.Fatal(err)
	}

	conn, _, err := websocket.DefaultDialer.Dial(wsendpoint, nil)
	if err != nil {
		log.Fatal(err)
	}
	var (
		ob     = NewOrderbook()
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
				fundingRate = data["r"].(string)
				currMarkPrice, _ = strconv.ParseFloat(priceStr, 64)
			}
		}
	}()

	isrunning := true

	margin := 2
	pheight := 3

	pticker := widgets.NewParagraph()
	pticker.Title = "Binancef"
	pticker.Text = "[BTCUSDT](fg:cyan)"
	pticker.SetRect(0, 0, 14, pheight)

	pprice := widgets.NewParagraph()
	pprice.Title = "Market price"
	ppriceOffset := 14 + 14 + margin + 2
	pprice.SetRect(14+margin, 0, ppriceOffset, pheight)

	pfund := widgets.NewParagraph()
	pfund.Title = "Funding rate"
	pfund.SetRect(ppriceOffset+margin, 0, ppriceOffset+margin+16, 3)

	tob := widgets.NewTable()
	out := make([][]string, 20)
	for i := 0; i < 20; i++ {
		out[i] = []string{"n/a", "n/a"}
	}
	tob.TextStyle = ui.NewStyle(ui.ColorWhite)
	tob.SetRect(0, pheight+2, 30, 22+pheight+2)
	tob.PaddingBottom = 0
	tob.PaddingTop = 0
	tob.RowSeparator = false
	tob.TextAlignment = ui.AlignCenter
	for isrunning {
		var (
			asks = ob.getAsks()
			bids = ob.getBids()
			// it   = 0
		)
		if len(asks) >= 10 {
			for i := 0; i < 10; i++ {
				out[i] = []string{fmt.Sprintf("[%.2f](fg:red)", asks[i].Price), fmt.Sprintf("[%.2f](fg:cyan)", asks[i].Volume)}
			}
		}
		if len(bids) >= 10 {
			for i := 0; i < 10; i++ {
				out[i+10] = []string{fmt.Sprintf("[%.2f](fg:green)", bids[i].Price), fmt.Sprintf("[%.2f](fg:cyan)", bids[i].Volume)}
			}
		}
		tob.Rows = out

		pprice.Text = getMarketPrice()
		pfund.Text = fmt.Sprintf("[%s](fg:yellow)", fundingRate)
		ui.Render(pticker, pprice, pfund, tob)
		time.Sleep(time.Millisecond * 20)
	}
}

func getMarketPrice() string {
	price := fmt.Sprintf("[%s %.2f](fg:green)", ARROW_UP, currMarkPrice)
	if prevMarkPrice > currMarkPrice {
		price = fmt.Sprintf("[%s %.2f](fg:red)", ARROW_DOWN, currMarkPrice)
	}
	return price
}

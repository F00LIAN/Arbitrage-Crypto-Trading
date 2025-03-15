// binance.go

package binance

import (
	"encoding/json"
	"fmt"
	"strconv"
	"time"

	"github.com/gorilla/websocket"
)

// FetchBinancePrice connects to the Binance WebSocket for VET/USDT and reads the latest trade price.
// Pass in the last known price to fall back on if the connection fails.
func FetchBinancePrice(lastPrice float64) float64 {
	url := "wss://stream.binance.com:9443/ws/vetusdt@trade?timeUnit=MICROSECOND"
	conn, _, err := websocket.DefaultDialer.Dial(url, nil)
	if err != nil {
		fmt.Printf("Binance WebSocket error: %v\n", err)
		return lastPrice // If dial fails, return the last known price
	}
	defer conn.Close()

	// Handle pings from Binance
	conn.SetPingHandler(func(payload string) error {
		err := conn.WriteControl(websocket.PongMessage, []byte(payload), time.Now().Add(5*time.Second))
		if err != nil {
			fmt.Printf("Failed to send pong: %v\n", err)
		}
		return nil
	})

	// Read messages continuously
	for {
		_, message, err := conn.ReadMessage()
		if err != nil {
			fmt.Printf("Binance read error: %v\n", err)
			return lastPrice // Return last known price on failure
		}

		var data struct {
			Price     string `json:"p"` // Price
			Timestamp int64  `json:"T"` // Trade time in microseconds
		}
		if err := json.Unmarshal(message, &data); err != nil {
			fmt.Printf("Binance parse error: %v\n", err)
			continue
		}

		price, err := strconv.ParseFloat(data.Price, 64)
		if err != nil {
			fmt.Printf("Binance price parse error: %v\n", err)
			continue
		}

		// Log timestamp to verify microsecond precision
		fmt.Printf("Binance price: %f, timestamp: %d (microseconds)\n", price, data.Timestamp)
		// Return on first valid trade message for simplicity
		return price
	}
}

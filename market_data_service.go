package main

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"sync"
	"time"

	"github.com/joho/godotenv"     // Library to load .env files
	"github.com/redis/go-redis/v9" // Redis client for Go

	// Adjust this import path to your actual module path or local replace directive.
	"arbitrage_bot/Arbitrage-Crypto-Trading/exchanges/binance.go"
	// If/when you add BingX or others, import them similarly:
	// "example.com/myproject/exchanges/bingx"
)

// ----------------------------------------------------------------------------
// Config / Env Loading
// ----------------------------------------------------------------------------

// Config holds API keys and other configuration data loaded from environment variables.
type Config struct {
	BinanceAPIKey string
	RedisAddr     string
}

// LoadConfig loads configuration from a .env file or environment variables.
func LoadConfig() *Config {
	// Load .env file if it exists
	err := godotenv.Load()
	if err != nil {
		fmt.Println("No .env file found, falling back to environment variables or defaults")
	}

	config := &Config{
		BinanceAPIKey: getEnv("BINANCE_API_KEY", "YOUR_BINANCE_KEY"),
		RedisAddr:     getEnv("REDIS_ADDR", "localhost:6379"),
	}
	// Only panic if Binance key is empty for this test
	if config.BinanceAPIKey == "" {
		panic("Missing required Binance API key in environment variables or .env file")
	}
	return config
}

// getEnv is a helper function to get environment variables with a default value.
func getEnv(key, fallback string) string {
	if value, exists := os.LookupEnv(key); exists {
		return value
	}
	return fallback
}

// ----------------------------------------------------------------------------
// MarketDataService
// ----------------------------------------------------------------------------

// MarketDataService manages real-time price data for VET from multiple exchanges.
type MarketDataService struct {
	binancePrice float64
	// You can add more fields here for other exchanges if needed, e.g.:
	// bingxPrice float64
	// ...

	lastUpdated time.Time
	mu          sync.Mutex
	priceChan   chan float64
	redisClient *redis.Client
	config      *Config
}

// NewMarketDataService initializes the MarketDataService with a Redis connection and channel.
func NewMarketDataService() *MarketDataService {
	config := LoadConfig()

	redisClient := redis.NewClient(&redis.Options{
		Addr:     config.RedisAddr,
		Password: "",
		DB:       0,
	})

	// Verify Redis connection
	if err := redisClient.Ping(context.Background()).Err(); err != nil {
		panic(fmt.Sprintf("Failed to connect to Redis: %v", err))
	}

	return &MarketDataService{
		priceChan:   make(chan float64, 100),
		redisClient: redisClient,
		config:      config,
	}
}

// FetchPrices concurrently fetches VET prices from multiple exchanges using goroutines.
func (m *MarketDataService) FetchPrices() {
	// Binance goroutine
	go func() {
		for {
			price := binance.FetchBinancePrice(m.binancePrice)
			m.updatePrice("binance", price)
			// Sleep or just continue if you want continuous fetch
			// time.Sleep(1 * time.Second)
		}
	}()

	// If you had more exchange fetchers, you'd do them similarly here:
	/*
		go func() {
			for {
				price := bingx.FetchBingXPrice(m.bingxPrice)
				m.updatePrice("bingx", price)
				time.Sleep(1 * time.Second)
			}
		}()
	*/
}

// updatePrice locks and stores the latest price in both local memory and Redis.
func (m *MarketDataService) updatePrice(exchange string, price float64) {
	m.mu.Lock()
	defer m.mu.Unlock()

	now := time.Now()
	switch exchange {
	case "binance":
		m.binancePrice = price
		// case "bingx":
		// 	m.bingxPrice = price
		// ...
	}
	m.lastUpdated = now

	key := fmt.Sprintf("price:%s", exchange)
	if err := m.redisClient.Set(context.Background(), key, price, 1*time.Minute).Err(); err != nil {
		fmt.Printf("Redis set error: %v\n", err)
	}

	select {
	case m.priceChan <- price:
	default:
		fmt.Println("Price channel full, dropping update")
	}
}

// -----------------------------------------------------------------------------
// Optional helper methods for storing, retrieving, and sending prices
// -----------------------------------------------------------------------------

func (m *MarketDataService) StorePriceData(price float64) {
	data := map[string]interface{}{
		"timestamp": time.Now().Unix(),
		"price":     price,
	}
	jsonData, err := json.Marshal(data)
	if err != nil {
		fmt.Printf("JSON marshal error: %v\n", err)
		return
	}
	fmt.Printf("Storing price data: %s\n", jsonData)
	// TODO: Replace with actual storage logic
}

func (m *MarketDataService) GetPriceData(exchange string) float64 {
	key := fmt.Sprintf("price:%s", exchange)
	price, err := m.redisClient.Get(context.Background(), key).Float64()
	if err != nil {
		fmt.Printf("Redis get error: %v\n", err)
		return 0.0
	}
	return price
}

func (m *MarketDataService) GetLastUpdated() time.Time {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.lastUpdated
}

// -----------------------------------------------------------------------------
// Channel management & sending updates to arbitrage engine
// -----------------------------------------------------------------------------

func (m *MarketDataService) SetupChannel() {
	if m.priceChan == nil {
		m.priceChan = make(chan float64, 100)
	}
}

func (m *MarketDataService) SendPriceUpdates() {
	go func() {
		for price := range m.priceChan {
			fmt.Printf("Sending price update to arbitrage engine: %f\n", price)
			// TODO: Implement gRPC (or other) client to send to Arbitrage Engine
		}
	}()
}

func (m *MarketDataService) CloseChannel() {
	close(m.priceChan)
}

// -----------------------------------------------------------------------------
// main() – starts the MarketDataService and keeps it running
// -----------------------------------------------------------------------------

func main() {
	service := NewMarketDataService()
	service.SetupChannel()
	service.SendPriceUpdates()
	service.FetchPrices()

	// Keep the program running
	select {}
}

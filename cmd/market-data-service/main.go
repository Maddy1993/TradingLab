// cmd/market-data-service/main.go
package main

import (
	"context"
	"encoding/json"
	"log"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/myapp/tradinglab/pkg/events"
	"github.com/myapp/tradinglab/pkg/market"
)

// ServiceStatus contains information about the service status
type ServiceStatus struct {
	Status    string    `json:"status"`
	Uptime    string    `json:"uptime"`
	StartTime time.Time `json:"start_time"`
	Tickers   []string  `json:"tickers"`
}

var (
	startTime = time.Now()
	status    = ServiceStatus{
		Status:    "UP",
		StartTime: startTime,
		Tickers:   []string{},
	}
)

func init() {
	// Set timezone to PST
	loc, err := time.LoadLocation("America/Los_Angeles")
	if err != nil {
		log.Printf("Failed to load PST timezone: %v", err)
		return
	}
	time.Local = loc
	log.SetFlags(log.LstdFlags | log.LUTC)
}

func main() {
	// Get NATS URL from environment or use default
	natsURL := os.Getenv("NATS_URL")
	if natsURL == "" {
		natsURL = "nats://localhost:4222"
	}

	// Get HTTP server port
	httpPort := os.Getenv("HTTP_PORT")
	if httpPort == "" {
		httpPort = "8080"
	}

	log.Printf("Market Data Service starting, connecting to NATS server at %s", natsURL)

	// Create event client
	client, err := events.NewEventClient(natsURL)
	if err != nil {
		log.Fatalf("Failed to create event client: %v", err)
	}
	defer client.Close()

	// Create context for clean shutdown
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Handle graceful shutdown
	signals := make(chan os.Signal, 1)
	signal.Notify(signals, syscall.SIGINT, syscall.SIGTERM)

	go func() {
		sig := <-signals
		log.Printf("Received signal: %v", sig)
		cancel()
	}()

	// Get Alpaca API credentials from environment
	apiKey := os.Getenv("ALPACA_API_KEY")
	apiSecret := os.Getenv("ALPACA_API_SECRET")

	// Check if credentials are provided
	if apiKey == "" || apiSecret == "" {
		log.Fatalf("ALPACA_API_KEY and ALPACA_API_SECRET environment variables are required")
	}

	// Determine if we should use paper trading
	usePaperTrading := true
	if os.Getenv("ALPACA_LIVE_TRADING") == "true" {
		usePaperTrading = false
	}

	// Create market data provider
	provider, err := market.NewAlpacaProvider(apiKey, apiSecret, usePaperTrading)
	if err != nil {
		log.Fatalf("Failed to create market data provider: %v", err)
	}

	// Define tickers to watch
	tickers := []string{"SPY", "AAPL", "MSFT", "GOOGL"}

	// Allow customizing tickers via environment variables
	if customTickers := os.Getenv("WATCH_TICKERS"); customTickers != "" {
		// Split the comma-separated string into individual tickers
		tickers = strings.Split(customTickers, ",")
	}

	// Update global status
	status.Tickers = tickers

	// Start streaming data for each ticker
	for _, ticker := range tickers {
		go streamMarketData(ctx, client, provider, ticker)
	}

	// Start HTTP server for health checks
	go startHealthServer(httpPort)

	// Keep running until signal received
	log.Println("Market Data Service running. Press Ctrl+C to exit")
	<-ctx.Done()
	log.Println("Shutting down Market Data Service")
}

func streamMarketData(ctx context.Context, client *events.EventClient, provider *market.AlpacaProvider, tickerSymbol string) {
	// Default polling interval is 60 seconds
	intervalStr := os.Getenv("POLLING_INTERVAL")
	interval := 60 * time.Second

	if intervalStr != "" {
		// Try to parse custom interval
		customInterval, err := time.ParseDuration(intervalStr)
		if err == nil && customInterval > 0 {
			interval = customInterval
		}
	}

	log.Printf("Starting market data stream for %s with interval %v", tickerSymbol, interval)

	t := time.NewTicker(interval)
	defer t.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-t.C:
			// Fetch latest data from the provider
			data, err := provider.GetLatestData(ctx, tickerSymbol)
			if err != nil {
				log.Printf("Failed to get data for %s: %v", tickerSymbol, err)
				continue
			}

			// Publish to event stream
			if err := client.PublishMarketData(ctx, tickerSymbol, data); err != nil {
				log.Printf("Failed to publish market data for %s: %v", tickerSymbol, err)
			} else {
				log.Printf("Published market data for %s: price=$%.2f, volume=%d",
					tickerSymbol, data.Price, data.Volume)
			}
		}
	}
}

func startHealthServer(port string) {
	// Define health check handler
	http.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
		// Update uptime
		status.Uptime = time.Since(startTime).String()

		// Return status as JSON
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(status)
	})

	// Start HTTP server
	serverAddr := ":" + port
	log.Printf("Starting health server on %s", serverAddr)
	if err := http.ListenAndServe(serverAddr, nil); err != nil {
		log.Fatalf("Health server failed: %v", err)
	}
}

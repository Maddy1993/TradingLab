// cmd/market-data-service/main.go
package main

import (
	"context"
	"encoding/json"
	"fmt"
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
	Status        string    `json:"status"`
	Uptime        string    `json:"uptime"`
	StartTime     time.Time `json:"start_time"`
	Tickers       []string  `json:"tickers"`
	MarketOpen    bool      `json:"market_open"`
	LastPublished time.Time `json:"last_published"`
	StreamStats   struct {
		LiveEvents     int64 `json:"live_events"`
		DailyEvents    int64 `json:"daily_events"`
		HistoricalReqs int64 `json:"historical_requests"`
	} `json:"stream_stats"`
}

var (
	startTime = time.Now()
	status    = ServiceStatus{
		Status:    "UP",
		StartTime: startTime,
		Tickers:   []string{},
	}
	currentTickers []string
	marketProvider *market.AlpacaProvider
	eventClient    *events.EventClient
)

func init() {
	// Set timezone to ET (Eastern Time) for market hours
	loc, err := time.LoadLocation("America/New_York")
	if err != nil {
		log.Printf("Failed to load ET timezone: %v", err)
	} else {
		time.Local = loc
	}
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
	var err error
	eventClient, err = events.NewEventClient(natsURL)
	if err != nil {
		log.Fatalf("Failed to create event client: %v", err)
	}
	defer eventClient.Close()

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
	marketProvider, err = market.NewAlpacaProvider(apiKey, apiSecret, usePaperTrading)
	if err != nil {
		log.Fatalf("Failed to create market data provider: %v", err)
	}

	// Define tickers to watch
	currentTickers = []string{"SPY", "AAPL", "MSFT", "GOOGL"}

	// Allow customizing tickers via environment variables
	if customTickers := os.Getenv("WATCH_TICKERS"); customTickers != "" {
		// Split the comma-separated string into individual tickers
		currentTickers = strings.Split(customTickers, ",")
	}

	// Update global status
	status.Tickers = currentTickers

	// Subscribe to historical data requests
	go subscribeToHistoricalRequests(ctx)

	// Start streaming data for each ticker
	for _, ticker := range currentTickers {
		go streamMarketData(ctx, ticker)
	}

	// Start HTTP server for health checks and API endpoints
	go startHTTPServer(httpPort)

	// Keep running until signal received
	log.Println("Market Data Service running. Press Ctrl+C to exit")
	<-ctx.Done()
	log.Println("Shutting down Market Data Service")
}

// streamMarketData handles both live and daily market data streaming
func streamMarketData(ctx context.Context, tickerSymbol string) {
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

	// Verify data availability before starting stream
	if !verifyDataAvailability(ctx, tickerSymbol) {
		log.Printf("Data not available for %s. Stream will not start until data becomes available.", tickerSymbol)
	}

	t := time.NewTicker(interval)
	defer t.Stop()

	// Create daily timer that fires at 4:30 PM ET (after market close)
	// Set safe default timezone
	loc := time.UTC
	etLoc, err := time.LoadLocation("America/New_York")
	if err == nil {
		loc = etLoc
	} else {
		log.Printf("Warning: Failed to load ET timezone, using UTC instead: %v", err)
	}

	now := time.Now().In(loc)
	marketCloseTime := time.Date(now.Year(), now.Month(), now.Day(), 16, 30, 0, 0, loc)

	// If we're past 4:30 PM, schedule for tomorrow
	if now.After(marketCloseTime) {
		marketCloseTime = marketCloseTime.Add(24 * time.Hour)
	}

	// Duration until next 4:30 PM
	initialDelay := marketCloseTime.Sub(now)
	dailyTicker := time.NewTimer(initialDelay)

	go func() {
		for {
			<-dailyTicker.C
			// Publish daily summary
			go publishDailyData(ctx, tickerSymbol)
			// Reset timer for next day
			dailyTicker.Reset(24 * time.Hour)
		}
	}()

	dataAvailable := false

	for {
		select {
		case <-ctx.Done():
			return
		case <-t.C:
			// If data wasn't available before, check again
			if !dataAvailable {
				dataAvailable = verifyDataAvailability(ctx, tickerSymbol)
				if !dataAvailable {
					log.Printf("Still waiting for data availability for %s", tickerSymbol)
					continue
				} else {
					log.Printf("Data now available for %s, starting regular stream", tickerSymbol)
				}
			}

			// Check if market is open
			isOpen, err := marketProvider.IsMarketOpen(ctx)
			if err != nil {
				log.Printf("Failed to check market status: %v", err)
			}

			status.MarketOpen = isOpen

			// Fetch and publish appropriate data
			if isOpen {
				// Market is open, publish live data
				publishLiveData(ctx, tickerSymbol)
			} else {
				// Market is closed, publish most recent data as daily data
				// We'll also publish a proper daily summary at 4:30 PM
				publishMostRecentData(ctx, tickerSymbol)
			}
		}
	}
}

// verifyDataAvailability checks if actual data (not sample data) is available for the ticker
func verifyDataAvailability(ctx context.Context, tickerSymbol string) bool {
	// Try to get data
	data, err := marketProvider.GetMostRecentData(ctx, tickerSymbol)
	if err != nil {
		log.Printf("Failed to verify data availability for %s: %v", tickerSymbol, err)
		return false
	}

	// Check if we got real data or sample data
	if data.Source == "Sample" {
		log.Printf("Only sample data available for %s, not starting stream yet", tickerSymbol)
		return false
	}

	log.Printf("Verified data availability for %s. Source: %s", tickerSymbol, data.Source)
	return true
}

// publishLiveData publishes real-time market data
func publishLiveData(ctx context.Context, tickerSymbol string) {
	// Fetch latest data from the provider
	data, err := marketProvider.GetLatestData(ctx, tickerSymbol)
	if err != nil {
		log.Printf("Failed to get live data for %s: %v", tickerSymbol, err)
		return
	}

	// Add data type metadata
	data.DataType = "live"

	// Publish to event stream
	if err := eventClient.PublishMarketLiveData(ctx, tickerSymbol, data); err != nil {
		log.Printf("Failed to publish live market data for %s: %v", tickerSymbol, err)
	} else {
		log.Printf("Published live market data for %s: price=$%.2f, volume=%d",
			tickerSymbol, data.Price, data.Volume)
		status.LastPublished = time.Now()
		status.StreamStats.LiveEvents++
	}
}

// publishMostRecentData publishes most recent data when market is closed
func publishMostRecentData(ctx context.Context, tickerSymbol string) {
	// Fetch recent data from the provider
	data, err := marketProvider.GetMostRecentData(ctx, tickerSymbol)
	if err != nil {
		log.Printf("Failed to get recent data for %s: %v", tickerSymbol, err)
		return
	}

	// Add data type metadata
	data.DataType = "recent"

	// Publish to event stream - we still use the live stream but with a "recent" flag
	if err := eventClient.PublishMarketLiveData(ctx, tickerSymbol, data); err != nil {
		log.Printf("Failed to publish recent market data for %s: %v", tickerSymbol, err)
	} else {
		log.Printf("Published recent market data for %s: price=$%.2f, volume=%d",
			tickerSymbol, data.Price, data.Volume)
		status.LastPublished = time.Now()
	}
}

// publishDailyData publishes end-of-day summary
func publishDailyData(ctx context.Context, tickerSymbol string) {
	// Fetch daily data from the provider
	data, err := marketProvider.GetDailyData(ctx, tickerSymbol)
	if err != nil {
		log.Printf("Failed to get daily data for %s: %v", tickerSymbol, err)
		return
	}

	// Add data type metadata
	data.DataType = "daily"

	// Publish to daily event stream
	if err := eventClient.PublishMarketDailyData(ctx, tickerSymbol, data); err != nil {
		log.Printf("Failed to publish daily market data for %s: %v", tickerSymbol, err)
	} else {
		log.Printf("Published daily market data for %s: close=$%.2f, volume=%d",
			tickerSymbol, data.Close, data.Volume)
		status.StreamStats.DailyEvents++
	}
}

// subscribeToHistoricalRequests listens for requests to fetch historical data
func subscribeToHistoricalRequests(ctx context.Context) {
	// Subscribe to historical data requests
	_, err := eventClient.SubscribeHistoricalRequests(func(ticker, timeframe string, days int, reqData []byte) {
		log.Printf("Received historical data request: %s, %s, %d days", ticker, timeframe, days)
		status.StreamStats.HistoricalReqs++

		// Parse request data for any additional parameters
		var request map[string]interface{}
		if err := json.Unmarshal(reqData, &request); err != nil {
			log.Printf("Failed to parse request data: %v", err)
		}

		// Fetch historical data
		historicalData, err := marketProvider.GetHistoricalData(ctx, ticker, days, timeframe)
		if err != nil {
			log.Printf("Failed to get historical data: %v", err)
			return
		}

		// Stream is limited so we'll publish in chunks if necessary
		const chunkSize = 100

		// If we have a large dataset, publish in chunks
		if len(historicalData) > chunkSize {
			chunks := len(historicalData) / chunkSize
			if len(historicalData)%chunkSize > 0 {
				chunks++
			}

			for i := 0; i < chunks; i++ {
				start := i * chunkSize
				end := start + chunkSize
				if end > len(historicalData) {
					end = len(historicalData)
				}

				// Prepare chunk data
				chunkData := map[string]interface{}{
					"data": historicalData[start:end],
					"metadata": map[string]interface{}{
						"ticker":       ticker,
						"timeframe":    timeframe,
						"days":         days,
						"chunk":        i + 1,
						"total_chunks": chunks,
						"data_type":    "historical",
					},
				}

				// Publish chunk
				if err := eventClient.PublishHistoricalData(ctx, ticker, timeframe, days, chunkData); err != nil {
					log.Printf("Failed to publish historical data chunk %d/%d: %v", i+1, chunks, err)
				} else {
					log.Printf("Published historical data chunk %d/%d for %s (%s, %d days)",
						i+1, chunks, ticker, timeframe, days)
				}

				// Small pause between chunks to avoid overwhelming the system
				time.Sleep(500 * time.Millisecond)
			}
		} else {
			// Prepare data package
			dataPackage := map[string]interface{}{
				"data": historicalData,
				"metadata": map[string]interface{}{
					"ticker":       ticker,
					"timeframe":    timeframe,
					"days":         days,
					"chunk":        1,
					"total_chunks": 1,
					"data_type":    "historical",
				},
			}

			// Publish all data at once for smaller datasets
			if err := eventClient.PublishHistoricalData(ctx, ticker, timeframe, days, dataPackage); err != nil {
				log.Printf("Failed to publish historical data: %v", err)
			} else {
				log.Printf("Published historical data for %s (%s, %d days)", ticker, timeframe, days)
			}
		}
	})

	if err != nil {
		log.Printf("Failed to subscribe to historical requests: %v", err)
	} else {
		log.Printf("Subscribed to historical data requests")
	}
}

// startHTTPServer starts an HTTP server for health checks and API endpoints
func startHTTPServer(port string) {
	// Define health check handler
	http.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
		// Update uptime
		status.Uptime = time.Since(startTime).String()

		// Return status as JSON
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(status)
	})

	// API endpoint to request historical data directly via HTTP
	http.HandleFunc("/api/historical", func(w http.ResponseWriter, r *http.Request) {
		// Only accept GET requests
		if r.Method != http.MethodGet {
			w.WriteHeader(http.StatusMethodNotAllowed)
			return
		}

		// Parse query parameters
		ticker := r.URL.Query().Get("ticker")
		timeframe := r.URL.Query().Get("timeframe")
		daysStr := r.URL.Query().Get("days")

		if ticker == "" || timeframe == "" || daysStr == "" {
			w.WriteHeader(http.StatusBadRequest)
			w.Write([]byte("Missing required parameters: ticker, timeframe, days"))
			return
		}

		var days int
		_, err := fmt.Sscanf(daysStr, "%d", &days)
		if err != nil || days <= 0 || days > 365 {
			w.WriteHeader(http.StatusBadRequest)
			w.Write([]byte("Invalid days parameter: must be a positive integer up to 365"))
			return
		}

		// Create request data
		requestID := fmt.Sprintf("%s-%d", r.RemoteAddr, time.Now().UnixNano())
		requestData := map[string]interface{}{
			"request_id": requestID,
			"source":     "http_api",
			"timestamp":  time.Now().Format(time.RFC3339),
		}

		// Publish request to NATS
		err = eventClient.RequestHistoricalData(r.Context(), ticker, timeframe, days, requestData)
		if err != nil {
			w.WriteHeader(http.StatusInternalServerError)
			w.Write([]byte(fmt.Sprintf("Failed to request data: %v", err)))
			return
		}

		// Return success response
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusAccepted)
		json.NewEncoder(w).Encode(map[string]interface{}{
			"status":     "accepted",
			"request_id": requestID,
			"message": fmt.Sprintf("Historical data request for %s (%s, %d days) has been submitted",
				ticker, timeframe, days),
		})
	})

	// Start HTTP server
	serverAddr := ":" + port
	log.Printf("Starting HTTP server on %s", serverAddr)
	if err := http.ListenAndServe(serverAddr, nil); err != nil {
		log.Fatalf("HTTP server failed: %v", err)
	}
}

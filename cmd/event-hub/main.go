// cmd/event-hub/main.go
package main

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/myapp/tradinglab/pkg/events"
	"github.com/myapp/tradinglab/pkg/utils"
	eventhub "github.com/myapp/tradinglab/pkg/hub"
)

func init() {
	// Set timezone to ET (Eastern Time) for market hours
	loc, err := time.LoadLocation("America/New_York")
	if err != nil {
		utils.Error("Failed to load ET timezone: %v", err)
	} else {
		time.Local = loc
	}
}

func main() {
	// Get NATS URL from environment or use default
	natsURL := os.Getenv("NATS_URL")
	if natsURL == "" {
		natsURL = "nats://localhost:4222"
	}

	// Get health server address from environment or use default
	healthAddr := os.Getenv("HEALTH_ADDR")
	if healthAddr == "" {
		healthAddr = ":8080"
	}

	// Get watched tickers from environment
	watchTickers := os.Getenv("WATCH_TICKERS")
	var tickers []string
	if watchTickers != "" {
		tickers = strings.Split(watchTickers, ",")
	} else {
		// Default tickers to watch
		tickers = []string{"SPY", "AAPL", "MSFT", "GOOGL", "AMZN"}
	}

	utils.Info("Event Hub starting, connecting to NATS server at %s", natsURL)
	utils.Info("Watching tickers: %v", tickers)

	// Create event client
	client, err := events.NewEventClient(natsURL)
	if err != nil {
		utils.Fatal("Failed to create event client: %v", err)
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
		utils.Info("Received signal: %v", sig)
		cancel()
	}()

	// Create event hub
	hub := eventhub.NewEventHub(client)

	// Set watched tickers
	hub.SetWatchedTickers(tickers)

	// Start the event hub with retry for critical components
	maxRetries := 10
	retryDelay := 5 * time.Second
	var lastError error

	for i := 0; i < maxRetries; i++ {
		err := hub.Start(ctx)
		lastError = err

		// If no error or only non-critical errors, we can proceed
		if err == nil {
			utils.Info("Event hub started successfully with all streams")
			break
		} else if !strings.Contains(err.Error(), "failed to start critical components") {
			// We had errors but none for critical components
			utils.Info("Event hub started with some non-critical streams unavailable")
			break
		}

		utils.Info("Attempt %d/%d: Required critical streams not yet available, waiting for services to initialize: %v",
			i+1, maxRetries, err)
		select {
		case <-ctx.Done():
			break
		case <-time.After(retryDelay):
			continue
		}
	}

	// Even if we couldn't start critical components after all retries,
	// just log the warning and continue - the retry mechanism will keep trying
	if lastError != nil && strings.Contains(lastError.Error(), "failed to start critical components") {
		utils.Info("Warning: Starting event hub with critical components unavailable. " +
			"Will continue to retry in the background.")
	}

	// Setup HTTP server for health checks and API endpoints
	http.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
		stats := hub.GetStats()
		streamStatus := hub.GetStreamStatus()

		// Check if all critical streams are up
		allCriticalStreamsUp := streamStatus["requests"]

		// Set status based on critical streams
		status := "DEGRADED"
		if allCriticalStreamsUp {
			status = "UP"
		}

		response := map[string]interface{}{
			"status":        status,
			"timestamp":     time.Now(),
			"stats":         stats,
			"streams":       streamStatus,
			"failedStreams": []string{},
		}

		// Add list of failed streams for easier monitoring
		var failedStreams []string
		for stream, status := range streamStatus {
			if !status {
				failedStreams = append(failedStreams, stream)
			}
		}
		response["failedStreams"] = failedStreams

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(response)
	})

	// API endpoint to request historical data
	http.HandleFunc("/api/historical", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			w.WriteHeader(http.StatusMethodNotAllowed)
			return
		}

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

		// Process the request through the client directly
		err = client.RequestHistoricalData(r.Context(), ticker, timeframe, days, map[string]interface{}{
			"request_id": requestID,
			"source":     "hub_api",
			"timestamp":  time.Now().Format(time.RFC3339),
		})

		if err != nil {
			w.WriteHeader(http.StatusInternalServerError)
			w.Write([]byte(fmt.Sprintf("Failed to process request: %v", err)))
			return
		}

		// Return success response
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusAccepted)
		json.NewEncoder(w).Encode(map[string]interface{}{
			"status":     "accepted",
			"request_id": requestID,
			"message":    fmt.Sprintf("Historical data request for %s (%s, %d days) has been submitted", ticker, timeframe, days),
		})
	})

	// Start HTTP server in a goroutine
	go func() {
		utils.Info("Starting HTTP server on %s", healthAddr)
		if err := http.ListenAndServe(healthAddr, nil); err != nil {
			utils.Fatal("HTTP server error: %v", err)
		}
	}()

	// Keep running until signal received
	utils.Info("Event Hub running. Press Ctrl+C to exit")
	<-ctx.Done()
	utils.Info("Shutting down Event Hub")

	// Allow time for clean shutdown
	time.Sleep(500 * time.Millisecond)
}
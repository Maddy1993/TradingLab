// pkg/hub/event_hub.go
package hub

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"strings"
	"sync"
	"time"

	"github.com/myapp/tradinglab/pkg/events"
	"github.com/myapp/tradinglab/pkg/utils"
)

// EventHub manages the routing, transformation, and coordination of events
type EventHub struct {
	client          *events.EventClient
	subscriptions   []*Subscription
	requestHandlers map[string]RequestHandler
	mu              sync.Mutex
	stats           EventStats
	watchedTickers  []string
	failedStreams   map[string]SubscriptionConfig // Tracks failed subscription attempts
	ctx             context.Context
	cancel          context.CancelFunc
}

// Subscription represents a subscription to an event stream
type Subscription struct {
	Subject  string
	Handler  func([]byte)
	Consumer string
	Active   bool // Whether the subscription is currently active
}

// SubscriptionConfig holds information needed to retry a subscription
type SubscriptionConfig struct {
	Type      string    // Type of subscription (live, daily, historical, signals)
	Subject   string    // Subject to subscribe to
	LastRetry time.Time // Last retry timestamp
}

// RequestHandler defines a function to handle data requests
type RequestHandler func(ctx context.Context, ticker string, timeframe string, days int, reqData []byte) error

// EventStats tracks statistics about events
type EventStats struct {
	TotalEvents      int64                  `json:"total_events"`
	LiveEvents       int64                  `json:"live_events"`
	DailyEvents      int64                  `json:"daily_events"`
	HistoricalEvents int64                  `json:"historical_events"`
	SignalEvents     int64                  `json:"signal_events"`
	Requests         int64                  `json:"requests"`
	ErrorCount       int64                  `json:"error_count"`
	TickerStats      map[string]TickerStats `json:"ticker_stats"`
	LastUpdated      time.Time              `json:"last_updated"`
}

// TickerStats tracks statistics for a specific ticker
type TickerStats struct {
	LiveEvents       int64     `json:"live_events"`
	DailyEvents      int64     `json:"daily_events"`
	HistoricalEvents int64     `json:"historical_events"`
	SignalEvents     int64     `json:"signal_events"`
	LastEventTime    time.Time `json:"last_event_time"`
}

// NewEventHub creates a new event hub
func NewEventHub(client *events.EventClient) *EventHub {
	ctx, cancel := context.WithCancel(context.Background())
	return &EventHub{
		client:          client,
		subscriptions:   make([]*Subscription, 0),
		requestHandlers: make(map[string]RequestHandler),
		stats: EventStats{
			TickerStats: make(map[string]TickerStats),
			LastUpdated: utils.Now(),
		},
		watchedTickers: []string{},
		failedStreams:  make(map[string]SubscriptionConfig),
		ctx:            ctx,
		cancel:         cancel,
	}
}

// Start initializes the event hub and subscribes to events
func (h *EventHub) Start(ctx context.Context) error {
	var startupErrors []string
	var criticalError bool

	// Try to subscribe to all streams, but continue even if some fail

	// Subscribe to market live data
	if err := h.subscribeToMarketLiveData(ctx); err != nil {
		log.Printf("Warning: failed to subscribe to market live data: %v", err)
		startupErrors = append(startupErrors, fmt.Sprintf("live data: %v", err))
		h.registerFailedStream("live", events.SubjectMarketLiveAll)
	}

	// Subscribe to market daily data
	if err := h.subscribeToMarketDailyData(ctx); err != nil {
		log.Printf("Warning: failed to subscribe to market daily data: %v", err)
		startupErrors = append(startupErrors, fmt.Sprintf("daily data: %v", err))
		h.registerFailedStream("daily", events.SubjectMarketDailyAll)
	}

	// Subscribe to historical data
	if err := h.subscribeToHistoricalData(ctx); err != nil {
		log.Printf("Warning: failed to subscribe to historical data: %v", err)
		startupErrors = append(startupErrors, fmt.Sprintf("historical data: %v", err))
		h.registerFailedStream("historical", events.SubjectMarketHistoricalAll)
	}

	// Subscribe to signals
	if err := h.subscribeToSignals(ctx); err != nil {
		log.Printf("Warning: failed to subscribe to signals: %v", err)
		startupErrors = append(startupErrors, fmt.Sprintf("signals: %v", err))
		h.registerFailedStream("signals", events.SubjectSignalsAll)
	}

	// Register handler for historical data requests
	h.RegisterRequestHandler("historical", h.handleHistoricalDataRequest)

	// Subscribe to requests - this is critical for functionality
	if err := h.subscribeToRequests(ctx); err != nil {
		log.Printf("Error: failed to subscribe to requests: %v", err)
		startupErrors = append(startupErrors, fmt.Sprintf("requests: %v", err))
		h.registerFailedStream("requests", "requests.historical.*.*.*")
		criticalError = true
	}

	// Start stats reporter
	go h.reportStats(ctx)

	// Start background process to retry failed streams
	go h.retryFailedStreams()

	// Log startup status
	if len(startupErrors) > 0 {
		if criticalError {
			log.Printf("Event Hub started with critical errors: %v", startupErrors)
			return fmt.Errorf("failed to start critical components: %v", strings.Join(startupErrors, ", "))
		}
		log.Printf("Event Hub started with some streams unavailable: %v", startupErrors)
	} else {
		log.Printf("Event Hub started successfully with all streams")
	}

	return nil
}

// SetWatchedTickers updates the list of tickers to watch
func (h *EventHub) SetWatchedTickers(tickers []string) {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.watchedTickers = tickers

	// Initialize stats for each ticker
	for _, ticker := range tickers {
		if _, exists := h.stats.TickerStats[ticker]; !exists {
			h.stats.TickerStats[ticker] = TickerStats{
				LastEventTime: time.Now(),
			}
		}
	}
}

// RegisterRequestHandler registers a handler for a specific request type
func (h *EventHub) RegisterRequestHandler(requestType string, handler RequestHandler) {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.requestHandlers[requestType] = handler
}

// subscribeToMarketLiveData subscribes to all live market data events
func (h *EventHub) subscribeToMarketLiveData(ctx context.Context) error {
	_, err := h.client.SubscribeMarketLiveData("*", func(data []byte) {
		// Update stats
		h.mu.Lock()
		h.stats.TotalEvents++
		h.stats.LiveEvents++
		h.stats.LastUpdated = time.Now()
		h.mu.Unlock()

		// Process and route live market data
		var marketData map[string]interface{}
		if err := json.Unmarshal(data, &marketData); err != nil {
			log.Printf("Error unmarshaling live market data: %v", err)
			return
		}

		// Extract ticker and update ticker-specific stats
		if ticker, ok := marketData["ticker"].(string); ok {
			h.mu.Lock()
			stats, exists := h.stats.TickerStats[ticker]
			if !exists {
				stats = TickerStats{}
			}
			stats.LiveEvents++
			stats.LastEventTime = time.Now()
			h.stats.TickerStats[ticker] = stats
			h.mu.Unlock()

			log.Printf("Processed live market data for %s", ticker)
		}
	})

	if err != nil {
		return err
	}

	h.mu.Lock()
	h.subscriptions = append(h.subscriptions, &Subscription{
		Subject:  events.SubjectMarketLiveAll,
		Handler:  func(data []byte) {},
		Consumer: "EventHub",
	})
	h.mu.Unlock()

	log.Printf("Subscribed to live market data")
	return nil
}

// subscribeToMarketDailyData subscribes to daily market data events
func (h *EventHub) subscribeToMarketDailyData(ctx context.Context) error {
	_, err := h.client.SubscribeMarketDailyData("*", func(data []byte) {
		// Update stats
		h.mu.Lock()
		h.stats.TotalEvents++
		h.stats.DailyEvents++
		h.stats.LastUpdated = time.Now()
		h.mu.Unlock()

		// Process and route daily market data
		var marketData map[string]interface{}
		if err := json.Unmarshal(data, &marketData); err != nil {
			log.Printf("Error unmarshaling daily market data: %v", err)
			return
		}

		// Extract ticker and update ticker-specific stats
		if ticker, ok := marketData["ticker"].(string); ok {
			h.mu.Lock()
			stats, exists := h.stats.TickerStats[ticker]
			if !exists {
				stats = TickerStats{}
			}
			stats.DailyEvents++
			stats.LastEventTime = time.Now()
			h.stats.TickerStats[ticker] = stats
			h.mu.Unlock()

			log.Printf("Processed daily market data for %s", ticker)
		}
	})

	if err != nil {
		return err
	}

	h.mu.Lock()
	h.subscriptions = append(h.subscriptions, &Subscription{
		Subject:  events.SubjectMarketDailyAll,
		Handler:  func(data []byte) {},
		Consumer: "EventHub",
	})
	h.mu.Unlock()

	log.Printf("Subscribed to daily market data")
	return nil
}

// subscribeToHistoricalData subscribes to historical data events
func (h *EventHub) subscribeToHistoricalData(ctx context.Context) error {
	_, err := h.client.SubscribeHistoricalData("*", "*", 0, func(data []byte) {
		// Update stats
		h.mu.Lock()
		h.stats.TotalEvents++
		h.stats.HistoricalEvents++
		h.stats.LastUpdated = time.Now()
		h.mu.Unlock()

		// Process historical data
		var histData map[string]interface{}
		if err := json.Unmarshal(data, &histData); err != nil {
			log.Printf("Error unmarshaling historical data: %v", err)
			return
		}

		// Extract metadata
		metadata, ok := histData["metadata"].(map[string]interface{})
		if !ok {
			log.Printf("Historical data missing metadata")
			return
		}

		ticker, _ := metadata["ticker"].(string)
		if ticker != "" {
			h.mu.Lock()
			stats, exists := h.stats.TickerStats[ticker]
			if !exists {
				stats = TickerStats{}
			}
			stats.HistoricalEvents++
			stats.LastEventTime = time.Now()
			h.stats.TickerStats[ticker] = stats
			h.mu.Unlock()

			chunkInfo := ""
			if chunk, ok := metadata["chunk"].(float64); ok {
				totalChunks, _ := metadata["total_chunks"].(float64)
				chunkInfo = fmt.Sprintf(" (chunk %d/%d)", int(chunk), int(totalChunks))
			}

			log.Printf("Processed historical data for %s%s", ticker, chunkInfo)
		}
	})

	if err != nil {
		return err
	}

	h.mu.Lock()
	h.subscriptions = append(h.subscriptions, &Subscription{
		Subject:  events.SubjectMarketHistoricalAll,
		Handler:  func(data []byte) {},
		Consumer: "EventHub",
	})
	h.mu.Unlock()

	log.Printf("Subscribed to historical market data")
	return nil
}

// subscribeToSignals subscribes to trading signal events
func (h *EventHub) subscribeToSignals(ctx context.Context) error {
	_, err := h.client.SubscribeSignals("*", func(data []byte) {
		// Update stats
		h.mu.Lock()
		h.stats.TotalEvents++
		h.stats.SignalEvents++
		h.stats.LastUpdated = time.Now()
		h.mu.Unlock()

		// Process signal data
		var signalData map[string]interface{}
		if err := json.Unmarshal(data, &signalData); err != nil {
			log.Printf("Error unmarshaling signal data: %v", err)
			return
		}

		// Extract ticker and update ticker-specific stats
		if ticker, ok := signalData["ticker"].(string); ok {
			h.mu.Lock()
			stats, exists := h.stats.TickerStats[ticker]
			if !exists {
				stats = TickerStats{}
			}
			stats.SignalEvents++
			stats.LastEventTime = time.Now()
			h.stats.TickerStats[ticker] = stats
			h.mu.Unlock()

			signalType, _ := signalData["signal_type"].(string)
			log.Printf("Processed %s signal for %s", signalType, ticker)
		}
	})

	if err != nil {
		return err
	}

	h.mu.Lock()
	h.subscriptions = append(h.subscriptions, &Subscription{
		Subject:  events.SubjectSignalsAll,
		Handler:  func(data []byte) {},
		Consumer: "EventHub",
	})
	h.mu.Unlock()

	log.Printf("Subscribed to trading signals")
	return nil
}

// subscribeToRequests subscribes to data request events
func (h *EventHub) subscribeToRequests(ctx context.Context) error {
	// Subscribe to historical data requests
	_, err := h.client.SubscribeHistoricalRequests(func(ticker, timeframe string, days int, reqData []byte) {
		// Update stats
		h.mu.Lock()
		h.stats.TotalEvents++
		h.stats.Requests++
		h.stats.LastUpdated = time.Now()
		h.mu.Unlock()

		log.Printf("Received request: historical data for %s (%s, %d days)", ticker, timeframe, days)

		// Find handler for the request type
		h.mu.Lock()
		handler, ok := h.requestHandlers["historical"]
		h.mu.Unlock()

		if !ok {
			log.Printf("No handler registered for historical data requests")
			return
		}

		// Process request
		if err := handler(ctx, ticker, timeframe, days, reqData); err != nil {
			log.Printf("Error handling historical data request: %v", err)
			h.mu.Lock()
			h.stats.ErrorCount++
			h.mu.Unlock()
		}
	})

	if err != nil {
		return err
	}

	h.mu.Lock()
	h.subscriptions = append(h.subscriptions, &Subscription{
		Subject:  "requests.historical.*.*.*",
		Handler:  func(data []byte) {},
		Consumer: "EventHub",
	})
	h.mu.Unlock()

	log.Printf("Subscribed to data requests")
	return nil
}

// handleHistoricalDataRequest processes a request for historical data
func (h *EventHub) handleHistoricalDataRequest(ctx context.Context, ticker, timeframe string, days int, reqData []byte) error {
	log.Printf("Processing historical data request for %s (%s, %d days)", ticker, timeframe, days)

	// Parse request details
	var request map[string]interface{}
	if err := json.Unmarshal(reqData, &request); err != nil {
		return fmt.Errorf("failed to parse request: %w", err)
	}

	// Extract requestID if available
	requestID, _ := request["request_id"].(string)
	if requestID == "" {
		requestID = fmt.Sprintf("%s-%s-%d-%d", ticker, timeframe, days, time.Now().UnixNano())
	}

	// For now, we just forward this request to the market data service
	// In a real implementation, we might check cache, validate parameters, etc.
	forwardRequest := map[string]interface{}{
		"request_id": requestID,
		"ticker":     ticker,
		"timeframe":  timeframe,
		"days":       days,
		"source":     "event_hub",
		"timestamp":  utils.FormatTime(utils.Now(), time.RFC3339),
	}

	// Forward the request
	return h.client.RequestHistoricalData(ctx, ticker, timeframe, days, forwardRequest)
}

// reportStats periodically logs event statistics
func (h *EventHub) reportStats(ctx context.Context) {
	ticker := time.NewTicker(1 * time.Minute)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			h.mu.Lock()
			totalEvents := h.stats.TotalEvents
			liveEvents := h.stats.LiveEvents
			dailyEvents := h.stats.DailyEvents
			histEvents := h.stats.HistoricalEvents
			signalEvents := h.stats.SignalEvents
			reqEvents := h.stats.Requests
			errCount := h.stats.ErrorCount
			h.mu.Unlock()

			log.Printf("Event Hub Stats - Total: %d (Live: %d, Daily: %d, Historical: %d, Signals: %d, Requests: %d, Errors: %d)",
				totalEvents, liveEvents, dailyEvents, histEvents, signalEvents, reqEvents, errCount)

			// Log per-ticker stats for active tickers (with recent events)
			h.mu.Lock()
			activeTickerCount := 0
			for ticker, stats := range h.stats.TickerStats {
				// Only log stats for tickers with activity in the last 10 minutes
				if time.Since(stats.LastEventTime) < 10*time.Minute {
					activeTickerCount++
					log.Printf("  %s: Live: %d, Daily: %d, Historical: %d, Signals: %d, Last: %s",
						ticker, stats.LiveEvents, stats.DailyEvents, stats.HistoricalEvents,
						stats.SignalEvents, utils.FormatTime(stats.LastEventTime, "15:04:05"))
				}
			}
			h.mu.Unlock()

			if activeTickerCount == 0 {
				log.Printf("  No active tickers in the last 10 minutes")
			}
		}
	}
}

// GetStats returns the current statistics
func (h *EventHub) GetStats() EventStats {
	h.mu.Lock()
	defer h.mu.Unlock()

	// Return a copy to avoid concurrent modification
	stats := h.stats

	// Copy the ticker stats map
	stats.TickerStats = make(map[string]TickerStats, len(h.stats.TickerStats))
	for ticker, tickerStats := range h.stats.TickerStats {
		stats.TickerStats[ticker] = tickerStats
	}

	return stats
}

// registerFailedStream adds a stream to the failed streams map for later retry
func (h *EventHub) registerFailedStream(streamType, subject string) {
	h.mu.Lock()
	defer h.mu.Unlock()

	h.failedStreams[streamType] = SubscriptionConfig{
		Type:      streamType,
		Subject:   subject,
		LastRetry: time.Now(),
	}
}

// retryFailedStreams periodically attempts to subscribe to failed streams
func (h *EventHub) retryFailedStreams() {
	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-h.ctx.Done():
			return
		case <-ticker.C:
			h.retryStreams()
		}
	}
}

// retryStreams attempts to resubscribe to all failed streams
func (h *EventHub) retryStreams() {
	h.mu.Lock()
	// Make a copy of failed streams to avoid holding the lock during subscription attempts
	failedStreams := make(map[string]SubscriptionConfig)
	for k, v := range h.failedStreams {
		failedStreams[k] = v
	}
	h.mu.Unlock()

	if len(failedStreams) == 0 {
		return
	}

	log.Printf("Attempting to reconnect to %d failed streams", len(failedStreams))

	for streamType := range failedStreams {
		var err error

		switch streamType {
		case "live":
			err = h.subscribeToMarketLiveData(h.ctx)
		case "daily":
			err = h.subscribeToMarketDailyData(h.ctx)
		case "historical":
			err = h.subscribeToHistoricalData(h.ctx)
		case "signals":
			err = h.subscribeToSignals(h.ctx)
		case "requests":
			err = h.subscribeToRequests(h.ctx)
		}

		// If successful, remove from failed streams
		if err == nil {
			h.mu.Lock()
			delete(h.failedStreams, streamType)
			h.mu.Unlock()
			log.Printf("Successfully reconnected to %s stream", streamType)
		} else {
			log.Printf("Failed to reconnect to %s stream: %v", streamType, err)
			// Update last retry time
			h.mu.Lock()
			if config, exists := h.failedStreams[streamType]; exists {
				config.LastRetry = time.Now()
				h.failedStreams[streamType] = config
			}
			h.mu.Unlock()
		}
	}
}

// GetStreamStatus returns the current status of all streams
func (h *EventHub) GetStreamStatus() map[string]bool {
	h.mu.Lock()
	defer h.mu.Unlock()

	status := map[string]bool{
		"live":       true,
		"daily":      true,
		"historical": true,
		"signals":    true,
		"requests":   true,
	}

	// Mark failed streams as false
	for streamType := range h.failedStreams {
		status[streamType] = false
	}

	return status
}

// Close stops all subscriptions and cleans up resources
func (h *EventHub) Close() {
	h.mu.Lock()
	defer h.mu.Unlock()

	log.Printf("Shutting down Event Hub with %d active subscriptions", len(h.subscriptions))

	// Cancel context to stop background goroutines
	if h.cancel != nil {
		h.cancel()
	}

	// Client handles the NATS connections
}

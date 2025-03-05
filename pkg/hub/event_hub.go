package hub

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"sync"
	"time"

	"github.com/myapp/tradinglab/pkg/events"
)

// EventHub manages the routing and storage of events
type EventHub struct {
	client        *events.EventClient
	subscriptions []*Subscription
	mu            sync.Mutex
	stats         EventStats
}

// Subscription represents a subscription to an event stream
type Subscription struct {
	Subject string
	Handler func([]byte)
}

// EventStats tracks statistics about events
type EventStats struct {
	TotalEvents     int64
	MarketDataCount int64
	SignalsCount    int64
	LastUpdated     time.Time
}

// NewEventHub creates a new event hub
func NewEventHub(client *events.EventClient) *EventHub {
	return &EventHub{
		client:        client,
		subscriptions: make([]*Subscription, 0),
		stats:         EventStats{LastUpdated: time.Now()},
	}
}

// Start initializes the event hub and subscribes to all relevant topics
func (h *EventHub) Start(ctx context.Context) error {
	// Subscribe to all market data
	if err := h.subscribeToMarketData(ctx); err != nil {
		return fmt.Errorf("failed to subscribe to market data: %w", err)
	}

	// Subscribe to all signals
	if err := h.subscribeToSignals(ctx); err != nil {
		return fmt.Errorf("failed to subscribe to signals: %w", err)
	}

	// Start stats reporter
	go h.reportStats(ctx)

	return nil
}

// subscribeToMarketData subscribes to all market data events
func (h *EventHub) subscribeToMarketData(ctx context.Context) error {
	marketDataHandler := func(data []byte) {
		h.mu.Lock()
		h.stats.TotalEvents++
		h.stats.MarketDataCount++
		h.stats.LastUpdated = time.Now()
		h.mu.Unlock()

		// Process market data
		var event map[string]interface{}
		if err := json.Unmarshal(data, &event); err != nil {
			log.Printf("Error unmarshaling market data: %v", err)
			return
		}

		ticker, ok := event["ticker"].(string)
		if !ok {
			log.Printf("Market data missing ticker")
			return
		}

		log.Printf("Processed market data for %s", ticker)
	}

	// Subscribe to all market data
	_, err := h.client.SubscribeMarketData("*", marketDataHandler)
	if err != nil {
		return err
	}

	h.mu.Lock()
	h.subscriptions = append(h.subscriptions, &Subscription{
		Subject: "market.data.*",
		Handler: marketDataHandler,
	})
	h.mu.Unlock()

	log.Printf("Subscribed to all market data")
	return nil
}

// subscribeToSignals subscribes to all signal events
func (h *EventHub) subscribeToSignals(ctx context.Context) error {
	signalHandler := func(data []byte) {
		h.mu.Lock()
		h.stats.TotalEvents++
		h.stats.SignalsCount++
		h.stats.LastUpdated = time.Now()
		h.mu.Unlock()

		// Process signal data
		var event map[string]interface{}
		if err := json.Unmarshal(data, &event); err != nil {
			log.Printf("Error unmarshaling signal: %v", err)
			return
		}

		ticker, ok := event["ticker"].(string)
		if !ok {
			log.Printf("Signal missing ticker")
			return
		}

		signalType, ok := event["signal_type"].(string)
		if !ok {
			log.Printf("Signal missing signal_type")
			return
		}

		log.Printf("Processed %s signal for %s", signalType, ticker)

		// Here we would apply any additional processing, filtering,
		// or forwarding to other services like the notification service
	}

	// Get signal subscription through event client
	// This is a placeholder - in a real implementation, you would use the client's
	// method for subscribing to signals
	_, err := h.client.SubscribeMarketData("*", signalHandler)
	if err != nil {
		return err
	}

	h.mu.Lock()
	h.subscriptions = append(h.subscriptions, &Subscription{
		Subject: "signals.*",
		Handler: signalHandler,
	})
	h.mu.Unlock()

	log.Printf("Subscribed to all signals")
	return nil
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
			stats := h.stats
			h.mu.Unlock()

			log.Printf("Event Hub Stats - Total: %d, Market Data: %d, Signals: %d, Last Updated: %v",
				stats.TotalEvents, stats.MarketDataCount, stats.SignalsCount, stats.LastUpdated)
		}
	}
}

// GetStats returns current statistics
func (h *EventHub) GetStats() EventStats {
	h.mu.Lock()
	defer h.mu.Unlock()
	return h.stats
}

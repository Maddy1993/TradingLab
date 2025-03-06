// pkg/events/client.go
package events

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"strings"
	"time"

	"github.com/nats-io/nats.go"
)

// EventClient handles publishing and subscribing to the event system
type EventClient struct {
	conn    *nats.Conn
	js      nats.JetStreamContext
	streams map[string]bool // Tracks created streams
}

// NewEventClient creates a new client connected to NATS and sets up streams
func NewEventClient(natsURL string) (*EventClient, error) {
	// Connect to NATS
	nc, err := nats.Connect(natsURL,
		nats.RetryOnFailedConnect(true),
		nats.MaxReconnects(10),
		nats.ReconnectWait(2*time.Second))
	if err != nil {
		return nil, err
	}

	// Create JetStream context
	js, err := nc.JetStream()
	if err != nil {
		return nil, err
	}

	client := &EventClient{
		conn:    nc,
		js:      js,
		streams: make(map[string]bool),
	}

	// Set up all streams
	if err := client.setupStreams(); err != nil {
		return nil, err
	}

	return client, nil
}

// setupStreams creates all required streams
func (c *EventClient) setupStreams() error {
	configs := GetStreamConfigs()
	for _, cfg := range configs {
		if err := c.createOrUpdateStream(cfg); err != nil {
			return fmt.Errorf("failed to setup stream %s: %w", cfg.Name, err)
		}
		c.streams[cfg.Name] = true
	}
	return nil
}

// createOrUpdateStream creates or updates a stream
func (c *EventClient) createOrUpdateStream(cfg StreamConfig) error {
	streamCfg := &nats.StreamConfig{
		Name:     cfg.Name,
		Subjects: cfg.Subjects,
		MaxAge:   time.Duration(cfg.MaxAge),
		Storage:  cfg.Storage,
		Replicas: cfg.Replicas,
		Discard:  cfg.Discard,
	}

	_, err := c.js.AddStream(streamCfg)
	if err != nil {
		if strings.Contains(err.Error(), "stream name already in use") {
			// Update existing stream
			_, err = c.js.UpdateStream(streamCfg)
			if err != nil {
				return err
			}
			log.Printf("Updated existing stream: %s", cfg.Name)
		} else {
			return err
		}
	} else {
		log.Printf("Created new stream: %s", cfg.Name)
	}

	return nil
}

// PublishMarketLiveData publishes live market data
func (c *EventClient) PublishMarketLiveData(ctx context.Context, ticker string, data interface{}) error {
	subject := fmt.Sprintf(SubjectMarketLiveTicker, ticker)
	payload, err := json.Marshal(data)
	if err != nil {
		return err
	}

	_, err = c.js.Publish(subject, payload)
	return err
}

// PublishMarketDailyData publishes daily market data
func (c *EventClient) PublishMarketDailyData(ctx context.Context, ticker string, data interface{}) error {
	subject := fmt.Sprintf(SubjectMarketDailyTicker, ticker)
	payload, err := json.Marshal(data)
	if err != nil {
		return err
	}

	_, err = c.js.Publish(subject, payload)
	return err
}

// PublishHistoricalData publishes historical market data
func (c *EventClient) PublishHistoricalData(ctx context.Context, ticker, timeframe string, days int, data interface{}) error {
	subject := fmt.Sprintf(SubjectMarketHistoricalData, ticker, timeframe, days)
	payload, err := json.Marshal(data)
	if err != nil {
		return err
	}

	_, err = c.js.Publish(subject, payload)
	return err
}

// RequestHistoricalData requests historical data for a ticker
func (c *EventClient) RequestHistoricalData(ctx context.Context, ticker, timeframe string, days int, requestData interface{}) error {
	subject := fmt.Sprintf(SubjectRequestsHistorical, ticker, timeframe, days)
	payload, err := json.Marshal(requestData)
	if err != nil {
		return err
	}

	_, err = c.js.Publish(subject, payload)
	return err
}

// SubscribeMarketLiveData subscribes to live market data for a ticker
func (c *EventClient) SubscribeMarketLiveData(ticker string, handler func([]byte)) (*nats.Subscription, error) {
	subject := fmt.Sprintf(SubjectMarketLiveTicker, ticker)
	return c.js.Subscribe(subject, func(msg *nats.Msg) {
		handler(msg.Data)
		msg.Ack()
	}, nats.DeliverAll())
}

// SubscribeMarketDailyData subscribes to daily market data for a ticker
func (c *EventClient) SubscribeMarketDailyData(ticker string, handler func([]byte)) (*nats.Subscription, error) {
	subject := fmt.Sprintf(SubjectMarketDailyTicker, ticker)
	return c.js.Subscribe(subject, func(msg *nats.Msg) {
		handler(msg.Data)
		msg.Ack()
	}, nats.DeliverAll())
}

// SubscribeHistoricalData subscribes to historical data for specific parameters
func (c *EventClient) SubscribeHistoricalData(ticker, timeframe string, days int, handler func([]byte)) (*nats.Subscription, error) {
	subject := fmt.Sprintf(SubjectMarketHistoricalData, ticker, timeframe, days)
	return c.js.Subscribe(subject, func(msg *nats.Msg) {
		handler(msg.Data)
		msg.Ack()
	}, nats.DeliverAll())
}

// SubscribeHistoricalRequests subscribes to historical data requests
func (c *EventClient) SubscribeHistoricalRequests(handler func(string, string, int, []byte)) (*nats.Subscription, error) {
	subject := "requests.historical.*.*.*"
	return c.js.Subscribe(subject, func(msg *nats.Msg) {
		// Parse subject to extract parameters
		parts := strings.Split(msg.Subject, ".")
		if len(parts) >= 5 {
			ticker := parts[2]
			timeframe := parts[3]
			var days int
			fmt.Sscanf(parts[4], "%d", &days)

			handler(ticker, timeframe, days, msg.Data)
			msg.Ack()
		}
	}, nats.DeliverAll())
}

// PublishSignal publishes a trading signal
func (c *EventClient) PublishSignal(ctx context.Context, ticker string, signalData interface{}) error {
	subject := fmt.Sprintf(SubjectSignalsTicker, ticker)
	payload, err := json.Marshal(signalData)
	if err != nil {
		return err
	}

	_, err = c.js.Publish(subject, payload)
	return err
}

// SubscribeSignals subscribes to trading signals for a ticker
func (c *EventClient) SubscribeSignals(ticker string, handler func([]byte)) (*nats.Subscription, error) {
	subject := fmt.Sprintf(SubjectSignalsTicker, ticker)
	return c.js.Subscribe(subject, func(msg *nats.Msg) {
		handler(msg.Data)
		msg.Ack()
	}, nats.DeliverAll())
}

// GetNATS returns the underlying NATS connection
func (c *EventClient) GetNATS() *nats.Conn {
	return c.conn
}

// Close closes the connection to NATS
func (c *EventClient) Close() {
	if c.conn != nil {
		c.conn.Close()
	}
}

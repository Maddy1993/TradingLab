// pkg/events/client.go
package events

import (
	"context"
	"encoding/json"
	"time"

	"github.com/nats-io/nats.go"
)

// EventClient handles publishing and subscribing to the event system
type EventClient struct {
	conn      *nats.Conn
	js        nats.JetStreamContext
	streamCfg *nats.StreamConfig
}

// NewEventClient creates a new client connected to NATS
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

	// Create default stream for market data
	streamCfg := &nats.StreamConfig{
		Name:     "MARKET_DATA",
		Subjects: []string{"market.data.*"},
		Storage:  nats.FileStorage,
		MaxAge:   24 * time.Hour,
	}

	_, err = js.AddStream(streamCfg)
	if err != nil {
		// Stream might already exist
		// Fix: Check if error code/type indicates the stream already exists
		if err.Error() == "nats: stream name already in use" {
			// Stream exists, ignore error
			_, err = js.UpdateStream(streamCfg)
			if err != nil {
				return nil, err
			}
		} else {
			return nil, err
		}
	}

	return &EventClient{
		conn:      nc,
		js:        js,
		streamCfg: streamCfg,
	}, nil
}

// PublishMarketData publishes market data events
func (c *EventClient) PublishMarketData(ctx context.Context, ticker string, data interface{}) error {
	subject := "market.data." + ticker
	payload, err := json.Marshal(data)
	if err != nil {
		return err
	}

	_, err = c.js.Publish(subject, payload)
	return err
}

// SubscribeMarketData subscribes to market data events
func (c *EventClient) SubscribeMarketData(ticker string, handler func([]byte)) (*nats.Subscription, error) {
	subject := "market.data." + ticker
	return c.js.Subscribe(subject, func(msg *nats.Msg) {
		handler(msg.Data)
		msg.Ack()
	}, nats.DeliverAll())
}

// Close closes the connection to NATS
func (c *EventClient) Close() {
	if c.conn != nil {
		c.conn.Close()
	}
}

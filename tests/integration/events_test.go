// tests/integration/events_test.go
package integration

import (
	"context"
	"encoding/json"
	"log"
	"os"
	"testing"
	"time"

	"github.com/myapp/tradinglab/pkg/events"
)

// TestEventFlow tests the complete flow of events through the system
func TestEventFlow(t *testing.T) {
	// Get NATS URL from environment or use default for testing
	natsURL := os.Getenv("NATS_URL")
	if natsURL == "" {
		natsURL = "nats://localhost:4222"
	}

	// Create context with timeout
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	// Create publisher client
	publisher, err := events.NewEventClient(natsURL)
	if err != nil {
		t.Fatalf("Failed to create publisher client: %v", err)
	}
	defer publisher.Close()

	// Create subscriber client
	subscriber, err := events.NewEventClient(natsURL)
	if err != nil {
		t.Fatalf("Failed to create subscriber client: %v", err)
	}
	defer subscriber.Close()

	// Create a channel to receive test events
	receivedEvents := make(chan map[string]interface{}, 5)

	// Subscribe to test events
	testTicker := "TEST_TICKER"
	_, err = subscriber.SubscribeMarketData(testTicker, func(data []byte) {
		var event map[string]interface{}
		if err := json.Unmarshal(data, &event); err != nil {
			t.Errorf("Failed to unmarshal event: %v", err)
			return
		}
		receivedEvents <- event
	})
	if err != nil {
		t.Fatalf("Failed to subscribe to test events: %v", err)
	}

	// Allow time for subscription to be established
	time.Sleep(1 * time.Second)

	// Publish test events
	for i := 0; i < 3; i++ {
		testEvent := map[string]interface{}{
			"ticker":    testTicker,
			"timestamp": time.Now().Format(time.RFC3339),
			"price":     100.0 + float64(i),
			"volume":    1000 * (i + 1),
			"test_id":   i,
		}

		if err := publisher.PublishMarketData(ctx, testTicker, testEvent); err != nil {
			t.Fatalf("Failed to publish test event: %v", err)
		}
		log.Printf("Published test event %d", i)
	}

	// Collect events with timeout
	receivedCount := 0
	timeout := time.After(5 * time.Second)

	for receivedCount < 3 {
		select {
		case event := <-receivedEvents:
			log.Printf("Received event: %v", event)
			receivedCount++
		case <-timeout:
			t.Fatalf("Timed out waiting for events. Received %d of 3", receivedCount)
			return
		}
	}

	// Verify all events were received
	if receivedCount != 3 {
		t.Errorf("Expected 3 events, got %d", receivedCount)
	}
}

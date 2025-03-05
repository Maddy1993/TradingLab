// cmd/event-client/main.go
package main

import (
	"context"
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/myapp/tradinglab/pkg/events"
)

func main() {
	// Get NATS URL from environment or use default
	natsURL := os.Getenv("NATS_URL")
	if natsURL == "" {
		natsURL = "nats://localhost:4222"
	}

	log.Printf("Connecting to NATS server at %s", natsURL)

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

	// Subscribe to market data for example ticker
	ticker := "SPY"
	sub, err := client.SubscribeMarketData(ticker, func(data []byte) {
		log.Printf("Received data for %s: %s", ticker, string(data))
	})
	if err != nil {
		log.Fatalf("Failed to subscribe to market data: %v", err)
	}
	defer sub.Unsubscribe()

	// Publish example data periodically
	go func() {
		ticker := time.NewTicker(5 * time.Second)
		defer ticker.Stop()

		for {
			select {
			case <-ctx.Done():
				return
			case t := <-ticker.C:
				exampleData := map[string]interface{}{
					"ticker":    "SPY",
					"timestamp": t.Format(time.RFC3339),
					"price":     420.69,
					"open":      419.50,
					"high":      421.25,
					"low":       418.75,
					"close":     420.69,
					"volume":    1234567,
				}

				if err := client.PublishMarketData(ctx, "SPY", exampleData); err != nil {
					log.Printf("Failed to publish market data: %v", err)
				} else {
					log.Printf("Published market data for SPY")
				}
			}
		}
	}()

	// Keep running until signal received
	log.Println("Event client running. Press Ctrl+C to exit")
	<-ctx.Done()
	log.Println("Shutting down event client")
}

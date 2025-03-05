// cmd/event-hub/main.go
package main

import (
	"context"
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/myapp/tradinglab/pkg/events"
	"github.com/myapp/tradinglab/pkg/hub"
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

	// Get health server address from environment or use default
	healthAddr := os.Getenv("HEALTH_ADDR")
	if healthAddr == "" {
		healthAddr = ":8080"
	}

	log.Printf("Event Hub Service starting, connecting to NATS server at %s", natsURL)

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

	// Create event hub
	eventHub := hub.NewEventHub(client)

	// Start the event hub
	if err := eventHub.Start(ctx); err != nil {
		log.Fatalf("Failed to start event hub: %v", err)
	}

	// Start health server in a goroutine
	go func() {
		if err := eventHub.StartHealthServer(healthAddr); err != nil {
			log.Fatalf("Health server error: %v", err)
		}
	}()

	// Keep running until signal received
	log.Println("Event Hub running. Press Ctrl+C to exit")
	<-ctx.Done()
	log.Println("Shutting down Event Hub")
}

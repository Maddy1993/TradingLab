// cmd/market-data-service/main.go
package main

import (
	"context"
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/myapp/tradinglab/pkg/events"
	"github.com/myapp/tradinglab/pkg/market"
)

func main() {
	// Get NATS URL from environment or use default
	natsURL := os.Getenv("NATS_URL")
	if natsURL == "" {
		natsURL = "nats://localhost:4222"
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

	// Create market data provider
	provider, err := market.NewAlphaVantageProvider(os.Getenv("ALPHA_VANTAGE_API_KEY"))
	if err != nil {
		log.Fatalf("Failed to create market data provider: %v", err)
	}

	// Define tickers to watch
	tickers := []string{"SPY", "AAPL", "MSFT", "GOOGL"}

	// Start streaming data for each ticker
	for _, ticker := range tickers {
		go streamMarketData(ctx, client, provider, ticker)
	}

	// Keep running until signal received
	log.Println("Market Data Service running. Press Ctrl+C to exit")
	<-ctx.Done()
	log.Println("Shutting down Market Data Service")
}

func streamMarketData(ctx context.Context, client *events.EventClient, provider *market.AlphaVantageProvider, tickerSymbol string) {
	interval := 60 * time.Second
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
				log.Printf("Published market data for %s", tickerSymbol)
			}
		}
	}
}

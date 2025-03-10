// cmd/event-client/main.go
package main

import (
	"context"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/myapp/tradinglab/pkg/events"
	"github.com/myapp/tradinglab/pkg/utils"
)

func init() {
	// Set timezone to PST
	loc, err := time.LoadLocation("America/Los_Angeles")
	if err != nil {
		utils.Error("Failed to load PST timezone: %v", err)
		return
	}
	time.Local = loc
}

func main() {
	// Get NATS URL from environment or use default
	natsURL := os.Getenv("NATS_URL")
	if natsURL == "" {
		natsURL = "nats://localhost:4222"
	}

	utils.Info("Connecting to NATS server at %s", natsURL)

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

	// Subscribe to market data for example ticker
	ticker := "SPY"
	sub, err := client.SubscribeMarketLiveData(ticker, func(data []byte) {
		utils.Info("Received live data for %s: %s", ticker, string(data))
	})
	if err != nil {
		utils.Fatal("Failed to subscribe to market live data: %v", err)
	}
	defer sub.Unsubscribe()

	// Also subscribe to daily data
	dailySub, err := client.SubscribeMarketDailyData(ticker, func(data []byte) {
		utils.Info("Received daily data for %s: %s", ticker, string(data))
	})
	if err != nil {
		utils.Fatal("Failed to subscribe to market daily data: %v", err)
	}
	defer dailySub.Unsubscribe()

	// Publish example data periodically
	go func() {
		ticker := time.NewTicker(5 * time.Second)
		defer ticker.Stop()

		for {
			select {
			case <-ctx.Done():
				return
			case t := <-ticker.C:
				// Create example live market data
				liveData := map[string]interface{}{
					"ticker":    "SPY",
					"timestamp": t.Format(time.RFC3339),
					"price":     420.69,
					"open":      419.50,
					"high":      421.25,
					"low":       418.75,
					"close":     420.69,
					"volume":    1234567,
					"data_type": "live",
				}

				if err := client.PublishMarketLiveData(ctx, "SPY", liveData); err != nil {
					utils.Error("Failed to publish market live data: %v", err)
				} else {
					utils.Info("Published market live data for SPY")
				}

				// Every 30 seconds, publish daily data as well
				if t.Second()%30 == 0 {
					dailyData := map[string]interface{}{
						"ticker":    "SPY",
						"timestamp": t.Format(time.RFC3339),
						"price":     421.42,
						"open":      418.75,
						"high":      422.50,
						"low":       417.25,
						"close":     421.42,
						"volume":    15678901,
						"data_type": "daily",
					}

					if err := client.PublishMarketDailyData(ctx, "SPY", dailyData); err != nil {
						utils.Error("Failed to publish market daily data: %v", err)
					} else {
						utils.Info("Published market daily data for SPY")
					}
				}
			}
		}
	}()

	// Keep running until signal received
	utils.Info("Event client running. Press Ctrl+C to exit")
	<-ctx.Done()
	utils.Info("Shutting down event client")
}
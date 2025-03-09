package events

import "github.com/nats-io/nats.go"

// Stream definitions for the event system
const (
	// StreamMarketLive handles real-time market data during trading hours
	StreamMarketLive = "MARKET_LIVE"
	// StreamMarketDaily handles end-of-day market data
	StreamMarketDaily = "MARKET_DAILY"
	// StreamMarketHistorical handles historical market data requests
	StreamMarketHistorical = "MARKET_HISTORICAL"
	// StreamSignals handles trading signals
	StreamSignals = "SIGNALS"
	// StreamRecommendations handles options recommendations
	StreamRecommendations = "RECOMMENDATIONS"
	// StreamRequests handles data requests from clients
	StreamRequests = "REQUESTS"
)

// Subject patterns for each stream
const (
	// Subject patterns for market live data - ticker specific
	SubjectMarketLiveTicker = "market.live.%s" // e.g., market.live.AAPL
	SubjectMarketLiveAll    = "market.live.*"  // All tickers

	// Subject patterns for market daily data
	SubjectMarketDailyTicker = "market.daily.%s" // e.g., market.daily.AAPL
	SubjectMarketDailyAll    = "market.daily.*"  // All tickers

	// Subject patterns for historical data
	// Format: market.historical.{ticker}.{timeframe}.{days}
	SubjectMarketHistoricalRequest = "market.historical.request.%s.%s.%d" // ticker, timeframe, days
	SubjectMarketHistoricalData    = "market.historical.data.%s.%s.%d"    // ticker, timeframe, days
	SubjectMarketHistoricalAll     = "market.historical.data.>"           // All historical data (use > for multi-level wildcard)

	// Subject patterns for signals
	SubjectSignalsTicker = "signals.%s" // e.g., signals.AAPL
	SubjectSignalsAll    = "signals.*"  // All signals

	// Subject patterns for recommendations
	SubjectRecommendationsTicker = "recommendations.%s" // e.g., recommendations.AAPL
	SubjectRecommendationsAll    = "recommendations.*"  // All recommendations

	// Subject patterns for data requests
	SubjectRequestsHistorical = "requests.historical.%s.%s.%d" // ticker, timeframe, days
)

// StreamConfig defines the configuration for each stream
type StreamConfig struct {
	Name      string
	Subjects  []string
	MaxAge    int64 // In nanoseconds
	Storage   nats.StorageType
	Replicas  int
	Discard   nats.DiscardPolicy
	Retention nats.RetentionPolicy
}

// GetStreamConfigs returns all stream configurations
func GetStreamConfigs() []StreamConfig {
	return []StreamConfig{
		{
			Name:      StreamMarketLive,
			Subjects:  []string{SubjectMarketLiveAll},
			MaxAge:    24 * 60 * 60 * 1e9, // 24 hours in nanoseconds
			Storage:   nats.MemoryStorage,
			Replicas:  1,
			Discard:   nats.DiscardOld,
			Retention: nats.LimitsPolicy,
		},
		{
			Name:      StreamMarketDaily,
			Subjects:  []string{SubjectMarketDailyAll},
			MaxAge:    30 * 24 * 60 * 60 * 1e9, // 30 days in nanoseconds
			Storage:   nats.FileStorage,
			Replicas:  1,
			Discard:   nats.DiscardOld,
			Retention: nats.LimitsPolicy,
		},
		{
			Name:      StreamMarketHistorical,
			Subjects:  []string{SubjectMarketHistoricalAll},
			MaxAge:    30 * 24 * 60 * 60 * 1e9, // 30 days in nanoseconds
			Storage:   nats.FileStorage,
			Replicas:  1,
			Discard:   nats.DiscardOld,
			Retention: nats.LimitsPolicy,
		},
		{
			Name:      StreamSignals,
			Subjects:  []string{SubjectSignalsAll},
			MaxAge:    90 * 24 * 60 * 60 * 1e9, // 90 days in nanoseconds
			Storage:   nats.FileStorage,
			Replicas:  1,
			Discard:   nats.DiscardOld,
			Retention: nats.LimitsPolicy,
		},
		{
			Name:      StreamRecommendations,
			Subjects:  []string{SubjectRecommendationsAll},
			MaxAge:    30 * 24 * 60 * 60 * 1e9, // 30 days in nanoseconds
			Storage:   nats.FileStorage,
			Replicas:  1,
			Discard:   nats.DiscardOld,
			Retention: nats.LimitsPolicy,
		},
		{
			Name:      StreamRequests,
			Subjects:  []string{"requests.>"},
			MaxAge:    1 * 60 * 60 * 1e9, // 1 hour in nanoseconds
			Storage:   nats.MemoryStorage,
			Replicas:  1,
			Discard:   nats.DiscardOld,
			Retention: nats.WorkQueuePolicy, // Process each request once
		},
	}
}

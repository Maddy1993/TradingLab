// pkg/market/alpaca_provider.go
package market

import (
	"context"
	"fmt"
	"log"
	"os"
	"strings"
	"time"

	"github.com/alpacahq/alpaca-trade-api-go/v3/alpaca"
	"github.com/alpacahq/alpaca-trade-api-go/v3/marketdata"
)

// AlpacaProvider implements market data fetching from Alpaca API
type AlpacaProvider struct {
	alpacaClient     *alpaca.Client
	marketDataClient *marketdata.Client
	paperTrading     bool
	dataFeed         marketdata.Feed        // Data feed to use (IEX, SIP)
	lastValidData    map[string]*MarketData // Cache last valid data by ticker
}

// MarketData represents OHLCV market data
type MarketData struct {
	Ticker     string    `json:"ticker"`
	Timestamp  time.Time `json:"timestamp"`
	Price      float64   `json:"price"`
	Open       float64   `json:"open"`
	High       float64   `json:"high"`
	Low        float64   `json:"low"`
	Close      float64   `json:"close"`
	Volume     int64     `json:"volume"`
	VWAP       float64   `json:"vwap,omitempty"`
	TradeCount int       `json:"trade_count,omitempty"`
	Interval   string    `json:"interval"`
	Source     string    `json:"source"`
	DataType   string    `json:"data_type,omitempty"` // "live", "daily", "historical", "recent"
}

// NewAlpacaProvider creates a new Alpaca data provider using the official SDK
func NewAlpacaProvider(apiKey, apiSecret string, paperTrading bool) (*AlpacaProvider, error) {
	if apiKey == "" || apiSecret == "" {
		return nil, fmt.Errorf("Alpaca API key and secret are required")
	}

	// Configure the alpaca client
	alpacaCfg := alpaca.ClientOpts{
		APIKey:    apiKey,
		APISecret: apiSecret,
	}

	// Create alpaca client for account info and trading operations
	alpacaClient := alpaca.NewClient(alpacaCfg)

	// Create market data client for market data operations
	marketDataClient := marketdata.NewClient(marketdata.ClientOpts{
		APIKey:    apiKey,
		APISecret: apiSecret,
	})

	// Determine which data feed to use from environment variable
	dataFeed := marketdata.IEX // Default to IEX
	if feedEnv := os.Getenv("ALPACA_DATA_FEED"); feedEnv != "" {
		switch strings.ToUpper(feedEnv) {
		case "SIP":
			dataFeed = marketdata.SIP
		case "IEX":
			dataFeed = marketdata.IEX
		default:
			log.Printf("Warning: Unknown ALPACA_DATA_FEED value '%s', using default (IEX)", feedEnv)
		}
	}
	log.Printf("Using Alpaca data feed: %s", dataFeed)

	return &AlpacaProvider{
		alpacaClient:     alpacaClient,
		marketDataClient: marketDataClient,
		paperTrading:     paperTrading,
		dataFeed:         dataFeed,
		lastValidData:    make(map[string]*MarketData),
	}, nil
}

// IsMarketOpen checks if the market is currently open
func (p *AlpacaProvider) IsMarketOpen(ctx context.Context) (bool, error) {
	// Use the Alpaca SDK to get the market clock
	clock, err := p.alpacaClient.GetClock()
	if err != nil {
		return false, fmt.Errorf("failed to get market clock: %w", err)
	}

	return clock.IsOpen, nil
}

// GetLatestData fetches real-time market data for a ticker
func (p *AlpacaProvider) GetLatestData(ctx context.Context, ticker string) (*MarketData, error) {
	// Check if market is open
	isOpen, err := p.IsMarketOpen(ctx)
	if err != nil {
		log.Printf("Failed to check market status: %v", err)
		// Proceed with the attempt even if we can't check market status
	}

	if !isOpen {
		log.Printf("Market is closed, using most recent data for %s", ticker)
		return p.GetMostRecentData(ctx, ticker)
	}

	// Market is open, try to get live quotes
	request := marketdata.GetLatestQuoteRequest{
		Feed: p.dataFeed,
	}
	quote, err := p.marketDataClient.GetLatestQuote(ticker, request)
	if err != nil {
		log.Printf("Failed to get latest quote for %s: %v, falling back to bars", ticker, err)
		return p.GetMostRecentData(ctx, ticker)
	}

	// Get the latest 1-minute bar to complete OHLC data
	bar, err := p.getLatestMinuteBar(ctx, ticker)
	if err != nil {
		log.Printf("Failed to get latest minute bar: %v", err)
		// If we can't get the bar, use the quote data to create a partial record

		// Calculate mid price from quote
		bidPrice := quote.BidPrice
		askPrice := quote.AskPrice
		midPrice := (bidPrice + askPrice) / 2

		timestamp := quote.Timestamp

		data := &MarketData{
			Ticker:    ticker,
			Timestamp: timestamp,
			Price:     midPrice,
			Open:      midPrice, // Use mid price as a fallback
			High:      midPrice,
			Low:       midPrice,
			Close:     midPrice,
			Volume:    0, // Unknown
			Interval:  "1min",
			Source:    "Alpaca Quotes",
			DataType:  "live",
		}

		// Cache the data
		p.lastValidData[ticker] = data
		return data, nil
	}

	// Create market data that combines quote and bar
	// Calculate mid price from quote
	bidPrice := quote.BidPrice
	askPrice := quote.AskPrice
	midPrice := (bidPrice + askPrice) / 2

	// Get bar data
	barOpen := bar.Open
	barHigh := bar.High
	barLow := bar.Low
	barClose := bar.Close
	barVWAP := bar.VWAP

	data := &MarketData{
		Ticker:     ticker,
		Timestamp:  quote.Timestamp,
		Price:      midPrice,
		Open:       barOpen,
		High:       barHigh,
		Low:        barLow,
		Close:      barClose,
		Volume:     int64(bar.Volume),
		VWAP:       barVWAP,
		TradeCount: int(bar.TradeCount),
		Interval:   "1min",
		Source:     "Alpaca",
		DataType:   "live",
	}

	// Cache the valid data
	p.lastValidData[ticker] = data

	return data, nil
}

// GetMostRecentData fetches the most recent available data for a ticker
func (p *AlpacaProvider) GetMostRecentData(ctx context.Context, ticker string) (*MarketData, error) {
	// Try to get the most recent 1-minute bar
	bar, err := p.getLatestMinuteBar(ctx, ticker)
	if err == nil {
		barOpen := bar.Open
		barHigh := bar.High
		barLow := bar.Low
		barClose := bar.Close
		barVWAP := bar.VWAP

		data := &MarketData{
			Ticker:     ticker,
			Timestamp:  bar.Timestamp,
			Price:      barClose,
			Open:       barOpen,
			High:       barHigh,
			Low:        barLow,
			Close:      barClose,
			Volume:     int64(bar.Volume),
			VWAP:       barVWAP,
			TradeCount: int(bar.TradeCount),
			Interval:   "1min",
			Source:     "Alpaca",
			DataType:   "recent",
		}

		p.lastValidData[ticker] = data
		return data, nil
	}

	// If that fails, try to get the latest daily bar
	log.Printf("Failed to get latest minute bar for %s: %v, trying daily bar", ticker, err)

	// Get the most recent daily bar
	dailyBar, err := p.getLatestDailyBar(ctx, ticker)
	if err == nil {
		barOpen := dailyBar.Open
		barHigh := dailyBar.High
		barLow := dailyBar.Low
		barClose := dailyBar.Close
		barVWAP := dailyBar.VWAP

		data := &MarketData{
			Ticker:     ticker,
			Timestamp:  dailyBar.Timestamp,
			Price:      barClose,
			Open:       barOpen,
			High:       barHigh,
			Low:        barLow,
			Close:      barClose,
			Volume:     int64(dailyBar.Volume),
			VWAP:       barVWAP,
			TradeCount: int(dailyBar.TradeCount),
			Interval:   "1day",
			Source:     "Alpaca",
			DataType:   "recent",
		}

		p.lastValidData[ticker] = data
		return data, nil
	}

	// If all else fails, check if we have cached data
	if cachedData, ok := p.lastValidData[ticker]; ok {
		log.Printf("Using cached data for %s", ticker)
		// Return a copy with updated timestamp
		dataCopy := *cachedData
		dataCopy.Timestamp = time.Now()
		dataCopy.DataType = "cached"
		return &dataCopy, nil
	}

	// Last resort: generate sample data
	log.Printf("No data available for %s, generating sample data", ticker)
	return p.generateSampleData(ticker), nil
}

// GetDailyData fetches end-of-day data for a ticker
func (p *AlpacaProvider) GetDailyData(ctx context.Context, ticker string) (*MarketData, error) {
	dailyBar, err := p.getLatestDailyBar(ctx, ticker)
	if err != nil {
		return nil, fmt.Errorf("failed to get daily bar: %w", err)
	}

	barOpen := dailyBar.Open
	barHigh := dailyBar.High
	barLow := dailyBar.Low
	barClose := dailyBar.Close
	barVWAP := dailyBar.VWAP

	data := &MarketData{
		Ticker:     ticker,
		Timestamp:  dailyBar.Timestamp,
		Price:      barClose,
		Open:       barOpen,
		High:       barHigh,
		Low:        barLow,
		Close:      barClose,
		Volume:     int64(dailyBar.Volume),
		VWAP:       barVWAP,
		TradeCount: int(dailyBar.TradeCount),
		Interval:   "1day",
		Source:     "Alpaca",
		DataType:   "daily",
	}

	return data, nil
}

// GetHistoricalData fetches historical data for a ticker with specified parameters
func (p *AlpacaProvider) GetHistoricalData(ctx context.Context, ticker string, days int, timeframe string) ([]*MarketData, error) {
	// Convert timeframe to Alpaca format
	alpacaTimeframe, err := convertToAlpacaTimeframe(timeframe)
	if err != nil {
		return nil, err
	}

	// Calculate time range
	now := time.Now()
	end := now
	start := now.AddDate(0, 0, -days)

	// Get bars using the SDK
	barsRequest := marketdata.GetBarsRequest{
		TimeFrame:  alpacaTimeframe,
		Start:      start,
		End:        end,
		Adjustment: marketdata.Raw,
		Feed:       p.dataFeed,
	}

	// Get bars for the requested symbol
	bars, err := p.marketDataClient.GetBars(ticker, barsRequest)
	if err != nil {
		return nil, fmt.Errorf("failed to get historical bars: %w", err)
	}

	// Convert to MarketData array
	data := make([]*MarketData, 0, len(bars))
	for _, bar := range bars {
		barOpen := bar.Open
		barHigh := bar.High
		barLow := bar.Low
		barClose := bar.Close
		barVWAP := bar.VWAP

		marketData := &MarketData{
			Ticker:     ticker,
			Timestamp:  bar.Timestamp,
			Price:      barClose,
			Open:       barOpen,
			High:       barHigh,
			Low:        barLow,
			Close:      barClose,
			Volume:     int64(bar.Volume),
			VWAP:       barVWAP,
			TradeCount: int(bar.TradeCount),
			Interval:   timeframe,
			Source:     "Alpaca",
			DataType:   "historical",
		}

		data = append(data, marketData)
	}

	if len(data) == 0 {
		return nil, fmt.Errorf("no historical data found for %s", ticker)
	}

	return data, nil
}

// getLatestMinuteBar fetches the most recent 1-minute bar for a ticker
func (p *AlpacaProvider) getLatestMinuteBar(ctx context.Context, ticker string) (*marketdata.Bar, error) {
	// Get current time
	now := time.Now().UTC()

	// Set time range - last 15 minutes to now
	end := now
	start := now.Add(-15 * time.Minute)

	// Create bars request
	barsRequest := marketdata.GetBarsRequest{
		TimeFrame:  marketdata.OneMin,
		Start:      start,
		End:        end,
		TotalLimit: 1,
		Adjustment: marketdata.Raw,
		Feed:       p.dataFeed,
	}

	// Get bars for the requested symbol
	bars, err := p.marketDataClient.GetBars(ticker, barsRequest)
	if err != nil {
		return nil, fmt.Errorf("failed to get minute bars: %w", err)
	}

	if len(bars) == 0 {
		return nil, fmt.Errorf("no recent minute bars found for %s", ticker)
	}

	return &bars[len(bars)-1], nil
}

// getLatestDailyBar fetches the most recent daily bar for a ticker
func (p *AlpacaProvider) getLatestDailyBar(ctx context.Context, ticker string) (*marketdata.Bar, error) {
	// Get current time
	now := time.Now().UTC()

	// Set time range - last 3 days to now
	end := now
	start := now.AddDate(0, 0, -3)

	// Create bars request
	barsRequest := marketdata.GetBarsRequest{
		TimeFrame:  marketdata.OneDay,
		Start:      start,
		End:        end,
		TotalLimit: 1,
		Adjustment: marketdata.Raw,
		Feed:       p.dataFeed,
	}

	// Get bars for the requested symbol
	bars, err := p.marketDataClient.GetBars(ticker, barsRequest)
	if err != nil {
		return nil, fmt.Errorf("failed to get daily bars: %w", err)
	}

	if len(bars) == 0 {
		return nil, fmt.Errorf("no recent daily bars found for %s", ticker)
	}

	return &bars[len(bars)-1], nil
}

// generateSampleData creates dummy market data for testing when market is closed
func (p *AlpacaProvider) generateSampleData(ticker string) *MarketData {
	// Use the ticker to generate a somewhat realistic but random price
	var basePrice float64
	switch ticker {
	case "SPY":
		basePrice = 420.69
	case "AAPL":
		basePrice = 175.15
	case "MSFT":
		basePrice = 402.65
	case "GOOGL":
		basePrice = 140.23
	case "AMZN":
		basePrice = 175.90
	default:
		basePrice = 100.00
	}

	// Add small random variation
	now := time.Now()

	return &MarketData{
		Ticker:    ticker,
		Timestamp: now,
		Price:     basePrice,
		Open:      basePrice * 0.99,
		High:      basePrice * 1.01,
		Low:       basePrice * 0.98,
		Close:     basePrice,
		Volume:    500000 + (now.Unix() % 1000000), // Some pseudo-random volume
		Interval:  "1min",
		Source:    "Alpaca (Simulated)",
		DataType:  "generated",
	}
}

// convertToAlpacaTimeframe converts common interval notation to Alpaca timeframe format
func convertToAlpacaTimeframe(interval string) (marketdata.TimeFrame, error) {
	// Convert to lowercase for easier comparison
	interval = strings.ToLower(interval)

	switch interval {
	case "1m", "1min", "1minute":
		return marketdata.OneMin, nil
	case "5m", "5min", "5minute":
		return marketdata.NewTimeFrame(5, marketdata.Min), nil
	case "15m", "15min", "15minute":
		return marketdata.NewTimeFrame(15, marketdata.Min), nil
	case "30m", "30min", "30minute":
		return marketdata.NewTimeFrame(30, marketdata.Min), nil
	case "1h", "1hour", "60min":
		return marketdata.OneHour, nil
	case "1d", "1day", "daily":
		return marketdata.OneDay, nil
	default:
		return marketdata.TimeFrame{}, fmt.Errorf("unsupported interval: %s", interval)
	}
}

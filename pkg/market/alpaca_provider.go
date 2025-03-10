// pkg/market/alpaca_provider.go
package market

import (
	"context"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/alpacahq/alpaca-trade-api-go/v3/alpaca"
	"github.com/alpacahq/alpaca-trade-api-go/v3/marketdata"
	"github.com/myapp/tradinglab/pkg/utils"
)

// AlpacaProvider implements market data fetching from Alpaca API
type AlpacaProvider struct {
	alpacaClient     *alpaca.Client
	marketDataClient *marketdata.Client
	paperTrading     bool
	dataFeed         marketdata.Feed        // Data feed to use (IEX, SIP)
	lastValidData    map[string]*MarketData // Cache last valid data by ticker
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
			utils.Warn("Unknown ALPACA_DATA_FEED value '%s', using default (IEX)", feedEnv)
		}
	}
	utils.Info("Using Alpaca data feed: %s", dataFeed)

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
	utils.Debug("Making request to Alpaca API to get market clock")

	// Use the Alpaca SDK to get the market clock
	clock, err := p.alpacaClient.GetClock()
	if err != nil {
		// Check for 401 unauthorized error
		if strings.Contains(err.Error(), "request is not authorized") ||
			strings.Contains(err.Error(), "HTTP 401") {
			utils.Debug("Alpaca API authentication failed: %v", err)
			utils.Warn("Authentication failure when checking market status. This may be due to invalid API keys or expired credentials")

			// Set fallback value based on current time (Eastern Time)
			loc, _ := time.LoadLocation("America/New_York")
			now := time.Now().In(loc)

			// Regular market hours are 9:30 AM - 4:00 PM ET, Mon-Fri
			hour, min, sec := now.Clock()
			marketTime := hour*3600 + min*60 + sec

			// Check if current time is within market hours (9:30 AM - 4:00 PM ET)
			isWithinHours := marketTime >= 9*3600+30*60 && marketTime < 16*3600

			// Check if it's a weekday (Monday = 1, Sunday = 0)
			isWeekday := now.Weekday() > 0 && now.Weekday() < 6

			isOpen := isWithinHours && isWeekday
			utils.Info("Using fallback market hours calculation: market is %s",
				map[bool]string{true: "OPEN", false: "CLOSED"}[isOpen])

			return isOpen, nil // Don't return error to allow the system to continue functioning
		}

		utils.Error("Error getting market clock: %v", err)
		return false, fmt.Errorf("failed to get market clock: %w", err)
	}

	utils.Debug("Received response from Alpaca API: Market is %s",
		map[bool]string{true: "OPEN", false: "CLOSED"}[clock.IsOpen])
	return clock.IsOpen, nil
}

// GetLatestData fetches real-time market data for a ticker
func (p *AlpacaProvider) GetLatestData(ctx context.Context, ticker string) (*MarketData, error) {
	utils.Debug("Fetching latest data for ticker %s", ticker)

	// Check if market is open
	isOpen, err := p.IsMarketOpen(ctx)
	if err != nil {
		utils.Warn("Failed to check market status: %v", err)
		// Proceed with the attempt even if we can't check market status
	}

	if !isOpen {
		utils.Info("Market is closed, using most recent data for %s", ticker)
		return p.GetMostRecentData(ctx, ticker)
	}

	// Market is open, try to get live quotes
	request := marketdata.GetLatestQuoteRequest{
		Feed: p.dataFeed,
	}

	utils.Debug("Making request to Alpaca API for latest quote for %s using %s feed", ticker, p.dataFeed)
	quote, err := p.marketDataClient.GetLatestQuote(ticker, request)
	if err != nil {
		utils.Debug("Error getting latest quote for %s: %v", ticker, err)
		utils.Warn("Failed to get latest quote for %s: %v, falling back to bars", ticker, err)
		return p.GetMostRecentData(ctx, ticker)
	}

	utils.Debug("Received quote response for %s: bid=%.2f, ask=%.2f", ticker, quote.BidPrice, quote.AskPrice)

	// Get the latest 1-minute bar to complete OHLC data
	bar, err := p.getLatestMinuteBar(ctx, ticker)
	if err != nil {
		utils.Warn("Failed to get latest minute bar: %v", err)
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
	utils.Debug("Failed to get latest minute bar for %s: %v, trying daily bar", ticker, err)

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
		utils.Info("Using cached data for %s", ticker)
		// Return a copy with updated timestamp
		dataCopy := *cachedData
		dataCopy.Timestamp = time.Now()
		dataCopy.DataType = "cached"
		return &dataCopy, nil
	}

	// Last resort: generate sample data
	utils.Warn("No data available for %s, generating sample data", ticker)
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
	utils.Debug("Fetching historical data for %s, %d days, timeframe %s", ticker, days, timeframe)

	// Convert timeframe to Alpaca format
	alpacaTimeframe, err := convertToAlpacaTimeframe(timeframe)
	if err != nil {
		utils.Error("Invalid timeframe format: %s - %v", timeframe, err)
		return nil, err
	}

	// Calculate time range
	now := time.Now()
	end := now
	start := now.AddDate(0, 0, -days)
	utils.Debug("Historical data period: %s to %s", start.Format(time.RFC3339), end.Format(time.RFC3339))

	// Get bars using the SDK
	barsRequest := marketdata.GetBarsRequest{
		TimeFrame:  alpacaTimeframe,
		Start:      start,
		End:        end,
		Adjustment: marketdata.Raw,
		Feed:       p.dataFeed,
	}

	// Get bars for the requested symbol
	utils.Debug("Making request to Alpaca API for historical bars for %s", ticker)
	bars, err := p.marketDataClient.GetBars(ticker, barsRequest)
	if err != nil {
		utils.Error("Failed to get historical bars for %s: %v", ticker, err)
		return nil, fmt.Errorf("failed to get historical bars: %w", err)
	}

	utils.Debug("Received %d historical bars for %s", len(bars), ticker)

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

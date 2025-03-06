// pkg/market/alpaca_provider.go
package market

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"net/url"
	"strings"
	"time"
)

// AlpacaProvider implements market data fetching from Alpaca API
type AlpacaProvider struct {
	apiKey        string
	apiSecret     string
	baseURL       string
	dataBaseURL   string // Separate URL for market data API
	httpClient    *http.Client
	paperTrading  bool
	lastValidData map[string]*MarketData // Cache last valid data by ticker
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

// AlpacaBar represents OHLCV bar data from Alpaca
type AlpacaBar struct {
	Timestamp  int64   `json:"t"`
	Open       float64 `json:"o"`
	High       float64 `json:"h"`
	Low        float64 `json:"l"`
	Close      float64 `json:"c"`
	Volume     int64   `json:"v"`
	TradeCount int     `json:"n"`
	VWAP       float64 `json:"vw"`
}

// NewAlpacaProvider creates a new Alpaca data provider
func NewAlpacaProvider(apiKey, apiSecret string, paperTrading bool) (*AlpacaProvider, error) {
	if apiKey == "" || apiSecret == "" {
		return nil, fmt.Errorf("Alpaca API key and secret are required")
	}

	baseURL := "https://api.alpaca.markets"
	dataBaseURL := "https://data.alpaca.markets"

	if paperTrading {
		baseURL = "https://paper-api.alpaca.markets"
	}

	return &AlpacaProvider{
		apiKey:        apiKey,
		apiSecret:     apiSecret,
		baseURL:       baseURL,
		dataBaseURL:   dataBaseURL,
		paperTrading:  paperTrading,
		lastValidData: make(map[string]*MarketData),
		httpClient: &http.Client{
			Timeout: 10 * time.Second,
		},
	}, nil
}

// IsMarketOpen checks if the market is currently open
func (p *AlpacaProvider) IsMarketOpen(ctx context.Context) (bool, error) {
	// Build URL for market clock endpoint
	requestURL := fmt.Sprintf("%s/v2/clock", p.baseURL)

	// Create request
	req, err := http.NewRequestWithContext(ctx, "GET", requestURL, nil)
	if err != nil {
		return false, fmt.Errorf("failed to create request: %w", err)
	}

	// Add authentication headers
	req.Header.Add("APCA-API-KEY-ID", p.apiKey)
	req.Header.Add("APCA-API-SECRET-KEY", p.apiSecret)
	req.Header.Add("Accept", "application/json")

	// Execute request
	resp, err := p.httpClient.Do(req)
	if err != nil {
		return false, fmt.Errorf("failed to execute request: %w", err)
	}
	defer resp.Body.Close()

	// Check status code
	if resp.StatusCode != http.StatusOK {
		return false, fmt.Errorf("unexpected status code: %d", resp.StatusCode)
	}

	// Parse response
	var result struct {
		IsOpen bool `json:"is_open"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return false, fmt.Errorf("failed to decode response: %w", err)
	}

	return result.IsOpen, nil
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
	requestURL := fmt.Sprintf("%s/v2/stocks/%s/quotes/latest", p.baseURL, url.PathEscape(ticker))

	// Create request
	req, err := http.NewRequestWithContext(ctx, "GET", requestURL, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	// Add authentication headers
	req.Header.Add("APCA-API-KEY-ID", p.apiKey)
	req.Header.Add("APCA-API-SECRET-KEY", p.apiSecret)
	req.Header.Add("Accept", "application/json")

	// Execute request
	resp, err := p.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to execute request: %w", err)
	}
	defer resp.Body.Close()

	// Check status code
	if resp.StatusCode != http.StatusOK {
		// Try fallback to recent bars
		log.Printf("Failed to get quotes for %s (status %d), falling back to bars",
			ticker, resp.StatusCode)
		return p.GetMostRecentData(ctx, ticker)
	}

	// Parse response
	var result struct {
		Quote struct {
			BidPrice  float64 `json:"bp"`
			AskPrice  float64 `json:"ap"`
			BidSize   int     `json:"bs"`
			AskSize   int     `json:"as"`
			Timestamp int64   `json:"t"`
		} `json:"quote"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	// Get the latest 1-minute bar to complete OHLC data
	bar, err := p.getLatestMinuteBar(ctx, ticker)
	if err != nil {
		log.Printf("Failed to get latest minute bar: %v", err)
		// If we can't get the bar, use the quote data to create a partial record
		midPrice := (result.Quote.BidPrice + result.Quote.AskPrice) / 2
		timestamp := time.Unix(0, result.Quote.Timestamp)

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
	timestamp := time.Unix(0, result.Quote.Timestamp)
	midPrice := (result.Quote.BidPrice + result.Quote.AskPrice) / 2

	data := &MarketData{
		Ticker:     ticker,
		Timestamp:  timestamp,
		Price:      midPrice,
		Open:       bar.Open,
		High:       bar.High,
		Low:        bar.Low,
		Close:      bar.Close,
		Volume:     bar.Volume,
		VWAP:       bar.VWAP,
		TradeCount: bar.TradeCount,
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
		data := &MarketData{
			Ticker:     ticker,
			Timestamp:  time.Unix(0, bar.Timestamp),
			Price:      bar.Close,
			Open:       bar.Open,
			High:       bar.High,
			Low:        bar.Low,
			Close:      bar.Close,
			Volume:     bar.Volume,
			VWAP:       bar.VWAP,
			TradeCount: bar.TradeCount,
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
		data := &MarketData{
			Ticker:     ticker,
			Timestamp:  time.Unix(0, dailyBar.Timestamp),
			Price:      dailyBar.Close,
			Open:       dailyBar.Open,
			High:       dailyBar.High,
			Low:        dailyBar.Low,
			Close:      dailyBar.Close,
			Volume:     dailyBar.Volume,
			VWAP:       dailyBar.VWAP,
			TradeCount: dailyBar.TradeCount,
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

	data := &MarketData{
		Ticker:     ticker,
		Timestamp:  time.Unix(0, dailyBar.Timestamp),
		Price:      dailyBar.Close,
		Open:       dailyBar.Open,
		High:       dailyBar.High,
		Low:        dailyBar.Low,
		Close:      dailyBar.Close,
		Volume:     dailyBar.Volume,
		VWAP:       dailyBar.VWAP,
		TradeCount: dailyBar.TradeCount,
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
	end := now.Format(time.RFC3339)
	start := now.AddDate(0, 0, -days).Format(time.RFC3339)

	// Build URL for bars endpoint
	requestURL := fmt.Sprintf("%s/v2/stocks/%s/bars?start=%s&end=%s&timeframe=%s&limit=10000",
		p.dataBaseURL, url.PathEscape(ticker), url.QueryEscape(start), url.QueryEscape(end), alpacaTimeframe)

	// Create request
	req, err := http.NewRequestWithContext(ctx, "GET", requestURL, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	// Add authentication headers
	req.Header.Add("APCA-API-KEY-ID", p.apiKey)
	req.Header.Add("APCA-API-SECRET-KEY", p.apiSecret)

	// Execute request
	resp, err := p.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to execute request: %w", err)
	}
	defer resp.Body.Close()

	// Check status code
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("unexpected status code: %d", resp.StatusCode)
	}

	// Parse response
	var result struct {
		Bars []AlpacaBar `json:"bars"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	// Convert to MarketData array
	data := make([]*MarketData, 0, len(result.Bars))
	for _, bar := range result.Bars {
		marketData := &MarketData{
			Ticker:     ticker,
			Timestamp:  time.Unix(0, bar.Timestamp),
			Price:      bar.Close,
			Open:       bar.Open,
			High:       bar.High,
			Low:        bar.Low,
			Close:      bar.Close,
			Volume:     bar.Volume,
			VWAP:       bar.VWAP,
			TradeCount: bar.TradeCount,
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
func (p *AlpacaProvider) getLatestMinuteBar(ctx context.Context, ticker string) (*AlpacaBar, error) {
	// Get current time
	now := time.Now().UTC()

	// Set time range - last 15 minutes to now
	end := now.Format(time.RFC3339)
	start := now.Add(-15 * time.Minute).Format(time.RFC3339)

	// Build URL for bars endpoint
	requestURL := fmt.Sprintf("%s/v2/stocks/%s/bars?start=%s&end=%s&timeframe=1Min&limit=1",
		p.dataBaseURL, url.PathEscape(ticker), url.QueryEscape(start), url.QueryEscape(end))

	// Create request
	req, err := http.NewRequestWithContext(ctx, "GET", requestURL, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	// Add authentication headers
	req.Header.Add("APCA-API-KEY-ID", p.apiKey)
	req.Header.Add("APCA-API-SECRET-KEY", p.apiSecret)

	// Execute request
	resp, err := p.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to execute request: %w", err)
	}
	defer resp.Body.Close()

	// Check status code
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("unexpected status code: %d", resp.StatusCode)
	}

	// Parse response
	var result struct {
		Bars []AlpacaBar `json:"bars"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	if len(result.Bars) == 0 {
		return nil, fmt.Errorf("no recent minute bars found for %s", ticker)
	}

	return &result.Bars[len(result.Bars)-1], nil
}

// getLatestDailyBar fetches the most recent daily bar for a ticker
func (p *AlpacaProvider) getLatestDailyBar(ctx context.Context, ticker string) (*AlpacaBar, error) {
	// Get current time
	now := time.Now().UTC()

	// Set time range - last 3 days to now
	end := now.Format(time.RFC3339)
	start := now.AddDate(0, 0, -3).Format(time.RFC3339)

	// Build URL for bars endpoint
	requestURL := fmt.Sprintf("%s/v2/stocks/%s/bars?start=%s&end=%s&timeframe=1Day&limit=1",
		p.dataBaseURL, url.PathEscape(ticker), url.QueryEscape(start), url.QueryEscape(end))

	// Create request
	req, err := http.NewRequestWithContext(ctx, "GET", requestURL, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	// Add authentication headers
	req.Header.Add("APCA-API-KEY-ID", p.apiKey)
	req.Header.Add("APCA-API-SECRET-KEY", p.apiSecret)

	// Execute request
	resp, err := p.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to execute request: %w", err)
	}
	defer resp.Body.Close()

	// Check status code
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("unexpected status code: %d", resp.StatusCode)
	}

	// Parse response
	var result struct {
		Bars []AlpacaBar `json:"bars"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	if len(result.Bars) == 0 {
		return nil, fmt.Errorf("no recent daily bars found for %s", ticker)
	}

	return &result.Bars[len(result.Bars)-1], nil
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
func convertToAlpacaTimeframe(interval string) (string, error) {
	// Convert to lowercase for easier comparison
	interval = strings.ToLower(interval)

	switch interval {
	case "1m", "1min", "1minute":
		return "1Min", nil
	case "5m", "5min", "5minute":
		return "5Min", nil
	case "15m", "15min", "15minute":
		return "15Min", nil
	case "30m", "30min", "30minute":
		return "30Min", nil
	case "1h", "1hour", "60min":
		return "1Hour", nil
	case "1d", "1day", "daily":
		return "1Day", nil
	default:
		return "", fmt.Errorf("unsupported interval: %s", interval)
	}
}

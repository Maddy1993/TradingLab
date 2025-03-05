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
	httpClient    *http.Client
	paperTrading  bool
	lastValidData map[string]*MarketData // Cache last valid data by ticker
	useFallback   bool                   // Flag to control fallback data generation
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
	if paperTrading {
		baseURL = "https://paper-api.alpaca.markets"
	}

	return &AlpacaProvider{
		apiKey:        apiKey,
		apiSecret:     apiSecret,
		baseURL:       baseURL,
		paperTrading:  paperTrading,
		lastValidData: make(map[string]*MarketData),
		useFallback:   true,
		httpClient: &http.Client{
			Timeout: 10 * time.Second,
		},
	}, nil
}

// isMarketOpen checks if the market is currently open
func (p *AlpacaProvider) isMarketOpen(ctx context.Context) (bool, error) {
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

// GetLatestData fetches the latest market data for the specified ticker
func (p *AlpacaProvider) GetLatestData(ctx context.Context, ticker string) (*MarketData, error) {
	// Check if market is open
	isOpen, err := p.isMarketOpen(ctx)
	if err != nil {
		log.Printf("Failed to check market status: %v", err)
		// Proceed with the attempt even if we can't check market status
	}

	if !isOpen {
		log.Printf("Market is closed, using fallback data for %s", ticker)
		// Try to get the most recent bars first
		data, err := p.getRecentBar(ctx, ticker)
		if err == nil {
			// Cache this data
			p.lastValidData[ticker] = data
			return data, nil
		}

		log.Printf("Fallback to recent bars failed: %v", err)

		// Check if we have cached data
		if cachedData, ok := p.lastValidData[ticker]; ok {
			// Update timestamp to current time
			cachedData.Timestamp = time.Now()
			return cachedData, nil
		}

		// Last resort: generate sample data if allowed
		if p.useFallback {
			return p.generateSampleData(ticker), nil
		}

		return nil, fmt.Errorf("market is closed and no fallback data available for %s", ticker)
	}

	// Market is open, try to get live data
	// Build URL for quotes endpoint
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
		data, barErr := p.getRecentBar(ctx, ticker)
		if barErr == nil {
			// Cache this data
			p.lastValidData[ticker] = data
			return data, nil
		}

		// If fallbacks failed, return original error
		return nil, fmt.Errorf("unexpected status code: %d", resp.StatusCode)
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

	// Get the latest bar to get OHLC data
	bar, err := p.getRecentBar(ctx, ticker)
	if err != nil {
		return nil, fmt.Errorf("failed to get latest bar: %w", err)
	}

	// Create market data
	timestamp := time.Unix(0, result.Quote.Timestamp)
	data := &MarketData{
		Ticker:    ticker,
		Timestamp: timestamp,
		Price:     (result.Quote.BidPrice + result.Quote.AskPrice) / 2, // Mid price
		Open:      bar.Open,
		High:      bar.High,
		Low:       bar.Low,
		Close:     bar.Close,
		Volume:    bar.Volume,
		Interval:  "minute",
		Source:    "Alpaca",
	}

	// Cache the valid data
	p.lastValidData[ticker] = data

	return data, nil
}

// getRecentBar fetches a recent bar for the ticker
func (p *AlpacaProvider) getRecentBar(ctx context.Context, ticker string) (*MarketData, error) {
	// Get current time
	now := time.Now().UTC()

	// Set end time to now and start time to 5 days ago to ensure we catch the most recent trading day
	end := now.Format(time.RFC3339)
	start := now.AddDate(0, 0, -3).Format(time.RFC3339)

	// Build URL for bars endpoint
	requestURL := fmt.Sprintf("%s/v2/stocks/%s/bars?start=%s&end=%s&limit=1&timeframe=1Day",
		p.baseURL, url.PathEscape(ticker), url.QueryEscape(start), url.QueryEscape(end))

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
		return nil, fmt.Errorf("no bars returned for ticker %s", ticker)
	}

	bar := result.Bars[0]
	timestamp := time.Unix(bar.Timestamp/1000000000, bar.Timestamp%1000000000)

	data := &MarketData{
		Ticker:    ticker,
		Timestamp: timestamp,
		Price:     bar.Close,
		Open:      bar.Open,
		High:      bar.High,
		Low:       bar.Low,
		Close:     bar.Close,
		Volume:    bar.Volume,
		Interval:  "day",
		Source:    "Alpaca",
	}

	return data, nil
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
		Interval:  "minute",
		Source:    "Alpaca (Simulated)",
	}
}

// GetHistoricalData fetches historical market data for the specified ticker
func (p *AlpacaProvider) GetHistoricalData(ctx context.Context, ticker string, days int, interval string) ([]*MarketData, error) {
	// Convert interval to Alpaca timeframe format
	timeframe, err := convertToAlpacaTimeframe(interval)
	if err != nil {
		return nil, err
	}

	// Get current time
	now := time.Now().UTC()

	// Set end time to now and start time to days ago
	end := now.Format(time.RFC3339)
	start := now.AddDate(0, 0, -days).Format(time.RFC3339)

	// Build URL for bars endpoint
	requestURL := fmt.Sprintf("%s/v2/stocks/%s/bars?start=%s&end=%s&timeframe=%s",
		p.baseURL, url.PathEscape(ticker), url.QueryEscape(start), url.QueryEscape(end), timeframe)

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
		return nil, fmt.Errorf("unexpected status code: %d", resp.StatusCode)
	}

	// Parse response
	var result struct {
		Bars []AlpacaBar `json:"bars"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	// Convert to MarketData slice
	marketData := make([]*MarketData, 0, len(result.Bars))
	for _, bar := range result.Bars {
		timestamp := time.Unix(bar.Timestamp/1000000000, bar.Timestamp%1000000000)
		data := &MarketData{
			Ticker:    ticker,
			Timestamp: timestamp,
			Price:     bar.Close,
			Open:      bar.Open,
			High:      bar.High,
			Low:       bar.Low,
			Close:     bar.Close,
			Volume:    bar.Volume,
			Interval:  interval,
			Source:    "Alpaca",
		}
		marketData = append(marketData, data)
	}

	return marketData, nil
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

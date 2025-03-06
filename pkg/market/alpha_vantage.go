// pkg/market/alpha_vantage.go
package market

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"time"
)

// AlphaVantageProvider implements market data fetching from Alpha Vantage API
type AlphaVantageProvider struct {
	apiKey     string
	baseURL    string
	httpClient *http.Client
}

// MarketData represents OHLCV market data
//type MarketData struct {
//	Ticker    string    `json:"ticker"`
//	Timestamp time.Time `json:"timestamp"`
//	Price     float64   `json:"price"`
//	Open      float64   `json:"open"`
//	High      float64   `json:"high"`
//	Low       float64   `json:"low"`
//	Close     float64   `json:"close"`
//	Volume    int64     `json:"volume"`
//	Interval  string    `json:"interval"`
//	Source    string    `json:"source"`
//}

// NewAlphaVantageProvider creates a new Alpha Vantage data provider
func NewAlphaVantageProvider(apiKey string) (*AlphaVantageProvider, error) {
	if apiKey == "" {
		return nil, fmt.Errorf("Alpha Vantage API key is required")
	}

	return &AlphaVantageProvider{
		apiKey:  apiKey,
		baseURL: "https://www.alphavantage.co/query",
		httpClient: &http.Client{
			Timeout: 10 * time.Second,
		},
	}, nil
}

// GetLatestData fetches the latest market data for the specified ticker
func (p *AlphaVantageProvider) GetLatestData(ctx context.Context, ticker string) (*MarketData, error) {
	// Build URL for Global Quote endpoint
	params := url.Values{}
	params.Add("function", "GLOBAL_QUOTE")
	params.Add("symbol", ticker)
	params.Add("apikey", p.apiKey)

	// Construct request URL
	requestURL := fmt.Sprintf("%s?%s", p.baseURL, params.Encode())

	// Create request
	req, err := http.NewRequestWithContext(ctx, "GET", requestURL, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

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
		GlobalQuote struct {
			Symbol           string `json:"01. symbol"`
			Open             string `json:"02. open"`
			High             string `json:"03. high"`
			Low              string `json:"04. low"`
			Price            string `json:"05. price"`
			Volume           string `json:"06. volume"`
			LatestTradingDay string `json:"07. latest trading day"`
			PreviousClose    string `json:"08. previous close"`
			Change           string `json:"09. change"`
			ChangePercent    string `json:"10. change percent"`
		} `json:"Global Quote"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	// Parse values
	open, err := parseFloat(result.GlobalQuote.Open)
	if err != nil {
		return nil, fmt.Errorf("invalid open value: %w", err)
	}

	high, err := parseFloat(result.GlobalQuote.High)
	if err != nil {
		return nil, fmt.Errorf("invalid high value: %w", err)
	}

	low, err := parseFloat(result.GlobalQuote.Low)
	if err != nil {
		return nil, fmt.Errorf("invalid low value: %w", err)
	}

	price, err := parseFloat(result.GlobalQuote.Price)
	if err != nil {
		return nil, fmt.Errorf("invalid price value: %w", err)
	}

	volume, err := parseInt(result.GlobalQuote.Volume)
	if err != nil {
		return nil, fmt.Errorf("invalid volume value: %w", err)
	}

	// Parse timestamp
	timestamp, err := time.Parse("2006-01-02", result.GlobalQuote.LatestTradingDay)
	if err != nil {
		return nil, fmt.Errorf("invalid timestamp: %w", err)
	}

	// Create market data
	data := &MarketData{
		Ticker:    result.GlobalQuote.Symbol,
		Timestamp: timestamp,
		Price:     price,
		Open:      open,
		High:      high,
		Low:       low,
		Close:     price, // Use latest price as close
		Volume:    volume,
		Interval:  "day",
		Source:    "Alpha Vantage",
	}

	return data, nil
}

// Helper to parse float from string
func parseFloat(s string) (float64, error) {
	var f float64
	if _, err := fmt.Sscanf(s, "%f", &f); err != nil {
		return 0, err
	}
	return f, nil
}

// Helper to parse int from string
func parseInt(s string) (int64, error) {
	var i int64
	if _, err := fmt.Sscanf(s, "%d", &i); err != nil {
		return 0, err
	}
	return i, nil
}

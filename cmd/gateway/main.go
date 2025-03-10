package main

import (
	"context"
	"encoding/json"
	"fmt"
	"math"
	"math/rand"
	"net/http"
	"os"
	"os/signal"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/gorilla/mux"
	"github.com/gorilla/websocket"
	"github.com/nats-io/nats.go"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"

	"github.com/myapp/tradinglab/pkg/events"
	"github.com/myapp/tradinglab/pkg/utils"
	pb "github.com/myapp/tradinglab/proto"
)

// API Gateway for TradingLab
// This service provides a REST API that proxies requests to the TradingLab gRPC service
// and provides WebSocket connections for real-time updates via NATS

type APIGateway struct {
	natsClient     *events.EventClient
	tradingClient  pb.TradingServiceClient
	tradingConn    *grpc.ClientConn
	router         *mux.Router
	wsClients      map[*websocket.Conn]bool
	wsClientsMutex sync.Mutex
	upgrader       websocket.Upgrader
	cache          *DataCache
}

func NewAPIGateway(natsURL, tradingServiceURL string) (*APIGateway, error) {
	// Connect to NATS
	natsClient, err := events.NewEventClient(natsURL)
	if err != nil {
		return nil, fmt.Errorf("failed to connect to NATS: %w", err)
	}

	// Connect to TradingLab gRPC service with timeout and retry options
	var tradingConn *grpc.ClientConn
	var tradingClient pb.TradingServiceClient

	// Set up gRPC connection options with increased timeout
	opts := []grpc.DialOption{
		grpc.WithTransportCredentials(insecure.NewCredentials()),
		grpc.WithBlock(),
		grpc.WithTimeout(10 * time.Second),
	}

	// Retry logic for establishing gRPC connection
	maxRetries := 3
	backoffTime := 1 * time.Second
	var connErr error

	for attempt := 1; attempt <= maxRetries; attempt++ {
		utils.Info("Connecting to trading service at %s (attempt %d/%d)", tradingServiceURL, attempt, maxRetries)
		tradingConn, connErr = grpc.Dial(tradingServiceURL, opts...)

		if connErr == nil {
			tradingClient = pb.NewTradingServiceClient(tradingConn)
			utils.Info("Successfully connected to trading service")
			break
		}

		utils.Info("Failed to connect to trading service (attempt %d/%d): %v", attempt, maxRetries, connErr)

		if attempt < maxRetries {
			// Exponential backoff
			waitTime := backoffTime * time.Duration(attempt)
			utils.Info("Retrying in %v", waitTime)
			time.Sleep(waitTime)
		}
	}

	if connErr != nil {
		return nil, fmt.Errorf("failed to connect to trading service after %d attempts: %w", maxRetries, connErr)
	}

	// Create router
	router := mux.NewRouter()

	// Configure websocket upgrader
	upgrader := websocket.Upgrader{
		ReadBufferSize:  1024,
		WriteBufferSize: 1024,
		CheckOrigin: func(r *http.Request) bool {
			return true // Allow any origin in dev; restrict in production
		},
	}

	return &APIGateway{
		natsClient:    natsClient,
		tradingClient: tradingClient,
		tradingConn:   tradingConn,
		router:        router,
		wsClients:     make(map[*websocket.Conn]bool),
		upgrader:      upgrader,
		cache:         NewDataCache(),
	}, nil
}

func (g *APIGateway) setupRoutes() {
	// API routes
	api := g.router.PathPrefix("/api").Subrouter()

	// Health check
	api.HandleFunc("/health", g.healthHandler).Methods("GET")

	// System status
	api.HandleFunc("/status", g.statusHandler).Methods("GET")

	// Available tickers
	api.HandleFunc("/tickers", g.tickersHandler).Methods("GET")

	// Historical data
	api.HandleFunc("/historical-data", g.historicalDataHandler).Methods("GET")

	// Trading signals
	api.HandleFunc("/signals", g.signalsHandler).Methods("GET")

	// Backtest
	api.HandleFunc("/backtest", g.backtestHandler).Methods("GET")

	// Recommendations
	api.HandleFunc("/recommendations", g.recommendationsHandler).Methods("GET")

	// WebSocket endpoint for real-time updates
	api.HandleFunc("/ws", g.websocketHandler)

	// Serve static files for the UI
	g.router.PathPrefix("/").Handler(http.FileServer(http.Dir("./ui/build")))
}

func (g *APIGateway) statusHandler(w http.ResponseWriter, r *http.Request) {
	// Get system status
	status := g.cache.GetServiceStatus()

	// Add connection information
	grpcStatus := "connected"
	natsStatus := "connected"

	if g.tradingConn == nil {
		grpcStatus = "disconnected"
	} else if g.tradingConn.GetState().String() != "READY" {
		grpcStatus = fmt.Sprintf("not ready: %s", g.tradingConn.GetState().String())
	}

	if g.natsClient == nil {
		natsStatus = "disconnected"
	} else if !g.natsClient.GetNATS().IsConnected() {
		natsStatus = "disconnected"
	}

	// Add connection status to response
	status["connections"] = map[string]string{
		"grpc": grpcStatus,
		"nats": natsStatus,
	}

	// Add cache stats
	g.cache.mutex.RLock()
	cacheStats := map[string]interface{}{
		"historical_data_count":  len(g.cache.historicalData),
		"signals_count":          len(g.cache.signals),
		"recommendations_count":  len(g.cache.recommendations),
		"backtest_results_count": len(g.cache.backtestResults),
	}
	g.cache.mutex.RUnlock()

	status["cache_stats"] = cacheStats
	status["timestamp"] = time.Now().Format(time.RFC3339)

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(status)
}

func (g *APIGateway) healthHandler(w http.ResponseWriter, r *http.Request) {
	// Quick health check without making external calls, to meet Kubernetes probes
	response := map[string]interface{}{
		"status":       "healthy",
		"timestamp":    time.Now().Format(time.RFC3339),
		"service_name": "tradinglab-api-gateway",
	}

	// Only perform deep health check for non-probe requests
	if r.Header.Get("User-Agent") != "kube-probe/1.27" {
		// Check gRPC connection with a ping rather than full historical data
		grpcStatus := "connected"
		natsStatus := "connected"

		// Check if connections exist at a basic level
		if g.tradingConn == nil {
			grpcStatus = "disconnected"
			utils.Info("gRPC connection is nil")
		} else if g.tradingConn.GetState().String() != "READY" {
			grpcStatus = fmt.Sprintf("not ready: %s", g.tradingConn.GetState().String())
			utils.Info("gRPC connection not ready: %s", g.tradingConn.GetState().String())
		}

		if g.natsClient == nil {
			natsStatus = "disconnected"
			utils.Info("NATS connection unavailable")
		} else if !g.natsClient.GetNATS().IsConnected() {
			natsStatus = "disconnected"
			utils.Info("NATS connection lost")
		}

		response["grpc_status"] = grpcStatus
		response["nats_status"] = natsStatus
		response["deep_check"] = true
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}

func (g *APIGateway) tickersHandler(w http.ResponseWriter, r *http.Request) {
	// Default tickers
	tickers := []string{"SPY", "AAPL", "MSFT", "GOOGL", "AMZN"}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(tickers)
}

func (g *APIGateway) historicalDataHandler(w http.ResponseWriter, r *http.Request) {
	// Extract query parameters
	ticker := r.URL.Query().Get("ticker")
	if ticker == "" {
		http.Error(w, "ticker parameter is required", http.StatusBadRequest)
		return
	}

	daysStr := r.URL.Query().Get("days")
	days := 30 // Default
	if daysStr != "" {
		var err error
		days, err = strconv.Atoi(daysStr)
		if err != nil {
			http.Error(w, "invalid days parameter", http.StatusBadRequest)
			return
		}
	}

	interval := r.URL.Query().Get("interval")
	if interval == "" {
		interval = "15min"
	}

	// Create cache key
	cacheKey := fmt.Sprintf("%s:%d:%s", ticker, days, interval)

	// Track failures for system status
	var systemFailures int
	defer func() {
		g.cache.updateServiceStatus("historical-data", systemFailures)
	}()

	// Create gRPC request with longer timeout
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()

	req := &pb.HistoricalDataRequest{
		Ticker:   ticker,
		Days:     int32(days),
		Interval: interval,
	}

	// Call gRPC service with retry logic
	var resp *pb.HistoricalDataResponse
	var err error
	maxRetries := 3

	for attempt := 1; attempt <= maxRetries; attempt++ {
		if attempt > 1 {
			utils.Info("Retrying historical data request for %s (attempt %d/%d)", ticker, attempt, maxRetries)
			time.Sleep(time.Duration(attempt) * time.Second) // Exponential backoff
		}

		resp, err = g.tradingClient.GetHistoricalData(ctx, req)
		if err == nil {
			break // Success, exit retry loop
		}

		utils.Info("Historical data request failed (attempt %d/%d): %v", attempt, maxRetries, err)
		systemFailures++

		if attempt == maxRetries || ctx.Err() != nil {
			// All retries failed or context timeout
			break
		}
	}

	// Convert to JSON-friendly format if we have a response
	var candles []map[string]interface{}

	if err == nil {
		// Process successful response
		candles = make([]map[string]interface{}, 0, len(resp.Candles))
		for _, candle := range resp.Candles {
			candles = append(candles, map[string]interface{}{
				"date":   candle.Date,
				"open":   candle.Open,
				"high":   candle.High,
				"low":    candle.Low,
				"close":  candle.Close,
				"volume": candle.Volume,
			})
		}

		// Cache the successful response
		g.cache.CacheHistoricalData(cacheKey, candles)

		// Return the data
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(candles)
		return
	}

	// All retries failed, try to use cached data
	cachedData, exists := g.cache.GetCachedHistoricalData(cacheKey)
	if exists {
		utils.Info("Using cached historical data for %s (%.1f minutes old)",
			ticker, time.Since(cachedData.Timestamp).Minutes())

		// Add headers to indicate cache usage
		w.Header().Set("Content-Type", "application/json")
		w.Header().Set("X-Data-Source", "cache")
		w.Header().Set("X-Data-Age", fmt.Sprintf("%.1f minutes", time.Since(cachedData.Timestamp).Minutes()))
		w.Header().Set("X-System-Mode", g.cache.GetServiceStatus()["mode"].(string))

		// Return cached data
		json.NewEncoder(w).Encode(cachedData.Data)
		return
	}

	// No cached data available
	if g.cache.GetServiceStatus()["mode"] == "readonly" {
		// In read-only mode, return a specific error
		http.Error(w, "System is in read-only mode. No cached data available for this request.", http.StatusServiceUnavailable)
	} else {
		// Otherwise return a standard error
		http.Error(w, fmt.Sprintf("Error fetching historical data after %d attempts: %v", maxRetries, err), http.StatusInternalServerError)
	}
}

// DataCache stores recent valid responses to serve in fallback mode
type DataCache struct {
	mutex             sync.RWMutex
	historicalData    map[string]CachedData
	signals           map[string]CachedData
	recommendations   map[string]CachedData
	backtestResults   map[string]CachedData
	serviceMode       string // "normal", "degraded", "readonly"
	lastStatusChange  time.Time
	statusDescription string
}

// CachedData stores response data with metadata
type CachedData struct {
	Data      interface{}
	Timestamp time.Time
	Source    string // Origin of the data (e.g., "alpaca", "cache")
}

// NewDataCache creates a new data cache
func NewDataCache() *DataCache {
	return &DataCache{
		historicalData:    make(map[string]CachedData),
		signals:           make(map[string]CachedData),
		recommendations:   make(map[string]CachedData),
		backtestResults:   make(map[string]CachedData),
		serviceMode:       "normal",
		lastStatusChange:  time.Now(),
		statusDescription: "System operating normally",
	}
}

// cacheSystems keeps track of which systems are having issues
func (c *DataCache) updateServiceStatus(failedSystem string, failureCount int) {
	c.mutex.Lock()
	defer c.mutex.Unlock()

	// Determine system state based on failures
	oldMode := c.serviceMode

	if failureCount > 5 {
		c.serviceMode = "readonly"
		c.statusDescription = fmt.Sprintf("System in read-only mode: %s unavailable", failedSystem)
	} else if failureCount > 2 {
		c.serviceMode = "degraded"
		c.statusDescription = fmt.Sprintf("System in degraded mode: %s experiencing issues", failedSystem)
	} else if failureCount == 0 {
		c.serviceMode = "normal"
		c.statusDescription = "System operating normally"
	}

	// If status changed, update timestamp
	if oldMode != c.serviceMode {
		c.lastStatusChange = time.Now()
		utils.Info("Service status changed to %s: %s", c.serviceMode, c.statusDescription)
	}
}

// GetServiceStatus returns the current system status
func (c *DataCache) GetServiceStatus() map[string]interface{} {
	c.mutex.RLock()
	defer c.mutex.RUnlock()

	return map[string]interface{}{
		"mode":               c.serviceMode,
		"description":        c.statusDescription,
		"last_status_change": c.lastStatusChange.Format(time.RFC3339),
		"readonly":           c.serviceMode == "readonly",
	}
}

// CacheHistoricalData caches historical data for a ticker
func (c *DataCache) CacheHistoricalData(key string, data interface{}) {
	c.mutex.Lock()
	defer c.mutex.Unlock()

	c.historicalData[key] = CachedData{
		Data:      data,
		Timestamp: time.Now(),
		Source:    "live",
	}
}

// GetCachedHistoricalData retrieves cached historical data
func (c *DataCache) GetCachedHistoricalData(key string) (CachedData, bool) {
	c.mutex.RLock()
	defer c.mutex.RUnlock()

	data, exists := c.historicalData[key]
	return data, exists
}

// CacheSignalData caches signal data
func (c *DataCache) CacheSignalData(key string, data interface{}) {
	c.mutex.Lock()
	defer c.mutex.Unlock()

	c.signals[key] = CachedData{
		Data:      data,
		Timestamp: time.Now(),
		Source:    "live",
	}
}

// GetCachedSignalData retrieves cached signal data
func (c *DataCache) GetCachedSignalData(key string) (CachedData, bool) {
	c.mutex.RLock()
	defer c.mutex.RUnlock()

	data, exists := c.signals[key]
	return data, exists
}

// Simple string hash function
func hash(s string) uint32 {
	h := uint32(0)
	for i := 0; i < len(s); i++ {
		h = h*31 + uint32(s[i])
	}
	return h
}

// generateFallbackCandles creates sample market data when real data is unavailable
func generateFallbackCandles(ticker string, days int, interval string) []map[string]interface{} {
	// Only generate fallback data for 30 days or less
	if days > 30 {
		return nil
	}

	// Seed random number generator with ticker name for consistent results
	source := rand.NewSource(int64(hash(ticker)))
	rng := rand.New(source)

	// Set base price based on ticker
	var basePrice float64
	switch ticker {
	case "SPY":
		basePrice = 420.0
	case "AAPL":
		basePrice = 175.0
	case "MSFT":
		basePrice = 400.0
	case "GOOGL":
		basePrice = 140.0
	case "AMZN":
		basePrice = 175.0
	default:
		basePrice = 100.0
	}

	// Determine number of candles based on interval
	candlesPerDay := 1
	if interval == "15min" {
		candlesPerDay = 26 // ~6.5 trading hours / 15min
	} else if interval == "5min" {
		candlesPerDay = 78 // ~6.5 trading hours / 5min
	} else if interval == "60min" {
		candlesPerDay = 7 // ~6.5 trading hours / 60min
	}

	totalCandles := days * candlesPerDay
	if totalCandles > 1000 {
		totalCandles = 1000 // Cap at 1000 candles
	}

	// Generate candles
	candles := make([]map[string]interface{}, totalCandles)
	now := time.Now()

	for i := 0; i < totalCandles; i++ {
		// Calculate time, moving backward from now
		var candleTime time.Time
		if interval == "day" {
			candleTime = now.AddDate(0, 0, -i)
		} else {
			minutesInterval := 15
			if interval == "5min" {
				minutesInterval = 5
			} else if interval == "60min" {
				minutesInterval = 60
			}
			candleTime = now.Add(-time.Duration(i*minutesInterval) * time.Minute)
		}

		// Generate price movements (basic random walk with trend)
		volatility := basePrice * 0.01 // 1% volatility
		priceChange := (rng.Float64()*2 - 1) * volatility
		trend := -0.0001 * float64(i) * basePrice // Slight downtrend

		// Calculate candle values
		close := basePrice + priceChange + trend
		open := close - (rng.Float64()*2-1)*volatility*0.5
		high := math.Max(open, close) + rng.Float64()*volatility*0.5
		low := math.Min(open, close) - rng.Float64()*volatility*0.5
		volume := 100000 + rng.Float64()*900000

		// Format date to match expected format
		date := candleTime.Format("2006-01-02T15:04:05Z")

		candles[i] = map[string]interface{}{
			"date":   date,
			"open":   open,
			"high":   high,
			"low":    low,
			"close":  close,
			"volume": volume,
		}

		// Update base price for next candle
		basePrice = close
	}

	return candles
}

func (g *APIGateway) signalsHandler(w http.ResponseWriter, r *http.Request) {
	// Extract query parameters
	ticker := r.URL.Query().Get("ticker")
	if ticker == "" {
		http.Error(w, "ticker parameter is required", http.StatusBadRequest)
		return
	}

	daysStr := r.URL.Query().Get("days")
	days := 30 // Default
	if daysStr != "" {
		var err error
		days, err = strconv.Atoi(daysStr)
		if err != nil {
			http.Error(w, "invalid days parameter", http.StatusBadRequest)
			return
		}
	}

	strategy := r.URL.Query().Get("strategy")
	if strategy == "" {
		strategy = "RedCandle"
	}

	interval := r.URL.Query().Get("interval")
	if interval == "" {
		interval = "15min"
	}

	// Create cache key
	cacheKey := fmt.Sprintf("%s:%d:%s:%s", ticker, days, strategy, interval)

	// Track failures for system status
	var systemFailures int
	defer func() {
		g.cache.updateServiceStatus("signals", systemFailures)
	}()

	// Create gRPC request with longer timeout
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()

	req := &pb.SignalRequest{
		Ticker:   ticker,
		Days:     int32(days),
		Strategy: strategy,
		Interval: interval,
	}

	// Call gRPC service with retry logic
	var resp *pb.SignalResponse
	var err error
	maxRetries := 3

	for attempt := 1; attempt <= maxRetries; attempt++ {
		if attempt > 1 {
			utils.Info("Retrying signal generation for %s (attempt %d/%d)", ticker, attempt, maxRetries)
			time.Sleep(time.Duration(attempt) * time.Second) // Exponential backoff
		}

		resp, err = g.tradingClient.GenerateSignals(ctx, req)
		if err == nil {
			break // Success, exit retry loop
		}

		utils.Info("Signal generation failed (attempt %d/%d): %v", attempt, maxRetries, err)
		systemFailures++

		if attempt == maxRetries || ctx.Err() != nil {
			// All retries failed or context timeout
			break
		}
	}

	// Convert to JSON-friendly format if we have a response
	var signals []map[string]interface{}

	if err == nil {
		// Process successful response
		signals = make([]map[string]interface{}, 0, len(resp.Signals))
		for _, signal := range resp.Signals {
			signals = append(signals, map[string]interface{}{
				"date":        signal.Date,
				"signal_type": signal.SignalType,
				"entry_price": signal.EntryPrice,
				"stoploss":    signal.Stoploss,
			})
		}

		// Cache the successful response
		g.cache.CacheSignalData(cacheKey, signals)

		// Return the data
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(signals)
		return
	}

	// All retries failed, try to use cached data
	cachedData, exists := g.cache.GetCachedSignalData(cacheKey)
	if exists {
		utils.Info("Using cached signal data for %s (%.1f minutes old)",
			ticker, time.Since(cachedData.Timestamp).Minutes())

		// Add headers to indicate cache usage
		w.Header().Set("Content-Type", "application/json")
		w.Header().Set("X-Data-Source", "cache")
		w.Header().Set("X-Data-Age", fmt.Sprintf("%.1f minutes", time.Since(cachedData.Timestamp).Minutes()))
		w.Header().Set("X-System-Mode", g.cache.GetServiceStatus()["mode"].(string))

		// Return cached data
		json.NewEncoder(w).Encode(cachedData.Data)
		return
	}

	// No cached data available
	if g.cache.GetServiceStatus()["mode"] == "readonly" {
		// In read-only mode, return a specific error
		w.Header().Set("Retry-After", "300") // Suggest retry after 5 minutes
		http.Error(w, "System is in read-only mode. No cached signals available for this request.", http.StatusServiceUnavailable)
	} else {
		// Otherwise return a standard error
		http.Error(w, fmt.Sprintf("Error generating signals after %d attempts: %v", maxRetries, err), http.StatusInternalServerError)
	}
}

func (g *APIGateway) backtestHandler(w http.ResponseWriter, r *http.Request) {
	// Extract query parameters
	ticker := r.URL.Query().Get("ticker")
	if ticker == "" {
		http.Error(w, "ticker parameter is required", http.StatusBadRequest)
		return
	}

	daysStr := r.URL.Query().Get("days")
	days := 30 // Default
	if daysStr != "" {
		var err error
		days, err = strconv.Atoi(daysStr)
		if err != nil {
			http.Error(w, "invalid days parameter", http.StatusBadRequest)
			return
		}
	}

	strategy := r.URL.Query().Get("strategy")
	if strategy == "" {
		strategy = "RedCandle"
	}

	interval := r.URL.Query().Get("interval")
	if interval == "" {
		interval = "15min"
	}

	// Parse profit targets
	var profitTargets []float64
	if ptStr := r.URL.Query().Get("profit_targets"); ptStr != "" {
		for _, pt := range strings.Split(ptStr, ",") {
			ptFloat, err := strconv.ParseFloat(pt, 64)
			if err != nil {
				continue
			}
			profitTargets = append(profitTargets, ptFloat)
		}
	}

	// Parse risk reward ratios
	var riskRewardRatios []float64
	if rrStr := r.URL.Query().Get("risk_reward_ratios"); rrStr != "" {
		for _, rr := range strings.Split(rrStr, ",") {
			rrFloat, err := strconv.ParseFloat(rr, 64)
			if err != nil {
				continue
			}
			riskRewardRatios = append(riskRewardRatios, rrFloat)
		}
	}

	// Parse profit targets dollar
	var profitTargetsDollar []float64
	if ptdStr := r.URL.Query().Get("profit_targets_dollar"); ptdStr != "" {
		for _, ptd := range strings.Split(ptdStr, ",") {
			ptdFloat, err := strconv.ParseFloat(ptd, 64)
			if err != nil {
				continue
			}
			profitTargetsDollar = append(profitTargetsDollar, ptdFloat)
		}
	}

	// Create gRPC request
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	req := &pb.BacktestRequest{
		Ticker:              ticker,
		Days:                int32(days),
		Strategy:            strategy,
		Interval:            interval,
		ProfitTargets:       profitTargets,
		RiskRewardRatios:    riskRewardRatios,
		ProfitTargetsDollar: profitTargetsDollar,
	}

	// Call gRPC service
	resp, err := g.tradingClient.RunBacktest(ctx, req)
	if err != nil {
		http.Error(w, fmt.Sprintf("error running backtest: %v", err), http.StatusInternalServerError)
		return
	}

	// Convert results map to JSON-friendly format
	results := make(map[string]interface{})
	for name, result := range resp.Results {
		results[name] = map[string]interface{}{
			"win_rate":         result.WinRate,
			"profit_factor":    result.ProfitFactor,
			"total_return":     result.TotalReturn,
			"total_return_pct": result.TotalReturnPct,
			"total_trades":     result.TotalTrades,
			"winning_trades":   result.WinningTrades,
			"losing_trades":    result.LosingTrades,
			"max_drawdown":     result.MaxDrawdown,
			"max_drawdown_pct": result.MaxDrawdownPct,
		}
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(results)
}

func (g *APIGateway) recommendationsHandler(w http.ResponseWriter, r *http.Request) {
	// Extract query parameters
	ticker := r.URL.Query().Get("ticker")
	if ticker == "" {
		http.Error(w, "ticker parameter is required", http.StatusBadRequest)
		return
	}

	daysStr := r.URL.Query().Get("days")
	days := 30 // Default
	if daysStr != "" {
		var err error
		days, err = strconv.Atoi(daysStr)
		if err != nil {
			http.Error(w, "invalid days parameter", http.StatusBadRequest)
			return
		}
	}

	strategy := r.URL.Query().Get("strategy")
	if strategy == "" {
		strategy = "RedCandle"
	}

	interval := r.URL.Query().Get("interval")
	if interval == "" {
		interval = "15min"
	}

	// Create gRPC request
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	req := &pb.RecommendationRequest{
		Ticker:   ticker,
		Days:     int32(days),
		Strategy: strategy,
		Interval: interval,
	}

	// Call gRPC service
	resp, err := g.tradingClient.GetOptionsRecommendations(ctx, req)
	if err != nil {
		http.Error(w, fmt.Sprintf("error getting recommendations: %v", err), http.StatusInternalServerError)
		return
	}

	// Convert gRPC response to JSON-friendly format
	recommendations := make([]map[string]interface{}, 0, len(resp.Recommendations))
	for _, rec := range resp.Recommendations {
		recommendations = append(recommendations, map[string]interface{}{
			"date":        rec.Date,
			"signal_type": rec.SignalType,
			"stock_price": rec.StockPrice,
			"stoploss":    rec.Stoploss,
			"option_type": rec.OptionType,
			"strike":      rec.Strike,
			"expiration":  rec.Expiration,
			"delta":       rec.Delta,
			"iv":          rec.Iv,
			"price":       rec.Price,
		})
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(recommendations)
}

func (g *APIGateway) websocketHandler(w http.ResponseWriter, r *http.Request) {
	// Log headers for debugging
	utils.Info("WebSocket request headers: %+v", r.Header)

	// Make sure we have the required headers for WebSocket upgrade
	upgradeHeader := r.Header.Get("Upgrade")
	connectionHeader := r.Header.Get("Connection")

	if !strings.Contains(strings.ToLower(upgradeHeader), "websocket") {
		utils.Info("Missing 'websocket' in Upgrade header: %s", upgradeHeader)
		http.Error(w, "WebSocket upgrade required", http.StatusBadRequest)
		return
	}

	if !strings.Contains(strings.ToLower(connectionHeader), "upgrade") {
		utils.Info("Missing 'upgrade' in Connection header: %s", connectionHeader)
		http.Error(w, "WebSocket upgrade required", http.StatusBadRequest)
		return
	}

	// Upgrade HTTP connection to WebSocket with more tolerant header checking
	upgrader := websocket.Upgrader{
		ReadBufferSize:  1024,
		WriteBufferSize: 1024,
		CheckOrigin: func(r *http.Request) bool {
			return true // Allow any origin in dev; restrict in production
		},
		// This is important - be more lenient with header checking
		Subprotocols: []string{"websocket"},
	}

	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		utils.Info("Failed to upgrade to websocket: %v", err)
		return
	}
	defer conn.Close()

	utils.Info("WebSocket connection established successfully")

	// Register client
	g.wsClientsMutex.Lock()
	g.wsClients[conn] = true
	g.wsClientsMutex.Unlock()

	// Clean up on disconnect
	defer func() {
		g.wsClientsMutex.Lock()
		delete(g.wsClients, conn)
		g.wsClientsMutex.Unlock()
		utils.Info("WebSocket connection closed")
	}()

	// Handle WebSocket messages (for subscription requests)
	messageHandler := make(chan error)
	go func() {
		messageHandler <- g.handleWebSocketMessages(conn)
	}()

	// Keep connection alive with ping/pong
	pingTicker := time.NewTicker(30 * time.Second)
	defer pingTicker.Stop()

	// Set ping-pong handlers for better connection monitoring
	conn.SetPingHandler(func(data string) error {
		// When we receive a ping, respond with a pong
		return conn.WriteControl(websocket.PongMessage, []byte(data), time.Now().Add(5*time.Second))
	})

	conn.SetPongHandler(func(data string) error {
		// When we receive a pong, log it for debugging
		utils.Info("Received pong from WebSocket client")
		return nil
	})

	// Main connection monitoring loop
	for {
		select {
		case err := <-messageHandler:
			utils.Info("WebSocket message handler returned: %v", err)
			return
		case <-pingTicker.C:
			// Send ping to client
			pingData := []byte(fmt.Sprintf("ping-%d", time.Now().Unix()))
			err := conn.WriteControl(websocket.PingMessage, pingData, time.Now().Add(5*time.Second))
			if err != nil {
				utils.Info("WebSocket ping failed: %v", err)
				return
			}
		}
	}
}

func (g *APIGateway) handleWebSocketMessages(conn *websocket.Conn) error {
	// Set up subscriptions based on client messages
	subscriptions := make(map[string]*nats.Subscription)
	defer func() {
		// Clean up subscriptions when connection closes
		for subject, sub := range subscriptions {
			utils.Info("Cleaning up subscription to %s", subject)
			if err := sub.Unsubscribe(); err != nil {
				utils.Info("Error unsubscribing from %s: %v", subject, err)
			}
		}
	}()

	// Message queue with a buffer to handle slow consumers
	const maxPendingMessages = 250 // Increased buffer size
	messageQueue := make(chan []byte, maxPendingMessages)

	// Start message sender goroutine - handles backpressure
	done := make(chan struct{})
	senderErrors := make(chan error, 1)

	go func() {
		defer close(done)
		for {
			select {
			case <-done:
				return
			case msg, ok := <-messageQueue:
				if !ok {
					return
				}

				// Try to write with timeout
				writeTimeout := time.Second * 5 // Increased timeout
				conn.SetWriteDeadline(time.Now().Add(writeTimeout))
				if err := conn.WriteMessage(websocket.TextMessage, msg); err != nil {
					utils.Info("Error forwarding message to WebSocket, closing: %v", err)
					senderErrors <- err
					return
				}
				conn.SetWriteDeadline(time.Time{}) // Reset deadline
			}
		}
	}()

	// Set initial read deadline
	conn.SetReadDeadline(time.Now().Add(10 * time.Minute))

	for {
		// Check for sender errors
		select {
		case err := <-senderErrors:
			return fmt.Errorf("message sender error: %w", err)
		default:
			// Continue if no errors
		}

		// Read message
		messageType, p, err := conn.ReadMessage()
		if err != nil {
			if websocket.IsUnexpectedCloseError(err,
				websocket.CloseGoingAway,
				websocket.CloseNormalClosure,
				websocket.CloseNoStatusReceived) {
				utils.Info("Unexpected WebSocket close: %v", err)
			} else {
				utils.Info("WebSocket closed: %v", err)
			}
			close(messageQueue) // Signal sender to stop
			return err
		}

		// Extend read deadline after each successful message
		conn.SetReadDeadline(time.Now().Add(10 * time.Minute))

		// Only process text messages
		if messageType != websocket.TextMessage {
			utils.Info("Ignoring non-text message type: %d", messageType)
			continue
		}

		utils.Info("Received WebSocket message: %s", string(p))

		// Parse subscription request
		var request struct {
			Action  string `json:"action"`  // "subscribe" or "unsubscribe"
			Type    string `json:"type"`    // "market", "signals", "recommendations"
			Ticker  string `json:"ticker"`  // Stock ticker
			Subject string `json:"subject"` // Optional specific NATS subject
		}

		if err := json.Unmarshal(p, &request); err != nil {
			utils.Info("Error parsing subscription request: %v, message: %s", err, string(p))
			// Send error message back to client
			errorMsg := map[string]string{
				"error": fmt.Sprintf("Invalid message format: %v", err),
			}
			errorJSON, _ := json.Marshal(errorMsg)
			messageQueue <- errorJSON
			continue
		}

		// Handle subscription request
		switch request.Action {
		case "subscribe":
			// Determine NATS subject based on request
			var subject string
			if request.Subject != "" {
				subject = request.Subject
			} else {
				switch request.Type {
				case "market":
					subject = fmt.Sprintf("market.live.%s", request.Ticker)
				case "signals":
					subject = fmt.Sprintf("signals.%s", request.Ticker)
				case "recommendations":
					subject = fmt.Sprintf("recommendations.%s", request.Ticker)
				default:
					continue // Unknown type
				}
			}

			// Check if already subscribed
			if _, exists := subscriptions[subject]; exists {
				continue
			}

			// Subscribe to NATS subject with circuit breaker pattern for slow consumers
			sub, err := g.natsClient.GetNATS().Subscribe(subject, func(msg *nats.Msg) {
				// Use non-blocking send to message queue
				select {
				case messageQueue <- msg.Data:
					// Message sent to queue
				default:
					// Queue full, discard message but keep connection alive
					utils.Info("WebSocket message queue full for %s, discarding message", subject)
				}
			})

			if err != nil {
				utils.Info("Error subscribing to NATS subject %s: %v", subject, err)
				continue
			}

			// Set pending limits to avoid overwhelming NATS with slow consumers
			// This sets how many messages/bytes can be pending before NATS drops them
			if err := sub.SetPendingLimits(256, 1024*1024); err != nil {
				utils.Info("Error setting pending limits: %v", err)
			}

			// Store subscription
			subscriptions[subject] = sub

			// Confirm subscription
			conn.WriteJSON(map[string]string{
				"event":   "subscribed",
				"subject": subject,
			})

		case "unsubscribe":
			// Determine NATS subject
			var subject string
			if request.Subject != "" {
				subject = request.Subject
			} else {
				switch request.Type {
				case "market":
					subject = fmt.Sprintf("market.live.%s", request.Ticker)
				case "signals":
					subject = fmt.Sprintf("signals.%s", request.Ticker)
				case "recommendations":
					subject = fmt.Sprintf("recommendations.%s", request.Ticker)
				default:
					continue // Unknown type
				}
			}

			// Check if subscribed
			sub, exists := subscriptions[subject]
			if !exists {
				continue
			}

			// Unsubscribe
			sub.Unsubscribe()
			delete(subscriptions, subject)

			// Confirm unsubscription
			conn.WriteJSON(map[string]string{
				"event":   "unsubscribed",
				"subject": subject,
			})
		}
	}
}

func (g *APIGateway) Serve(addr string) error {
	// Configure server
	server := &http.Server{
		Addr:         addr,
		Handler:      g.router,
		ReadTimeout:  15 * time.Second,
		WriteTimeout: 15 * time.Second,
		IdleTimeout:  120 * time.Second,
	}

	// Start server in a goroutine
	go func() {
		utils.Info("API Gateway listening on %s", addr)
		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			utils.Fatal("Server error: %v", err)
		}
	}()

	// Set up graceful shutdown
	quit := make(chan os.Signal, 1)
	// Listen for more signals including SIGKILL and SIGQUIT
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM, syscall.SIGQUIT, syscall.SIGHUP)
	<-quit

	// Shutdown server
	utils.Info("Shutting down server...")

	// Close all WebSocket connections first to avoid hanging
	g.wsClientsMutex.Lock()
	for conn := range g.wsClients {
		conn.WriteMessage(websocket.CloseMessage, websocket.FormatCloseMessage(websocket.CloseNormalClosure, "Server shutting down"))
		conn.Close()
		delete(g.wsClients, conn)
	}
	g.wsClientsMutex.Unlock()

	// Close NATS client before closing HTTP server to avoid hanging NATS subscriptions
	if g.natsClient != nil {
		utils.Info("Closing NATS connection...")
		g.natsClient.Close()
	}

	// Close gRPC connection
	if g.tradingConn != nil {
		utils.Info("Closing gRPC connection...")
		g.tradingConn.Close()
	}

	// Now shutdown the HTTP server
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second) // Increased timeout
	defer cancel()

	if err := server.Shutdown(ctx); err != nil {
		utils.Info("HTTP server shutdown error: %v", err)
		return fmt.Errorf("server shutdown failed: %w", err)
	}

	utils.Info("Server gracefully stopped")
	return nil
}

func main() {
	// Get configuration from environment variables
	natsURL := os.Getenv("NATS_URL")
	if natsURL == "" {
		natsURL = "nats://nats:4222"
	}

	tradingServiceURL := os.Getenv("TRADINGLAB_SERVICE_URL")
	if tradingServiceURL == "" {
		tradingServiceURL = "tradinglab-service:50052"
	}

	addr := os.Getenv("LISTEN_ADDR")
	if addr == "" {
		addr = ":5000"
	}

	// Create API Gateway
	gateway, err := NewAPIGateway(natsURL, tradingServiceURL)
	if err != nil {
		utils.Fatal("Failed to create API Gateway: %v", err)
	}

	// Set up routes
	gateway.setupRoutes()

	// Start server
	if err := gateway.Serve(addr); err != nil {
		utils.Fatal("Server error: %v", err)
	}
}
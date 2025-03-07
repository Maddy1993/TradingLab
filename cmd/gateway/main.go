package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
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
}

func NewAPIGateway(natsURL, tradingServiceURL string) (*APIGateway, error) {
	// Connect to NATS
	natsClient, err := events.NewEventClient(natsURL)
	if err != nil {
		return nil, fmt.Errorf("failed to connect to NATS: %w", err)
	}

	// Connect to TradingLab gRPC service
	conn, err := grpc.Dial(tradingServiceURL, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		return nil, fmt.Errorf("failed to connect to trading service: %w", err)
	}
	tradingClient := pb.NewTradingServiceClient(conn)

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
		tradingConn:   conn,
		router:        router,
		wsClients:     make(map[*websocket.Conn]bool),
		upgrader:      upgrader,
	}, nil
}

func (g *APIGateway) setupRoutes() {
	// API routes
	api := g.router.PathPrefix("/api").Subrouter()

	// Health check
	api.HandleFunc("/health", g.healthHandler).Methods("GET")

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

func (g *APIGateway) healthHandler(w http.ResponseWriter, r *http.Request) {
	// Quick health check without making external calls, to meet Kubernetes probes
	response := map[string]interface{}{
		"status":       "healthy",
		"timestamp":    time.Now().Format(time.RFC3339),
		"service_name": "tradinglab-api-gateway",
	}

	// Only perform deep health check for non-probe requests
	if r.Header.Get("User-Agent") != "kube-probe/1.27" {
		// Check gRPC connection with longer timeout
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()

		// Create a simple request to check if the gRPC service is responsive
		req := &pb.HistoricalDataRequest{
			Ticker: "SPY",
			Days:   1,
		}

		// Try to get a response
		_, err := g.tradingClient.GetHistoricalData(ctx, req)
		grpcStatus := "connected"
		if err != nil {
			grpcStatus = fmt.Sprintf("error: %v", err)
			log.Printf("gRPC health check failed: %v", err)
		}

		// Check NATS connection
		natsStatus := "connected"
		if g.natsClient == nil {
			natsStatus = "disconnected"
			log.Printf("NATS connection unavailable")
		} else if !g.natsClient.GetNATS().IsConnected() {
			natsStatus = "disconnected"
			log.Printf("NATS connection lost")
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

	// Create gRPC request
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	req := &pb.HistoricalDataRequest{
		Ticker:   ticker,
		Days:     int32(days),
		Interval: interval,
	}

	// Call gRPC service
	resp, err := g.tradingClient.GetHistoricalData(ctx, req)
	if err != nil {
		http.Error(w, fmt.Sprintf("error fetching historical data: %v", err), http.StatusInternalServerError)
		return
	}

	// Convert gRPC response to JSON-friendly format
	candles := make([]map[string]interface{}, 0, len(resp.Candles))
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

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(candles)
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

	// Create gRPC request
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	req := &pb.SignalRequest{
		Ticker:   ticker,
		Days:     int32(days),
		Strategy: strategy,
		Interval: interval,
	}

	// Call gRPC service
	resp, err := g.tradingClient.GenerateSignals(ctx, req)
	if err != nil {
		http.Error(w, fmt.Sprintf("error generating signals: %v", err), http.StatusInternalServerError)
		return
	}

	// Convert gRPC response to JSON-friendly format
	signals := make([]map[string]interface{}, 0, len(resp.Signals))
	for _, signal := range resp.Signals {
		signals = append(signals, map[string]interface{}{
			"date":        signal.Date,
			"signal_type": signal.SignalType,
			"entry_price": signal.EntryPrice,
			"stoploss":    signal.Stoploss,
		})
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(signals)
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
	// Upgrade HTTP connection to WebSocket
	conn, err := g.upgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Printf("Failed to upgrade to websocket: %v", err)
		return
	}
	defer conn.Close()

	// Register client
	g.wsClientsMutex.Lock()
	g.wsClients[conn] = true
	g.wsClientsMutex.Unlock()

	// Clean up on disconnect
	defer func() {
		g.wsClientsMutex.Lock()
		delete(g.wsClients, conn)
		g.wsClientsMutex.Unlock()
	}()

	// Handle WebSocket messages (for subscription requests)
	go g.handleWebSocketMessages(conn)

	// Keep connection alive with ping/pong
	done := make(chan struct{})
	go func() {
		ticker := time.NewTicker(30 * time.Second)
		defer ticker.Stop()

		for {
			select {
			case <-done:
				return
			case <-ticker.C:
				if err := conn.WriteMessage(websocket.PingMessage, nil); err != nil {
					log.Printf("WebSocket ping failed: %v", err)
					return
				}
			}
		}
	}()

	// Block until connection is closed
	<-done
}

func (g *APIGateway) handleWebSocketMessages(conn *websocket.Conn) {
	// Set up subscriptions based on client messages
	subscriptions := make(map[string]*nats.Subscription)
	defer func() {
		// Clean up subscriptions when connection closes
		for _, sub := range subscriptions {
			sub.Unsubscribe()
		}
	}()

	// Message queue with a buffer to handle slow consumers
	const maxPendingMessages = 100
	messageQueue := make(chan []byte, maxPendingMessages)

	// Start message sender goroutine - handles backpressure
	done := make(chan struct{})
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
				writeTimeout := time.Second * 2
				conn.SetWriteDeadline(time.Now().Add(writeTimeout))
				if err := conn.WriteMessage(websocket.TextMessage, msg); err != nil {
					log.Printf("Error forwarding message to WebSocket, closing: %v", err)
					return
				}
				conn.SetWriteDeadline(time.Time{}) // Reset deadline
			}
		}
	}()

	for {
		// Read message
		messageType, p, err := conn.ReadMessage()
		if err != nil {
			log.Printf("Error reading websocket message: %v", err)
			close(messageQueue) // Signal sender to stop
			return
		}

		// Only process text messages
		if messageType != websocket.TextMessage {
			continue
		}

		// Parse subscription request
		var request struct {
			Action  string `json:"action"`  // "subscribe" or "unsubscribe"
			Type    string `json:"type"`    // "market", "signals", "recommendations"
			Ticker  string `json:"ticker"`  // Stock ticker
			Subject string `json:"subject"` // Optional specific NATS subject
		}

		if err := json.Unmarshal(p, &request); err != nil {
			log.Printf("Error parsing subscription request: %v", err)
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
					log.Printf("WebSocket message queue full for %s, discarding message", subject)
				}
			})

			if err != nil {
				log.Printf("Error subscribing to NATS subject %s: %v", subject, err)
				continue
			}

			// Set pending limits to avoid overwhelming NATS with slow consumers
			// This sets how many messages/bytes can be pending before NATS drops them
			if err := sub.SetPendingLimits(256, 1024*1024); err != nil {
				log.Printf("Error setting pending limits: %v", err)
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
		log.Printf("API Gateway listening on %s", addr)
		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("Server error: %v", err)
		}
	}()

	// Set up graceful shutdown
	quit := make(chan os.Signal, 1)
	// Listen for more signals including SIGKILL and SIGQUIT
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM, syscall.SIGQUIT, syscall.SIGHUP)
	<-quit

	// Shutdown server
	log.Println("Shutting down server...")

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
		log.Println("Closing NATS connection...")
		g.natsClient.Close()
	}

	// Close gRPC connection
	if g.tradingConn != nil {
		log.Println("Closing gRPC connection...")
		g.tradingConn.Close()
	}

	// Now shutdown the HTTP server
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second) // Increased timeout
	defer cancel()

	if err := server.Shutdown(ctx); err != nil {
		log.Printf("HTTP server shutdown error: %v", err)
		return fmt.Errorf("server shutdown failed: %w", err)
	}

	log.Println("Server gracefully stopped")
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
		log.Fatalf("Failed to create API Gateway: %v", err)
	}

	// Set up routes
	gateway.setupRoutes()

	// Start server
	if err := gateway.Serve(addr); err != nil {
		log.Fatalf("Server error: %v", err)
	}
}

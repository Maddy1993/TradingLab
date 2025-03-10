package hub

import (
	"encoding/json"
	"net/http"
	"time"
	
	"github.com/myapp/tradinglab/pkg/utils"
)

// HealthResponse represents the health check response
type HealthResponse struct {
	Status    string     `json:"status"`
	Timestamp time.Time  `json:"timestamp"`
	Version   string     `json:"version"`
	Stats     EventStats `json:"stats"`
}

// StartHealthServer starts a HTTP server for health checks
func (h *EventHub) StartHealthServer(addr string) error {
	mux := http.NewServeMux()

	// Health check endpoint
	mux.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
		stats := h.GetStats()

		response := HealthResponse{
			Status:    "UP",
			Timestamp: time.Now(),
			Version:   "1.0.0",
			Stats:     stats,
		}

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)

		if err := json.NewEncoder(w).Encode(response); err != nil {
			utils.Error("Error encoding health response: %v", err)
		}
	})

	// Start HTTP server
	utils.Info("Starting health server on %s", addr)
	return http.ListenAndServe(addr, mux)
}
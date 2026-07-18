package handlers

import (
	"encoding/json"
	"net/http"
	"time"
)

// HealthResponse represents the health check response
type HealthResponse struct {
	Status  string `json:"status"`
	Service string `json:"service"`
	Uptime  string `json:"uptime"`
}

var startedAt = time.Now()

// Health returns the health status of this playlist node
func (h *Handler) Health(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}

	resp := HealthResponse{
		Status:  "ok",
		Service: "content-node",
		Uptime:  time.Since(startedAt).Round(time.Second).String(),
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(resp)
}

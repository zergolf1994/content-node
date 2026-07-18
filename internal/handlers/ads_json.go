package handlers

import (
	"encoding/json"
	"log"
	"net/http"
	"strings"

	"content-node/internal/services"
)

// AdvertJSON handles GET /advert/{adSlug}.json — unified advert feed (script, image, video).
func (h *Handler) AdvertJSON(w http.ResponseWriter, r *http.Request) {
	adSlug := strings.TrimSuffix(strings.TrimPrefix(r.URL.Path, "/advert/"), ".json")
	if adSlug == "" || strings.Contains(adSlug, "/") {
		writeAdsFeedError(w, http.StatusNotFound, "not found")
		return
	}

	writeAdsFeedJSON(w, services.BuildAdvertFeed(adSlug))
}

func writeAdsFeedJSON(w http.ResponseWriter, v interface{}) {
	data, err := json.Marshal(v)
	if err != nil {
		log.Printf("⚠️ Failed to encode ads feed: %v", err)
		writeAdsFeedError(w, http.StatusInternalServerError, "encode error")
		return
	}

	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.Header().Set("Access-Control-Allow-Origin", "*")
	w.Header().Set("Cache-Control", "no-store")
	w.Header().Set("CDN-Cache-Control", "max-age=300")
	w.Write(data)
}

func writeAdsFeedError(w http.ResponseWriter, status int, message string) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.Header().Set("Access-Control-Allow-Origin", "*")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(map[string]string{"error": message})
}

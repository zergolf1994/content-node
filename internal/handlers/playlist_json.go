package handlers

import (
	"encoding/json"
	"log"
	"net/http"
	"strings"

	"content-node/internal/services"
)

// PlaylistJSON handles GET /playlist/{fileSlug}.json — JW Player playlist feed.
func (h *Handler) PlaylistJSON(w http.ResponseWriter, r *http.Request) {
	if services.IsMaintenanceMode() {
		writePlaylistError(w, http.StatusServiceUnavailable, "maintenance")
		return
	}

	path := strings.TrimPrefix(r.URL.Path, "/playlist/")
	slug := strings.TrimSuffix(path, ".json")
	if slug == "" || strings.Contains(slug, "/") {
		writePlaylistError(w, http.StatusNotFound, "not found")
		return
	}

	resolved, resolveErr := h.resolveEmbed(r, slug)
	if resolveErr != nil {
		if resolveErr.Processing != nil {
			writePlaylistError(w, http.StatusNotFound, resolveErr.Processing.State)
			return
		}
		writePlaylistError(w, resolveErr.Status, resolveErr.Message)
		return
	}

	feed := services.BuildJWPlaylistFeed(
		resolved.File.Name,
		resolved.Slug,
		resolved.Content.PosterURL,
		resolved.Content.PlaylistM3U8,
		resolved.Content.SpriteVttURL,
	)

	data, err := json.Marshal(feed)
	if err != nil {
		log.Printf("⚠️ Failed to encode playlist.json: %v", err)
		writePlaylistError(w, http.StatusInternalServerError, "encode error")
		return
	}

	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.Header().Set("Access-Control-Allow-Origin", "*")
	w.Header().Set("Cache-Control", "no-store")
	w.Header().Set("CDN-Cache-Control", "max-age=300")
	w.Write(data)
}

func writePlaylistError(w http.ResponseWriter, status int, message string) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.Header().Set("Access-Control-Allow-Origin", "*")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(map[string]string{"error": message})
}

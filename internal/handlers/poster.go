package handlers

import (
	"context"
	"fmt"
	"io"
	"log"
	"net/http"
	"strings"
	"time"

	"content-node/internal/core/enums"
	"content-node/internal/db/models"

	"go.mongodb.org/mongo-driver/bson"
)

// HandlePoster handles GET /thumb/{fileSlug}/{n}.jpg
// Proxies thumbnail from nginx-vod-module via storage
func (h *Handler) HandlePoster(w http.ResponseWriter, r *http.Request) {
	path := strings.TrimPrefix(r.URL.Path, "/thumb/")
	if path == "" {
		sendNotFound(w, r, http.StatusNotFound)
		return
	}

	lastSlash := strings.LastIndex(path, "/")
	if lastSlash <= 0 {
		sendNotFound(w, r, http.StatusNotFound)
		return
	}

	slug := path[:lastSlash]
	filename := path[lastSlash+1:]

	if !strings.HasSuffix(filename, ".jpg") {
		sendNotFound(w, r, http.StatusNotFound)
		return
	}
	timePart := strings.TrimSuffix(filename, ".jpg")
	if timePart == "" {
		sendNotFound(w, r, http.StatusNotFound)
		return
	}
	for _, c := range timePart {
		if c < '0' || c > '9' {
			sendNotFound(w, r, http.StatusNotFound)
			return
		}
	}

	if strings.Contains(slug, "/") {
		sendNotFound(w, r, http.StatusNotFound)
		return
	}

	ctx, cancel := context.WithTimeout(r.Context(), 15*time.Second)
	defer cancel()

	// ─── Step 1: Find file by slug ───────────────────────────────────────
	var file models.File
	err := models.FileModel.Col().FindOne(ctx, bson.M{"slug": slug}).Decode(&file)
	if err != nil {
		log.Printf("[Poster] File not found: %s", slug)
		sendNotFound(w, r, http.StatusNotFound)
		return
	}

	if file.IsTrashed() || file.IsDeleted() {
		sendNotFound(w, r, http.StatusNotFound)
		return
	}

	// ─── Step 2: Find video media ────────────────────────────────────────
	var media models.Media
	err = models.MediaModel.Col().FindOne(ctx, bson.M{
		"fileId":    file.ID,
		"type":      enums.MediaTypeVideo,
		"deletedAt": nil,
	}).Decode(&media)
	if err != nil {
		log.Printf("[Poster] Video media not found for fileId=%s: %v", file.ID, err)
		sendNotFound(w, r, http.StatusNotFound)
		return
	}

	// ─── Step 3: Find storage ────────────────────────────────────────────
	storageID := ""
	if media.StorageID != nil {
		storageID = *media.StorageID
	}

	var storage models.Storage
	err = models.StorageModel.Col().FindOne(ctx, bson.M{"_id": storageID}).Decode(&storage)
	if err != nil {
		log.Printf("[Poster] Storage not found: %s", storageID)
		sendNotFound(w, r, http.StatusNotFound)
		return
	}

	storageHost := storage.GetHost()
	if storageHost == "" {
		log.Printf("[Poster] Storage has no host: %s", storage.ID)
		sendNotFound(w, r, http.StatusNotFound)
		return
	}

	// ─── Step 4: Proxy to VOD server ─────────────────────────────────────
	vodPort := storage.GetPort() + 1 // VOD port = storage port + 1 (e.g. 8888 → 8889)
	timeMs := timePart + "000"       // seconds → milliseconds
	thumbURL := fmt.Sprintf("http://%s:%d/thumb/%s.json/thumb-%s-w500.jpg",
		storageHost, vodPort, media.Slug, timeMs)
	log.Printf("[Poster] Fetching poster: %s", thumbURL)

	upstreamReq, err := http.NewRequestWithContext(ctx, http.MethodGet, thumbURL, nil)
	if err != nil {
		sendNotFound(w, r, http.StatusNotFound)
		return
	}

	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Do(upstreamReq)
	if err != nil {
		log.Printf("[Poster] Upstream request failed: %s → %v", thumbURL, err)
		sendNotFound(w, r, http.StatusNotFound)
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		log.Printf("[Poster] Upstream returned %d: %s", resp.StatusCode, thumbURL)
		sendNotFound(w, r, http.StatusNotFound)
		return
	}

	w.Header().Set("Content-Type", "image/jpeg")
	if cl := resp.Header.Get("Content-Length"); cl != "" {
		w.Header().Set("Content-Length", cl)
	}
	w.Header().Set("Cache-Control", "public, max-age=86400, immutable")
	w.Header().Set("Access-Control-Allow-Origin", "*")
	w.WriteHeader(http.StatusOK)

	buf := make([]byte, 32*1024)
	io.CopyBuffer(w, resp.Body, buf)
}

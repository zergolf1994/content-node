package handlers

import (
	"context"
	"fmt"
	"io"
	"log"
	"math"
	"net/http"
	"strconv"
	"strings"
	"time"

	"content-node/internal/core/enums"
	"content-node/internal/db/models"

	"go.mongodb.org/mongo-driver/bson"
)

// HandlePoster handles GET /thumb/{fileSlug}/{n}.jpg and /thumb/{fileSlug}/poster.jpg.
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
	isDefaultPoster := timePart == "poster"
	if !isDefaultPoster {
		for _, c := range timePart {
			if c < '0' || c > '9' {
				sendNotFound(w, r, http.StatusNotFound)
				return
			}
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
	mediaCursor, err := models.MediaModel.Col().Find(ctx, bson.M{
		"fileId":    file.ID,
		"type":      enums.MediaTypeVideo,
		"deletedAt": nil,
	})
	if err != nil {
		log.Printf("[Poster] Video media not found for fileId=%s: %v", file.ID, err)
		sendNotFound(w, r, http.StatusNotFound)
		return
	}
	defer mediaCursor.Close(ctx)

	var videoMedias []models.Media
	for mediaCursor.Next(ctx) {
		var candidate models.Media
		if err := mediaCursor.Decode(&candidate); err == nil {
			videoMedias = append(videoMedias, candidate)
		}
	}
	if err := mediaCursor.Err(); err != nil {
		log.Printf("[Poster] Error reading video media for fileId=%s: %v", file.ID, err)
		sendNotFound(w, r, http.StatusNotFound)
		return
	}

	media, ok := selectPosterMedia(videoMedias)
	if !ok {
		log.Printf("[Poster] Video media not found for fileId=%s", file.ID)
		sendNotFound(w, r, http.StatusNotFound)
		return
	}
	timePart = strconv.Itoa(resolvePosterSecond(timePart, isDefaultPoster, file, media))

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

	vodBaseURL := storage.GetVODBaseURL()
	if vodBaseURL == "" {
		log.Printf("[Poster] Storage has no VOD URL: %s", storage.ID)
		sendNotFound(w, r, http.StatusNotFound)
		return
	}

	// ─── Step 4: Proxy to VOD server ─────────────────────────────────────
	timeMs := timePart + "000" // seconds → milliseconds
	thumbURL := fmt.Sprintf("%s/thumb/%s.json/thumb-%s-w500.jpg", vodBaseURL, media.Slug, timeMs)
	if storage.Type == enums.StorageTypeS3 {
		thumbURL = fmt.Sprintf("%s/%s/thumb-%s-w500.jpg", vodBaseURL, media.Slug, timeMs)
	}
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

// selectPosterMedia chooses the lowest numeric resolution available. Original
// is used only when no processed numeric resolution exists.
func selectPosterMedia(medias []models.Media) (models.Media, bool) {
	var lowest models.Media
	var original models.Media
	lowestResolution := int(^uint(0) >> 1)
	hasLowest := false
	hasOriginal := false

	for _, media := range medias {
		if media.Resolution == nil {
			continue
		}
		resolution := strings.TrimSpace(*media.Resolution)
		if resolution == enums.ResolutionOriginal {
			original = media
			hasOriginal = true
			continue
		}
		numeric, err := strconv.Atoi(resolution)
		if err == nil && numeric > 0 && numeric < lowestResolution {
			lowest = media
			lowestResolution = numeric
			hasLowest = true
		}
	}

	if hasLowest {
		return lowest, true
	}
	return original, hasOriginal
}

func resolvePosterSecond(timePart string, isDefault bool, file models.File, media models.Media) int {
	duration := 0.0
	if media.Metadata != nil && media.Metadata.Duration > 0 {
		duration = media.Metadata.Duration
	} else if file.Metadata != nil && file.Metadata.Duration != nil && *file.Metadata.Duration > 0 {
		duration = *file.Metadata.Duration
	}

	second := 0
	if isDefault {
		second = int(duration / 2)
	} else if parsed, err := strconv.Atoi(timePart); err == nil && parsed > 0 {
		second = parsed
	}

	if duration > 0 && float64(second) >= duration {
		second = int(math.Ceil(duration)) - 1
		if second < 0 {
			second = 0
		}
	}
	return second
}

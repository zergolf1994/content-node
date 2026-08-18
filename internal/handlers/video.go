package handlers

import (
	"context"
	"errors"
	"fmt"
	"log"
	"net/http"
	"strings"
	"time"

	"content-node/internal/cache"
	"content-node/internal/db/models"
	"content-node/internal/utils"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
)

// HandleVideo handles GET /{mediaSlug}/video.m3u8
// Proxies the HLS segment playlist from storage and rewrites segment URLs
func (h *Handler) HandleVideo(w http.ResponseWriter, r *http.Request) {
	path := strings.TrimPrefix(r.URL.Path, "/")
	slug := strings.TrimSuffix(path, "/video.m3u8")

	if slug == "" {
		HandleNotFound(w, r)
		return
	}

	ctx, cancel := context.WithTimeout(r.Context(), 10*time.Second)
	defer cancel()

	// ─── Step 1+2: Resolve media → storage (ผ่าน Redis lookup cache) ─────
	// เก็บเฉพาะ playback URL + public domains ไม่เก็บ playlist
	// ทั้งก้อน — body ใหญ่และ CF cache ปลายทางอยู่แล้ว
	type videoLookup struct {
		PlaybackBaseURL string   `json:"playbackBaseUrl"`
		PublicDomains   []string `json:"publicDomains"`
	}
	cacheKey := "playlist_video_v2:" + slug

	var lk videoLookup
	if !cache.GetJSON(cacheKey, &lk) {
		var media models.Media
		err := models.MediaModel.Col().FindOne(ctx, bson.M{
			"slug":      slug,
			"deletedAt": bson.M{"$eq": nil},
		}).Decode(&media)
		if err != nil {
			log.Printf("[Video] Media lookup failed for %s: %v", slug, err)
			if errors.Is(err, mongo.ErrNoDocuments) {
				HandleNotFound(w, r)
			} else {
				HandleCachedError(w, r, http.StatusInternalServerError)
			}
			return
		}

		storageID := ""
		if media.StorageID != nil {
			storageID = *media.StorageID
		}

		var storage models.Storage
		err = models.StorageModel.Col().FindOne(ctx, bson.M{"_id": storageID}).Decode(&storage)
		if err != nil {
			log.Printf("[Video] Storage not found for media=%s (storageId=%s)", slug, storageID)
			HandleCachedError(w, r, http.StatusBadGateway)
			return
		}

		lk.PlaybackBaseURL = storage.GetPlaybackBaseURL()
		lk.PublicDomains = storage.GetPublicDomains()
		cache.SetJSON(cacheKey, &lk)
	}

	if len(lk.PublicDomains) == 0 {
		log.Printf("[Video] Storage has no publicUrl (media=%s)", slug)
		HandleCachedError(w, r, http.StatusBadGateway)
		return
	}

	// ─── Step 3: Parse publicUrl domains (comma-separated) ──────────────
	domains := lk.PublicDomains

	// ─── Step 4: Fetch HLS playlist from storage server ─────────────────
	if lk.PlaybackBaseURL == "" {
		log.Printf("[Video] Storage has no playback URL (media=%s)", slug)
		HandleCachedError(w, r, http.StatusBadGateway)
		return
	}

	storageHLSURL := fmt.Sprintf("%s/%s/video.m3u8", lk.PlaybackBaseURL, slug)

	playlistContent, err := utils.FetchURLContent(ctx, storageHLSURL)
	if err != nil {
		log.Printf("[Video] Failed to fetch playlist from %s: %v", storageHLSURL, err)
		if errors.Is(err, context.DeadlineExceeded) || errors.Is(ctx.Err(), context.DeadlineExceeded) {
			HandleCachedError(w, r, http.StatusGatewayTimeout)
		} else {
			HandleCachedError(w, r, http.StatusBadGateway)
		}
		return
	}

	// ─── Step 5: Rewrite segment URLs to use publicUrl domains ──────────
	rewrittenPlaylist := utils.RewritePlaylist(playlistContent, domains, slug)

	responseBody := []byte(rewrittenPlaylist)
	w.Header().Set("Content-Type", "application/vnd.apple.mpegurl")
	w.Header().Set("Content-Length", fmt.Sprintf("%d", len(responseBody)))
	w.Header().Set("Access-Control-Allow-Origin", "*")
	w.Header().Set("Cache-Control", "no-store")
	w.Header().Set("CDN-Cache-Control", "public, max-age=2592000")

	w.Write(responseBody)
}

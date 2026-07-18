package handlers

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"strings"
	"time"

	"content-node/internal/cache"
	"content-node/internal/db/models"
	"content-node/internal/utils"

	"go.mongodb.org/mongo-driver/bson"
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
	// เก็บเฉพาะค่าที่ใช้เข้าถึง (host + publicUrl ~100B) ไม่เก็บ playlist
	// ทั้งก้อน — body ใหญ่และ CF cache ปลายทางอยู่แล้ว
	type videoLookup struct {
		Host      string `json:"host"`
		PublicURL string `json:"publicUrl"`
	}
	cacheKey := "playlist_video:" + slug

	var lk videoLookup
	if !cache.GetJSON(cacheKey, &lk) {
		var media models.Media
		err := models.MediaModel.Col().FindOne(ctx, bson.M{
			"slug":      slug,
			"deletedAt": bson.M{"$eq": nil},
		}).Decode(&media)
		if err != nil {
			log.Printf("[Video] Media not found: %s", slug)
			HandleNotFound(w, r)
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
			HandleNotFound(w, r)
			return
		}

		if storage.PublicURL != nil {
			lk.PublicURL = *storage.PublicURL
		}
		lk.Host = storage.GetHost()
		cache.SetJSON(cacheKey, &lk)
	}

	if lk.PublicURL == "" {
		log.Printf("[Video] Storage has no publicUrl (media=%s)", slug)
		HandleNotFound(w, r)
		return
	}

	// ─── Step 3: Parse publicUrl domains (comma-separated) ──────────────
	parts := strings.Split(lk.PublicURL, ",")
	domains := make([]string, 0, len(parts))
	for _, d := range parts {
		d = strings.TrimSpace(d)
		if d != "" {
			domains = append(domains, d)
		}
	}

	// ─── Step 4: Fetch HLS playlist from storage server ─────────────────
	if lk.Host == "" {
		log.Printf("[Video] Storage has no host (media=%s)", slug)
		HandleNotFound(w, r)
		return
	}

	storageHLSURL := fmt.Sprintf("http://%s/%s/video.m3u8", lk.Host, slug)

	playlistContent, err := utils.FetchURLContent(ctx, storageHLSURL)
	if err != nil {
		log.Printf("[Video] Failed to fetch playlist from %s: %v", storageHLSURL, err)
		HandleNotFound(w, r)
		return
	}

	// ─── Step 5: Rewrite segment URLs to use publicUrl domains ──────────
	rewrittenPlaylist := utils.RewritePlaylist(playlistContent, domains, slug)

	responseBody := []byte(rewrittenPlaylist)
	w.Header().Set("Content-Type", "application/vnd.apple.mpegurl")
	w.Header().Set("Content-Length", fmt.Sprintf("%d", len(responseBody)))
	w.Header().Set("Access-Control-Allow-Origin", "*")
	w.Header().Set("Cache-Control", "no-store")
	w.Header().Set("CDN-Cache-Control", "max-age=31536000")

	w.Write(responseBody)
}

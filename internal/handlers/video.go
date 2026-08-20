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
	"content-node/internal/core/enums"
	"content-node/internal/db/models"
	"content-node/internal/utils"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
)

// HandleVideo handles GET /{mediaSlug}/video.m3u8
// Proxies the HLS segment playlist from storage and rewrites segment URLs
func (h *Handler) HandleVideo(w http.ResponseWriter, r *http.Request) {
	h.handleMediaPlaylist(w, r, mediaPlaylistOptions{
		publicSuffix:     "/video.m3u8",
		mediaType:        enums.MediaTypeVideo,
		cachePrefix:      "playlist_video_v4:",
		logLabel:         "Video",
		upstreamPlaylist: "video.m3u8",
	})
}

// HandleAudio handles GET /{mediaSlug}/audio.m3u8.
func (h *Handler) HandleAudio(w http.ResponseWriter, r *http.Request) {
	h.handleMediaPlaylist(w, r, mediaPlaylistOptions{
		publicSuffix:     "/audio.m3u8",
		mediaType:        enums.MediaTypeAudio,
		cachePrefix:      "playlist_audio_v2:",
		logLabel:         "Audio",
		upstreamPlaylist: "audio.m3u8",
	})
}

type mediaPlaylistOptions struct {
	publicSuffix     string
	mediaType        string
	cachePrefix      string
	logLabel         string
	upstreamPlaylist string
}

func (h *Handler) handleMediaPlaylist(w http.ResponseWriter, r *http.Request, options mediaPlaylistOptions) {
	path := strings.TrimPrefix(r.URL.Path, "/")
	slug := strings.TrimSuffix(path, options.publicSuffix)

	if slug == "" {
		HandleNotFound(w, r)
		return
	}

	ctx, cancel := context.WithTimeout(r.Context(), 10*time.Second)
	defer cancel()

	// ─── Step 1+2: Resolve media → storage (ผ่าน Redis lookup cache) ─────
	// เก็บเฉพาะ playback URL + public domains ไม่เก็บ playlist
	// ทั้งก้อน — body ใหญ่และ CF cache ปลายทางอยู่แล้ว
	type mediaLookup struct {
		PlaybackBaseURL string   `json:"playbackBaseUrl"`
		PublicDomains   []string `json:"publicDomains"`
		PlaylistName    string   `json:"playlistName"`
	}
	cacheKey := options.cachePrefix + slug

	var lk mediaLookup
	if !cache.GetJSON(cacheKey, &lk) {
		var media models.Media
		err := models.MediaModel.Col().FindOne(ctx, bson.M{
			"slug":      slug,
			"type":      options.mediaType,
			"deletedAt": bson.M{"$eq": nil},
		}).Decode(&media)
		if err != nil {
			log.Printf("[%s] Media lookup failed for %s: %v", options.logLabel, slug, err)
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
			log.Printf("[%s] Storage not found for media=%s (storageId=%s)", options.logLabel, slug, storageID)
			HandleCachedError(w, r, http.StatusBadGateway)
			return
		}

		lk.PlaybackBaseURL = storage.GetPlaybackBaseURL()
		lk.PublicDomains = storage.GetPublicDomains()
		lk.PlaylistName = options.upstreamPlaylist
		cache.SetJSON(cacheKey, &lk)
	}

	if len(lk.PublicDomains) == 0 {
		log.Printf("[%s] Storage has no publicUrl (media=%s)", options.logLabel, slug)
		HandleCachedError(w, r, http.StatusBadGateway)
		return
	}

	// ─── Step 3: Parse publicUrl domains (comma-separated) ──────────────
	domains := lk.PublicDomains

	// ─── Step 4: Fetch HLS playlist from storage server ─────────────────
	if lk.PlaybackBaseURL == "" {
		log.Printf("[%s] Storage has no playback URL (media=%s)", options.logLabel, slug)
		HandleCachedError(w, r, http.StatusBadGateway)
		return
	}

	storageHLSURL := fmt.Sprintf("%s/%s/%s", lk.PlaybackBaseURL, slug, lk.PlaylistName)

	playlistContent, err := utils.FetchURLContent(ctx, storageHLSURL)
	if err != nil {
		log.Printf("[%s] Failed to fetch playlist from %s: %v", options.logLabel, storageHLSURL, err)
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

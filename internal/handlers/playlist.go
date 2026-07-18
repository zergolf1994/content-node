package handlers

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"strings"
	"time"

	"content-node/internal/core/enums"
	"content-node/internal/db/models"
	"content-node/internal/utils"

	"go.mongodb.org/mongo-driver/bson"
)

// HandlePlaylist handles GET /{fileSlug}/playlist.m3u8
// Flow: fileSlug → file._id → medias (video type) → master HLS playlist
func (h *Handler) HandlePlaylist(w http.ResponseWriter, r *http.Request) {
	path := strings.TrimPrefix(r.URL.Path, "/")
	slug := strings.TrimSuffix(path, "/playlist.m3u8")

	if slug == "" {
		HandleNotFound(w, r)
		return
	}

	ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
	defer cancel()

	// ─── Step 1: Find file by slug ───────────────────────────────────────
	var file models.File
	err := models.FileModel.Col().FindOne(ctx, bson.M{"slug": slug}).Decode(&file)
	if err != nil {
		log.Printf("[Playlist] File not found for slug=%s: %v", slug, err)
		HandleNotFound(w, r)
		return
	}

	// ─── Step 2: Find all video media for this file ──────────────────────
	mediaFilter := bson.M{
		"fileId": file.ID,
		"type":   enums.MediaTypeVideo,
		"resolution": bson.M{"$in": []string{
			enums.ResolutionOriginal,
			enums.Resolution1080,
			enums.Resolution720,
			enums.Resolution480,
			enums.Resolution360,
		}},
		"deletedAt": bson.M{"$eq": nil},
	}

	cursor, err := models.MediaModel.Col().Find(ctx, mediaFilter)
	if err != nil {
		log.Printf("[Playlist] Error finding media for fileId=%s: %v", file.ID, err)
		HandleNotFound(w, r)
		return
	}
	defer cursor.Close(ctx)

	var medias []models.Media
	for cursor.Next(ctx) {
		var media models.Media
		if err := cursor.Decode(&media); err != nil {
			continue
		}
		medias = append(medias, media)
	}

	if len(medias) == 0 {
		log.Printf("[Playlist] No video media found for fileId=%s", file.ID)
		HandleNotFound(w, r)
		return
	}

	// If standard resolutions exist (1080/720/480/360), hide "original"
	hasStandard := false
	for _, m := range medias {
		res := ""
		if m.Resolution != nil {
			res = *m.Resolution
		}
		if res == enums.Resolution1080 || res == enums.Resolution720 ||
			res == enums.Resolution480 || res == enums.Resolution360 {
			hasStandard = true
			break
		}
	}
	if hasStandard {
		filtered := make([]models.Media, 0, len(medias))
		for _, m := range medias {
			if m.Resolution == nil || *m.Resolution != enums.ResolutionOriginal {
				filtered = append(filtered, m)
			}
		}
		medias = filtered
	}

	// ─── Step 3: Generate master playlist ────────────────────────────────
	host := r.Host
	var playlist strings.Builder

	playlist.WriteString("#EXTM3U\n")
	playlist.WriteString("#EXT-X-VERSION:6\n")

	storageCache := make(map[string]models.Storage)

	for _, media := range medias {
		var streamInf string

		storageID := ""
		if media.StorageID != nil {
			storageID = *media.StorageID
		}

		if storageID != "" {
			storage, ok := storageCache[storageID]
			if !ok {
				err := models.StorageModel.Col().FindOne(ctx, bson.M{"_id": storageID}).Decode(&storage)
				if err == nil {
					storageCache[storageID] = storage
				} else {
					log.Printf("[Playlist] Storage not found: %s", storageID)
				}
			}

			storageHost := storage.GetHost()
			if storageHost != "" {
				masterURL := fmt.Sprintf("http://%s/%s/master.m3u8", storageHost, media.Slug)
				content, err := utils.FetchURLContent(ctx, masterURL)
				if err == nil {
					streamInf = extractStreamInf(content)
				} else {
					log.Printf("[Playlist] Failed to fetch master from %s: %v", masterURL, err)
				}
			}
		}

		// Fallback to estimation if fetch failed
		if streamInf == "" {
			resolution := ""
			if media.Resolution != nil {
				resolution = *media.Resolution
			}
			bandwidth := getEstimatedBandwidth(resolution)
			width, height := getResolutionDimensions(resolution)

			streamInf = "#EXT-X-STREAM-INF:BANDWIDTH=" + bandwidth
			if width > 0 && height > 0 {
				streamInf += ",RESOLUTION=" + formatResolution(width, height)
			}
		}

		playlist.WriteString(streamInf + "\n")
		playlist.WriteString("//" + host + "/" + media.Slug + "/video.m3u8\n")
	}

	w.Header().Set("Content-Type", "application/vnd.apple.mpegurl")
	w.Header().Set("Access-Control-Allow-Origin", "*")
	w.Header().Set("Cache-Control", "no-store")
	w.Header().Set("CDN-Cache-Control", "max-age=2592000")

	w.Write([]byte(playlist.String()))
}

func extractStreamInf(content string) string {
	lines := strings.Split(content, "\n")
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, "#EXT-X-STREAM-INF") {
			return trimmed
		}
	}
	return ""
}

func getEstimatedBandwidth(resolution string) string {
	switch resolution {
	case "original":
		return "8000000"
	case "2160", "4k", "4K":
		return "15000000"
	case "1080", "1080p":
		return "5000000"
	case "720", "720p":
		return "2500000"
	case "480", "480p":
		return "1000000"
	case "360", "360p":
		return "500000"
	default:
		return "2500000"
	}
}

func getResolutionDimensions(resolution string) (int, int) {
	switch resolution {
	case "original":
		return 0, 0
	case "2160", "4k", "4K":
		return 3840, 2160
	case "1080", "1080p":
		return 1920, 1080
	case "720", "720p":
		return 1280, 720
	case "480", "480p":
		return 854, 480
	case "360", "360p":
		return 640, 360
	default:
		return 1280, 720
	}
}

func formatResolution(width, height int) string {
	return fmt.Sprintf("%dx%d", width, height)
}

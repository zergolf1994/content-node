package handlers

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/url"
	"strings"
	"time"

	"content-node/internal/core/enums"
	"content-node/internal/db/models"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
)

// HandleSubtitle serves GET /{mediaSlug}/subtitle.vtt. Subtitle media is
// addressed by its own slug because one video can have multiple tracks.
func (h *Handler) HandleSubtitle(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet && r.Method != http.MethodHead {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}

	path := strings.Trim(strings.TrimSuffix(r.URL.Path, "/subtitle.vtt"), "/")
	if path == "" || strings.Contains(path, "/") {
		HandleNotFound(w, r)
		return
	}
	mediaSlug, err := url.PathUnescape(path)
	if err != nil || strings.TrimSpace(mediaSlug) == "" {
		HandleNotFound(w, r)
		return
	}

	ctx, cancel := context.WithTimeout(r.Context(), 15*time.Second)
	defer cancel()

	var media models.Media
	if err := models.MediaModel.Col().FindOne(ctx, bson.M{
		"slug":      mediaSlug,
		"type":      enums.MediaTypeSubtitle,
		"deletedAt": bson.M{"$eq": nil},
	}).Decode(&media); err != nil {
		if !errors.Is(err, mongo.ErrNoDocuments) {
			log.Printf("[Subtitle] media lookup failed slug=%s: %v", mediaSlug, err)
		}
		subtitleError(w, r, http.StatusNotFound)
		return
	}
	if media.StorageID == nil || strings.TrimSpace(*media.StorageID) == "" ||
		media.FileID == nil || strings.TrimSpace(*media.FileID) == "" {
		subtitleError(w, r, http.StatusNotFound)
		return
	}

	var file models.File
	if err := models.FileModel.Col().FindOne(ctx, bson.M{"_id": *media.FileID}).Decode(&file); err != nil || file.IsTrashed() || file.IsDeleted() {
		subtitleError(w, r, http.StatusNotFound)
		return
	}

	var storage models.Storage
	if err := models.StorageModel.Col().FindOne(ctx, bson.M{"_id": *media.StorageID}).Decode(&storage); err != nil {
		subtitleError(w, r, http.StatusBadGateway)
		return
	}

	sourceURL, err := subtitleSourceURL(media, storage, file)
	if err != nil {
		log.Printf("[Subtitle] resolve source failed slug=%s: %v", mediaSlug, err)
		subtitleError(w, r, http.StatusNotFound)
		return
	}

	upstreamReq, err := http.NewRequestWithContext(ctx, r.Method, sourceURL, nil)
	if err != nil {
		subtitleError(w, r, http.StatusBadGateway)
		return
	}
	upstreamReq.Header.Set("User-Agent", "ContentNode/1.0")
	resp, err := (&http.Client{Timeout: 15 * time.Second}).Do(upstreamReq)
	if err != nil {
		log.Printf("[Subtitle] fetch failed %s: %v", sourceURL, err)
		subtitleError(w, r, http.StatusBadGateway)
		return
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		log.Printf("[Subtitle] origin returned %d for %s", resp.StatusCode, sourceURL)
		subtitleError(w, r, http.StatusBadGateway)
		return
	}

	w.Header().Set("Content-Type", "text/vtt; charset=utf-8")
	w.Header().Set("Cache-Control", "public, max-age=2592000")
	w.Header().Set("CDN-Cache-Control", "public, max-age=2592000")
	if r.Method == http.MethodHead {
		return
	}
	if _, err := io.Copy(w, resp.Body); err != nil {
		log.Printf("[Subtitle] stream failed slug=%s: %v", mediaSlug, err)
	}
}

func subtitleError(w http.ResponseWriter, r *http.Request, status int) {
	w.Header().Set("Cache-Control", "public, max-age=60")
	w.Header().Set("CDN-Cache-Control", "public, max-age=60")
	HandleCachedError(w, r, status)
}

func subtitleSourceURL(media models.Media, storage models.Storage, file models.File) (string, error) {
	fileName := ""
	if media.FileName != nil {
		fileName = strings.TrimSpace(*media.FileName)
	}
	if fileName == "" {
		return "", fmt.Errorf("fileName is empty")
	}

	if storage.Type == enums.StorageTypeS3 {
		base := storage.GetS3ObjectBaseURL()
		objectPath := ""
		if media.Path != nil {
			objectPath = strings.Trim(*media.Path, "/")
		}
		if objectPath == "" {
			objectPath = strings.TrimSpace(media.EffectiveFileID()) + "/" + fileName
		}
		if base == "" || objectPath == "" {
			return "", fmt.Errorf("S3 subtitle origin/path is unavailable")
		}
		return base + "/" + escapeSubtitlePath(objectPath), nil
	}

	if storage.GetHostPort() == "" || strings.TrimSpace(file.Slug) == "" {
		return "", fmt.Errorf("local subtitle file/storage is unavailable")
	}
	return "http://" + storage.GetHostPort() + "/" + url.PathEscape(file.Slug) + "/" + url.PathEscape(fileName), nil
}

func escapeSubtitlePath(value string) string {
	parts := strings.Split(strings.Trim(value, "/"), "/")
	for index, part := range parts {
		parts[index] = url.PathEscape(part)
	}
	return strings.Join(parts, "/")
}

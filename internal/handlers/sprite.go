package handlers

import (
	"context"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/url"
	"strings"
	"time"

	"content-node/internal/core/enums"
	"content-node/internal/db/models"
	"content-node/internal/utils"

	"go.mongodb.org/mongo-driver/bson"
)

// HandleSpriteVTT handles GET /{fileSlug}/sprite/sprite.vtt
func (h *Handler) HandleSpriteVTT(w http.ResponseWriter, r *http.Request) {
	path := strings.TrimPrefix(r.URL.Path, "/")
	slug := strings.TrimSuffix(path, "/sprite/sprite.vtt")

	if slug == "" || strings.Contains(slug, "/") {
		HandleNotFound(w, r)
		return
	}

	ctx, cancel := context.WithTimeout(r.Context(), 10*time.Second)
	defer cancel()

	// ─── Step 1: Find file by slug ───────────────────────────────────────
	var file models.File
	err := models.FileModel.Col().FindOne(ctx, bson.M{"slug": slug}).Decode(&file)
	if err != nil {
		log.Printf("[Sprite] File not found: %s", slug)
		HandleNotFound(w, r)
		return
	}

	if file.IsTrashed() || file.IsDeleted() {
		HandleNotFound(w, r)
		return
	}

	// ─── Step 2: Find thumbnail media ────────────────────────────────────
	var media models.Media
	err = models.MediaModel.Col().FindOne(ctx, bson.M{
		"fileId":    file.ID,
		"type":      enums.MediaTypeThumbnail,
		"deletedAt": nil,
	}).Decode(&media)
	if err != nil {
		log.Printf("[Sprite] Thumbnail media not found for fileId=%s: %v", file.ID, err)
		HandleNotFound(w, r)
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
		log.Printf("[Sprite] Storage not found: %s", storageID)
		HandleNotFound(w, r)
		return
	}

	vttURL, err := spriteSourceURL(&storage, &media, file.Slug, "sprite.vtt")
	if err != nil {
		log.Printf("[Sprite] Cannot resolve VTT source for storage %s: %v", storage.ID, err)
		HandleNotFound(w, r)
		return
	}

	// ─── Step 4: Fetch VTT from storage ──────────────────────────────────
	vttContent, err := utils.FetchURLContent(ctx, vttURL)
	if err != nil {
		log.Printf("[Sprite] Failed to fetch VTT from %s: %v", vttURL, err)
		HandleNotFound(w, r)
		return
	}

	responseBody := []byte(vttContent)
	w.Header().Set("Content-Type", "text/vtt; charset=utf-8")
	w.Header().Set("Content-Length", fmt.Sprintf("%d", len(responseBody)))
	w.Header().Set("Access-Control-Allow-Origin", "*")
	w.Header().Set("Cache-Control", "public, max-age=63072000, immutable")
	w.Write(responseBody)
}

// HandleSpriteImage handles GET /{fileSlug}/sprite/{n}.jpg
func (h *Handler) HandleSpriteImage(w http.ResponseWriter, r *http.Request) {
	path := strings.TrimPrefix(r.URL.Path, "/")
	parts := strings.SplitN(path, "/sprite/", 2)
	if len(parts) != 2 || parts[0] == "" || parts[1] == "" {
		HandleNotFound(w, r)
		return
	}

	slug := parts[0]
	filename := parts[1]

	if !isValidSpriteFilename(filename) {
		HandleNotFound(w, r)
		return
	}

	if strings.Contains(slug, "/") {
		HandleNotFound(w, r)
		return
	}

	ctx, cancel := context.WithTimeout(r.Context(), 15*time.Second)
	defer cancel()

	// ─── Step 1: Find file by slug ───────────────────────────────────────
	var file models.File
	err := models.FileModel.Col().FindOne(ctx, bson.M{"slug": slug}).Decode(&file)
	if err != nil {
		HandleNotFound(w, r)
		return
	}

	if file.IsTrashed() || file.IsDeleted() {
		HandleNotFound(w, r)
		return
	}

	// ─── Step 2: Find thumbnail media ────────────────────────────────────
	var media models.Media
	err = models.MediaModel.Col().FindOne(ctx, bson.M{
		"fileId":    file.ID,
		"type":      enums.MediaTypeThumbnail,
		"deletedAt": nil,
	}).Decode(&media)
	if err != nil {
		HandleNotFound(w, r)
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
		HandleNotFound(w, r)
		return
	}

	sourceURL, err := spriteSourceURL(&storage, &media, file.Slug, filename)
	if err != nil {
		log.Printf("[Sprite] Cannot resolve image source for storage %s: %v", storage.ID, err)
		HandleNotFound(w, r)
		return
	}

	// ─── Step 4: Proxy image from storage ────────────────────────────────
	upstreamReq, err := http.NewRequestWithContext(ctx, http.MethodGet, sourceURL, nil)
	if err != nil {
		HandleNotFound(w, r)
		return
	}

	client := &http.Client{Timeout: 30 * time.Second}
	resp, err := client.Do(upstreamReq)
	if err != nil {
		log.Printf("[Sprite] Upstream request failed: %s → %v", sourceURL, err)
		HandleNotFound(w, r)
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		HandleNotFound(w, r)
		return
	}

	w.Header().Set("Content-Type", "image/jpeg")
	if cl := resp.Header.Get("Content-Length"); cl != "" {
		w.Header().Set("Content-Length", cl)
	}
	w.Header().Set("Cache-Control", "public, max-age=63072000, immutable")
	w.Header().Set("Access-Control-Allow-Origin", "*")
	w.WriteHeader(http.StatusOK)

	buf := make([]byte, 32*1024)
	io.CopyBuffer(w, resp.Body, buf)
}

func spriteSourceURL(storage *models.Storage, media *models.Media, fileSlug, filename string) (string, error) {
	baseURL := storage.GetStorageBaseURL()
	assetID := fileSlug
	if storage.Type == enums.StorageTypeS3 {
		baseURL = storage.GetOriginBaseURL()
		assetID = media.EffectiveFileID()
	}
	if baseURL == "" {
		return "", fmt.Errorf("storage base URL is empty")
	}
	if assetID == "" {
		return "", fmt.Errorf("sprite asset ID is empty")
	}
	return url.JoinPath(baseURL, assetID, "sprite", filename)
}

func isValidSpriteFilename(filename string) bool {
	// server-spritesheet สร้างไฟล์ชื่อ sprite-{n}.jpg (อ้างใน sprite.vtt)
	if !strings.HasPrefix(filename, "sprite-") || !strings.HasSuffix(filename, ".jpg") {
		return false
	}
	name := strings.TrimSuffix(strings.TrimPrefix(filename, "sprite-"), ".jpg")
	if name == "" {
		return false
	}
	for _, c := range name {
		if c < '0' || c > '9' {
			return false
		}
	}
	return true
}

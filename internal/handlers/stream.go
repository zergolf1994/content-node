package handlers

import (
	"context"
	"io"
	"log"
	"net/http"
	"strconv"
	"strings"
	"time"

	"content-node/internal/core/enums"
	"content-node/internal/db/models"

	"go.mongodb.org/mongo-driver/bson"
)

// StreamFile handles GET /{fileSlug}.{ext}
// Flow: slug → file._id → media (by fileId == file._id) → storage → proxy stream
func (h *Handler) StreamFile(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	path := strings.TrimPrefix(r.URL.Path, "/")
	if path == "" {
		sendNotFound(w, r, http.StatusNotFound)
		return
	}

	// Reject multi-segment paths
	if strings.Contains(path, "/") {
		HandleNotFound(w, r)
		return
	}

	lastDot := strings.LastIndex(path, ".")
	if lastDot == -1 {
		sendNotFound(w, r, http.StatusBadRequest)
		return
	}

	fileSlug := path[:lastDot]
	if fileSlug == "" {
		sendNotFound(w, r, http.StatusBadRequest)
		return
	}

	ctx, cancel := context.WithTimeout(r.Context(), 15*time.Second)
	defer cancel()

	// ─── Step 1: Find file by slug ───────────────────────────────────────
	var file models.File
	err := models.FileModel.Col().FindOne(ctx, bson.M{"slug": fileSlug}).Decode(&file)
	if err != nil {
		log.Printf("[Stream] File not found for slug=%s: %v", fileSlug, err)
		sendNotFound(w, r, http.StatusNotFound)
		return
	}

	if file.IsTrashed() || file.IsDeleted() {
		sendNotFound(w, r, http.StatusGone)
		return
	}

	// ─── Step 2: Find media by fileId == file._id ────────────────────────
	mediaFilter := bson.M{
		"fileId":    file.ID,
		"deletedAt": nil,
	}
	// If file is a video → find thumbnail image instead
	if file.Type == enums.FileTypeVideo {
		mediaFilter["type"] = enums.MediaTypeThumbnail
	}

	var media models.Media
	err = models.MediaModel.Col().FindOne(ctx, mediaFilter).Decode(&media)
	if err != nil {
		log.Printf("[Stream] Media not found for fileId=%s: %v", file.ID, err)
		sendNotFound(w, r, http.StatusNotFound)
		return
	}

	// ─── Step 3: Find storage by storageId ───────────────────────────────
	storageID := ""
	if media.StorageID != nil {
		storageID = *media.StorageID
	}
	if storageID == "" {
		sendNotFound(w, r, http.StatusNotFound)
		return
	}

	var storage models.Storage
	err = models.StorageModel.Col().FindOne(ctx, bson.M{"_id": storageID}).Decode(&storage)
	if err != nil {
		log.Printf("[Stream] Storage not found for storageId=%s: %v", storageID, err)
		sendNotFound(w, r, http.StatusNotFound)
		return
	}

	if !storage.IsOnline() {
		sendNotFound(w, r, http.StatusServiceUnavailable)
		return
	}

	publicURL := ""
	if storage.PublicURL != nil {
		publicURL = *storage.PublicURL
	}
	if publicURL == "" {
		log.Printf("[Stream] Storage %s has no publicUrl", storage.ID)
		sendNotFound(w, r, http.StatusInternalServerError)
		return
	}

	// ─── Step 4: Build source URL & proxy stream ─────────────────────────
	mediaPath := ""
	if media.Path != nil {
		mediaPath = *media.Path
	}
	sourceURL := strings.TrimRight(publicURL, "/") + "/" + strings.TrimLeft(mediaPath, "/")
	log.Printf("[Stream] Proxying: slug=%s → %s", fileSlug, sourceURL)

	upstreamReq, err := http.NewRequestWithContext(ctx, http.MethodGet, sourceURL, nil)
	if err != nil {
		log.Printf("[Stream] Failed to create upstream request: %v", err)
		sendNotFound(w, r, http.StatusInternalServerError)
		return
	}

	if rangeHeader := r.Header.Get("Range"); rangeHeader != "" {
		upstreamReq.Header.Set("Range", rangeHeader)
	}

	client := &http.Client{Timeout: 0} // no timeout — streaming
	resp, err := client.Do(upstreamReq)
	if err != nil {
		log.Printf("[Stream] Upstream request failed: %v", err)
		sendNotFound(w, r, http.StatusBadGateway)
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusNotFound {
		sendNotFound(w, r, http.StatusNotFound)
		return
	}

	// ─── Step 5: Check for image resize params ───────────────────────────
	imgParams := parseImageParams(r)
	contentType := resp.Header.Get("Content-Type")
	if contentType == "" && media.MimeType != nil {
		contentType = *media.MimeType
	}

	if imgParams != nil && isImageContentType(contentType) {
		imgData, err := io.ReadAll(resp.Body)
		if err != nil {
			log.Printf("[Stream] Failed to read image body: %v", err)
			sendNotFound(w, r, http.StatusInternalServerError)
			return
		}

		resized, outType, err := resizeImage(imgData, contentType, imgParams)
		if err != nil {
			log.Printf("[Stream] Failed to resize image: %v", err)
			// Fallback: serve original
			w.Header().Set("Content-Type", contentType)
			w.Header().Set("Content-Length", strconv.Itoa(len(imgData)))
			w.Header().Set("Cache-Control", "public, max-age=63072000, immutable")
			w.WriteHeader(http.StatusOK)
			w.Write(imgData)
			return
		}

		w.Header().Set("Content-Type", outType)
		w.Header().Set("Content-Length", strconv.Itoa(len(resized)))
		w.Header().Set("Cache-Control", "public, max-age=63072000, immutable")
		w.WriteHeader(http.StatusOK)
		w.Write(resized)
		return
	}

	// ─── Step 6: Proxy response headers & body ───────────────────────────
	forwardHeaders := []string{
		"Content-Type", "Content-Length", "Content-Range",
		"Accept-Ranges", "Content-Disposition", "ETag", "Last-Modified",
	}
	for _, header := range forwardHeaders {
		if v := resp.Header.Get(header); v != "" {
			w.Header().Set(header, v)
		}
	}

	w.Header().Set("Cache-Control", "public, max-age=63072000, immutable")

	if w.Header().Get("Content-Type") == "" && media.MimeType != nil {
		w.Header().Set("Content-Type", *media.MimeType)
	}
	if w.Header().Get("Accept-Ranges") == "" {
		w.Header().Set("Accept-Ranges", "bytes")
	}

	w.WriteHeader(resp.StatusCode)

	buf := make([]byte, 32*1024)
	io.CopyBuffer(w, resp.Body, buf)
}

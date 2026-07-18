package handlers

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"

	"content-node/internal/core/enums"
	"content-node/internal/db/models"
	"content-node/internal/services"

	"go.mongodb.org/mongo-driver/bson"
)

// ProcessingData is passed to processing.html when video is not ready.
type ProcessingData struct {
	State   string
	Message string
	Percent float64
}

// EmbedContent holds resolved media URLs for playlist feed.
type EmbedContent struct {
	PosterURL    string
	PlaylistM3U8 string
	SpriteVttURL string
}

// EmbedResolveResult is the shared embed / playlist feed resolution output.
type EmbedResolveResult struct {
	File        models.File
	Slug        string
	Content     EmbedContent
	EmbedConfig services.EmbedPlayerConfig
}

// EmbedResolveError describes a failed embed resolution.
type EmbedResolveError struct {
	Status     int
	Message    string
	Processing *ProcessingData
}

func requestHost(r *http.Request) string {
	if h := r.Header.Get("X-Forwarded-Host"); h != "" {
		return strings.TrimSpace(strings.Split(h, ",")[0])
	}
	return r.Host
}

func isLocalRequest(r *http.Request) bool {
	host := strings.ToLower(requestHost(r))
	if i := strings.Index(host, ":"); i >= 0 {
		host = host[:i]
	}
	return host == "localhost" || host == "127.0.0.1" || host == "0.0.0.0"
}

func cfVisitorScheme(r *http.Request) string {
	raw := r.Header.Get("CF-Visitor")
	if raw == "" {
		return ""
	}
	var v struct {
		Scheme string `json:"scheme"`
	}
	if err := json.Unmarshal([]byte(raw), &v); err != nil {
		raw = strings.ToLower(raw)
		if strings.Contains(raw, `"scheme":"https"`) || strings.Contains(raw, `"scheme": "https"`) {
			return "https"
		}
		if strings.Contains(raw, `"scheme":"http"`) || strings.Contains(raw, `"scheme": "http"`) {
			return "http"
		}
		return ""
	}
	return strings.ToLower(strings.TrimSpace(v.Scheme))
}

func forwardedProto(r *http.Request) string {
	p := r.Header.Get("X-Forwarded-Proto")
	if p == "" {
		return ""
	}
	return strings.ToLower(strings.TrimSpace(strings.Split(p, ",")[0]))
}

func requestProtocol(r *http.Request) string {
	if isLocalRequest(r) {
		return "http"
	}

	if scheme := cfVisitorScheme(r); scheme == "https" || scheme == "http" {
		return scheme
	}

	if p := forwardedProto(r); p == "https" || p == "http" {
		return p
	}

	// Cloudflare edge — visitor HTTPS even when origin connection is plain HTTP
	if r.Header.Get("CF-Ray") != "" {
		return "https"
	}

	if r.Header.Get("X-Forwarded-Ssl") == "on" ||
		r.Header.Get("X-Forwarded-Scheme") == "https" ||
		r.Header.Get("X-Url-Scheme") == "https" {
		return "https"
	}

	if r.TLS != nil {
		return "https"
	}

	return "http"
}

func (h *Handler) resolveEmbed(r *http.Request, slug string) (*EmbedResolveResult, *EmbedResolveError) {
	ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
	defer cancel()

	var file models.File
	err := models.FileModel.Col().FindOne(ctx, bson.M{"slug": slug}).Decode(&file)
	if err != nil {
		return nil, &EmbedResolveError{Status: http.StatusNotFound, Message: "File not found"}
	}

	if file.IsTrashed() || file.IsDeleted() {
		return nil, &EmbedResolveError{Status: http.StatusNotFound, Message: "File not found"}
	}

	// content-node ไม่ตรวจ/ไม่ปฏิเสธโดเมน — feed ถูกดึงผ่านโดเมน static/CDN
	// การตรวจ domain/space เป็นหน้าที่ของ player-node (หน้า embed)
	// lookup ไว้เฉยๆ เผื่อ host เป็น custom domain จะได้ player config ของโดเมนนั้น
	domain, _ := services.FindDomain(r.Host)

	if file.SpaceID != nil && *file.SpaceID != "" {
		space := services.FindSpace(*file.SpaceID)
		if space != nil && space.Status == "error" {
			return nil, &EmbedResolveError{Status: http.StatusNotFound, Message: "This content is currently unavailable"}
		}
	}

	cursor, err := models.MediaModel.Col().Find(ctx, bson.M{
		"fileId":     file.ID,
		"type":       enums.MediaTypeVideo,
		"resolution": bson.M{"$in": []string{"original", "1080", "720", "480", "360"}},
		"deletedAt":  bson.M{"$eq": nil},
	})
	if err != nil {
		return nil, &EmbedResolveError{Status: http.StatusInternalServerError, Message: "Error loading video"}
	}
	defer cursor.Close(ctx)

	medias := make(map[string]string)
	hasTranscoded := false
	for cursor.Next(ctx) {
		var media models.Media
		if err := cursor.Decode(&media); err != nil {
			continue
		}
		res := ""
		if media.Resolution != nil {
			res = *media.Resolution
		}
		if res != "" {
			medias[res] = media.Slug
			if res == "1080" || res == "720" || res == "480" || res == "360" {
				hasTranscoded = true
			}
		}
	}

	if hasTranscoded {
		delete(medias, "original")
	}

	if len(medias) == 0 {
		var vp models.VideoProcess
		vpErr := models.VideoProcessModel.Col().FindOne(ctx, bson.M{"fileId": file.ID}).Decode(&vp)

		pd := &ProcessingData{State: "queue"}
		if vpErr == nil {
			status := ""
			if vp.Status != nil {
				status = *vp.Status
			}
			if status == "failed" {
				errMsg := "เกิดข้อผิดพลาดในการประมวลผล"
				if vp.Error != nil && *vp.Error != "" {
					errMsg = *vp.Error
				}
				pd = &ProcessingData{State: "error", Message: errMsg}
			} else {
				pct := 0.0
				if vp.OverallPercent != nil {
					pct = *vp.OverallPercent
				}
				pd = &ProcessingData{State: "processing", Percent: pct}
			}
		}

		return nil, &EmbedResolveError{
			Status:     http.StatusNotFound,
			Message:    "Video not ready",
			Processing: pd,
		}
	}

	reqProto := requestProtocol(r)
	playlistHost := services.GetDomainPlaylist(r.Host)
	previewHost := services.GetDomainPreview()
	staticHost := services.GetDomainStatic()

	posterURL := ""
	var posterMedia models.Media
	err = models.MediaModel.Col().FindOne(ctx, bson.M{
		"fileId":     file.ID,
		"type":       enums.MediaTypeImage,
		"resolution": enums.ResolutionPoster,
		"deletedAt":  bson.M{"$eq": nil},
	}).Decode(&posterMedia)
	if err == nil && posterMedia.StorageID != nil && *posterMedia.StorageID != "" {
		var storage models.Storage
		if sErr := models.StorageModel.Col().FindOne(ctx, bson.M{"_id": *posterMedia.StorageID}).Decode(&storage); sErr == nil {
			if storage.PublicURL != nil && *storage.PublicURL != "" {
				posterURL = strings.TrimRight(*storage.PublicURL, "/") + "/" + posterMedia.Slug + "/poster.jpg"
			}
		}
	}

	playlistM3U8 := reqProto + "://" + playlistHost + "/" + slug + "/playlist.m3u8"

	if posterURL == "" {
		thumbTime := 0
		if file.Metadata != nil && file.Metadata.Duration != nil {
			thumbTime = int(*file.Metadata.Duration / 2)
		}
		if staticHost != "" {
			posterURL = reqProto + "://" + staticHost + "/thumb/" + slug + "/" + fmt.Sprintf("%d", thumbTime) + ".jpg"
		} else if previewHost != "" {
			posterURL = reqProto + "://" + previewHost + "/thumb/" + slug + "/" + fmt.Sprintf("%d", thumbTime) + ".jpg"
		} else {
			posterURL = "/thumb/" + slug + "/" + fmt.Sprintf("%d", thumbTime) + ".jpg"
		}
	}

	spriteVttURL := ""
	var spriteMedia models.Media
	if err := models.MediaModel.Col().FindOne(ctx, bson.M{
		"fileId":    file.ID,
		"type":      enums.MediaTypeThumbnail,
		"fileName":  "sprite.vtt",
		"deletedAt": bson.M{"$eq": nil},
	}).Decode(&spriteMedia); err == nil {
		if staticHost != "" {
			spriteVttURL = reqProto + "://" + staticHost + "/" + slug + "/sprite/sprite.vtt"
		} else if previewHost != "" {
			spriteVttURL = reqProto + "://" + previewHost + "/" + slug + "/sprite/sprite.vtt"
		} else {
			spriteVttURL = "/" + slug + "/sprite/sprite.vtt"
		}
	}

	planType := "hobby"
	if file.SpaceID != nil && *file.SpaceID != "" {
		if plan := services.GetSpacePlan(*file.SpaceID); plan != nil {
			planType = plan.PlanType
		}
	}

	baseColor := "#ff6700"
	autostart := false
	mute := false
	continuePlay := true
	continuePlayArk := false
	if domain != nil && domain.Player != nil {
		if domain.Player.BaseColor != "" {
			baseColor = domain.Player.BaseColor
		}
		autostart = domain.Player.AutoPlay
		mute = domain.Player.MuteSound
		continuePlay = domain.Player.ContinuePlay
		continuePlayArk = domain.Player.ContinuePlayArk
	} else {
		globalSettings := services.GetPlayerSettings()
		baseColor = globalSettings.BaseColor
		autostart = globalSettings.AutoPlay
		mute = globalSettings.MuteSound
		continuePlay = globalSettings.ContinuePlay
		continuePlayArk = globalSettings.ContinuePlayArk
	}

	adSlug := services.ResolveAdSlug(planType, domain, file.SpaceID)

	advertHost := strings.Split(requestHost(r), ":")[0]
	if staticHost != "" {
		advertHost = staticHost
	}

	embedConfig := services.EmbedPlayerConfig{
		Lang:      "auto",
		Adverts:   adSlug,
		BaseColor: baseColor,
		Autostart: autostart,
		Mute:      mute,
		ContinuePlayBack: services.EmbedContinuePlayback{
			Enable:     continuePlay,
			Ark:        continuePlayArk,
			AutoResume: false,
			Countdown:  20,
		},
		Slug:        slug,
		AdvertLocal: false,
		Static:      advertHost,
	}

	return &EmbedResolveResult{
		File:        file,
		Slug:        slug,
		EmbedConfig: embedConfig,
		Content: EmbedContent{
			PosterURL:    posterURL,
			PlaylistM3U8: playlistM3U8,
			SpriteVttURL: spriteVttURL,
		},
	}, nil
}

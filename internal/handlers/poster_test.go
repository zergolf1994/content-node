package handlers

import (
	"testing"

	"content-node/internal/db/models"
)

func TestSelectPosterMediaUsesLowestNumericResolution(t *testing.T) {
	resolutionOriginal := "original"
	resolution1080 := "1080"
	resolution360 := "360"
	medias := []models.Media{
		{Slug: "original", Resolution: &resolutionOriginal},
		{Slug: "1080", Resolution: &resolution1080},
		{Slug: "360", Resolution: &resolution360},
	}

	media, ok := selectPosterMedia(medias)
	if !ok || media.Slug != "360" {
		t.Fatalf("expected lowest resolution 360, got %#v (ok=%v)", media, ok)
	}
}

func TestSelectPosterMediaFallsBackToOriginal(t *testing.T) {
	resolutionOriginal := "original"
	media, ok := selectPosterMedia([]models.Media{{
		Slug:       "original",
		Resolution: &resolutionOriginal,
	}})
	if !ok || media.Slug != "original" {
		t.Fatalf("expected original fallback, got %#v (ok=%v)", media, ok)
	}
}

func TestResolvePosterSecondUsesMediaDurationAndClamps(t *testing.T) {
	fileDuration := 100.0
	file := models.File{Metadata: &models.FileMetadata{Duration: &fileDuration}}
	media := models.Media{Metadata: &models.MediaMetadata{Duration: 39.2}}

	if got := resolvePosterSecond("poster", true, file, media); got != 19 {
		t.Fatalf("expected media midpoint 19, got %d", got)
	}
	if got := resolvePosterSecond("50", false, file, media); got != 39 {
		t.Fatalf("expected clamped second 39, got %d", got)
	}
}

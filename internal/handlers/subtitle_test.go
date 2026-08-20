package handlers

import (
	"testing"

	"content-node/internal/core/enums"
	"content-node/internal/db/models"
)

func TestSubtitleSourceURLForS3UsesOriginAndMediaPath(t *testing.T) {
	fileName := "subtitle_1.vtt"
	objectPath := "file id/subtitles/Thai track.vtt"
	origin := "origin.example.com/base"
	media := models.Media{FileName: &fileName, Path: &objectPath}
	storage := models.Storage{Type: enums.StorageTypeS3, OriginURL: &origin}

	got, err := subtitleSourceURL(media, storage, models.File{})
	if err != nil {
		t.Fatalf("subtitleSourceURL: %v", err)
	}
	if want := "https://origin.example.com/base/file%20id/subtitles/Thai%20track.vtt"; got != want {
		t.Fatalf("got %q, want %q", got, want)
	}
}

func TestSubtitleSourceURLForLocalUsesFileSlug(t *testing.T) {
	fileName := "subtitle_1.vtt"
	media := models.Media{FileName: &fileName}
	storage := models.Storage{Type: enums.StorageTypeLocal, Local: &models.StorageLocalConfig{Host: "10.0.0.2", Port: 8888}}
	file := models.File{Slug: "video-slug"}

	got, err := subtitleSourceURL(media, storage, file)
	if err != nil {
		t.Fatalf("subtitleSourceURL: %v", err)
	}
	if want := "http://10.0.0.2:8888/video-slug/subtitle_1.vtt"; got != want {
		t.Fatalf("got %q, want %q", got, want)
	}
}

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

func TestSubtitleSourceURLForS3FallsBackToEndpointAndBucket(t *testing.T) {
	fileName := "subtitle_1.vtt"
	objectPath := "file-id/subtitle_1.vtt"
	endpoint := "https://s3.eu-central-003.backblazeb2.com"
	media := models.Media{FileName: &fileName, Path: &objectPath}
	storage := models.Storage{
		Type: enums.StorageTypeS3,
		S3: &models.StorageS3Config{
			Endpoint: &endpoint,
			Bucket:   "vdohide",
		},
	}

	got, err := subtitleSourceURL(media, storage, models.File{})
	if err != nil {
		t.Fatalf("subtitleSourceURL: %v", err)
	}
	if want := "https://s3.eu-central-003.backblazeb2.com/vdohide/file-id/subtitle_1.vtt"; got != want {
		t.Fatalf("got %q, want %q", got, want)
	}
}

func TestSubtitleSourceURLForS3PrefersPublicURLBeforeEndpoint(t *testing.T) {
	fileName := "subtitle_1.vtt"
	objectPath := "file-id/subtitle_1.vtt"
	publicURL := "https://f003.backblazeb2.com/file/vdohide"
	endpoint := "https://s3.eu-central-003.backblazeb2.com"
	media := models.Media{FileName: &fileName, Path: &objectPath}
	storage := models.Storage{
		Type:      enums.StorageTypeS3,
		PublicURL: &publicURL,
		S3: &models.StorageS3Config{
			Endpoint: &endpoint,
			Bucket:   "vdohide",
		},
	}

	got, err := subtitleSourceURL(media, storage, models.File{})
	if err != nil {
		t.Fatalf("subtitleSourceURL: %v", err)
	}
	if want := "https://f003.backblazeb2.com/file/vdohide/file-id/subtitle_1.vtt"; got != want {
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

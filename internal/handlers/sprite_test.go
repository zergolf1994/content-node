package handlers

import (
	"testing"

	"content-node/internal/db/models"
)

func TestSpriteSourceURLForS3UsesOriginAndEffectiveFileID(t *testing.T) {
	origin := "origin.example.com/base"
	fileID := "clone-file"
	clonedFrom := "source-file"
	storage := &models.Storage{Type: "s3", OriginURL: &origin}
	media := &models.Media{FileID: &fileID, ClonedFrom: &clonedFrom}

	got, err := spriteSourceURL(storage, media, "public-slug", "sprite-1.jpg")
	if err != nil {
		t.Fatalf("spriteSourceURL() error = %v", err)
	}
	want := "https://origin.example.com/base/source-file/sprite/sprite-1.jpg"
	if got != want {
		t.Fatalf("spriteSourceURL() = %q, want %q", got, want)
	}
}

func TestSpriteSourceURLForLocalUsesFileSlug(t *testing.T) {
	storage := &models.Storage{Type: "local", Local: &models.StorageLocalConfig{Host: "10.0.0.8", Port: 8888}}
	media := &models.Media{}

	got, err := spriteSourceURL(storage, media, "public-slug", "sprite.vtt")
	if err != nil {
		t.Fatalf("spriteSourceURL() error = %v", err)
	}
	want := "http://10.0.0.8:8888/public-slug/sprite/sprite.vtt"
	if got != want {
		t.Fatalf("spriteSourceURL() = %q, want %q", got, want)
	}
}

package models

import "testing"

func TestS3StorageURLs(t *testing.T) {
	publicURL := "cdn-a.example.com, https://cdn-b.example.com/base/"
	originURL := "origin.example.com/raw/"
	storage := &Storage{Type: "s3", PublicURL: &publicURL, OriginURL: &originURL}

	if got, want := storage.GetPlaybackBaseURL(), "https://cdn-a.example.com"; got != want {
		t.Fatalf("GetPlaybackBaseURL() = %q, want %q", got, want)
	}
	domains := storage.GetPublicDomains()
	if len(domains) != 2 || domains[0] != "cdn-a.example.com" || domains[1] != "cdn-b.example.com" {
		t.Fatalf("GetPublicDomains() = %#v", domains)
	}
	if got, want := storage.GetVODBaseURL(), "https://cdn-a.example.com"; got != want {
		t.Fatalf("GetVODBaseURL() = %q, want %q", got, want)
	}
	if got, want := storage.GetOriginBaseURL(), "https://origin.example.com/raw"; got != want {
		t.Fatalf("GetOriginBaseURL() = %q, want %q", got, want)
	}
}

func TestLocalStorageURLs(t *testing.T) {
	storage := &Storage{
		Type: "local",
		Local: &StorageLocalConfig{
			Host: "10.0.0.8",
			Port: 8888,
		},
	}

	if got, want := storage.GetPlaybackBaseURL(), "http://10.0.0.8"; got != want {
		t.Fatalf("GetPlaybackBaseURL() = %q, want %q", got, want)
	}
	if got, want := storage.GetStorageBaseURL(), "http://10.0.0.8:8888"; got != want {
		t.Fatalf("GetStorageBaseURL() = %q, want %q", got, want)
	}
	if got, want := storage.GetVODBaseURL(), "http://10.0.0.8:8889"; got != want {
		t.Fatalf("GetVODBaseURL() = %q, want %q", got, want)
	}
}

func TestPublicURLRejectsInvalidValue(t *testing.T) {
	publicURL := "://invalid"
	storage := &Storage{Type: "s3", PublicURL: &publicURL}
	if got := storage.GetPublicBaseURL(); got != "" {
		t.Fatalf("GetPublicBaseURL() = %q, want empty", got)
	}
}

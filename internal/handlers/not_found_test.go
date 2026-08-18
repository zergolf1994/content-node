package handlers

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestHandleNotFoundUsesCloudflareNegativeCache(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/missing/playlist.m3u8", nil)
	rec := httptest.NewRecorder()

	HandleNotFound(rec, req)

	if rec.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusNotFound)
	}
	if got := rec.Header().Get("Cache-Control"); got != "no-store" {
		t.Fatalf("Cache-Control = %q, want no-store", got)
	}
	if got := rec.Header().Get("CDN-Cache-Control"); got != "public, max-age=60" {
		t.Fatalf("CDN-Cache-Control = %q", got)
	}
	if !strings.Contains(rec.Body.String(), "<Code>NoSuchKey</Code>") {
		t.Fatalf("body does not contain NoSuchKey: %s", rec.Body.String())
	}
}

func TestHandleCachedErrorPreservesUpstreamStatus(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/media/video.m3u8", nil)
	rec := httptest.NewRecorder()

	HandleCachedError(rec, req, http.StatusBadGateway)

	if rec.Code != http.StatusBadGateway {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusBadGateway)
	}
	if got := rec.Header().Get("Cache-Control"); got != "no-store" {
		t.Fatalf("Cache-Control = %q, want no-store", got)
	}
	if got := rec.Header().Get("CDN-Cache-Control"); got != "public, max-age=60" {
		t.Fatalf("CDN-Cache-Control = %q", got)
	}
	if !strings.Contains(rec.Body.String(), "<Code>InternalError</Code>") {
		t.Fatalf("body does not contain InternalError: %s", rec.Body.String())
	}
}

func TestSendNotFoundPreservesNonImageStatus(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/media/video.m3u8", nil)
	rec := httptest.NewRecorder()

	sendNotFound(rec, req, http.StatusServiceUnavailable)

	if rec.Code != http.StatusServiceUnavailable {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusServiceUnavailable)
	}
}

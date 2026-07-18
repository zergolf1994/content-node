package handlers

import (
	"fmt"
	"html"
	"math/rand"
	"net/http"
	"strings"

	"content-node/internal/core/enums"
	"content-node/internal/db/models"
	"content-node/internal/services"
)

// Vast handles GET /vast/{domainSlug}.xml — generates VAST 3.0 XML from ads
func (h *Handler) Vast(w http.ResponseWriter, r *http.Request) {
	// ── Extract slug from path: /vast/{slug}.xml ──
	path := strings.TrimPrefix(r.URL.Path, "/vast/")
	slug := strings.TrimSuffix(path, ".xml")

	if slug == "" {
		writeEmptyVast(w)
		return
	}

	// ── Special case: hobby plan ads ──
	if slug == "hobby" {
		hobby := services.GetAdvertHobbyIDs()
		if len(hobby.Vdo) == 0 {
			writeEmptyVast(w)
			return
		}
		// Build VAST from hobby vdo items
		buildHobbyVast(w, hobby.Vdo)
		return
	}

	// ── Find domain by slug ──
	domain := services.FindDomainBySlug(slug)
	if domain == nil || !domain.Enable || domain.Status != enums.DomainStatusActive {
		writeEmptyVast(w)
		return
	}

	// ── Check if domain has video ads configured ──
	if domain.Ads != nil && len(domain.Ads.Video) == 0 {
		writeEmptyVast(w)
		return
	}

	// ── Resolve ads from domain's space ──
	if domain.SpaceID != nil && *domain.SpaceID != "" {
		ads := services.FindAdsBySpaceID(*domain.SpaceID)
		if len(ads) > 0 {
			buildAdsVast(w, ads)
			return
		}
	}

	// Domain has ads config but no matching ads found → empty
	writeEmptyVast(w)
}

// buildHobbyVast builds VAST XML from hobby Ad IDs.
func buildHobbyVast(w http.ResponseWriter, adIDs []string) {
	var ads strings.Builder
	hasActive := false

	for _, adID := range adIDs {
		ad := services.FindAdByID(adID)
		if ad == nil || ad.Content == nil || ad.Content.Mp4URL == nil || *ad.Content.Mp4URL == "" {
			continue
		}
		hasActive = true

		vastID := randomID(10)
		skipSeconds := 5
		if ad.Content.SkipSeconds != nil {
			skipSeconds = *ad.Content.SkipSeconds
		}
		skipOffset := fmt.Sprintf("00:00:%02d", skipSeconds)

		name := ad.Name
		websiteUrl := ""
		if ad.Content.WebsiteURL != nil {
			websiteUrl = *ad.Content.WebsiteURL
		}

		ads.WriteString(fmt.Sprintf(`
    <Ad id="%s" sequence="0">
      <InLine>
        <AdSystem version="2.0">JW Player</AdSystem>
        <AdTitle>%s</AdTitle>
        <Creatives>
          <Creative sequence="0">
            <Linear skipoffset="%s">
              <VideoClicks>
                <ClickThrough>%s</ClickThrough>
              </VideoClicks>
              <MediaFiles>
                <MediaFile
                  id="%s"
                  delivery="progressive"
                  type="video/mp4"
                  bitrate="400"
                  width="640"
                  height="360"
                >%s</MediaFile>
              </MediaFiles>
            </Linear>
          </Creative>
          <Creative> </Creative>
        </Creatives>
      </InLine>
    </Ad>`, vastID, html.EscapeString(name), skipOffset, html.EscapeString(websiteUrl), vastID, html.EscapeString(*ad.Content.Mp4URL)))
	}

	if !hasActive {
		writeEmptyVast(w)
		return
	}

	vast := fmt.Sprintf(`<?xml version="1.0"?>
<VAST
  version="3.0"
  xmlns:xsi="http://www.w3.org/2001/XMLSchema-instance"
  xsi:noNamespaceSchemaLocation="vast3_draft.xsd"
>%s
</VAST>`, ads.String())

	w.Header().Set("Content-Type", "application/xml; charset=utf-8")
	w.Header().Set("Cache-Control", "public, max-age=60")
	w.Write([]byte(vast))
}

// writeEmptyVast writes an empty VAST response
func writeEmptyVast(w http.ResponseWriter) {
	w.Header().Set("Content-Type", "application/xml; charset=utf-8")
	w.Write([]byte(`<?xml version="1.0"?>
<VAST version="3.0" xmlns:xsi="http://www.w3.org/2001/XMLSchema-instance" xsi:noNamespaceSchemaLocation="vast3_draft.xsd">
</VAST>`))
}

// randomID generates a random alphanumeric string of given length
func randomID(n int) string {
	const letters = "abcdefghijklmnopqrstuvwxyz0123456789"
	b := make([]byte, n)
	for i := range b {
		b[i] = letters[rand.Intn(len(letters))]
	}
	return string(b)
}

// buildAdsVast builds VAST XML from Ads model entries.
func buildAdsVast(w http.ResponseWriter, adList []models.Ads) {
	var ads strings.Builder
	hasActive := false

	for _, ad := range adList {
		if ad.Type != enums.AdsTypeVideo || ad.Content == nil {
			continue
		}
		if ad.Content.Mp4URL == nil || *ad.Content.Mp4URL == "" {
			continue
		}
		hasActive = true

		adID := randomID(10)
		skipSeconds := 0
		if ad.Content.SkipSeconds != nil {
			skipSeconds = *ad.Content.SkipSeconds
		}
		skipOffset := fmt.Sprintf("00:00:%02d", skipSeconds)

		websiteURL := ""
		if ad.Content.WebsiteURL != nil {
			websiteURL = *ad.Content.WebsiteURL
		}

		ads.WriteString(fmt.Sprintf(`
    <Ad id="%s" sequence="0">
      <InLine>
        <AdSystem version="2.0">JW Player</AdSystem>
        <AdTitle>%s</AdTitle>
        <Creatives>
          <Creative sequence="0">
            <Linear skipoffset="%s">
              <VideoClicks>
                <ClickThrough>%s</ClickThrough>
              </VideoClicks>
              <MediaFiles>
                <MediaFile
                  id="%s"
                  delivery="progressive"
                  type="video/mp4"
                  bitrate="400"
                  width="640"
                  height="360"
                >%s</MediaFile>
              </MediaFiles>
            </Linear>
          </Creative>
          <Creative> </Creative>
        </Creatives>
      </InLine>
    </Ad>`, adID, html.EscapeString(ad.Name), skipOffset, html.EscapeString(websiteURL), adID, html.EscapeString(*ad.Content.Mp4URL)))
	}

	if !hasActive {
		writeEmptyVast(w)
		return
	}

	vast := fmt.Sprintf(`<?xml version="1.0"?>
<VAST
  version="3.0"
  xmlns:xsi="http://www.w3.org/2001/XMLSchema-instance"
  xsi:noNamespaceSchemaLocation="vast3_draft.xsd"
>%s
</VAST>`, ads.String())

	w.Header().Set("Content-Type", "application/xml; charset=utf-8")
	w.Header().Set("Cache-Control", "public, max-age=60")
	w.Write([]byte(vast))
}

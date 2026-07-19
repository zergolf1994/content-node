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

// Vast handles GET /vast/{slug}.xml — VAST 3.0 จาก adverts ที่ฝังใน custom_domain
// (slug พิเศษ "hobby" = setting advert_hobby)
func (h *Handler) Vast(w http.ResponseWriter, r *http.Request) {
	// ── Extract slug from path: /vast/{slug}.xml ──
	path := strings.TrimPrefix(r.URL.Path, "/vast/")
	slug := strings.TrimSuffix(path, ".xml")

	if slug == "" {
		writeEmptyVast(w)
		return
	}

	var adverts *models.DomainAdverts

	if slug == "hobby" {
		adverts = services.GetAdvertHobby()
	} else {
		domain := services.FindDomainBySlug(slug)
		if domain == nil || !domain.Enable || domain.Status != enums.DomainStatusActive {
			writeEmptyVast(w)
			return
		}
		adverts = domain.Adverts
	}

	buildVideoVast(w, adverts)
}

// buildVideoVast builds VAST XML from embedded video adverts.
func buildVideoVast(w http.ResponseWriter, adverts *models.DomainAdverts) {
	if adverts == nil || !adverts.Video.Enabled || len(adverts.Video.List) == 0 {
		writeEmptyVast(w)
		return
	}
	buildAdvertsVast(w, adverts.Video.List)
}

// buildAdvertsVast builds VAST XML from embedded advert items.
func buildAdvertsVast(w http.ResponseWriter, items []models.AdContent) {
	var ads strings.Builder
	hasActive := false

	for _, item := range items {
		if !item.Enabled || item.MP4URL == nil || *item.MP4URL == "" {
			continue
		}
		hasActive = true

		adID := item.ID
		if adID == "" {
			adID = randomID(10)
		}

		skipSeconds := 0
		if item.SkipSeconds != nil {
			skipSeconds = *item.SkipSeconds
		}
		skipOffset := fmt.Sprintf("00:00:%02d", skipSeconds)

		websiteURL := ""
		if item.WebsiteURL != nil {
			websiteURL = *item.WebsiteURL
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
    </Ad>`, adID, html.EscapeString(item.Name), skipOffset, html.EscapeString(websiteURL), adID, html.EscapeString(*item.MP4URL)))
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

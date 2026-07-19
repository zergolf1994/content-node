package services

import (
	"encoding/json"
	"log"

	"content-node/internal/core/enums"
	"content-node/internal/db/models"
)

// ─── Advert Hobby (setting advert_hobby) ─────────────────────────────
// admin เก็บเป็น {video, image, script} รูปเดียวกับ custom_domains.adverts

// GetAdvertHobby reads advert_hobby from setting.json as embedded advert
// objects (used by /vast/hobby.xml and /advert/hobby.json).
func GetAdvertHobby() *models.DomainAdverts {
	settings, err := ReadSettingFile()
	if err != nil {
		return nil
	}
	raw, exists := settings[enums.SettingAdvertHobby]
	if !exists {
		return nil
	}
	var result models.DomainAdverts
	if err := json.Unmarshal(raw, &result); err != nil {
		log.Printf("⚠️ Cannot parse advert_hobby: %v", err)
		return nil
	}
	return &result
}

// ─── Ad Slug / Feed Resolution ────────────────────────────────────────

// ResolveAdSlug returns the ad feed slug: "hobby" or custom domain slug.
func ResolveAdSlug(planType string, domain *models.CustomDomain, spaceID *string) string {
	if planType != "" && planType != enums.PlanTypeHobby {
		if domain != nil && domain.Slug != "" {
			return domain.Slug
		}
		if spaceID != nil && *spaceID != "" {
			if slug := FindDomainSlugBySpaceID(*spaceID); slug != "" {
				return slug
			}
		}
		return ""
	}
	return "hobby"
}

// ResolveAdvertsBySlug loads advert config for hobby or a domain slug.
// adverts ฝังอยู่ใน custom_domains.adverts — ใช้เฉพาะโดเมนที่ enable + active
func ResolveAdvertsBySlug(adSlug string) *models.DomainAdverts {
	if adSlug == "" || adSlug == "hobby" {
		return GetAdvertHobby()
	}
	domain := FindDomainBySlug(adSlug)
	if domain == nil || !domain.Enable || domain.Status != enums.DomainStatusActive {
		return nil
	}
	return domain.Adverts
}

// BuildAdvertFeed builds /advert/{adSlug}.json (script + image + video).
func BuildAdvertFeed(adSlug string) *models.DomainAdverts {
	adverts := ResolveAdvertsBySlug(adSlug)
	if adverts != nil {
		return adverts
	}
	empty := models.DomainAdverts{}
	return &empty
}

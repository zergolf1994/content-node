package services

import (
	"encoding/json"
	"log"
	"sync"

	"content-node/internal/core/enums"
	"content-node/internal/db/models"
)

// ─── Advert Hobby (advert_hobby) ─────────────────────────────────────

// AdvertHobby represents the advert_hobby setting value.
// Fields contain Ad document IDs (not full objects).
type AdvertHobby struct {
	Vdo        []string `json:"vdo"`
	Image      []string `json:"image"`
	Javascript []string `json:"javascript"`
}

// GetAdvertHobbyIDs reads advert_hobby from setting.json as Ad ID lists
// (used by /vast/hobby.xml and ResolveAdsFromPlan).
func GetAdvertHobbyIDs() AdvertHobby {
	settings, err := ReadSettingFile()
	if err != nil {
		return AdvertHobby{}
	}
	raw, exists := settings["advert_hobby"]
	if !exists {
		return AdvertHobby{}
	}
	var result AdvertHobby
	if err := json.Unmarshal(raw, &result); err != nil {
		log.Printf("⚠️ Cannot parse advert_hobby: %v", err)
		return AdvertHobby{}
	}
	return result
}

// GetAdvertHobby reads advert_hobby from setting.json as embedded advert
// objects (used by the /advert/{slug}.json feed).
func GetAdvertHobby() *models.DomainAdverts {
	settings, err := ReadSettingFile()
	if err != nil {
		return nil
	}
	raw, exists := settings["advert_hobby"]
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

// ─── Ads Cache ───────────────────────────────────────────────────────

var (
	adCache   map[string][]models.Ads // spaceId → active ads
	adCacheMu sync.RWMutex
)

// LoadAds loads ads into the in-memory cache grouped by spaceId.
func LoadAds(ads []models.Ads) {
	cache := make(map[string][]models.Ads)
	for i := range ads {
		ad := ads[i]
		if ad.Status != enums.AdsStatusActive {
			continue
		}
		cache[ad.SpaceID] = append(cache[ad.SpaceID], ad)
	}

	adCacheMu.Lock()
	adCache = cache
	adCacheMu.Unlock()

	log.Printf("📋 Loaded %d active ads → ads cache", len(ads))
}

// FindAdByID looks up an ad by ID from the in-memory ads cache.
func FindAdByID(adID string) *models.Ads {
	adCacheMu.RLock()
	defer adCacheMu.RUnlock()

	for _, ads := range adCache {
		for i := range ads {
			if ads[i].ID == adID {
				return &ads[i]
			}
		}
	}
	return nil
}

// FindAdsBySpaceID returns active ads for a given spaceId from the cache.
func FindAdsBySpaceID(spaceID string) []models.Ads {
	if spaceID == "" {
		return nil
	}

	adCacheMu.RLock()
	defer adCacheMu.RUnlock()

	return adCache[spaceID]
}

// ─── Resolved Ad Config ───────────────────────────────────────────────

// AdvertImageConfig holds image ad overlay config passed to the player JS.
type AdvertImageConfig struct {
	ImageUrl   string   `json:"imageUrl"`
	WebsiteUrl string   `json:"websiteUrl"`
	ShowOn     []string `json:"showOn"`
}

// ResolvedAds is the final ad configuration passed to the player/vast
type ResolvedAds struct {
	// VAST video ads
	VastEnabled bool

	// Image overlay ads (multiple supported, JS picks randomly)
	AdvertImages []AdvertImageConfig

	// Javascript ad scripts (all injected)
	AdJavascripts []string
}

// ResolveAdsFromAds converts []models.Ads into ResolvedAds for paid plan.
func ResolveAdsFromAds(ads []models.Ads) ResolvedAds {
	result := ResolvedAds{}

	for _, ad := range ads {
		if ad.Content == nil {
			continue
		}
		switch ad.Type {
		case enums.AdsTypeVideo:
			if ad.Content.Mp4URL != nil && *ad.Content.Mp4URL != "" {
				result.VastEnabled = true
			}
		case enums.AdsTypeImage:
			if ad.Content.ImageURL != nil && *ad.Content.ImageURL != "" {
				websiteUrl := ""
				if ad.Content.WebsiteURL != nil {
					websiteUrl = *ad.Content.WebsiteURL
				}
				result.AdvertImages = append(result.AdvertImages, AdvertImageConfig{
					ImageUrl:   *ad.Content.ImageURL,
					WebsiteUrl: websiteUrl,
					ShowOn:     ad.Content.ShowOn,
				})
			}
		case enums.AdsTypeScript, "javascript":
			if ad.Content.Script != nil && *ad.Content.Script != "" {
				result.AdJavascripts = append(result.AdJavascripts, *ad.Content.Script)
			}
		}
	}

	return result
}

// ResolveAdsFromPlan selects ad config based on plan type.
//
//   - planType "hobby" or "" → ads from advert_hobby setting
//   - planType "pro"/"business"/"enterprise" → use ads from ads collection (by spaceId)
func ResolveAdsFromPlan(planType string, spaceID string) ResolvedAds {
	if planType != "" && planType != enums.PlanTypeHobby && spaceID != "" {
		ads := FindAdsBySpaceID(spaceID)
		if len(ads) > 0 {
			return ResolveAdsFromAds(ads)
		}
		// No ads configured for this space → no ads
		return ResolvedAds{}
	}

	// Hobby (or no plan): read Ad IDs from advert_hobby, lookup from ads cache
	hobby := GetAdvertHobbyIDs()
	result := ResolvedAds{}

	// Resolve video ad IDs → enable VAST if any active
	for _, adID := range hobby.Vdo {
		ad := FindAdByID(adID)
		if ad != nil && ad.Content != nil && ad.Content.Mp4URL != nil && *ad.Content.Mp4URL != "" {
			result.VastEnabled = true
			break
		}
	}

	// Resolve image ad IDs
	for _, adID := range hobby.Image {
		ad := FindAdByID(adID)
		if ad != nil && ad.Content != nil && ad.Content.ImageURL != nil && *ad.Content.ImageURL != "" {
			websiteUrl := ""
			if ad.Content.WebsiteURL != nil {
				websiteUrl = *ad.Content.WebsiteURL
			}
			result.AdvertImages = append(result.AdvertImages, AdvertImageConfig{
				ImageUrl:   *ad.Content.ImageURL,
				WebsiteUrl: websiteUrl,
				ShowOn:     ad.Content.ShowOn,
			})
		}
	}

	// Resolve javascript ad IDs
	for _, adID := range hobby.Javascript {
		ad := FindAdByID(adID)
		if ad != nil && ad.Content != nil && ad.Content.Script != nil && *ad.Content.Script != "" {
			result.AdJavascripts = append(result.AdJavascripts, *ad.Content.Script)
		}
	}

	return result
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
func ResolveAdvertsBySlug(adSlug string) *models.DomainAdverts {
	if adSlug == "" || adSlug == "hobby" {
		return GetAdvertHobby()
	}
	domain := FindDomainBySlug(adSlug)
	if domain == nil {
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

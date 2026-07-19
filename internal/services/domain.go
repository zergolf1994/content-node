package services

import (
	"log"
	"path/filepath"
	"strings"
	"sync"

	"content-node/internal/db/models"
)

// ─── File Paths ───────────────────────────────────────────────────────

// settingFilePath returns the absolute path to conf/setting.json
func settingFilePath() string {
	exe, err := executableDir()
	if err != nil {
		log.Printf("⚠️ Cannot get executable path: %v", err)
		return filepath.Join("conf", "setting.json")
	}
	return filepath.Join(exe, "conf", "setting.json")
}

// domainsFilePath returns the path to conf/domains.json
func domainsFilePath() string {
	exe, err := executableDir()
	if err != nil {
		return filepath.Join("conf", "domains.json")
	}
	return filepath.Join(exe, "conf", "domains.json")
}

// spacesFilePath returns the path to conf/spaces.json
func spacesFilePath() string {
	exe, err := executableDir()
	if err != nil {
		return filepath.Join("conf", "spaces.json")
	}
	return filepath.Join(exe, "conf", "spaces.json")
}

// ─── Domain Cache ─────────────────────────────────────────────────────

var (
	domainCache   map[string]*models.CustomDomain // hostname → domain
	domainCacheMu sync.RWMutex
)

// FindDomain looks up a domain by hostname from the in-memory cache.
// Returns (domain, isDomainRequest):
//   - (nil, false) → localhost request, no domain check needed
//   - (nil, true)  → domain request but not registered → 404
//   - (*domain, true) → domain found (caller checks Status/Enable)
func FindDomain(hostname string) (*models.CustomDomain, bool) {
	host := strings.Split(hostname, ":")[0]
	host = strings.ToLower(host)

	if host == "localhost" || host == "127.0.0.1" || host == "0.0.0.0" {
		return nil, false
	}

	domainCacheMu.RLock()
	defer domainCacheMu.RUnlock()

	if domainCache == nil {
		return nil, true
	}

	domain, exists := domainCache[host]
	if !exists {
		return nil, true
	}

	return domain, true
}

// LoadDomains loads domains into the in-memory cache
func LoadDomains(domains []models.CustomDomain) {
	cache := make(map[string]*models.CustomDomain, len(domains))
	for i := range domains {
		name := strings.ToLower(domains[i].Name)
		cache[name] = &domains[i]
	}

	domainCacheMu.Lock()
	domainCache = cache
	domainCacheMu.Unlock()

	// log.Printf("📋 Loaded %d custom domains → conf/domains.json", len(cache))
}

// FindDomainSlugBySpaceID returns the slug of the first enabled active domain in a space.
func FindDomainSlugBySpaceID(spaceID string) string {
	if spaceID == "" {
		return ""
	}

	domainCacheMu.RLock()
	defer domainCacheMu.RUnlock()

	for _, d := range domainCache {
		if d == nil || !d.Enable || d.Status != "active" || d.Slug == "" {
			continue
		}
		if d.SpaceID != nil && *d.SpaceID == spaceID {
			return d.Slug
		}
	}
	return ""
}

// FindDomainBySlug looks up a domain by slug from the in-memory cache.
func FindDomainBySlug(slug string) *models.CustomDomain {
	if slug == "" {
		return nil
	}

	domainCacheMu.RLock()
	defer domainCacheMu.RUnlock()

	for _, d := range domainCache {
		if d.Slug == slug {
			return d
		}
	}
	return nil
}

// ─── Space Cache ──────────────────────────────────────────────────────

var (
	spaceCache   map[string]*models.Workspace // spaceId → Workspace
	spaceCacheMu sync.RWMutex
)

// FindSpace looks up a space (Workspace) by its ID from the in-memory cache.
// Returns nil if not found.
func FindSpace(spaceID string) *models.Workspace {
	if spaceID == "" {
		return nil
	}

	spaceCacheMu.RLock()
	defer spaceCacheMu.RUnlock()

	return spaceCache[spaceID]
}

// LoadSpaces loads Workspaces into the in-memory cache
func LoadSpaces(spaces []models.Workspace) {
	cache := make(map[string]*models.Workspace, len(spaces))
	for i := range spaces {
		cache[spaces[i].ID] = &spaces[i]
	}

	spaceCacheMu.Lock()
	spaceCache = cache
	spaceCacheMu.Unlock()

	// log.Printf("📋 Loaded %d spaces → conf/spaces.json", len(cache))
}

// GetSpacePlan returns the plan for a space, nil if not found or no plan.
func GetSpacePlan(spaceID string) *models.WorkspacePlan {
	space := FindSpace(spaceID)
	if space == nil {
		return nil
	}
	return space.Plan
}

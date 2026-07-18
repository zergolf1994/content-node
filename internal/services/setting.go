package services

import (
	"encoding/json"
	"os"
	"strings"

	"content-node/internal/config"
)

// ReadSettingFile reads and parses conf/setting.json.
// Returns a map of setting name → raw JSON bytes.
func ReadSettingFile() (map[string]json.RawMessage, error) {
	data, err := os.ReadFile(settingFilePath())
	if err != nil {
		return nil, err
	}
	var settings map[string]json.RawMessage
	if err := json.Unmarshal(data, &settings); err != nil {
		return nil, err
	}
	return settings, nil
}

// ─── Database Setting Helpers ─────────────────────────────────────────

// getStringSetting is a helper to read a string setting from conf/setting.json
func getStringSetting(key string) string {
	settings, err := ReadSettingFile()
	if err != nil {
		return ""
	}
	raw, exists := settings[key]
	if !exists {
		return ""
	}
	var val string
	if err := json.Unmarshal(raw, &val); err != nil {
		return ""
	}
	return val
}

// GetDomainContent fetches the domain_content setting. Fallbacks to fallbackHost.
func GetDomainContent(fallbackHost string) string {
	val := getStringSetting("domain_content")
	if val != "" {
		return val
	}
	return fallbackHost
}

// GetDomainPlaylist fetches the domain_playlist setting. Fallbacks to domain_content, then fallbackHost.
func GetDomainPlaylist(fallbackHost string) string {
	val := getStringSetting("domain_playlist")
	if val != "" {
		return val
	}
	return GetDomainContent(fallbackHost)
}

// GetDomainAds fetches the domain_ads setting. Fallbacks to domain_content, then fallbackHost.
// func GetDomainAds(fallbackHost string) string {
// 	val := getStringSetting("domain_ads")
// 	if val != "" {
// 		return val
// 	}
// 	return GetDomainContent(fallbackHost)
// }

// GetDomainPreview fetches the domain_preview setting. Returns empty if not set.
func GetDomainPreview() string {
	return normalizeDomainHost(getStringSetting("domain_preview"))
}

// GetDomainStatic fetches the domain_static setting.
// Synced setting.json takes priority; DOMAIN_STATIC env is a dev fallback only.
func GetDomainStatic() string {
	if val := normalizeDomainHost(getStringSetting("domain_static")); val != "" {
		return val
	}
	return normalizeDomainHost(config.AppConfig.DomainStatic)
}

// normalizeDomainHost strips scheme/trailing slash from a host setting value.
func normalizeDomainHost(host string) string {
	host = strings.TrimSpace(host)
	if host == "" {
		return ""
	}
	if strings.HasPrefix(host, "https://") {
		host = strings.TrimPrefix(host, "https://")
	} else if strings.HasPrefix(host, "http://") {
		host = strings.TrimPrefix(host, "http://")
	}
	return strings.TrimRight(host, "/")
}

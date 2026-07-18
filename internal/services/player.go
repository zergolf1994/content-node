package services

import "encoding/json"

// PlayerSettings holds global default player options used when no domain override exists.
type PlayerSettings struct {
	BaseColor       string `json:"baseColor"`
	AutoPlay        bool   `json:"autoPlay"`
	MuteSound       bool   `json:"muteSound"`
	ContinuePlay    bool   `json:"continuePlay"`
	ContinuePlayArk bool   `json:"continuePlayArk"`
}

// EmbedContinuePlayback matches test_jwplayer continuePlayBack object.
type EmbedContinuePlayback struct {
	Enable     bool `json:"enable"`
	Ark        bool `json:"ark"`
	AutoResume bool `json:"autoResume"`
	Countdown  int  `json:"countdown"`
}

// EmbedPlayerConfig is injected into embed.html for cdn.fembed.co player bundle.
type EmbedPlayerConfig struct {
	Lang             string                `json:"lang"`
	Adverts          string                `json:"adverts"`
	BaseColor        string                `json:"baseColor"`
	Autostart        bool                  `json:"autostart"`
	Mute             bool                  `json:"mute"`
	ContinuePlayBack EmbedContinuePlayback `json:"continuePlayBack"`
	Slug             string                `json:"slug"`
	AdvertLocal      bool                  `json:"advertLocal"`
	Static           string                `json:"static,omitempty"`
}

// GetPlayerSettings returns the hardcoded global default player settings.
func GetPlayerSettings() PlayerSettings {
	return PlayerSettings{
		BaseColor:       "#ff8800",
		AutoPlay:        false,
		MuteSound:       false,
		ContinuePlay:    true,
		ContinuePlayArk: false,
	}
}

// IsMaintenanceMode reads player_maintenance from setting.json.
func IsMaintenanceMode() bool {
	settings, err := ReadSettingFile()
	if err != nil {
		return false
	}
	raw, exists := settings["player_maintenance"]
	if !exists {
		return false
	}
	var val bool
	if err := json.Unmarshal(raw, &val); err != nil {
		return false
	}
	return val
}

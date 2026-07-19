package services

import (
	"context"
	"encoding/json"
	"log"
	"os"
	"path/filepath"
	"time"

	"content-node/internal/db/models"

	"go.mongodb.org/mongo-driver/bson"
)

// executableDir returns the directory of the current executable.
func executableDir() (string, error) {
	exe, err := os.Executable()
	if err != nil {
		return "", err
	}
	return filepath.Dir(exe), nil
}

// ─── Atomic File Write Helper ────────────────────────────────────────

// writeJSONFile writes data to a conf/ file atomically (tmp → rename).
func writeJSONFile(filePath string, v interface{}) error {
	data, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(filePath), 0755); err != nil {
		return err
	}
	tmpPath := filePath + ".tmp"
	if err := os.WriteFile(tmpPath, data, 0644); err != nil {
		return err
	}
	if err := os.Rename(tmpPath, filePath); err != nil {
		return os.WriteFile(filePath, data, 0644)
	}
	return nil
}

// ─── Settings Sync ───────────────────────────────────────────────────

// SyncSettings fetches "player" and "advert" settings from MongoDB
// and writes them to conf/setting.json
func SyncSettings() error {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	settingNames := []string{"player_maintenance", "advert_hobby", "domain_content", "domain_playlist", "domain_preview", "domain_static"}
	cursor, err := models.SettingModel.Col().Find(ctx, bson.M{
		"name": bson.M{"$in": settingNames},
	})
	if err != nil {
		return err
	}
	defer cursor.Close(ctx)

	result := make(map[string]interface{})
	for cursor.Next(ctx) {
		var raw bson.M
		if err := cursor.Decode(&raw); err != nil {
			log.Printf("⚠️ Failed to decode setting: %v", err)
			continue
		}
		name, _ := raw["name"].(string)
		value := raw["value"]
		if name != "" {
			result[name] = value
		}
	}
	if err := cursor.Err(); err != nil {
		return err
	}

	return writeJSONFile(settingFilePath(), result)
}

// ─── Domains Sync ────────────────────────────────────────────────────

// SyncDomains fetches all custom domains from MongoDB,
// writes them to conf/domains.json, and loads them into memory cache.
func SyncDomains() error {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	cursor, err := models.CustomDomainModel.Col().Find(ctx, bson.M{})
	if err != nil {
		return err
	}
	defer cursor.Close(ctx)

	var domains []models.CustomDomain
	if err := cursor.All(ctx, &domains); err != nil {
		return err
	}

	// Write to conf/domains.json
	if err := writeJSONFile(domainsFilePath(), domains); err != nil {
		log.Printf("⚠️ Failed to write domains.json: %v", err)
	}

	// Load into memory
	LoadDomains(domains)

	return nil
}

// ─── Spaces Sync ─────────────────────────────────────────────────────

// SyncSpaces fetches all workspaces from MongoDB,
// writes them to conf/spaces.json, and loads them into memory cache.
func SyncSpaces() error {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	cursor, err := models.WorkspaceModel.Col().Find(ctx, bson.M{})
	if err != nil {
		return err
	}
	defer cursor.Close(ctx)

	var spaces []models.Workspace
	if err := cursor.All(ctx, &spaces); err != nil {
		return err
	}

	// Write to conf/spaces.json
	if err := writeJSONFile(spacesFilePath(), spaces); err != nil {
		log.Printf("⚠️ Failed to write spaces.json: %v", err)
	}

	// Load into memory
	LoadSpaces(spaces)

	return nil
}

// ─── Scheduler ───────────────────────────────────────────────────────

// StartSettingSyncScheduler starts a background goroutine that syncs settings,
// domains, and spaces immediately and then every 1 minute.
func StartSettingSyncScheduler(ctx context.Context) {
	log.Println("📋 Syncing settings, domains, spaces from database...")

	syncAll := func() {
		if err := SyncSettings(); err != nil {
			log.Printf("⚠️ Failed to sync settings: %v", err)
		}
		if err := SyncDomains(); err != nil {
			log.Printf("⚠️ Failed to sync domains: %v", err)
		}
		if err := SyncSpaces(); err != nil {
			log.Printf("⚠️ Failed to sync spaces: %v", err)
		}
	}

	syncAll()

	for {
		// Calculate time until the next exact minute (00 second)
		now := time.Now()
		next := now.Truncate(time.Minute).Add(time.Minute)
		duration := time.Until(next)

		select {
		case <-ctx.Done():
			log.Println("⏹️ Settings sync scheduler stopped")
			return
		case <-time.After(duration):
			syncAll()
		}
	}
}

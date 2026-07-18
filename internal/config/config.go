package config

import (
	"os"

	"github.com/joho/godotenv"
)

// AppConfig holds the application configuration loaded from environment variables.
var AppConfig Config

// Config represents the application configuration.
type Config struct {
	Port     string
	MongoURI string

	DomainStatic string // dev fallback for domain_static setting (env: DOMAIN_STATIC)

	// Redis (optional) — ไม่ตั้ง = ไม่ใช้ cache (env: REDIS_URL, รองรับ RADIS_URL สะกดเดิม)
	RedisURL string

	LogPath string // Path to rotating log file (env: LOG_PATH)
}

// Load reads configuration from environment variables (and .env file).
func Load() {
	// Load .env file if present (ignore error if not found)
	godotenv.Load()

	AppConfig = Config{
		Port:         getEnv("PORT", "8082"),
		MongoURI:     getEnv("DATABASE_URL", "mongodb://localhost:27017"),
		DomainStatic: getEnv("DOMAIN_STATIC", ""),
		RedisURL:     getEnv("REDIS_URL", getEnv("RADIS_URL", "")),
		LogPath:      getEnv("LOG_PATH", "logs/content-node.log"),
	}
}

func getEnv(key, fallback string) string {
	if val := os.Getenv(key); val != "" {
		return val
	}
	return fallback
}

package config

import (
	"os"
	"strconv"
)

type Config struct {
	Port               string
	DatabaseURL        string
	StorageRoot        string
	MaxUploadBytes     int64
	APIToken           string
	PollIntervalSecs   int64
	MaxAgentIterations int64
}

func Load() Config {
	return Config{
		Port:               getEnv("PORT", "8080"),
		DatabaseURL:        getEnv("DATABASE_URL", "postgres://docagent:docagent@localhost:5432/docagent?sslmode=disable"),
		StorageRoot:        getEnv("STORAGE_ROOT", "/data/uploads"),
		MaxUploadBytes:     getEnvInt64("MAX_UPLOAD_BYTES", 20*1024*1024), // 20MB default
		APIToken:           getEnv("API_TOKEN", "dev-token"),
		PollIntervalSecs:   getEnvInt64("POLL_INTERVAL_SECONDS", 3),
		MaxAgentIterations: getEnvInt64("MAX_AGENT_ITERATIONS", 5),
	}
}

func getEnv(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

func getEnvInt64(key string, fallback int64) int64 {
	v := os.Getenv(key)
	if v == "" {
		return fallback
	}
	n, err := strconv.ParseInt(v, 10, 64)
	if err != nil {
		return fallback
	}
	return n
}

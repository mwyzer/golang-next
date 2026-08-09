package config

import (
	"os"
	"strconv"
	"strings"
)

type Config struct {
	Port               string
	DatabaseURL        string
	StorageRoot        string
	MaxUploadBytes     int64
	APIToken           string
	PollIntervalSecs   int64
	MaxAgentIterations int64
	ToolTimeoutSecs    int64
	MaxRetries         int64
	CORSAllowedOrigins []string
}

func Load() Config {
	return Config{
		Port:               getEnv("PORT", "8080"),
		DatabaseURL:        getEnv("DATABASE_URL", "postgres://docagent:docagent@localhost:5432/docagent?sslmode=disable"),
		StorageRoot:        getEnv("STORAGE_ROOT", "/data/uploads"),
		MaxUploadBytes:     getEnvInt64("MAX_UPLOAD_BYTES", 20*1024*1024), // 20MB default
		APIToken:           getEnv("API_TOKEN", "dev-token"),              // bootstraps the seeded dev user's bearer token; see cmd/api/main.go
		PollIntervalSecs:   getEnvInt64("POLL_INTERVAL_SECONDS", 3),
		MaxAgentIterations: getEnvInt64("MAX_AGENT_ITERATIONS", 10), // full pipeline is 7 steps; leaves headroom
		ToolTimeoutSecs:    getEnvInt64("TOOL_TIMEOUT_SECONDS", 30), // bounds each OCR/LLM provider call (NFR-3)
		MaxRetries:         getEnvInt64("MAX_RETRIES", 2),           // bounded retry for recoverable failures (FR-27, NFR-11)
		CORSAllowedOrigins: getEnvList("CORS_ALLOWED_ORIGINS", []string{"http://localhost:3000"}),
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

// getEnvList parses a comma-separated env var into a slice, trimming
// whitespace around each entry.
func getEnvList(key string, fallback []string) []string {
	v := os.Getenv(key)
	if v == "" {
		return fallback
	}
	parts := strings.Split(v, ",")
	for i, p := range parts {
		parts[i] = strings.TrimSpace(p)
	}
	return parts
}

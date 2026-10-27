package config

import (
	"os"
	"strconv"
)

type Config struct {
	Addr           string
	DataDir        string
	WorkerCount    int
	MaxUploadBytes int64
}

func Load() Config {
	return Config{
		Addr:           env("ADDR", ":8080"),
		DataDir:        env("DATA_DIR", "./data"),
		WorkerCount:    envInt("WORKERS", 2),
		MaxUploadBytes: int64(envInt("MAX_UPLOAD_MB", 2048)) << 20,
	}
}

func env(key, fallback string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return fallback
}

func envInt(key string, fallback int) int {
	value, err := strconv.Atoi(os.Getenv(key))
	if err != nil || value <= 0 {
		return fallback
	}
	return value
}

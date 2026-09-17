package config

import (
	"os"
	"strconv"
	"time"
)

type Config struct {
	Addr             string
	WorkDir          string
	MaxConcurrent    int
	MaxUploadBytes   int64
	MaxUncompressed  int64
	MaxZipFiles      int
	MaxCloneBytes    int64
	MaxWalkFiles     int
	MaxFileReadBytes int64
	DetectTimeout    time.Duration
	CloneTimeout     time.Duration
	GitHubToken      string
	AllowLocalPath   bool
	LinguistWorkers  int
	ManifestWorkers  int
}

func Load() Config {
	loadDotEnv()
	return Config{
		Addr:             env("DETECTOR_ADDR", "127.0.0.1:8090"),
		WorkDir:          env("DETECTOR_WORKDIR", os.TempDir()),
		MaxConcurrent:    envInt("DETECTOR_MAX_CONCURRENT", 8),
		MaxUploadBytes:   envInt64("DETECTOR_MAX_UPLOAD_BYTES", 80<<20),
		MaxUncompressed:  envInt64("DETECTOR_MAX_UNCOMPRESSED_BYTES", 400<<20),
		MaxZipFiles:      envInt("DETECTOR_MAX_ZIP_FILES", 20000),
		MaxCloneBytes:    envInt64("DETECTOR_MAX_CLONE_BYTES", 400<<20),
		MaxWalkFiles:     envInt("DETECTOR_MAX_WALK_FILES", 40000),
		MaxFileReadBytes: envInt64("DETECTOR_MAX_FILE_READ_BYTES", 1<<20),
		DetectTimeout:    envDuration("DETECTOR_TIMEOUT", 90*time.Second),
		CloneTimeout:     envDuration("DETECTOR_CLONE_TIMEOUT", 60*time.Second),
		GitHubToken:      os.Getenv("GITHUB_TOKEN"),
		AllowLocalPath:   env("DETECTOR_ALLOW_LOCAL_PATH", "") == "1",
		LinguistWorkers:  envInt("DETECTOR_LINGUIST_WORKERS", 16),
		ManifestWorkers:  envInt("DETECTOR_MANIFEST_WORKERS", 8),
	}
}

func env(key, fallback string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return fallback
}

func envInt(key string, fallback int) int {
	if value := os.Getenv(key); value != "" {
		if parsed, err := strconv.Atoi(value); err == nil && parsed > 0 {
			return parsed
		}
	}
	return fallback
}

func envInt64(key string, fallback int64) int64 {
	if value := os.Getenv(key); value != "" {
		if parsed, err := strconv.ParseInt(value, 10, 64); err == nil && parsed > 0 {
			return parsed
		}
	}
	return fallback
}

func envDuration(key string, fallback time.Duration) time.Duration {
	if value := os.Getenv(key); value != "" {
		if parsed, err := time.ParseDuration(value); err == nil && parsed > 0 {
			return parsed
		}
	}
	return fallback
}

package config

import (
	"errors"
	"fmt"
	"os"
	"strings"

	"github.com/KOFI-GYIMAH/github-monitor/pkg/logger"
	"github.com/joho/godotenv"
)

type Config struct {
	GitHubToken       string
	DBURL             string
	SyncInterval      string
	DefaultRepository string
}

// * LoadConfiguration reads the configuration from the .env file and returns a pointer to a Config
func LoadConfiguration() (*Config, error) {
	_ = godotenv.Load(".env")

	cfg := &Config{
		GitHubToken:       os.Getenv("GITHUB_TOKEN"),
		DBURL:             os.Getenv("DB_PATH"),
		SyncInterval:      os.Getenv("SYNC_INTERVAL"),
		DefaultRepository: os.Getenv("DEFAULT_REPOSITORY"),
	}

	if cfg.GitHubToken == "" {
		return nil, errors.New("GITHUB_TOKEN is required")
	}

	if cfg.DBURL == "" {
		return nil, errors.New("DB_PATH is required")
	}

	if cfg.SyncInterval == "" {
		logger.Warn("No sync interval specified. Using '1h' as default")
		cfg.SyncInterval = "1h"
	}

	if cfg.DefaultRepository == "" {
		logger.Warn("No default repository specified. Using 'chromium/chromium' as default")
		cfg.DefaultRepository = "chromium/chromium"
	}

	logger.Info("env content loaded successfully 🎉")
	return cfg, nil
}

// * ParseRepository takes a string in the format owner/name and returns the
// * owner and name as two separate strings. If the string does not match
// * the expected format, an error is returned.
func ParseRepository(repo string) (owner, name string, err error) {
	parts := strings.Split(repo, "/")
	if len(parts) != 2 {
		logger.Warn("Invalid repository name format: %s", repo)
		return "", "", fmt.Errorf("repository should be in format owner/name")
	}
	return parts[0], parts[1], nil
}

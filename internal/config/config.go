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
	GitHubToken  string
	DBURL        string
	SyncInterval string
	Repository   string
	RabbitMQURL  string
}

// * LoadConfiguration reads the configuration from the .env file and returns a pointer to a Config
func LoadConfiguration() (*Config, error) {
	_ = godotenv.Load(".env")

	cfg := &Config{
		GitHubToken:  os.Getenv("GITHUB_TOKEN"),
		DBURL:        os.Getenv("DB_PATH"),
		SyncInterval: os.Getenv("SYNC_INTERVAL"),
		Repository:   os.Getenv("REPOSITORY"),
		RabbitMQURL:   os.Getenv("RabbitMQURL"),
	}

	if cfg.GitHubToken == "" {
		return nil, errors.New("GITHUB_TOKEN is required")
	}

	if cfg.DBURL == "" {
		return nil, errors.New("DB_PATH is required")
	}

	if cfg.SyncInterval == "" {
		cfg.SyncInterval = "1h"
	}

	if cfg.Repository == "" {
		return nil, errors.New("REPOSITORY is required")
	}

	if cfg.RabbitMQURL == "" {
		return nil, errors.New("RabbitMQURL is required")
	}

	logger.Info("✅ env content loaded successfully 🎉")
	return cfg, nil
}

// * ParseRepository takes a string in the format owner/name and returns the
// * owner and name as two separate strings. If the string does not match
// * the expected format, an error is returned.
func ParseRepository(repo string) (owner, name string, err error) {
	parts := strings.Split(repo, "/")
	if len(parts) != 2 {
		return "", "", fmt.Errorf("repository should be in format owner/name")
	}
	return parts[0], parts[1], nil
}

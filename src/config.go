package main

import (
	"errors"
	"os"
	"regexp"
	"strings"

	"github.com/BurntSushi/toml"
	"github.com/robfig/cron/v3"
)

// Config represents the application configuration
type Config struct {
	IPABaseURL      string   `toml:"ipa_base_url"`
	RefreshSchedule string   `toml:"refresh_schedule"`
	Targets         []Target `toml:"targets"`
}

// Target represents a GitHub repository target
type Target struct {
	GitHubRepo  string `toml:"github_repo"`
	GitHubToken string `toml:"github_token"`
}

// LoadConfig loads the configuration from the specified path
func LoadConfig(path string) (*Config, error) {
	if path == "" {
		path = "config.toml"
		// Check if CONFIG_PATH env var is set
		if envPath := os.Getenv("CONFIG_PATH"); envPath != "" {
			path = envPath
		}
	}

	var config Config
	if _, err := toml.DecodeFile(path, &config); err != nil {
		return nil, err
	}

	if err := validateConfig(&config); err != nil {
		return nil, err
	}

	return &config, nil
}

// validateConfig validates the configuration
func validateConfig(config *Config) error {
	// Validate IPA Base URL
	if config.IPABaseURL == "" {
		return errors.New("ipa_base_url is required")
	}
	if !strings.HasPrefix(config.IPABaseURL, "http://") && !strings.HasPrefix(config.IPABaseURL, "https://") {
		return errors.New("ipa_base_url must be a valid URL")
	}

	// Validate cron schedule
	if config.RefreshSchedule == "" {
		return errors.New("refresh_schedule is required")
	}
	_, err := cron.ParseStandard(config.RefreshSchedule)
	if err != nil {
		return errors.New("invalid cron expression: " + err.Error())
	}

	// Validate targets
	if len(config.Targets) == 0 {
		return errors.New("at least one target is required")
	}

	repoRegex := regexp.MustCompile(`^[a-zA-Z0-9-]+/[a-zA-Z0-9-]+$`)
	for _, target := range config.Targets {
		if target.GitHubRepo == "" {
			return errors.New("github_repo is required for all targets")
		}
		if !repoRegex.MatchString(target.GitHubRepo) {
			return errors.New("github_repo must be in the format 'owner/repo'")
		}
		if target.GitHubToken == "" {
			return errors.New("github_token is required for all targets")
		}
	}

	return nil
}

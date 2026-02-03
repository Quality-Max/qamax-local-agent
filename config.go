package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// Config represents the persistent configuration stored in ~/.qamax/config.json.
type Config struct {
	Token              string `json:"token,omitempty"`
	APIURL             string `json:"api_url,omitempty"`
	AgentID            string `json:"agent_id,omitempty"`
	APIKey             string `json:"api_key,omitempty"`
	RegistrationSecret string `json:"registration_secret,omitempty"`
}

const (
	configDirName  = ".qamax"
	configFileName = "config.json"
)

// ConfigPath returns the full path to the config file.
func ConfigPath() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("cannot determine home directory: %w", err)
	}
	return filepath.Join(home, configDirName, configFileName), nil
}

// ConfigDir returns the config directory path.
func ConfigDir() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("cannot determine home directory: %w", err)
	}
	return filepath.Join(home, configDirName), nil
}

// LoadConfig reads the config from ~/.qamax/config.json.
// Returns a zero Config (no error) if the file does not exist.
func LoadConfig() (*Config, error) {
	path, err := ConfigPath()
	if err != nil {
		return nil, err
	}

	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return &Config{}, nil
		}
		return nil, fmt.Errorf("read config: %w", err)
	}

	var cfg Config
	if err := json.Unmarshal(data, &cfg); err != nil {
		return nil, fmt.Errorf("parse config: %w", err)
	}
	return &cfg, nil
}

// Save writes the config to ~/.qamax/config.json with secure permissions.
func (c *Config) Save() error {
	dir, err := ConfigDir()
	if err != nil {
		return err
	}

	if err := os.MkdirAll(dir, 0700); err != nil {
		return fmt.Errorf("create config directory: %w", err)
	}

	data, err := json.MarshalIndent(c, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal config: %w", err)
	}

	path := filepath.Join(dir, configFileName)
	if err := os.WriteFile(path, data, 0600); err != nil {
		return fmt.Errorf("write config: %w", err)
	}
	return nil
}

// GetAPIBaseURL returns the API URL with any trailing /app suffix removed.
func (c *Config) GetAPIBaseURL() string {
	u := strings.TrimRight(c.APIURL, "/")
	u = strings.TrimSuffix(u, "/app")
	return u
}

// Package config manages GoJira's persistent configuration.
// Stores config at ~/.config/gojira/config.yaml (XDG-style).
// API token is stored in the OS keychain when available, with YAML fallback.
package config

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/zalando/go-keyring"
	"gopkg.in/yaml.v3"
)

const (
	appName        = "gojira"
	keyringService = "gojira"
	keyringUser    = "api-token"
)

// Config holds the application's persistent configuration.
type Config struct {
	Domain  string            `yaml:"domain"`
	Email   string            `yaml:"email"`
	Board   string            `yaml:"default_board,omitempty"`
	Project string            `yaml:"default_project,omitempty"`
	People  map[string]string `yaml:"people,omitempty"` // alias → email
	Token   string            `yaml:"token,omitempty"`  // only when keychain unavailable
}

// ConfigPath returns the full path to config.yaml.
func ConfigPath() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("cannot determine home directory: %w", err)
	}
	return filepath.Join(home, ".config", appName, "config.yaml"), nil
}

// ConfigDir returns the directory containing config.yaml.
func ConfigDir() (string, error) {
	path, err := ConfigPath()
	if err != nil {
		return "", err
	}
	return filepath.Dir(path), nil
}

// Exists returns true if a config file is on disk.
func Exists() bool {
	path, err := ConfigPath()
	if err != nil {
		return false
	}
	_, err = os.Stat(path)
	return err == nil
}

// Load reads the config from disk. Returns an empty Config (not an error) when
// the file does not exist yet.
func Load() (*Config, error) {
	path, err := ConfigPath()
	if err != nil {
		return nil, err
	}

	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return &Config{}, nil
		}
		return nil, err
	}

	var cfg Config
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return nil, fmt.Errorf("parsing %s: %w", path, err)
	}
	return &cfg, nil
}

// Save writes the config to disk with restricted permissions (0600).
func (c *Config) Save() error {
	dir, err := ConfigDir()
	if err != nil {
		return err
	}
	if err := os.MkdirAll(dir, 0755); err != nil {
		return err
	}

	path, err := ConfigPath()
	if err != nil {
		return err
	}

	data, err := yaml.Marshal(c)
	if err != nil {
		return err
	}

	return os.WriteFile(path, data, 0600)
}

// ─── Token storage (keychain → YAML fallback) ─────────────────────────

// SaveToken stores the API token. Tries the OS keychain first;
// if that fails, stores it in the config file.
func SaveToken(token string) error {
	// Try keychain
	if err := keyring.Set(keyringService, keyringUser, token); err == nil {
		// Also clear any leftover token in the YAML
		cfg, err := Load()
		if err == nil && cfg.Token != "" {
			cfg.Token = ""
			_ = cfg.Save() // best effort
		}
		return nil
	}

	// Fallback: store in config file
	cfg, err := Load()
	if err != nil {
		return err
	}
	cfg.Token = token
	return cfg.Save()
}

// LoadToken retrieves the API token. Tries the OS keychain first,
// then falls back to the config file.
func LoadToken() (string, error) {
	// Try keychain
	token, err := keyring.Get(keyringService, keyringUser)
	if err == nil && token != "" {
		return token, nil
	}

	// Fallback: read from config
	cfg, err := Load()
	if err != nil {
		return "", err
	}
	if cfg.Token != "" {
		return cfg.Token, nil
	}

	return "", fmt.Errorf("no API token configured (run: gojira config init)")
}

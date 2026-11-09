// Package config manages GoJira's persistent configuration.
// Stores config at ~/.config/gojira/config.yaml (XDG-style).
// API token is stored in the OS keychain when available, with YAML fallback.
package config

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/zalando/go-keyring"
	"gopkg.in/yaml.v3"
)

const (
	appName        = "gojira"
	keyringService = "gojira"
	keyringUser    = "api-token"
)

// UpdateCheckInterval is the minimum age of the last recorded release check
// before the TUI checks for a newer release again.
const UpdateCheckInterval = 24 * time.Hour

var (
	keyringSet = keyring.Set
	keyringGet = keyring.Get
	saveConfig = func(cfg *Config) error { return cfg.Save() }
)

// Config holds the application's persistent configuration.
type Config struct {
	Domain  string            `yaml:"domain"`
	Email   string            `yaml:"email"`
	Board   string            `yaml:"default_board,omitempty"`
	Project string            `yaml:"default_project,omitempty"`
	People  map[string]string `yaml:"people,omitempty"` // alias → email
	Token   string            `yaml:"token,omitempty"`  // only when keychain unavailable

	UpdateCheck         *bool  `yaml:"update_check,omitempty"`          // nil defaults to enabled
	UpdateLastCheck     string `yaml:"update_last_check,omitempty"`     // RFC3339, written by the binary
	UpdateLatestVersion string `yaml:"update_latest_version,omitempty"` // release tag, written by the binary
	UpdateETag          string `yaml:"update_etag,omitempty"`           // response ETag of the last 200 check
	UpdateCheckInterval string `yaml:"update_check_interval,omitempty"` // duration such as "6h"; "" or "0" checks every start
}

// UpdateCheckEnabled reports whether the automatic release check is enabled.
// It is true unless update_check is explicitly set to false.
func (c *Config) UpdateCheckEnabled() bool {
	return c.UpdateCheck == nil || *c.UpdateCheck
}

// UpdateCheckDue reports whether a new release check should run at now. The
// check is due when nothing was recorded yet, when the recorded timestamp is
// not valid RFC3339, or when the last check is at least interval old.
func (c *Config) UpdateCheckDue(now time.Time, interval time.Duration) bool {
	if c.UpdateLastCheck == "" {
		return true
	}
	last, err := time.Parse(time.RFC3339, c.UpdateLastCheck)
	if err != nil {
		return true
	}
	return !now.Before(last.Add(interval))
}

// UpdateCheckIntervalDuration parses update_check_interval into a duration.
// It fails safe: an absent, zero, unparsable or negative value behaves like 0,
// which means a check on every start — it never behaves like "never check".
func (c *Config) UpdateCheckIntervalDuration() time.Duration {
	raw := strings.TrimSpace(c.UpdateCheckInterval)
	if raw == "" {
		return 0
	}
	interval, err := time.ParseDuration(raw)
	if err != nil || interval < 0 {
		return 0
	}
	return interval
}

// RecordUpdateCheck stores the moment of the last release check as a UTC
// RFC3339 timestamp together with the latest release tag. It stores nothing
// else — no credentials ever go into these fields.
func (c *Config) RecordUpdateCheck(at time.Time, latestVersion string) {
	c.UpdateLastCheck = at.UTC().Format(time.RFC3339)
	c.UpdateLatestVersion = latestVersion
}

// RecordUpdateCheckResult stores everything a conditional start-up check
// produced: the moment of the check, the latest release tag and the response
// ETag the next request must send as If-None-Match. RecordUpdateCheck remains
// the ETag-neutral variant for callers that do not manage conditional
// requests; this method keeps its behaviour and adds the ETag on top.
func (c *Config) RecordUpdateCheckResult(at time.Time, latestVersion, etag string) {
	c.RecordUpdateCheck(at, latestVersion)
	c.UpdateETag = etag
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
	if err := keyringSet(keyringService, keyringUser, token); err == nil {
		// Also clear any leftover token in the YAML.
		cfg, err := Load()
		if err == nil && cfg.Token != "" {
			cfg.Token = ""
			if err := saveConfig(cfg); err != nil {
				return fmt.Errorf("clearing token from config: %w", err)
			}
		}
		return nil
	}

	// Fallback: store in config file
	cfg, err := Load()
	if err != nil {
		return err
	}
	cfg.Token = token
	return saveConfig(cfg)
}

// LoadToken retrieves the API token. Tries the OS keychain first,
// then falls back to the config file.
func LoadToken() (string, error) {
	// Try keychain
	token, err := keyringGet(keyringService, keyringUser)
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

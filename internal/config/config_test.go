package config

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/zalando/go-keyring"
)

func useTempHome(t *testing.T) {
	t.Helper()
	t.Setenv("HOME", t.TempDir())
	keyringSet = func(string, string, string) error { return errors.New("keyring unavailable") }
	keyringGet = func(string, string) (string, error) { return "", errors.New("keyring unavailable") }
	saveConfig = func(cfg *Config) error { return cfg.Save() }
	t.Cleanup(func() {
		keyringSet = keyring.Set
		keyringGet = keyring.Get
		saveConfig = func(cfg *Config) error { return cfg.Save() }
	})
}

func TestSaveLoadRoundTrip(t *testing.T) {
	useTempHome(t)
	want := &Config{Domain: "example.atlassian.net", Email: "user@example.com", Board: "Engineering", Project: "GO", People: map[string]string{"me": "user@example.com"}, Token: "fallback-token"}
	if err := want.Save(); err != nil {
		t.Fatalf("Save() error = %v", err)
	}
	got, err := Load()
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	if got.Domain != want.Domain || got.Email != want.Email || got.Board != want.Board || got.Project != want.Project || got.Token != want.Token || got.People["me"] != want.People["me"] {
		t.Fatalf("Load() = %#v, want %#v", got, want)
	}
	path, _ := ConfigPath()
	if mode := fileMode(t, path); mode != 0600 {
		t.Fatalf("config mode = %#o, want 0600", mode)
	}
}

func TestLoadMissingConfig(t *testing.T) {
	useTempHome(t)
	got, err := Load()
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	if got.Domain != "" || got.Email != "" || got.Board != "" || got.Project != "" || got.Token != "" || got.People != nil {
		t.Fatalf("Load() = %#v, want empty config", got)
	}
}

func TestLoadMalformedConfig(t *testing.T) {
	useTempHome(t)
	path, _ := ConfigPath()
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte("domain: ["), 0600); err != nil {
		t.Fatal(err)
	}
	_, err := Load()
	if err == nil || !strings.Contains(err.Error(), "parsing") {
		t.Fatalf("Load() error = %v, want parsing error", err)
	}
}

func TestSaveTokenKeyringSuccessClearsYAML(t *testing.T) {
	useTempHome(t)
	if err := (&Config{Token: "legacy-token"}).Save(); err != nil {
		t.Fatal(err)
	}
	var stored string
	keyringSet = func(service, user, token string) error {
		if service != keyringService || user != keyringUser {
			t.Fatalf("keyring.Set(%q, %q), want service/user %q/%q", service, user, keyringService, keyringUser)
		}
		stored = token
		return nil
	}
	if err := SaveToken("keyring-token"); err != nil {
		t.Fatalf("SaveToken() error = %v", err)
	}
	if stored != "keyring-token" {
		t.Fatalf("stored token = %q", stored)
	}
	cfg, err := Load()
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Token != "" {
		t.Fatalf("YAML token = %q, want cleared", cfg.Token)
	}
}

func TestSaveTokenKeyringFailureFallsBackToYAML(t *testing.T) {
	useTempHome(t)
	keyringSet = func(string, string, string) error { return errors.New("keyring unavailable") }
	if err := SaveToken("yaml-token"); err != nil {
		t.Fatalf("SaveToken() error = %v", err)
	}
	cfg, err := Load()
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Token != "yaml-token" {
		t.Fatalf("YAML token = %q, want yaml-token", cfg.Token)
	}
}

func TestSaveTokenReportsPersistenceFailureAfterKeyringSuccess(t *testing.T) {
	useTempHome(t)
	if err := (&Config{Token: "legacy-token"}).Save(); err != nil {
		t.Fatal(err)
	}
	keyringSet = func(string, string, string) error { return nil }
	wantErr := errors.New("disk full")
	saveConfig = func(*Config) error { return wantErr }
	if err := SaveToken("keyring-token"); !errors.Is(err, wantErr) {
		t.Fatalf("SaveToken() error = %v, want wrapped %v", err, wantErr)
	}
}

func TestLoadTokenMissing(t *testing.T) {
	useTempHome(t)
	_, err := LoadToken()
	if err == nil || !strings.Contains(err.Error(), "no API token configured") {
		t.Fatalf("LoadToken() error = %v, want missing-token error", err)
	}
}

func fileMode(t *testing.T, path string) os.FileMode {
	t.Helper()
	info, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	return info.Mode().Perm()
}

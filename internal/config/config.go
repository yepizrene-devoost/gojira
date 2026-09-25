// Package config manages GoJira's persistent configuration.
// Phase 5 creates the skeleton; Phase 6 adds XDG paths, YAML persistence,
// keychain integration, and the TUI onboarding wizard.
package config

// Config holds the application's persistent configuration.
type Config struct {
	Domain  string            `yaml:"domain"`
	Email   string            `yaml:"email"`
	Board   string            `yaml:"default_board"`
	Project string            `yaml:"default_project"`
	People  map[string]string `yaml:"people,omitempty"` // alias → email
}

// TODO (Phase 6): Load, Save, Path, Set, Get, Init, Test

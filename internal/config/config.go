package config

import (
	"fmt"
	"os"
	"path/filepath"

	"gopkg.in/yaml.v3"
)

// Config holds all whport configuration.
type Config struct {
	RefreshInterval int      `yaml:"refresh_interval"` // seconds
	DefaultView     string   `yaml:"default_view"`     // "listen" or "all"
	KillSignal      string   `yaml:"kill_signal"`      // default signal name
	Exclude         []string `yaml:"exclude"`          // process names to hide
	ColorEnabled    bool     `yaml:"color_enabled"`
}

// Default returns a Config with sensible default values.
func Default() *Config {
	return &Config{
		RefreshInterval: 2,
		DefaultView:     "listen",
		KillSignal:      "SIGTERM",
		Exclude:         []string{},
		ColorEnabled:    true,
	}
}

// Load loads config from the given path. If path is empty, it uses the
// default location (~/.config/whport/config.yaml). If the file does not
// exist, it returns defaults without creating the file.
func Load(path string) (*Config, error) {
	if path == "" {
		home, err := os.UserHomeDir()
		if err != nil {
			return Default(), nil
		}
		path = filepath.Join(home, ".config", "whport", "config.yaml")
	}

	if _, err := os.Stat(path); os.IsNotExist(err) {
		return Default(), nil
	}

	return LoadFrom(path)
}

// LoadFrom loads and parses config from the given path. Missing fields
// keep their default values.
func LoadFrom(path string) (*Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("failed to read config file: %w", err)
	}

	cfg := Default()
	if err := yaml.Unmarshal(data, cfg); err != nil {
		return nil, fmt.Errorf("failed to parse config file: %w", err)
	}

	return cfg, nil
}

// Save marshals the config to YAML and writes it to the given path,
// creating parent directories as needed.
func (c *Config) Save(path string) error {
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return fmt.Errorf("failed to create config directory: %w", err)
	}

	data, err := yaml.Marshal(c)
	if err != nil {
		return fmt.Errorf("failed to marshal config: %w", err)
	}

	if err := os.WriteFile(path, data, 0o644); err != nil {
		return fmt.Errorf("failed to write config file: %w", err)
	}

	return nil
}

// DefaultPath returns the default config file path.
func DefaultPath() string {
	home, err := os.UserHomeDir()
	if err != nil {
		return ""
	}
	return filepath.Join(home, ".config", "whport", "config.yaml")
}

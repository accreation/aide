package config

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/goccy/go-yaml"
)

type Config struct {
	Provider string `yaml:"provider"`
	Args     string `yaml:"args,omitempty"`
	Mode     string `yaml:"mode,omitempty"`
	Tools    []Tool `yaml:"tools"`
}

// IsIsolated returns true when Mode is "isolated".
func (c *Config) IsIsolated() bool {
	return c.Mode == "isolated"
}

type Tool struct {
	Name    string `yaml:"name"`
	Version string `yaml:"version,omitempty"`
}

// FindPath looks for aide.yaml starting at startDir and walking up to root.
// Returns the full path to the file, or error if not found.
func FindPath(startDir string) (string, error) {
	dir := startDir
	for {
		path := filepath.Join(dir, "aide.yaml")
		if _, err := os.Stat(path); err == nil {
			return path, nil
		} else if !os.IsNotExist(err) {
			return "", fmt.Errorf("checking %s: %w", path, err)
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			return "", fmt.Errorf("aide.yaml not found (searched from %s upward)", startDir)
		}
		dir = parent
	}
}

// FindAndParse looks for aide.yaml starting at startDir and walking up to root.
// Returns parsed Config or error if not found / invalid.
func FindAndParse(startDir string) (*Config, error) {
	dir := startDir
	for {
		path := filepath.Join(dir, "aide.yaml")
		data, err := os.ReadFile(path)
		if err == nil {
			return parse(data)
		}
		if !os.IsNotExist(err) {
			return nil, fmt.Errorf("reading %s: %w", path, err)
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			return nil, fmt.Errorf("aide.yaml not found (searched from %s upward)", startDir)
		}
		dir = parent
	}
}

// GenerateDefault returns a default Config for aide init.
func GenerateDefault(provider string) *Config {
	return &Config{
		Provider: provider,
		Tools:    []Tool{},
	}
}

// GenerateIsolated returns a Config for aide init with mode: isolated.
func GenerateIsolated(provider string) *Config {
	return &Config{
		Provider: provider,
		Mode:     "isolated",
		Tools:    []Tool{},
	}
}

func parse(data []byte) (*Config, error) {
	var cfg Config
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return nil, fmt.Errorf("parsing aide.yaml: %w", err)
	}
	if cfg.Provider == "" {
		return nil, fmt.Errorf("aide.yaml: 'provider' is required")
	}
	return &cfg, nil
}

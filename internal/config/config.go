package config

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/goccy/go-yaml"
)

type Config struct {
	Provider string `yaml:"provider"`
	Tools    []Tool `yaml:"tools"`
}

type Tool struct {
	Name    string `yaml:"name"`
	Version string `yaml:"version,omitempty"`
}

// FindAndParse looks for aion.yaml starting at startDir and walking up to root.
// Returns parsed Config or error if not found / invalid.
func FindAndParse(startDir string) (*Config, error) {
	dir := startDir
	for {
		path := filepath.Join(dir, "aion.yaml")
		data, err := os.ReadFile(path)
		if err == nil {
			return parse(data)
		}
		if !os.IsNotExist(err) {
			return nil, fmt.Errorf("reading %s: %w", path, err)
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			return nil, fmt.Errorf("aion.yaml not found (searched from %s upward)", startDir)
		}
		dir = parent
	}
}

// GenerateDefault returns a default Config for aion init.
func GenerateDefault(provider string) *Config {
	return &Config{
		Provider: provider,
		Tools:    []Tool{},
	}
}

func parse(data []byte) (*Config, error) {
	var cfg Config
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return nil, fmt.Errorf("parsing aion.yaml: %w", err)
	}
	if cfg.Provider == "" {
		return nil, fmt.Errorf("aion.yaml: 'provider' is required")
	}
	return &cfg, nil
}

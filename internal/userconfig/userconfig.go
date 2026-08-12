// Package userconfig parses ~/.aide/config.yaml, the user-owned account
// binding layer that sits above aide.yaml's repo-declared account: field.
// Unlike ~/.aide/accounts.json, aide never writes this file — the user
// edits it by hand, which is what makes it a safe place to override what a
// cloned, committed aide.yaml declares (see #38 / #35 §3.4).
package userconfig

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/goccy/go-yaml"
)

// Binding maps a path prefix to an account name, optionally varying by
// provider. Path uses git includeIf gitdir-style prefix matching: a
// trailing "**" (with or without a preceding separator) is stripped before
// comparison, so "~/work/acme/**" matches "~/work/acme" itself and
// everything under it.
type Binding struct {
	Path string `yaml:"path"`

	// Account is used when Accounts has no entry for the current
	// aide.yaml's provider.
	Account string `yaml:"account,omitempty"`

	// Accounts maps provider name -> account name, so a binding can keep
	// the same identity across a repo's provider switching (e.g. claude
	// today, copilot tomorrow) without editing ~/.aide/config.yaml.
	Accounts map[string]string `yaml:"accounts,omitempty"`
}

// Config is the parsed contents of ~/.aide/config.yaml.
type Config struct {
	Bindings []Binding `yaml:"bindings"`
}

// Path returns ~/.aide/config.yaml.
func Path() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("getting home directory: %w", err)
	}
	return filepath.Join(home, ".aide", "config.yaml"), nil
}

// Load parses ~/.aide/config.yaml. A missing file returns an empty, non-nil
// Config rather than an error — this file is entirely optional.
func Load() (*Config, error) {
	p, err := Path()
	if err != nil {
		return nil, err
	}
	data, err := os.ReadFile(p)
	if err != nil {
		if os.IsNotExist(err) {
			return &Config{}, nil
		}
		return nil, fmt.Errorf("reading %s: %w", p, err)
	}
	var cfg Config
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return nil, fmt.Errorf("parsing %s: %w", p, err)
	}
	return &cfg, nil
}

// normalizePath expands a leading ~ and strips a trailing "**" glob
// suffix (plus the separator before it, if any), then resolves the result
// to a cleaned absolute path suitable for prefix comparison.
func normalizePath(p string) (string, error) {
	trimmed := strings.TrimSuffix(strings.TrimSpace(p), "**")
	trimmed = strings.TrimRight(trimmed, `/\`)
	if trimmed == "" {
		return "", fmt.Errorf("empty binding path")
	}

	switch {
	case trimmed == "~":
		home, err := os.UserHomeDir()
		if err != nil {
			return "", err
		}
		trimmed = home
	case strings.HasPrefix(trimmed, "~/") || strings.HasPrefix(trimmed, `~\`):
		home, err := os.UserHomeDir()
		if err != nil {
			return "", err
		}
		trimmed = filepath.Join(home, trimmed[2:])
	}

	abs, err := filepath.Abs(trimmed)
	if err != nil {
		return "", err
	}
	return filepath.Clean(abs), nil
}

// isUnderOrEqual reports whether dir is bindingPath itself or nested under
// it, respecting path-separator boundaries so "~/work/acme" does not match
// "~/work/acme2".
func isUnderOrEqual(dir, bindingPath string) bool {
	if dir == bindingPath {
		return true
	}
	return strings.HasPrefix(dir, bindingPath+string(filepath.Separator))
}

// Resolve returns the account name bound to projectDir for provider, by
// longest-prefix match over c.Bindings. A binding whose Path matches
// projectDir but carries no usable name for provider (no Accounts[provider]
// and no bare Account) is not a match — resolution keeps considering other
// bindings rather than stopping on one that doesn't apply here. c may be
// nil (equivalent to an empty Config).
func (c *Config) Resolve(projectDir, provider string) (string, bool) {
	if c == nil {
		return "", false
	}
	dir, err := normalizePath(projectDir)
	if err != nil {
		return "", false
	}

	bestLen := -1
	best := ""
	for _, b := range c.Bindings {
		name := ""
		if b.Accounts != nil {
			name = b.Accounts[provider]
		}
		if name == "" {
			name = b.Account
		}
		if name == "" {
			continue
		}

		bindingPath, err := normalizePath(b.Path)
		if err != nil {
			continue
		}
		if !isUnderOrEqual(dir, bindingPath) {
			continue
		}
		if len(bindingPath) > bestLen {
			bestLen = len(bindingPath)
			best = name
		}
	}
	if bestLen < 0 {
		return "", false
	}
	return best, true
}

// ResolveAccountName applies the full account-binding precedence: flag >
// env > uc's path bindings (longest-prefix match against projectDir and
// provider) > repoAccount (aide.yaml's account: field). uc may be nil when
// no ~/.aide/config.yaml exists (or it failed to parse) — that is treated
// the same as an empty Config, not an error, so a typo in the user's own
// config file can never brick an otherwise-working aide.yaml.
func ResolveAccountName(flag, env string, uc *Config, projectDir, provider, repoAccount string) string {
	if flag != "" {
		return flag
	}
	if env != "" {
		return env
	}
	if name, ok := uc.Resolve(projectDir, provider); ok {
		return name
	}
	return repoAccount
}

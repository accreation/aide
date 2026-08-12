package userconfig

import (
	"os"
	"path/filepath"
	"testing"
)

func setHome(t *testing.T, dir string) {
	t.Helper()
	t.Setenv("HOME", dir)
	t.Setenv("USERPROFILE", dir)
}

func TestLoadMissingFileReturnsEmptyConfig(t *testing.T) {
	setHome(t, t.TempDir())

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load failed: %v", err)
	}
	if len(cfg.Bindings) != 0 {
		t.Errorf("expected no bindings, got %v", cfg.Bindings)
	}
}

func TestLoadParsesBindings(t *testing.T) {
	home := t.TempDir()
	setHome(t, home)

	dir := filepath.Join(home, ".aide")
	if err := os.MkdirAll(dir, 0755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	yamlContent := `
bindings:
  - path: ~/work/acme/**
    account: acme-corp
  - path: ~/work/dcb/**
    accounts:
      claude: dcb-claude
      copilot: dcb-gh
`
	if err := os.WriteFile(filepath.Join(dir, "config.yaml"), []byte(yamlContent), 0644); err != nil {
		t.Fatalf("writing config.yaml: %v", err)
	}

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load failed: %v", err)
	}
	if len(cfg.Bindings) != 2 {
		t.Fatalf("expected 2 bindings, got %d", len(cfg.Bindings))
	}
	if cfg.Bindings[0].Account != "acme-corp" {
		t.Errorf("unexpected first binding: %+v", cfg.Bindings[0])
	}
	if cfg.Bindings[1].Accounts["claude"] != "dcb-claude" || cfg.Bindings[1].Accounts["copilot"] != "dcb-gh" {
		t.Errorf("unexpected second binding: %+v", cfg.Bindings[1])
	}
}

func TestConfigResolveMatchesPathAndFallsThroughAccounts(t *testing.T) {
	home := t.TempDir()
	setHome(t, home)

	cfg := &Config{Bindings: []Binding{
		{Path: "~/work/acme/**", Account: "acme-corp"},
	}}

	projectDir := filepath.Join(home, "work", "acme", "some-repo")
	name, ok := cfg.Resolve(projectDir, "claude")
	if !ok || name != "acme-corp" {
		t.Errorf("expected acme-corp, got %q ok=%v", name, ok)
	}
}

func TestConfigResolveExactPathMatches(t *testing.T) {
	home := t.TempDir()
	setHome(t, home)

	cfg := &Config{Bindings: []Binding{
		{Path: "~/work/acme/repo/**", Account: "acme-corp"},
	}}

	projectDir := filepath.Join(home, "work", "acme", "repo")
	name, ok := cfg.Resolve(projectDir, "claude")
	if !ok || name != "acme-corp" {
		t.Errorf("expected acme-corp for exact match, got %q ok=%v", name, ok)
	}
}

func TestConfigResolveDoesNotMatchSimilarPrefix(t *testing.T) {
	home := t.TempDir()
	setHome(t, home)

	cfg := &Config{Bindings: []Binding{
		{Path: "~/work/acme/**", Account: "acme-corp"},
	}}

	projectDir := filepath.Join(home, "work", "acme2")
	if name, ok := cfg.Resolve(projectDir, "claude"); ok {
		t.Errorf("expected no match for a sibling directory with a similar name, got %q", name)
	}
}

func TestConfigResolveLongestPrefixWins(t *testing.T) {
	home := t.TempDir()
	setHome(t, home)

	cfg := &Config{Bindings: []Binding{
		{Path: "~/work/**", Account: "generic-work"},
		{Path: "~/work/acme/**", Account: "acme-corp"},
	}}

	projectDir := filepath.Join(home, "work", "acme", "sub-repo")
	name, ok := cfg.Resolve(projectDir, "claude")
	if !ok || name != "acme-corp" {
		t.Errorf("expected the more specific acme-corp binding to win, got %q ok=%v", name, ok)
	}
}

func TestConfigResolvePerProviderAccountsMap(t *testing.T) {
	home := t.TempDir()
	setHome(t, home)

	cfg := &Config{Bindings: []Binding{
		{Path: "~/work/acme/**", Accounts: map[string]string{"claude": "acme-claude", "copilot": "acme-gh"}},
	}}
	projectDir := filepath.Join(home, "work", "acme", "repo")

	if name, ok := cfg.Resolve(projectDir, "claude"); !ok || name != "acme-claude" {
		t.Errorf("expected acme-claude for claude, got %q ok=%v", name, ok)
	}
	if name, ok := cfg.Resolve(projectDir, "copilot"); !ok || name != "acme-gh" {
		t.Errorf("expected acme-gh for copilot, got %q ok=%v", name, ok)
	}
	if name, ok := cfg.Resolve(projectDir, "codex"); ok {
		t.Errorf("expected no match for a provider with no Accounts entry and no bare Account, got %q", name)
	}
}

func TestConfigResolveFallsThroughToShorterMatchWhenBestHasNoUsableName(t *testing.T) {
	home := t.TempDir()
	setHome(t, home)

	cfg := &Config{Bindings: []Binding{
		{Path: "~/work/**", Account: "generic-work"},
		// More specific path, but no entry for "codex" and no bare Account —
		// must not shadow the shorter, applicable binding above.
		{Path: "~/work/acme/**", Accounts: map[string]string{"claude": "acme-claude"}},
	}}
	projectDir := filepath.Join(home, "work", "acme", "repo")

	name, ok := cfg.Resolve(projectDir, "codex")
	if !ok || name != "generic-work" {
		t.Errorf("expected fall-through to generic-work, got %q ok=%v", name, ok)
	}
}

func TestConfigResolveNoMatchReturnsFalse(t *testing.T) {
	home := t.TempDir()
	setHome(t, home)

	cfg := &Config{Bindings: []Binding{
		{Path: "~/work/acme/**", Account: "acme-corp"},
	}}
	projectDir := filepath.Join(home, "personal", "hobby")

	if name, ok := cfg.Resolve(projectDir, "claude"); ok {
		t.Errorf("expected no match, got %q", name)
	}
}

func TestConfigResolveNilConfigReturnsFalse(t *testing.T) {
	var cfg *Config
	if name, ok := cfg.Resolve("/anything", "claude"); ok {
		t.Errorf("expected nil Config to never match, got %q", name)
	}
}

func TestResolveAccountNamePrecedence(t *testing.T) {
	home := t.TempDir()
	setHome(t, home)

	uc := &Config{Bindings: []Binding{
		{Path: "~/work/acme/**", Account: "binding-account"},
	}}
	projectDir := filepath.Join(home, "work", "acme", "repo")

	cases := []struct {
		name       string
		flag, env  string
		uc         *Config
		repo       string
		wantResult string
	}{
		{"flag wins over everything", "flag-account", "env-account", uc, "repo-account", "flag-account"},
		{"env wins over binding and repo", "", "env-account", uc, "repo-account", "env-account"},
		{"binding wins over repo", "", "", uc, "repo-account", "binding-account"},
		{"repo is the final fallback", "", "", nil, "repo-account", "repo-account"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := ResolveAccountName(tc.flag, tc.env, tc.uc, projectDir, "claude", tc.repo)
			if got != tc.wantResult {
				t.Errorf("got %q, want %q", got, tc.wantResult)
			}
		})
	}
}

func TestResolveAccountNameNoBindingMatchFallsThroughToRepo(t *testing.T) {
	home := t.TempDir()
	setHome(t, home)

	uc := &Config{Bindings: []Binding{
		{Path: "~/work/other-corp/**", Account: "other-account"},
	}}
	projectDir := filepath.Join(home, "work", "acme", "repo")

	got := ResolveAccountName("", "", uc, projectDir, "claude", "repo-account")
	if got != "repo-account" {
		t.Errorf("expected fall-through to repo-account when no binding matches, got %q", got)
	}
}

package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestParseValidConfig(t *testing.T) {
	tmp := t.TempDir()
	content := `provider: claude
tools:
  - name: gh
    version: ">=2.0.0"
  - name: az
`
	os.WriteFile(filepath.Join(tmp, "aion.yaml"), []byte(content), 0644)

	cfg, err := FindAndParse(tmp)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg.Provider != "claude" {
		t.Errorf("expected provider 'claude', got %q", cfg.Provider)
	}
	if len(cfg.Tools) != 2 {
		t.Fatalf("expected 2 tools, got %d", len(cfg.Tools))
	}
	if cfg.Tools[0].Name != "gh" || cfg.Tools[0].Version != ">=2.0.0" {
		t.Errorf("unexpected tool[0]: %+v", cfg.Tools[0])
	}
	if cfg.Tools[1].Name != "az" || cfg.Tools[1].Version != "" {
		t.Errorf("unexpected tool[1]: %+v", cfg.Tools[1])
	}
}

func TestWalkUpFindsConfig(t *testing.T) {
	tmp := t.TempDir()
	sub := filepath.Join(tmp, "a", "b", "c")
	os.MkdirAll(sub, 0755)
	content := `provider: codex
tools: []
`
	os.WriteFile(filepath.Join(tmp, "aion.yaml"), []byte(content), 0644)

	cfg, err := FindAndParse(sub)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg.Provider != "codex" {
		t.Errorf("expected provider 'codex', got %q", cfg.Provider)
	}
}

func TestConfigNotFound(t *testing.T) {
	tmp := t.TempDir()
	_, err := FindAndParse(tmp)
	if err == nil {
		t.Fatal("expected error, got nil")
	}
}

func TestGenerateDefault(t *testing.T) {
	cfg := GenerateDefault("copilot")
	if cfg.Provider != "copilot" {
		t.Errorf("expected 'copilot', got %q", cfg.Provider)
	}
	if len(cfg.Tools) != 0 {
		t.Errorf("expected 0 tools, got %d", len(cfg.Tools))
	}
}

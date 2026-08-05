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
	os.WriteFile(filepath.Join(tmp, "aide.yaml"), []byte(content), 0644)

	cfg, err := FindAndParse(tmp)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg.Provider != "claude" {
		t.Errorf("expected provider 'claude', got %q", cfg.Provider)
	}
	if cfg.Args != "" {
		t.Errorf("expected empty args, got %q", cfg.Args)
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

func TestParseConfigWithArgs(t *testing.T) {
	tmp := t.TempDir()
	content := `provider: claude
args: --permission-mode auto
tools: []
`
	os.WriteFile(filepath.Join(tmp, "aide.yaml"), []byte(content), 0644)

	cfg, err := FindAndParse(tmp)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg.Provider != "claude" {
		t.Errorf("expected provider 'claude', got %q", cfg.Provider)
	}
	if cfg.Args != "--permission-mode auto" {
		t.Errorf("expected args '--permission-mode auto', got %q", cfg.Args)
	}
}

func TestWalkUpFindsConfig(t *testing.T) {
	tmp := t.TempDir()
	sub := filepath.Join(tmp, "a", "b", "c")
	os.MkdirAll(sub, 0755)
	content := `provider: codex
tools: []
`
	os.WriteFile(filepath.Join(tmp, "aide.yaml"), []byte(content), 0644)

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

func TestParseConfigWithAccount(t *testing.T) {
	tmp := t.TempDir()
	content := `provider: claude
account: company-x
tools: []
`
	os.WriteFile(filepath.Join(tmp, "aide.yaml"), []byte(content), 0644)

	cfg, err := FindAndParse(tmp)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg.Account != "company-x" {
		t.Errorf("expected account 'company-x', got %q", cfg.Account)
	}
}

func TestParseConfigWithoutAccount(t *testing.T) {
	tmp := t.TempDir()
	content := `provider: claude
tools: []
`
	os.WriteFile(filepath.Join(tmp, "aide.yaml"), []byte(content), 0644)

	cfg, err := FindAndParse(tmp)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg.Account != "" {
		t.Errorf("expected empty account, got %q", cfg.Account)
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

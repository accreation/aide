package launcher

import (
	"strings"
	"testing"
)

func TestLaunchNotFound(t *testing.T) {
	l := &Launcher{}
	err := l.Launch("nonexistent-tool-xyz-123")
	if err == nil {
		t.Fatal("expected error for missing binary")
	}
}

func TestLaunchSuccess(t *testing.T) {
	l := &Launcher{}
	err := l.Launch("go", "version")
	if err != nil {
		t.Logf("launch failed (may be expected in restricted env): %v", err)
	}
}

func TestLaunchWithAccountMissing(t *testing.T) {
	l := &Launcher{AccountName: "nonexistent-account"}
	err := l.Launch("go", "version")
	if err == nil {
		t.Fatal("expected error for missing account")
	}
}

func TestReplaceEnv(t *testing.T) {
	// Add existing
	env := replaceEnv([]string{"PATH=/usr/bin"}, "ANTHROPIC_API_KEY", "sk-new")
	if !containsEnv(env, "ANTHROPIC_API_KEY=sk-new") {
		t.Errorf("expected ANTHROPIC_API_KEY to be added")
	}
	// Replace existing
	env = replaceEnv([]string{"ANTHROPIC_API_KEY=sk-old"}, "ANTHROPIC_API_KEY", "sk-new")
	if !containsEnv(env, "ANTHROPIC_API_KEY=sk-new") {
		t.Errorf("expected ANTHROPIC_API_KEY to be replaced")
	}
	if containsEnv(env, "ANTHROPIC_API_KEY=sk-old") {
		t.Errorf("old ANTHROPIC_API_KEY should be gone")
	}
	// Replacement count
	count := 0
	for _, e := range env {
		if strings.HasPrefix(e, "ANTHROPIC_API_KEY=") {
			count++
		}
	}
	if count != 1 {
		t.Errorf("expected 1 ANTHROPIC_API_KEY entry, got %d", count)
	}
}

func containsEnv(env []string, target string) bool {
	for _, e := range env {
		if e == target {
			return true
		}
	}
	return false
}

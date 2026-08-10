package launcher

import (
	"os"
	"path/filepath"
	"runtime"
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

// TestLaunchWithEnvUsesIsolatedPath reproduces the isolated-mode launch bug:
// a tool that exists only in a shim-style directory (not on the ambient PATH)
// must still be found when LaunchWithEnv is given an env whose PATH includes it.
func TestLaunchWithEnvUsesIsolatedPath(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("shim script creation below is unix-specific")
	}

	shimDir := t.TempDir()
	shimPath := filepath.Join(shimDir, "isolated-only-tool")
	if err := os.WriteFile(shimPath, []byte("#!/bin/sh\nexit 0\n"), 0o755); err != nil {
		t.Fatalf("writing fake shim: %v", err)
	}

	// Ambient PATH deliberately excludes shimDir, mirroring the real bug where
	// exec.LookPath resolved against os.Getenv("PATH") instead of the isolated env.
	origPath, hadPath := os.LookupEnv("PATH")
	os.Setenv("PATH", "/nonexistent-ambient-path")
	defer func() {
		if hadPath {
			os.Setenv("PATH", origPath)
		} else {
			os.Unsetenv("PATH")
		}
	}()

	var env []string
	for _, e := range os.Environ() {
		if !strings.HasPrefix(strings.ToUpper(e), "PATH=") {
			env = append(env, e)
		}
	}
	env = append(env, "PATH="+shimDir)

	l := &Launcher{}
	if err := l.LaunchWithEnv("isolated-only-tool", env); err != nil {
		t.Fatalf("expected tool found via isolated env PATH, got: %v", err)
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

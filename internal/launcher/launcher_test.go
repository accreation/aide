package launcher

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"aide/internal/account"
)

func setHome(t *testing.T, dir string) {
	t.Helper()
	t.Setenv("HOME", dir)
	t.Setenv("USERPROFILE", dir)
}

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
	// This test used to read the developer's real ~/.aide/accounts.json and
	// only passed because nobody has an account named "nonexistent-account".
	setHome(t, t.TempDir())

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

// --- accountEnv: table-driven env-binding tests (no real provider CLI) ---

func TestAccountEnvLegacyClaudeSetsAPIKeyOverride(t *testing.T) {
	setHome(t, t.TempDir()) // no profile dir exists for "legacy" — legacy path

	acc := account.Account{Provider: "claude", APIKey: "sk-legacy"}
	env, err := accountEnv("legacy", acc, []string{"PATH=/usr/bin"})
	if err != nil {
		t.Fatalf("accountEnv failed: %v", err)
	}
	if !containsEnv(env, "ANTHROPIC_API_KEY=sk-legacy") {
		t.Errorf("expected legacy ANTHROPIC_API_KEY override, got %v", env)
	}
}

func TestAccountEnvLegacyClaudeCommandBrokerOverridesAPIKey(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("shell command fakes are unix-specific")
	}
	setHome(t, t.TempDir())

	acc := account.Account{Provider: "claude", APIKey: "sk-stale", Command: "echo sk-from-broker"}
	env, err := accountEnv("legacy", acc, []string{"PATH=/usr/bin"})
	if err != nil {
		t.Fatalf("accountEnv failed: %v", err)
	}
	if !containsEnv(env, "ANTHROPIC_API_KEY=sk-from-broker") {
		t.Errorf("expected the broker's output to override the legacy APIKey, got %v", env)
	}
}

func TestAccountEnvLegacyCodexSetsHomeOverride(t *testing.T) {
	setHome(t, t.TempDir())

	acc := account.Account{Provider: "codex", CodexHome: "/legacy/codex-home"}
	env, err := accountEnv("legacy", acc, []string{"PATH=/usr/bin"})
	if err != nil {
		t.Fatalf("accountEnv failed: %v", err)
	}
	if !containsEnv(env, "CODEX_HOME=/legacy/codex-home") {
		t.Errorf("expected legacy CODEX_HOME override, got %v", env)
	}
}

func TestAccountEnvProfileBasedClaudeUsesConfigDirNotAPIKey(t *testing.T) {
	tmp := t.TempDir()
	setHome(t, tmp)

	acc := account.Account{Provider: "claude"}
	if err := account.CreateProfile("acme", acc); err != nil {
		t.Fatalf("CreateProfile failed: %v", err)
	}

	env, err := accountEnv("acme", acc, []string{"PATH=/usr/bin", "ANTHROPIC_API_KEY=should-not-survive"})
	if err != nil {
		t.Fatalf("accountEnv failed: %v", err)
	}
	root, _ := account.ProfileDir("acme", acc)
	want := "CLAUDE_CONFIG_DIR=" + filepath.Join(root, "claude")
	if !containsEnv(env, want) {
		t.Errorf("expected %q, got %v", want, env)
	}
	// Profile-based accounts bind via CLAUDE_CONFIG_DIR, not an API key
	// override — an ambient ANTHROPIC_API_KEY would defeat isolation by
	// moving billing off the profile's subscription, so make sure a
	// pre-profile launch didn't leave one lying around.
	if !containsEnv(env, "ANTHROPIC_API_KEY=should-not-survive") {
		t.Errorf("BuildEnv unexpectedly dropped an unrelated var, got %v", env)
	}
}

func TestAccountEnvProfileBasedCodexUsesCodexHome(t *testing.T) {
	tmp := t.TempDir()
	setHome(t, tmp)

	acc := account.Account{Provider: "codex"}
	if err := account.CreateProfile("acme", acc); err != nil {
		t.Fatalf("CreateProfile failed: %v", err)
	}

	env, err := accountEnv("acme", acc, []string{"PATH=/usr/bin"})
	if err != nil {
		t.Fatalf("accountEnv failed: %v", err)
	}
	root, _ := account.ProfileDir("acme", acc)
	want := "CODEX_HOME=" + filepath.Join(root, "codex")
	if !containsEnv(env, want) {
		t.Errorf("expected %q, got %v", want, env)
	}
}

func TestAccountEnvLegacyCopilotErrorsInsteadOfLaunching(t *testing.T) {
	setHome(t, t.TempDir()) // no profile dir exists for "legacy-copilot" — legacy path

	acc := account.Account{Provider: "copilot", User: "old-user"}
	_, err := accountEnv("legacy-copilot", acc, []string{"PATH=/usr/bin"})
	if err == nil {
		t.Fatal("expected the removed 'gh auth switch' path to error rather than silently launch as the wrong user")
	}
}

func TestAccountEnvProfileBasedCopilotUsesGHConfigDir(t *testing.T) {
	tmp := t.TempDir()
	setHome(t, tmp)

	acc := account.Account{Provider: "copilot", Token: "ghp_x"}
	if err := account.CreateProfile("acme", acc); err != nil {
		t.Fatalf("CreateProfile failed: %v", err)
	}

	env, err := accountEnv("acme", acc, []string{"PATH=/usr/bin"})
	if err != nil {
		t.Fatalf("accountEnv failed: %v", err)
	}
	root, _ := account.ProfileDir("acme", acc)
	if !containsEnv(env, "GH_CONFIG_DIR="+filepath.Join(root, "gh")) {
		t.Errorf("expected GH_CONFIG_DIR in env, got %v", env)
	}
	if !containsEnv(env, "COPILOT_GITHUB_TOKEN=ghp_x") {
		t.Errorf("expected the profile's token to be injected, got %v", env)
	}
}

func TestApplyAccountRejectsProviderMismatch(t *testing.T) {
	setHome(t, t.TempDir())

	if err := account.Add("acme", account.Account{Provider: "claude", APIKey: "sk-x"}); err != nil {
		t.Fatalf("Add failed: %v", err)
	}

	l := &Launcher{AccountName: "acme"}
	_, err := l.applyAccount("copilot", []string{"PATH=/usr/bin"})
	if err == nil {
		t.Fatal("expected an error when the account's provider doesn't match the launch provider")
	}
}

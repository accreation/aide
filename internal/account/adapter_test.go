package account

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func TestClaudeAdapterEnvIsAbsoluteAndScrubsSecureStorage(t *testing.T) {
	root := t.TempDir()
	adapter := Adapters["claude"]

	base := []string{"CLAUDE_SECURESTORAGE_CONFIG_DIR=", "SOMETHING=else"}
	env, err := BuildEnv(adapter, root, Account{Provider: "claude"}, base)
	if err != nil {
		t.Fatalf("BuildEnv failed: %v", err)
	}

	want := "CLAUDE_CONFIG_DIR=" + filepath.Join(root, "claude")
	if !containsEnv(env, want) {
		t.Errorf("expected %q in env, got %v", want, env)
	}
	for _, e := range env {
		if strings.HasPrefix(e, "CLAUDE_SECURESTORAGE_CONFIG_DIR=") {
			t.Errorf("expected CLAUDE_SECURESTORAGE_CONFIG_DIR to be scrubbed, got %v", env)
		}
	}
	if !containsEnv(env, "SOMETHING=else") {
		t.Errorf("expected unrelated env to survive, got %v", env)
	}
}

func TestCodexAdapterScrubsIsolationDefeatingVars(t *testing.T) {
	root := t.TempDir()
	adapter := Adapters["codex"]

	base := []string{"CODEX_API_KEY=leaked", "CODEX_ACCESS_TOKEN=leaked", "CODEX_SQLITE_HOME=/shared", "PATH=/usr/bin"}
	env, err := BuildEnv(adapter, root, Account{Provider: "codex"}, base)
	if err != nil {
		t.Fatalf("BuildEnv failed: %v", err)
	}

	for _, key := range []string{"CODEX_API_KEY", "CODEX_ACCESS_TOKEN", "CODEX_SQLITE_HOME"} {
		for _, e := range env {
			if strings.HasPrefix(e, key+"=") {
				t.Errorf("expected %s to be scrubbed, got %v", key, env)
			}
		}
	}
	want := "CODEX_HOME=" + filepath.Join(root, "codex")
	if !containsEnv(env, want) {
		t.Errorf("expected %q in env, got %v", want, env)
	}
	if !containsEnv(env, "PATH=/usr/bin") {
		t.Errorf("expected unrelated env to survive, got %v", env)
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

func TestScrubEnvOmitsRatherThanBlanks(t *testing.T) {
	env := scrubEnv([]string{"A=1", "B=2", "C=3"}, []string{"B"})
	if len(env) != 2 {
		t.Fatalf("expected 2 entries after scrub, got %v", env)
	}
	for _, e := range env {
		if strings.HasPrefix(e, "B=") {
			t.Errorf("expected B to be fully omitted, got %v", env)
		}
	}
}

func TestMergeEnvKVReplacesExisting(t *testing.T) {
	env := mergeEnvKV([]string{"A=old"}, []string{"A=new", "B=added"})
	if !containsEnv(env, "A=new") || containsEnv(env, "A=old") {
		t.Errorf("expected A to be replaced, got %v", env)
	}
	if !containsEnv(env, "B=added") {
		t.Errorf("expected B to be added, got %v", env)
	}
}

// fakeExecScript writes a shell script that dumps a marker plus its own
// environment to stdout, and returns an execCommand-compatible func that
// always runs it regardless of the requested argv[0]. This lets identity
// tests assert exactly what a real login/status subcommand would receive,
// without depending on a real provider CLI being installed.
func fakeExecScript(t *testing.T, body string) func(name string, arg ...string) *exec.Cmd {
	t.Helper()
	if runtime.GOOS == "windows" {
		t.Skip("shell script fakes are unix-specific")
	}
	dir := t.TempDir()
	script := filepath.Join(dir, "fake")
	if err := os.WriteFile(script, []byte("#!/bin/sh\n"+body), 0o755); err != nil {
		t.Fatalf("writing fake script: %v", err)
	}
	return func(name string, arg ...string) *exec.Cmd {
		return exec.Command(script, arg...)
	}
}

func TestRunIdentityCheckLoggedIn(t *testing.T) {
	orig := execCommand
	defer func() { execCommand = orig }()
	execCommand = fakeExecScript(t, `echo '{"loggedIn":true,"email":"anar@acme.example","orgId":"acme"}'`)

	id, err := runIdentityCheck(nil, []string{"claude", "auth", "status", "--json"}, parseClaudeIdentity)
	if err != nil {
		t.Fatalf("runIdentityCheck failed: %v", err)
	}
	if !id.LoggedIn {
		t.Fatal("expected LoggedIn = true")
	}
	if id.Label != "anar@acme.example (acme)" {
		t.Errorf("unexpected label: %q", id.Label)
	}
}

func TestRunIdentityCheckNotLoggedInOnNonZeroExit(t *testing.T) {
	orig := execCommand
	defer func() { execCommand = orig }()
	execCommand = fakeExecScript(t, `exit 1`)

	id, err := runIdentityCheck(nil, []string{"codex", "login", "status"}, nil)
	if err != nil {
		t.Fatalf("expected a failed status command to report not-logged-in, not an error: %v", err)
	}
	if id.LoggedIn {
		t.Error("expected LoggedIn = false")
	}
}

func TestRunIdentityCheckMissingBinaryIsNotLoggedIn(t *testing.T) {
	orig := execCommand
	defer func() { execCommand = orig }()
	execCommand = exec.Command // real exec, but the binary name below doesn't exist

	id, err := runIdentityCheck(nil, []string{"definitely-not-a-real-binary-xyz"}, nil)
	if err != nil {
		t.Fatalf("expected a missing binary to report not-logged-in, not an error: %v", err)
	}
	if id.LoggedIn {
		t.Error("expected LoggedIn = false")
	}
}

func TestRunIdentityCheckReceivesGivenEnv(t *testing.T) {
	orig := execCommand
	defer func() { execCommand = orig }()
	execCommand = fakeExecScript(t, `printenv MARKER`)

	id, err := runIdentityCheck([]string{"MARKER=hello"}, []string{"codex", "login", "status"}, nil)
	if err != nil {
		t.Fatalf("runIdentityCheck failed: %v", err)
	}
	if id.Label != "hello" {
		t.Errorf("expected the fake script to see MARKER=hello via cmd.Env, got label %q", id.Label)
	}
}

func TestParseClaudeIdentityFallsBackWithoutEmail(t *testing.T) {
	if got := parseClaudeIdentity([]byte(`{"loggedIn":true}`)); got != "" {
		t.Errorf("expected empty label when no email present, got %q", got)
	}
	if got := parseClaudeIdentity([]byte("not json")); got != "" {
		t.Errorf("expected empty label on unparseable output, got %q", got)
	}
}

func TestCreateProfileIsIdempotent(t *testing.T) {
	tmp := t.TempDir()
	setHome(t, tmp)

	acc := Account{Provider: "codex"}
	if err := CreateProfile("acme", acc); err != nil {
		t.Fatalf("first CreateProfile failed: %v", err)
	}
	root, _ := ProfileDir("acme", acc)
	seedPath := filepath.Join(root, "codex", "config.toml")
	custom := []byte("cli_auth_credentials_store = \"keyring\"\n# hand-edited\n")
	if err := os.WriteFile(seedPath, custom, 0600); err != nil {
		t.Fatalf("seeding custom config: %v", err)
	}

	if err := CreateProfile("acme", acc); err != nil {
		t.Fatalf("second CreateProfile failed: %v", err)
	}

	got, err := os.ReadFile(seedPath)
	if err != nil {
		t.Fatalf("reading config: %v", err)
	}
	if string(got) != string(custom) {
		t.Errorf("expected repeated CreateProfile to never overwrite an existing config.toml, got: %s", got)
	}
}

func init() {
	// Guard against a typo turning any adapter test into a no-op: every
	// adapter this package ships must have every required field set.
	for name, a := range Adapters {
		if a.Env == nil || a.Identity == nil || a.LoginArgv == nil {
			panic(fmt.Sprintf("adapter %q is missing a required field", name))
		}
	}
}

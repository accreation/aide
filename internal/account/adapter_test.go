package account

import (
	"fmt"
	"net/http"
	"net/http/httptest"
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

func TestClaudeAdapterDisablesAutoUpdater(t *testing.T) {
	root := t.TempDir()
	adapter := Adapters["claude"]

	env, err := BuildEnv(adapter, root, Account{Provider: "claude"}, nil)
	if err != nil {
		t.Fatalf("BuildEnv failed: %v", err)
	}

	if !containsEnv(env, "DISABLE_AUTOUPDATER=1") {
		t.Errorf("expected DISABLE_AUTOUPDATER=1 in env (profiles must not race the global auto-updater install), got %v", env)
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

// --- copilot adapter ---

func TestCopilotAdapterEnvScrubsAmbientTokensWhenNoneConfigured(t *testing.T) {
	root := t.TempDir()
	adapter := Adapters["copilot"]

	base := []string{"COPILOT_GITHUB_TOKEN=leaked", "GH_TOKEN=leaked", "GITHUB_TOKEN=leaked", "PATH=/usr/bin"}
	env, err := BuildEnv(adapter, root, Account{Provider: "copilot"}, base)
	if err != nil {
		t.Fatalf("BuildEnv failed: %v", err)
	}

	for _, key := range []string{"COPILOT_GITHUB_TOKEN", "GH_TOKEN", "GITHUB_TOKEN"} {
		for _, e := range env {
			if strings.HasPrefix(e, key+"=") {
				t.Errorf("expected %s to be scrubbed with no acc.Token set, got %v", key, env)
			}
		}
	}
	if !containsEnv(env, "COPILOT_HOME="+filepath.Join(root, "copilot")) {
		t.Errorf("expected COPILOT_HOME in env, got %v", env)
	}
	if !containsEnv(env, "GH_CONFIG_DIR="+filepath.Join(root, "gh")) {
		t.Errorf("expected GH_CONFIG_DIR in env, got %v", env)
	}
	if !containsEnv(env, "COPILOT_AUTO_UPDATE=false") {
		t.Errorf("expected COPILOT_AUTO_UPDATE=false in env, got %v", env)
	}
	if !containsEnv(env, "PATH=/usr/bin") {
		t.Errorf("expected unrelated env to survive, got %v", env)
	}
}

func TestCopilotAdapterEnvSetsTokenWhenProvided(t *testing.T) {
	root := t.TempDir()
	adapter := Adapters["copilot"]

	env, err := BuildEnv(adapter, root, Account{Provider: "copilot", Token: "ghp_example"}, nil)
	if err != nil {
		t.Fatalf("BuildEnv failed: %v", err)
	}
	if !containsEnv(env, "COPILOT_GITHUB_TOKEN=ghp_example") {
		t.Errorf("expected acc.Token to become COPILOT_GITHUB_TOKEN, got %v", env)
	}
}

func TestCopilotAdapterEnvUsesCommandBrokerOverToken(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("shell command fakes are unix-specific")
	}
	root := t.TempDir()
	adapter := Adapters["copilot"]

	env, err := BuildEnv(adapter, root, Account{Provider: "copilot", Token: "stale", Command: "echo from-broker"}, nil)
	if err != nil {
		t.Fatalf("BuildEnv failed: %v", err)
	}
	if !containsEnv(env, "COPILOT_GITHUB_TOKEN=from-broker") {
		t.Errorf("expected the broker's output to become COPILOT_GITHUB_TOKEN, got %v", env)
	}
}

func withCopilotIdentityServer(t *testing.T, handler http.HandlerFunc) {
	t.Helper()
	server := httptest.NewServer(handler)
	t.Cleanup(server.Close)

	origClient, origURL := copilotHTTPClient, copilotUserURL
	copilotHTTPClient = server.Client()
	copilotUserURL = server.URL
	t.Cleanup(func() {
		copilotHTTPClient = origClient
		copilotUserURL = origURL
	})
}

func TestCopilotIdentityUsesTokenFromEnv(t *testing.T) {
	withCopilotIdentityServer(t, func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "Bearer sometoken" {
			t.Errorf("expected the env token in the Authorization header, got %q", r.Header.Get("Authorization"))
		}
		fmt.Fprint(w, `{"login":"octocat"}`)
	})

	id, err := copilotIdentity([]string{"COPILOT_GITHUB_TOKEN=sometoken"})
	if err != nil {
		t.Fatalf("copilotIdentity failed: %v", err)
	}
	if !id.LoggedIn || id.Label != "octocat" {
		t.Errorf("expected logged in as octocat, got %+v", id)
	}
}

func TestCopilotIdentityFallsBackToGhAuthToken(t *testing.T) {
	orig := execCommand
	defer func() { execCommand = orig }()
	execCommand = fakeExecScript(t, `echo fallbacktoken`)

	withCopilotIdentityServer(t, func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "Bearer fallbacktoken" {
			t.Errorf("expected the `gh auth token` fallback in the Authorization header, got %q", r.Header.Get("Authorization"))
		}
		fmt.Fprint(w, `{"login":"hubot"}`)
	})

	id, err := copilotIdentity(nil)
	if err != nil {
		t.Fatalf("copilotIdentity failed: %v", err)
	}
	if !id.LoggedIn || id.Label != "hubot" {
		t.Errorf("expected logged in as hubot via gh auth token fallback, got %+v", id)
	}
}

func TestCopilotIdentityNotLoggedInWhenNoTokenAnywhere(t *testing.T) {
	orig := execCommand
	defer func() { execCommand = orig }()
	execCommand = fakeExecScript(t, `exit 1`)

	id, err := copilotIdentity(nil)
	if err != nil {
		t.Fatalf("copilotIdentity failed: %v", err)
	}
	if id.LoggedIn {
		t.Error("expected LoggedIn = false when neither COPILOT_GITHUB_TOKEN nor `gh auth token` yield a token")
	}
}

func TestCopilotIdentityNotLoggedInOnUnauthorized(t *testing.T) {
	withCopilotIdentityServer(t, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
	})

	id, err := copilotIdentity([]string{"COPILOT_GITHUB_TOKEN=badtoken"})
	if err != nil {
		t.Fatalf("copilotIdentity failed: %v", err)
	}
	if id.LoggedIn {
		t.Error("expected LoggedIn = false on a 401 from GET /user")
	}
}

// --- opencode adapter ---

func TestOpencodeAdapterEnvSetsXDGDataHomeAndScrubsAuthContent(t *testing.T) {
	root := t.TempDir()
	adapter := Adapters["opencode"]

	base := []string{"OPENCODE_AUTH_CONTENT=leaked", "PATH=/usr/bin"}
	env, err := BuildEnv(adapter, root, Account{Provider: "opencode"}, base)
	if err != nil {
		t.Fatalf("BuildEnv failed: %v", err)
	}

	want := "XDG_DATA_HOME=" + filepath.Join(root, "opencode")
	if !containsEnv(env, want) {
		t.Errorf("expected %q in env, got %v", want, env)
	}
	for _, e := range env {
		if strings.HasPrefix(e, "OPENCODE_AUTH_CONTENT=") {
			t.Errorf("expected OPENCODE_AUTH_CONTENT to be scrubbed, got %v", env)
		}
	}
	if !containsEnv(env, "PATH=/usr/bin") {
		t.Errorf("expected unrelated env to survive, got %v", env)
	}
}

func TestOpencodeIdentityNotLoggedInWithoutAuthFile(t *testing.T) {
	dataHome := t.TempDir()
	id, err := opencodeIdentity([]string{"XDG_DATA_HOME=" + dataHome})
	if err != nil {
		t.Fatalf("opencodeIdentity failed: %v", err)
	}
	if id.LoggedIn {
		t.Error("expected LoggedIn = false when auth.json does not exist")
	}
}

func TestOpencodeIdentityLoggedInWhenAuthFileExistsEvenIfCLIMissing(t *testing.T) {
	orig := execCommand
	defer func() { execCommand = orig }()
	execCommand = exec.Command // real exec, but "opencode" below need not exist

	dataHome := t.TempDir()
	if err := os.MkdirAll(filepath.Join(dataHome, "opencode"), 0700); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(dataHome, "opencode", "auth.json"), []byte("{}"), 0600); err != nil {
		t.Fatalf("writing auth.json: %v", err)
	}

	id, err := opencodeIdentity([]string{"XDG_DATA_HOME=" + dataHome})
	if err != nil {
		t.Fatalf("opencodeIdentity failed: %v", err)
	}
	if !id.LoggedIn {
		t.Error("expected auth.json's existence alone to report LoggedIn = true")
	}
}

func TestOpencodeIdentityUsesAuthListLabelWhenAvailable(t *testing.T) {
	orig := execCommand
	defer func() { execCommand = orig }()
	execCommand = fakeExecScript(t, `echo "github  connected"`)

	dataHome := t.TempDir()
	if err := os.MkdirAll(filepath.Join(dataHome, "opencode"), 0700); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(dataHome, "opencode", "auth.json"), []byte("{}"), 0600); err != nil {
		t.Fatalf("writing auth.json: %v", err)
	}

	id, err := opencodeIdentity([]string{"XDG_DATA_HOME=" + dataHome})
	if err != nil {
		t.Fatalf("opencodeIdentity failed: %v", err)
	}
	if id.Label != "github  connected" {
		t.Errorf("expected the `opencode auth list` output as label, got %q", id.Label)
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

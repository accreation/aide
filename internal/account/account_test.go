package account

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"testing"
)

func setHome(t *testing.T, dir string) {
	t.Helper()
	t.Setenv("HOME", dir)
	t.Setenv("USERPROFILE", dir)
}

func TestAddAndGet(t *testing.T) {
	tmp := t.TempDir()
	setHome(t, tmp)

	err := Add("company-x", Account{
		Provider: "copilot",
		User:     "company-x-gh",
	})
	if err != nil {
		t.Fatalf("Add failed: %v", err)
	}

	a, err := Get("company-x")
	if err != nil {
		t.Fatalf("Get failed: %v", err)
	}
	if a.Provider != "copilot" {
		t.Errorf("expected provider 'copilot', got %q", a.Provider)
	}
	if a.User != "company-x-gh" {
		t.Errorf("expected user 'company-x-gh', got %q", a.User)
	}
}

func TestValidateProviderFieldsAcceptsAllFourProvidersWithNoFields(t *testing.T) {
	for _, provider := range []string{"copilot", "claude", "codex", "opencode"} {
		if err := ValidateProviderFields(Account{Provider: provider}); err != nil {
			t.Errorf("expected %q with no fields to be valid (profile-based), got: %v", provider, err)
		}
	}
}

func TestValidateProviderFieldsRejectsUnknownProvider(t *testing.T) {
	if err := ValidateProviderFields(Account{Provider: "no-such-provider"}); err == nil {
		t.Error("expected an error for an unknown provider")
	}
}

func TestAddRejectsInvalidName(t *testing.T) {
	tmp := t.TempDir()
	setHome(t, tmp)

	for _, name := range []string{"", ".", "..", "a/b", `a\b`, "a:b", "trailing.", " leading", "trailing ", "con", "COM1", "com1.txt"} {
		if err := Add(name, Account{Provider: "claude"}); err == nil {
			t.Errorf("Add(%q) expected error, got nil", name)
		}
	}
}

func TestGetNotFound(t *testing.T) {
	tmp := t.TempDir()
	setHome(t, tmp)

	_, err := Get("nonexistent")
	if err == nil {
		t.Fatal("expected error for missing account")
	}
}

func TestRemove(t *testing.T) {
	tmp := t.TempDir()
	setHome(t, tmp)

	Add("temp", Account{Provider: "claude", APIKey: "sk-test"})
	err := Remove("temp", RemoveOptions{})
	if err != nil {
		t.Fatalf("Remove failed: %v", err)
	}
	_, err = Get("temp")
	if err == nil {
		t.Fatal("expected error after remove")
	}
}

func TestRemoveRefusesToDeleteProfileWithoutForceOrKeep(t *testing.T) {
	tmp := t.TempDir()
	setHome(t, tmp)

	Add("acme", Account{Provider: "claude"})
	if err := CreateProfile("acme", Account{Provider: "claude"}); err != nil {
		t.Fatalf("CreateProfile failed: %v", err)
	}
	dir, err := ProfileDir("acme", Account{Provider: "claude"})
	if err != nil {
		t.Fatalf("ProfileDir failed: %v", err)
	}

	if err := Remove("acme", RemoveOptions{}); err == nil {
		t.Fatal("expected Remove to refuse deleting an existing profile without --force/--keep-credentials")
	}

	// The refusal must not have partially removed anything: the index entry
	// and the profile directory should both still be there.
	if _, err := Get("acme"); err != nil {
		t.Errorf("expected accounts.json entry to survive a refused Remove, got: %v", err)
	}
	if _, err := os.Stat(dir); err != nil {
		t.Errorf("expected profile dir to survive a refused Remove, got: %v", err)
	}
}

func TestRemoveKeepCredentialsLeavesProfileOnDisk(t *testing.T) {
	tmp := t.TempDir()
	setHome(t, tmp)

	Add("acme", Account{Provider: "claude"})
	if err := CreateProfile("acme", Account{Provider: "claude"}); err != nil {
		t.Fatalf("CreateProfile failed: %v", err)
	}
	dir, err := ProfileDir("acme", Account{Provider: "claude"})
	if err != nil {
		t.Fatalf("ProfileDir failed: %v", err)
	}

	if err := Remove("acme", RemoveOptions{KeepCredentials: true}); err != nil {
		t.Fatalf("Remove failed: %v", err)
	}
	if _, err := Get("acme"); err == nil {
		t.Error("expected accounts.json entry to be gone")
	}
	if _, err := os.Stat(dir); err != nil {
		t.Errorf("expected profile dir to survive --keep-credentials, got: %v", err)
	}
}

func TestRemoveForceDeletesProfile(t *testing.T) {
	tmp := t.TempDir()
	setHome(t, tmp)

	Add("acme", Account{Provider: "claude"})
	if err := CreateProfile("acme", Account{Provider: "claude"}); err != nil {
		t.Fatalf("CreateProfile failed: %v", err)
	}
	dir, err := ProfileDir("acme", Account{Provider: "claude"})
	if err != nil {
		t.Fatalf("ProfileDir failed: %v", err)
	}

	if err := Remove("acme", RemoveOptions{Force: true}); err != nil {
		t.Fatalf("Remove failed: %v", err)
	}
	if _, err := os.Stat(dir); !os.IsNotExist(err) {
		t.Errorf("expected profile dir to be deleted, stat returned: %v", err)
	}
}

func TestRemoveNeverDeletesAdoptedDir(t *testing.T) {
	tmp := t.TempDir()
	setHome(t, tmp)

	adopted := filepath.Join(tmp, "my-existing-codex-home")
	if err := os.MkdirAll(adopted, 0700); err != nil {
		t.Fatalf("seeding adopted dir: %v", err)
	}

	Add("existing", Account{Provider: "codex", Dir: adopted})
	if err := Remove("existing", RemoveOptions{Force: true}); err != nil {
		t.Fatalf("Remove failed: %v", err)
	}
	if _, err := os.Stat(adopted); err != nil {
		t.Errorf("expected adopted dir to survive Remove even with --force, got: %v", err)
	}
}

func TestList(t *testing.T) {
	tmp := t.TempDir()
	setHome(t, tmp)

	Add("a", Account{Provider: "copilot", User: "a"})
	Add("b", Account{Provider: "claude", APIKey: "sk-b"})

	accounts, err := List()
	if err != nil {
		t.Fatalf("List failed: %v", err)
	}
	if len(accounts) != 2 {
		t.Fatalf("expected 2 accounts, got %d", len(accounts))
	}
}

// TestAddAPIKeyStoredInFile documents (rather than assumes) that legacy API
// keys are stored in plaintext in accounts.json, protected only by 0600
// file permissions — there is no keyring dependency (see CLAUDE.md's
// three-dependency rule). A previous version of this test only asserted
// the file was non-empty, which passed regardless of what was in it.
func TestAddAPIKeyStoredInFile(t *testing.T) {
	tmp := t.TempDir()
	setHome(t, tmp)

	Add("test", Account{Provider: "claude", APIKey: "sk-ant-secret123"})

	home, _ := os.UserHomeDir()
	data, err := os.ReadFile(filepath.Join(home, ".aide", "accounts.json"))
	if err != nil {
		t.Fatalf("reading accounts.json: %v", err)
	}
	if !strings.Contains(string(data), "sk-ant-secret123") {
		t.Fatalf("expected api_key to be stored in accounts.json, got: %s", data)
	}
}

func TestAccountsFilePermissionsReenforcedOnWrite(t *testing.T) {
	tmp := t.TempDir()
	setHome(t, tmp)

	if err := Add("a", Account{Provider: "claude", APIKey: "sk-a"}); err != nil {
		t.Fatalf("Add failed: %v", err)
	}

	p := filepath.Join(tmp, ".aide", "accounts.json")
	if err := os.Chmod(p, 0644); err != nil {
		t.Fatalf("chmod failed: %v", err)
	}

	if err := Add("b", Account{Provider: "claude", APIKey: "sk-b"}); err != nil {
		t.Fatalf("second Add failed: %v", err)
	}

	info, err := os.Stat(p)
	if err != nil {
		t.Fatalf("stat failed: %v", err)
	}
	if info.Mode().Perm() != 0600 {
		t.Errorf("expected accounts.json to be re-tightened to 0600, got %o", info.Mode().Perm())
	}
}

func TestConcurrentAddDoesNotLoseUpdates(t *testing.T) {
	tmp := t.TempDir()
	setHome(t, tmp)

	const n = 20
	var wg sync.WaitGroup
	for i := 0; i < n; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			name := fmt.Sprintf("acct-%d", i)
			if err := Add(name, Account{Provider: "claude", APIKey: fmt.Sprintf("sk-%d", i)}); err != nil {
				t.Errorf("Add(%s) failed: %v", name, err)
			}
		}(i)
	}
	wg.Wait()

	accounts, err := List()
	if err != nil {
		t.Fatalf("List failed: %v", err)
	}
	if len(accounts) != n {
		t.Errorf("expected %d accounts after concurrent adds, got %d (lost updates)", n, len(accounts))
	}
}

func TestProfileDirUsesExplicitDir(t *testing.T) {
	tmp := t.TempDir()
	setHome(t, tmp)

	dir, err := ProfileDir("whatever", Account{Provider: "codex", Dir: filepath.Join(tmp, "custom")})
	if err != nil {
		t.Fatalf("ProfileDir failed: %v", err)
	}
	want, _ := filepath.Abs(filepath.Join(tmp, "custom"))
	if dir != want {
		t.Errorf("expected %q, got %q", want, dir)
	}
}

func TestProfileDirDefaultsUnderAccountsDir(t *testing.T) {
	tmp := t.TempDir()
	setHome(t, tmp)

	dir, err := ProfileDir("acme", Account{Provider: "claude"})
	if err != nil {
		t.Fatalf("ProfileDir failed: %v", err)
	}
	want := filepath.Join(tmp, ".aide", "accounts", "acme")
	if dir != want {
		t.Errorf("expected %q, got %q", want, dir)
	}
}

func TestIsProfileBasedFalseUntilCreated(t *testing.T) {
	tmp := t.TempDir()
	setHome(t, tmp)

	acc := Account{Provider: "claude"}
	if IsProfileBased("acme", acc) {
		t.Error("expected IsProfileBased to be false before CreateProfile")
	}
	if err := CreateProfile("acme", acc); err != nil {
		t.Fatalf("CreateProfile failed: %v", err)
	}
	if !IsProfileBased("acme", acc) {
		t.Error("expected IsProfileBased to be true after CreateProfile")
	}
}

func TestLegacyAccountIsNotProfileBased(t *testing.T) {
	tmp := t.TempDir()
	setHome(t, tmp)

	// An account created before credential profiles existed has no
	// directory on disk, so it must keep using its legacy fields.
	acc := Account{Provider: "claude", APIKey: "sk-legacy"}
	Add("legacy", acc)
	if IsProfileBased("legacy", acc) {
		t.Error("expected a pre-existing legacy account with no profile dir to not be profile-based")
	}
}

func TestCreateProfileCreatesAdapterDirs(t *testing.T) {
	tmp := t.TempDir()
	setHome(t, tmp)

	acc := Account{Provider: "codex"}
	if err := CreateProfile("acme", acc); err != nil {
		t.Fatalf("CreateProfile failed: %v", err)
	}
	root, err := ProfileDir("acme", acc)
	if err != nil {
		t.Fatalf("ProfileDir failed: %v", err)
	}
	codexHome := filepath.Join(root, "codex")
	if info, err := os.Stat(codexHome); err != nil || !info.IsDir() {
		t.Fatalf("expected %s to exist as a directory: %v", codexHome, err)
	}
	if runtime.GOOS != "windows" {
		info, _ := os.Stat(codexHome)
		if info.Mode().Perm() != 0700 {
			t.Errorf("expected 0700, got %o", info.Mode().Perm())
		}
	}
	if _, err := os.Stat(filepath.Join(codexHome, "config.toml")); err != nil {
		t.Errorf("expected codex config.toml to be seeded: %v", err)
	}
}

func TestResolveTokenPrefersCommandOverToken(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("shell command fakes are unix-specific")
	}
	acc := Account{Provider: "copilot", Token: "stale-token", Command: "echo from-broker"}
	got, err := ResolveToken(acc)
	if err != nil {
		t.Fatalf("ResolveToken failed: %v", err)
	}
	if got != "from-broker" {
		t.Errorf("expected Command's output to win over Token, got %q", got)
	}
}

func TestResolveTokenFallsBackToTokenWithoutCommand(t *testing.T) {
	acc := Account{Provider: "copilot", Token: "ghp_example"}
	got, err := ResolveToken(acc)
	if err != nil {
		t.Fatalf("ResolveToken failed: %v", err)
	}
	if got != "ghp_example" {
		t.Errorf("expected Token, got %q", got)
	}
}

func TestResolveTokenTrimsCommandOutput(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("shell command fakes are unix-specific")
	}
	acc := Account{Provider: "copilot", Command: "printf '  secret-value\\n\\n'"}
	got, err := ResolveToken(acc)
	if err != nil {
		t.Fatalf("ResolveToken failed: %v", err)
	}
	if got != "secret-value" {
		t.Errorf("expected trimmed output, got %q", got)
	}
}

func TestResolveTokenPropagatesCommandFailure(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("shell command fakes are unix-specific")
	}
	acc := Account{Provider: "copilot", Command: "exit 1"}
	if _, err := ResolveToken(acc); err == nil {
		t.Fatal("expected an error when the broker command fails")
	}
}

func TestResolveAPIKeyPrefersCommandOverAPIKey(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("shell command fakes are unix-specific")
	}
	acc := Account{Provider: "claude", APIKey: "sk-ant-stale", Command: "echo sk-ant-from-broker"}
	got, err := ResolveAPIKey(acc)
	if err != nil {
		t.Fatalf("ResolveAPIKey failed: %v", err)
	}
	if got != "sk-ant-from-broker" {
		t.Errorf("expected Command's output to win over APIKey, got %q", got)
	}
}

func TestResolveAPIKeyFallsBackToAPIKeyWithoutCommand(t *testing.T) {
	acc := Account{Provider: "claude", APIKey: "sk-ant-xxx"}
	got, err := ResolveAPIKey(acc)
	if err != nil {
		t.Fatalf("ResolveAPIKey failed: %v", err)
	}
	if got != "sk-ant-xxx" {
		t.Errorf("expected APIKey, got %q", got)
	}
}

func TestCreateProfileUnsupportedProvider(t *testing.T) {
	tmp := t.TempDir()
	setHome(t, tmp)

	if err := CreateProfile("acme", Account{Provider: "no-such-provider"}); err == nil {
		t.Fatal("expected error for a provider with no adapter")
	}
}

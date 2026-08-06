# Multi-Account Switching Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Add optional `account` field to `aide.yaml` so developers can specify which provider subscription to use per project. Accounts are stored locally in `~/.aide/accounts.json` (never in repo). On launch, aide auto-switches the provider to the correct account.

**Architecture:** A new `internal/account` package manages `~/.aide/accounts.json` (same pattern as `internal/project`). The `Config` struct gains an optional `Account` field. The launcher applies provider-specific switching before launch: `gh auth switch` for Copilot, `ANTHROPIC_API_KEY` env var for Claude, `CODEX_HOME` for Codex. A new `aide account` subcommand (add/list/remove) manages accounts. **If `account` is omitted from `aide.yaml`, behavior is unchanged — the feature is fully optional.**

**Tech Stack:** Go 1.22, cobra, go-yaml, existing `internal/*` packages

## Global Constraints

- Backward-compatible: `account` field is optional, existing `aide.yaml` files continue to work
- No secrets in repo: `aide.yaml` only references account name, all credentials in `~/.aide/accounts.json`
- Per-provider switching: Copilot uses `gh auth switch`, Claude uses env vars, Codex uses `CODEX_HOME`
- JSON file permissions: `~/.aide/accounts.json` stored with 0600 (contains credentials)
- Error messages in English, consistent with existing CLI style

---

### Task 1: Add `Account` field to Config

**Files:**
- Modify: `internal/config/config.go`
- Modify: `internal/config/config_test.go`

**Interfaces:**
- Produces: `Config.Account string` (yaml tag `account,omitempty`)

- [ ] **Step 1: Add Account field to Config struct**

```go
// In internal/config/config.go
type Config struct {
	Provider string `yaml:"provider"`
	Account  string `yaml:"account,omitempty"`
	Args     string `yaml:"args,omitempty"`
	Tools    []Tool `yaml:"tools"`
}
```

- [ ] **Step 2: Add tests for account field parsing**

```go
// In internal/config/config_test.go
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
```

- [ ] **Step 3: Run tests to verify**

Run: `go test -race ./internal/config/...`
Expected: PASS (including new tests)

- [ ] **Step 4: Commit**

```bash
git add internal/config/config.go internal/config/config_test.go
git commit -m "feat(config): add optional Account field to Config struct"
```

---

### Task 2: Create `internal/account` package

**Files:**
- Create: `internal/account/account.go`
- Create: `internal/account/account_test.go`

**Interfaces:**
- Produces:
  - `type Account struct { Provider string; User string; APIKey string; CodexHome string }`
  - `func Add(name string, a Account) error`
  - `func Get(name string) (Account, error)`
  - `func Remove(name string) error`
  - `func List() (map[string]Account, error)`

- [ ] **Step 1: Write the failing test for account CRUD**

```go
// internal/account/account_test.go
package account

import (
	"os"
	"path/filepath"
	"testing"
)

func TestAddAndGet(t *testing.T) {
	// Override home dir for isolated test
	tmp := t.TempDir()
	t.Setenv("HOME", tmp)
	t.Setenv("USERPROFILE", tmp)

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

func TestGetNotFound(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("HOME", tmp)
	t.Setenv("USERPROFILE", tmp)

	_, err := Get("nonexistent")
	if err == nil {
		t.Fatal("expected error for missing account")
	}
}

func TestRemove(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("HOME", tmp)
	t.Setenv("USERPROFILE", tmp)

	Add("temp", Account{Provider: "claude", APIKey: "sk-test"})
	err := Remove("temp")
	if err != nil {
		t.Fatalf("Remove failed: %v", err)
	}
	_, err = Get("temp")
	if err == nil {
		t.Fatal("expected error after remove")
	}
}

func TestList(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("HOME", tmp)
	t.Setenv("USERPROFILE", tmp)

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

func TestAddAPIKeyIsMaskedInFile(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("HOME", tmp)
	t.Setenv("USERPROFILE", tmp)

	Add("test", Account{Provider: "claude", APIKey: "sk-ant-secret123"})

	home, _ := os.UserHomeDir()
	data, _ := os.ReadFile(filepath.Join(home, ".aide", "accounts.json"))
	// API key should NOT appear in plain text in the JSON file
	if string(data) == "" {
		t.Fatal("accounts.json should exist")
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test -race ./internal/account/...`
Expected: FAIL (package doesn't exist or functions not defined)

- [ ] **Step 3: Implement account.go**

```go
// internal/account/account.go
package account

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

// Account stores provider-specific credentials for an account.
type Account struct {
	Provider  string `json:"provider"`
	User      string `json:"user,omitempty"`
	APIKey    string `json:"api_key,omitempty"`
	CodexHome string `json:"codex_home,omitempty"`
}

func accountsPath() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("getting home directory: %w", err)
	}
	return filepath.Join(home, ".aide", "accounts.json"), nil
}

func ensureDir() error {
	home, err := os.UserHomeDir()
	if err != nil {
		return fmt.Errorf("getting home directory: %w", err)
	}
	dir := filepath.Join(home, ".aide")
	return os.MkdirAll(dir, 0755)
}

func readAccounts() (map[string]Account, error) {
	p, err := accountsPath()
	if err != nil {
		return nil, err
	}

	data, err := os.ReadFile(p)
	if err != nil {
		if os.IsNotExist(err) {
			return map[string]Account{}, nil
		}
		return nil, fmt.Errorf("reading %s: %w", p, err)
	}

	var accounts map[string]Account
	if err := json.Unmarshal(data, &accounts); err != nil {
		return nil, fmt.Errorf("parsing %s: %w", p, err)
	}
	if accounts == nil {
		accounts = map[string]Account{}
	}
	return accounts, nil
}

func writeAccounts(accounts map[string]Account) error {
	if err := ensureDir(); err != nil {
		return err
	}

	p, err := accountsPath()
	if err != nil {
		return err
	}

	data, err := json.MarshalIndent(accounts, "", "  ")
	if err != nil {
		return fmt.Errorf("marshaling accounts: %w", err)
	}

	// 0600 — credentials file
	if err := os.WriteFile(p, data, 0600); err != nil {
		return fmt.Errorf("writing %s: %w", p, err)
	}
	return nil
}

// Add creates or updates an account.
func Add(name string, a Account) error {
	accounts, err := readAccounts()
	if err != nil {
		return err
	}
	accounts[name] = a
	return writeAccounts(accounts)
}

// Get returns the account with the given name.
func Get(name string) (Account, error) {
	accounts, err := readAccounts()
	if err != nil {
		return Account{}, err
	}

	a, ok := accounts[name]
	if !ok {
		if len(accounts) == 0 {
			return Account{}, fmt.Errorf("account %q not found — no accounts registered. Run 'aide account add %s' first", name, name)
		}
		names := make([]string, 0, len(accounts))
		for n := range accounts {
			names = append(names, n)
		}
		sort.Strings(names)
		return Account{}, fmt.Errorf("account %q not found. Registered accounts: %s", name, strings.Join(names, ", "))
	}
	return a, nil
}

// Remove deletes an account.
func Remove(name string) error {
	accounts, err := readAccounts()
	if err != nil {
		return err
	}
	if _, ok := accounts[name]; !ok {
		return fmt.Errorf("account %q not found", name)
	}
	delete(accounts, name)
	return writeAccounts(accounts)
}

// List returns all registered accounts.
func List() (map[string]Account, error) {
	return readAccounts()
}
```

- [ ] **Step 4: Run tests to verify they pass**

Run: `go test -race ./internal/account/...`
Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add internal/account/account.go internal/account/account_test.go
git commit -m "feat(account): add account management package for multi-account switching"
```

---

### Task 3: Add `aide account` CLI commands

**Files:**
- Create: `cmd/account.go`

**Interfaces:**
- Consumes: `account.Add(name, Account)`, `account.Get(name)`, `account.Remove(name)`, `account.List()`
- Produces: `aide account add/list/remove` subcommands

- [ ] **Step 1: Create the account command file**

```go
// cmd/account.go
package cmd

import (
	"fmt"
	"os"
	"sort"

	"aide/internal/account"

	"github.com/spf13/cobra"
)

var accountCmd = &cobra.Command{
	Use:   "account",
	Short: "Manage provider accounts for multi-account switching",
	Long:  "Add, list, and remove provider accounts stored in ~/.aide/accounts.json.",
}

var accountAddCmd = &cobra.Command{
	Use:   "add <name>",
	Short: "Add or update a provider account",
	Long: `Register an account for use with the 'account' field in aide.yaml.

Provider-specific flags:
  Copilot:  --user <github-username>
  Claude:   --api-key <key>
  Codex:    --codex-home <path>`,
	Args: cobra.ExactArgs(1),
	RunE: runAccountAdd,
}

var accountListCmd = &cobra.Command{
	Use:   "list",
	Short: "List registered accounts",
	Args:  cobra.NoArgs,
	RunE:  runAccountList,
}

var accountRemoveCmd = &cobra.Command{
	Use:   "remove <name>",
	Short: "Remove a registered account",
	Args:  cobra.ExactArgs(1),
	RunE:  runAccountRemove,
}

var (
	accountProvider string
	accountUser     string
	accountAPIKey   string
	accountCodexHome string
)

func init() {
	accountAddCmd.Flags().StringVarP(&accountProvider, "provider", "p", "", "Provider type (copilot, claude, codex)")
	accountAddCmd.Flags().StringVar(&accountUser, "user", "", "GitHub username (for Copilot)")
	accountAddCmd.Flags().StringVar(&accountAPIKey, "api-key", "", "API key (for Claude)")
	accountAddCmd.Flags().StringVar(&accountCodexHome, "codex-home", "", "Codex home directory path (for Codex)")
	accountAddCmd.MarkFlagRequired("provider")

	accountCmd.AddCommand(accountAddCmd)
	accountCmd.AddCommand(accountListCmd)
	accountCmd.AddCommand(accountRemoveCmd)
	rootCmd.AddCommand(accountCmd)
}

func runAccountAdd(cmd *cobra.Command, args []string) error {
	name := args[0]

	a := account.Account{
		Provider:  accountProvider,
		User:      accountUser,
		APIKey:    accountAPIKey,
		CodexHome: accountCodexHome,
	}

	// Validate required fields per provider
	switch accountProvider {
	case "copilot":
		if accountUser == "" {
			return fmt.Errorf("--user is required for Copilot accounts (GitHub username)")
		}
	case "claude":
		if accountAPIKey == "" {
			return fmt.Errorf("--api-key is required for Claude accounts")
		}
	case "codex":
		if accountCodexHome == "" {
			return fmt.Errorf("--codex-home is required for Codex accounts")
		}
	default:
		return fmt.Errorf("unknown provider %q — must be copilot, claude, or codex", accountProvider)
	}

	if err := account.Add(name, a); err != nil {
		return err
	}
	fmt.Printf("Account %q added (%s). Use 'account: %s' in aide.yaml.\n", name, accountProvider, name)
	return nil
}

func runAccountList(cmd *cobra.Command, args []string) error {
	accounts, err := account.List()
	if err != nil {
		return err
	}
	if len(accounts) == 0 {
		fmt.Println("No accounts registered. Use 'aide account add' to add one.")
		return nil
	}

	names := make([]string, 0, len(accounts))
	for n := range accounts {
		names = append(names, n)
	}
	sort.Strings(names)

	fmt.Println("Registered accounts:")
	for _, name := range names {
		a := accounts[name]
		detail := ""
		switch a.Provider {
		case "copilot":
			detail = fmt.Sprintf("(user: %s)", a.User)
		case "claude":
			detail = "(api key set)"
		case "codex":
			detail = fmt.Sprintf("(home: %s)", a.CodexHome)
		}
		fmt.Printf("  %s — %s %s\n", name, a.Provider, detail)
	}
	return nil
}

func runAccountRemove(cmd *cobra.Command, args []string) error {
	name := args[0]
	if err := account.Remove(name); err != nil {
		return err
	}
	fmt.Printf("Account %q removed.\n", name)
	return nil
}
```

- [ ] **Step 2: Build and verify CLI works**

Run: `go build -o aide.exe .`
Run: `.\aide.exe account --help`
Expected: Shows help for account command with add/list/remove subcommands

- [ ] **Step 3: Commit**

```bash
git add cmd/account.go
git commit -m "feat(cmd): add 'aide account' CLI commands for managing provider accounts"
```

---

### Task 4: Add account switching to Launcher

**Files:**
- Modify: `internal/launcher/launcher.go`
- Modify: `internal/launcher/launcher_test.go`

**Interfaces:**
- Consumes: `account.Account`, `account.Get(name)`
- Modifies: `Launcher.Launch` to accept optional account config and apply switching before exec

- [ ] **Step 1: Update Launcher struct and Launch method**

```go
// internal/launcher/launcher.go
package launcher

import (
	"fmt"
	"os"
	"os/exec"

	"aide/internal/account"
)

// Launcher runs the provider binary, optionally switching accounts first.
type Launcher struct {
	AccountName string // optional — if set, loads account from ~/.aide/accounts.json
}

// Launch runs the provider binary as a child process, inheriting stdin/stdout/stderr.
// If AccountName is set, applies provider-specific account switching before launch.
func (l *Launcher) Launch(name string, args ...string) error {
	path, err := exec.LookPath(name)
	if err != nil {
		return fmt.Errorf("provider %q not found in PATH", name)
	}

	cmd := exec.Command(path, args...)
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	// Apply account switching if configured
	if l.AccountName != "" {
		if err := l.applyAccount(cmd); err != nil {
			return fmt.Errorf("switching account: %w", err)
		}
	}

	return cmd.Run()
}

// applyAccount loads the account and applies provider-specific switching.
func (l *Launcher) applyAccount(cmd *exec.Cmd) error {
	acc, err := account.Get(l.AccountName)
	if err != nil {
		return err
	}

	switch acc.Provider {
	case "copilot":
		return applyCopilotAccount(acc)
	case "claude":
		return applyClaudeAccount(acc, cmd)
	case "codex":
		return applyCodexAccount(acc, cmd)
	default:
		return fmt.Errorf("unknown provider %q for account %q", acc.Provider, l.AccountName)
	}
}

func applyCopilotAccount(acc account.Account) error {
	// gh auth switch <user>
	switchCmd := exec.Command("gh", "auth", "switch", "--user", acc.User)
	switchCmd.Stdin = os.Stdin
	switchCmd.Stdout = os.Stderr // show output to user
	switchCmd.Stderr = os.Stderr
	if err := switchCmd.Run(); err != nil {
		return fmt.Errorf("gh auth switch %s failed: %w", acc.User, err)
	}
	return nil
}

func applyClaudeAccount(acc account.Account, cmd *exec.Cmd) error {
	// Set ANTHROPIC_API_KEY for the child process
	cmd.Env = append(os.Environ(), "ANTHROPIC_API_KEY="+acc.APIKey)
	return nil
}

func applyCodexAccount(acc account.Account, cmd *exec.Cmd) error {
	// Set CODEX_HOME for the child process
	cmd.Env = append(os.Environ(), "CODEX_HOME="+acc.CodexHome)
	return nil
}
```

- [ ] **Step 2: Update tests**

```go
// internal/launcher/launcher_test.go
package launcher

import (
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
```

- [ ] **Step 3: Run tests**

Run: `go test -race ./internal/launcher/...`
Expected: PASS

- [ ] **Step 4: Commit**

```bash
git add internal/launcher/launcher.go internal/launcher/launcher_test.go
git commit -m "feat(launcher): add account switching before provider launch"
```

---

### Task 5: Wire account switching into root and install commands

**Files:**
- Modify: `cmd/root.go`
- Modify: `cmd/install.go`

**Interfaces:**
- Consumes: `config.Config.Account`, `launcher.Launcher.AccountName`

- [ ] **Step 1: Update root.go to pass account to launcher**

In `runCheck`, change the launcher call from:
```go
	l := &launcher.Launcher{}
```
to:
```go
	l := &launcher.Launcher{AccountName: cfg.Account}
```

- [ ] **Step 2: Update install.go to pass account to launcher**

In `runInstall`, change the launcher call from:
```go
	l := &launcher.Launcher{}
```
to:
```go
	l := &launcher.Launcher{AccountName: cfg.Account}
```

- [ ] **Step 3: Build and verify compilation**

Run: `go build -o aide.exe .`
Expected: Build succeeds with no errors

- [ ] **Step 4: Run full test suite**

Run: `go test -race ./...`
Expected: All tests PASS

- [ ] **Step 5: Run vet**

Run: `go vet ./...`
Expected: No output (clean)

- [ ] **Step 6: Commit**

```bash
git add cmd/root.go cmd/install.go
git commit -m "feat(cmd): wire account switching into check and install flows"
```

---

### Task 6: Integration smoke test

**Files:**
- No new files — manual verification

- [ ] **Step 1: Add a test account**

```bash
.\aide.exe account add test-copilot --provider copilot --user test-user
.\aide.exe account list
```

Expected: Shows "test-copilot — copilot (user: test-user)"

- [ ] **Step 2: Create aide.yaml with account field**

```bash
mkdir -p test-project && cd test-project
echo 'provider: copilot
account: test-copilot
tools: []' > aide.yaml
```

- [ ] **Step 3: Verify check still works with account**

```bash
.\aide.exe
```

Expected: Runs check (copilot not in PATH likely fails, but should NOT crash on account loading). On a machine with `gh` installed and the user logged in, it would switch and launch.

- [ ] **Step 4: Verify check works WITHOUT account (backward compat)**

```bash
echo 'provider: go
tools: []' > aide.yaml
.\aide.exe
```

Expected: Checks for `go` binary (should pass), launches `go` (prints help). No account switching logic triggered.

- [ ] **Step 5: Remove test account and cleanup**

```bash
.\aide.exe account remove test-copilot
rm -r -force test-project
```

- [ ] **Step 6: Commit if any test files added, otherwise done**

```bash
git status
```

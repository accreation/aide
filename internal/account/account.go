// Package account manages AI provider credentials stored in ~/.aide/accounts.json.
package account

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"sort"
	"strings"
	"time"

	"aide/internal/fsutil"
)

// lockTimeout bounds how long Add/Remove wait for a concurrent aide
// process to finish its own read-modify-write of accounts.json.
const lockTimeout = 5 * time.Second

// Account stores provider-specific credentials for an account.
//
// Dir marks an account as a credential profile: a directory aide owns (or,
// if Dir is set to a path outside accountsDir, adopts) that a provider
// adapter binds a child process to via environment variables, instead of
// the legacy per-provider fields below. See ProfileDir and IsProfileBased.
type Account struct {
	Provider string `json:"provider"`
	Dir      string `json:"dir,omitempty"`

	// Token is a pre-provisioned credential for adapters whose profile can't
	// start blank (unlike claude/codex, which get authenticated later via
	// 'aide account login'): copilot's COPILOT_GITHUB_TOKEN needs a real PAT
	// up front. Not a legacy field — it's consumed by the profile-based
	// Adapter's Env, not applied directly by the launcher.
	Token string `json:"token,omitempty"`

	// Command is an optional credential-broker escape hatch, shaped like
	// AWS's credential_process or git's credential.helper: when set, its
	// stdout (trimmed) is used in place of Token or APIKey, so the real
	// secret never has to sit in accounts.json — only a command that
	// fetches it from a keyring (`op read op://...`, `security
	// find-generic-password`, `secret-tool lookup`, `pass show`). See
	// ResolveToken and ResolveAPIKey. Run fresh on every launch, never
	// cached.
	Command string `json:"command,omitempty"`

	// Legacy fields, kept for backward compatibility with accounts created
	// before credential profiles existed. Applied directly by the launcher
	// only when no profile directory exists on disk for the account name.
	User      string `json:"user,omitempty"`
	APIKey    string `json:"api_key,omitempty"`
	CodexHome string `json:"codex_home,omitempty"`
}

// HasLegacyFields reports whether a was configured with the pre-profile
// per-provider fields, meaning it predates (or explicitly opts out of)
// credential-profile isolation.
func HasLegacyFields(a Account) bool {
	return a.APIKey != "" || a.CodexHome != "" || a.User != ""
}

// runCredentialCommand runs command through the shell and returns its
// stdout trimmed of surrounding whitespace — the AWS credential_process /
// git credential.helper convention where stdout is the secret itself.
func runCredentialCommand(command string) (string, error) {
	shell, flag := "sh", "-c"
	if runtime.GOOS == "windows" {
		shell, flag = "cmd", "/C"
	}
	out, err := exec.Command(shell, flag, command).Output()
	if err != nil {
		return "", fmt.Errorf("running credential command %q: %w", command, err)
	}
	return strings.TrimSpace(string(out)), nil
}

// ResolveToken returns acc.Command's output if set, or acc.Token
// otherwise — the credential-broker escape hatch for a pre-provisioned
// secret (currently consumed by the copilot adapter's
// COPILOT_GITHUB_TOKEN). Command takes precedence so a configured broker
// stays authoritative even if a stale Token value is also present.
func ResolveToken(acc Account) (string, error) {
	if acc.Command != "" {
		return runCredentialCommand(acc.Command)
	}
	return acc.Token, nil
}

// ResolveAPIKey returns acc.Command's output if set, or acc.APIKey
// otherwise — the same broker escape hatch as ResolveToken, for legacy
// claude accounts whose secret is injected directly as ANTHROPIC_API_KEY.
func ResolveAPIKey(acc Account) (string, error) {
	if acc.Command != "" {
		return runCredentialCommand(acc.Command)
	}
	return acc.APIKey, nil
}

func accountsPath() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("getting home directory: %w", err)
	}
	return filepath.Join(home, ".aide", "accounts.json"), nil
}

func ensureDir() error {
	_, err := fsutil.AideDir()
	return err
}

// accountsDir returns ~/.aide/accounts, the directory under which
// credential profiles live (~/.aide/accounts/<name>/). Like ~/.aide itself
// it is 0700, but here the mode is re-asserted strictly (see
// ensureAccountsDir): this tree holds nothing but credentials.
func accountsDir() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("getting home directory: %w", err)
	}
	return filepath.Join(home, ".aide", "accounts"), nil
}

func ensureAccountsDir() (string, error) {
	dir, err := accountsDir()
	if err != nil {
		return "", err
	}
	if err := os.MkdirAll(dir, 0700); err != nil {
		return "", fmt.Errorf("creating %s: %w", dir, err)
	}
	// os.MkdirAll will not re-tighten an existing dir's permissions.
	if runtime.GOOS != "windows" {
		if err := os.Chmod(dir, 0700); err != nil {
			return "", fmt.Errorf("tightening permissions on %s: %w", dir, err)
		}
	}
	return dir, nil
}

// ProfileDir returns the credential profile directory for name: acc.Dir if
// explicitly set (an adopted directory, e.g. an existing CODEX_HOME), or
// accountsDir()/name otherwise. The returned path is always absolute.
func ProfileDir(name string, acc Account) (string, error) {
	if acc.Dir != "" {
		return filepath.Abs(acc.Dir)
	}
	if err := validateName(name); err != nil {
		return "", err
	}
	dir, err := accountsDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, name), nil
}

// IsProfileBased reports whether name has an on-disk credential profile
// directory already, meaning it should be launched via its provider's
// Adapter rather than acc's legacy fields. A missing directory (including
// every account created before credential profiles existed) means legacy
// mode, preserving current behavior for at least one release.
func IsProfileBased(name string, acc Account) bool {
	dir, err := ProfileDir(name, acc)
	if err != nil {
		return false
	}
	info, err := os.Stat(dir)
	return err == nil && info.IsDir()
}

// reservedWindowsNames are device names that cannot be used as a file or
// directory name on Windows, regardless of extension.
var reservedWindowsNames = map[string]bool{
	"CON": true, "PRN": true, "AUX": true, "NUL": true,
	"COM1": true, "COM2": true, "COM3": true, "COM4": true, "COM5": true,
	"COM6": true, "COM7": true, "COM8": true, "COM9": true,
	"LPT1": true, "LPT2": true, "LPT3": true, "LPT4": true, "LPT5": true,
	"LPT6": true, "LPT7": true, "LPT8": true, "LPT9": true,
}

// validateName rejects account names that cannot safely become a single
// path component of a credential profile directory: traversal sequences,
// path separators, control characters, leading/trailing whitespace or
// dots, and Windows reserved device names (checked cross-platform, since
// a profile created on Linux may later need to work on Windows).
func validateName(name string) error {
	if name == "" {
		return fmt.Errorf("account name cannot be empty")
	}
	if name == "." || name == ".." {
		return fmt.Errorf("account name %q is not allowed", name)
	}
	if strings.ContainsAny(name, `/\:`) {
		return fmt.Errorf("account name %q must not contain path separators or colons", name)
	}
	for _, r := range name {
		if r < 0x20 {
			return fmt.Errorf("account name %q must not contain control characters", name)
		}
	}
	if trimmed := strings.TrimSpace(name); trimmed != name {
		return fmt.Errorf("account name %q must not have leading or trailing whitespace", name)
	}
	if strings.HasSuffix(name, ".") {
		return fmt.Errorf("account name %q must not end with a dot", name)
	}
	base := strings.ToUpper(name)
	if i := strings.IndexByte(base, '.'); i >= 0 {
		base = base[:i]
	}
	if reservedWindowsNames[base] {
		return fmt.Errorf("account name %q is a reserved device name on Windows", name)
	}
	return nil
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

	// 0600 — credentials file. Applied on every write (not just creation)
	// so permissions can't drift if the file is ever restored/copied with
	// looser permissions.
	if err := fsutil.WriteFileAtomic(p, data, 0600); err != nil {
		return fmt.Errorf("writing %s: %w", p, err)
	}
	return nil
}

// withLock runs fn while holding an exclusive lock on accounts.json,
// ensuring concurrent Add/Remove calls don't lose each other's updates.
func withLock(fn func() error) error {
	if err := ensureDir(); err != nil {
		return err
	}
	p, err := accountsPath()
	if err != nil {
		return err
	}
	unlock, err := fsutil.Lock(p, lockTimeout)
	if err != nil {
		return err
	}
	defer unlock()
	return fn()
}

// ValidateProviderFields checks that a carries what its provider needs to
// launch. Profile-based accounts (no legacy fields set) need nothing here —
// CreateProfile enforces their requirements when the profile is
// materialized. Copilot's legacy --user path still works for accounts that
// opt into it, but is no longer required: omitting --user now creates a
// credential profile instead of the removed 'gh auth switch' mechanism.
func ValidateProviderFields(a Account) error {
	switch a.Provider {
	case "copilot", "claude", "codex", "opencode":
		// Legacy fields (--user / --api-key / --codex-home) are optional:
		// omitting them creates a profile-based account instead.
	default:
		return fmt.Errorf("unknown provider %q — must be copilot, claude, codex, or opencode", a.Provider)
	}
	return nil
}

// Add creates or updates an account.
func Add(name string, a Account) error {
	if err := validateName(name); err != nil {
		return err
	}
	return withLock(func() error {
		accounts, err := readAccounts()
		if err != nil {
			return err
		}
		accounts[name] = a
		return writeAccounts(accounts)
	})
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

// RemoveOptions controls what Remove does with an account's on-disk
// credential profile, if it has one.
type RemoveOptions struct {
	// Force allows deleting the profile directory. Without it, Remove
	// refuses to touch a profile that still exists on disk.
	Force bool
	// KeepCredentials removes only the accounts.json entry, leaving the
	// profile directory (if any) untouched.
	KeepCredentials bool
}

// Remove deletes an account's accounts.json entry and, for a profile-based
// account under accountsDir, the credential profile directory itself
// (gated by opts — see RemoveOptions). A directory explicitly adopted via
// acc.Dir is never deleted, since aide does not own it.
func Remove(name string, opts RemoveOptions) error {
	return withLock(func() error {
		accounts, err := readAccounts()
		if err != nil {
			return err
		}
		acc, ok := accounts[name]
		if !ok {
			return fmt.Errorf("account %q not found", name)
		}

		// Decide up front whether a profile directory will be deleted, and
		// refuse before mutating accounts.json — a partial removal (index
		// entry gone, credentials left dangling with no matching entry, or
		// vice versa) is worse than refusing outright.
		var profileDir string
		if acc.Dir == "" {
			dir, err := accountsDir()
			if err != nil {
				return err
			}
			candidate := filepath.Join(dir, name)
			if info, statErr := os.Stat(candidate); statErr == nil && info.IsDir() {
				absCandidate, err := filepath.Abs(candidate)
				if err != nil {
					return err
				}
				absExpected, err := filepath.Abs(filepath.Join(dir, name))
				if err != nil {
					return err
				}
				// Defense in depth: candidate is built from accountsDir+name
				// above, so this can only fail if filepath.Join ever stops
				// agreeing with Abs.
				if absCandidate != absExpected {
					return fmt.Errorf("refusing to remove %s: outside the accounts directory", candidate)
				}
				profileDir = candidate
			}
		}
		if profileDir != "" && !opts.KeepCredentials && !opts.Force {
			return fmt.Errorf("account %q has a credential profile at %s — pass --force to delete it or --keep-credentials to leave it on disk", name, profileDir)
		}

		delete(accounts, name)
		if err := writeAccounts(accounts); err != nil {
			return err
		}

		if profileDir == "" || opts.KeepCredentials {
			return nil
		}
		return os.RemoveAll(profileDir)
	})
}

// List returns all registered accounts.
func List() (map[string]Account, error) {
	return readAccounts()
}

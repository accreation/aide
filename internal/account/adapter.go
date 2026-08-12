package account

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"

	"aide/internal/fsutil"
)

// Identity is the result of a cheap, unbilled check of who a credential
// profile WILL run as once bound to a child process.
type Identity struct {
	LoggedIn bool
	Label    string // e.g. "user@acme.com (ACME Corp)" or "octocat" — shown to the user
}

// Adapter is a provider's credential-profile binding: the directories its
// profile needs, how to turn a profile root into an environment plan, which
// ambient variables would silently defeat that plan, and how to verify and
// authenticate it. This table lives in Go rather than recipes.yaml because
// recipes.yaml is remotely overridable, and remote control over a
// credentialed child process's environment is an injection vector.
type Adapter struct {
	Provider string

	// Dirs are subdirectories of the profile root the provider needs,
	// created 0700 before launch (e.g. "claude" for CLAUDE_CONFIG_DIR).
	Dirs []string

	// Env returns "KEY=VALUE" pairs binding a child process to profileRoot.
	// Paths must be absolute.
	Env func(profileRoot string, acc Account) ([]string, error)

	// Scrub lists ambient variables that would silently defeat isolation if
	// inherited. These are omitted from the child env, never set to "".
	Scrub []string

	// Prepare runs before exec: creating Dirs, seeding config files. It must
	// be safe to call repeatedly and must never overwrite an existing seed
	// file (a later Prepare must not clobber real credentials or config).
	Prepare func(profileRoot string, acc Account) error

	// Identity performs a cheap, unbilled check of who a child WILL run as
	// with env applied.
	Identity func(profileRoot string, env []string, acc Account) (Identity, error)

	// LoginArgv is the command `aide account login <name>` runs, with Env
	// applied, to authenticate this profile interactively.
	LoginArgv func(profileRoot string, acc Account) []string
}

// Adapters holds the credential-profile adapter for each provider that
// supports one. A provider absent from this map (e.g. copilot, opencode in
// this release) can only be used via legacy per-provider fields.
var Adapters = map[string]*Adapter{
	"claude": {
		Provider: "claude",
		Dirs:     []string{"claude"},
		Scrub:    []string{"CLAUDE_SECURESTORAGE_CONFIG_DIR"},
		Env: func(profileRoot string, acc Account) ([]string, error) {
			dir, err := filepath.Abs(filepath.Join(profileRoot, "claude"))
			if err != nil {
				return nil, err
			}
			return []string{"CLAUDE_CONFIG_DIR=" + dir}, nil
		},
		Prepare: func(profileRoot string, acc Account) error {
			return mkdirProfile(filepath.Join(profileRoot, "claude"))
		},
		Identity: func(profileRoot string, env []string, acc Account) (Identity, error) {
			return runIdentityCheck(env, []string{"claude", "auth", "status", "--json"}, parseClaudeIdentity)
		},
		LoginArgv: func(profileRoot string, acc Account) []string {
			return []string{"claude", "auth", "login"}
		},
	},
	"codex": {
		Provider: "codex",
		Dirs:     []string{"codex"},
		// CODEX_API_KEY / CODEX_ACCESS_TOKEN short-circuit codex's own
		// credential store ahead of CODEX_HOME; CODEX_SQLITE_HOME points
		// every profile at one shared state DB regardless of CODEX_HOME.
		Scrub: []string{"CODEX_API_KEY", "CODEX_ACCESS_TOKEN", "CODEX_SQLITE_HOME"},
		Env: func(profileRoot string, acc Account) ([]string, error) {
			dir, err := filepath.Abs(filepath.Join(profileRoot, "codex"))
			if err != nil {
				return nil, err
			}
			return []string{"CODEX_HOME=" + dir}, nil
		},
		Prepare: func(profileRoot string, acc Account) error {
			dir := filepath.Join(profileRoot, "codex")
			if err := mkdirProfile(dir); err != nil {
				return err
			}
			return seedCodexConfig(dir)
		},
		Identity: func(profileRoot string, env []string, acc Account) (Identity, error) {
			return runIdentityCheck(env, []string{"codex", "login", "status"}, nil)
		},
		LoginArgv: func(profileRoot string, acc Account) []string {
			return []string{"codex", "login"}
		},
	},
}

// CreateProfile materializes name's credential profile on disk: the
// profile root, every directory its provider's adapter needs, and any
// adapter-specific seed files. Safe to call repeatedly — Prepare
// implementations must not overwrite an existing seed.
func CreateProfile(name string, acc Account) error {
	adapter, ok := Adapters[acc.Provider]
	if !ok {
		return fmt.Errorf("provider %q does not support credential profiles yet", acc.Provider)
	}
	if acc.Dir == "" {
		if _, err := ensureAccountsDir(); err != nil {
			return err
		}
	}
	root, err := ProfileDir(name, acc)
	if err != nil {
		return err
	}
	if err := mkdirProfile(root); err != nil {
		return err
	}
	for _, d := range adapter.Dirs {
		if err := mkdirProfile(filepath.Join(root, d)); err != nil {
			return err
		}
	}
	if adapter.Prepare != nil {
		return adapter.Prepare(root, acc)
	}
	return nil
}

func mkdirProfile(dir string) error {
	if err := os.MkdirAll(dir, 0700); err != nil {
		return fmt.Errorf("creating %s: %w", dir, err)
	}
	if runtime.GOOS != "windows" {
		if err := os.Chmod(dir, 0700); err != nil {
			return fmt.Errorf("tightening permissions on %s: %w", dir, err)
		}
	}
	return nil
}

// seedCodexConfig pins cli_auth_credentials_store to "file" before first
// login, so codex writes auth.json (relocatable via CODEX_HOME) instead of
// the OS keyring (which is not namespaced by CODEX_HOME and would delete
// any pre-seeded auth.json). It never overwrites an existing config.toml.
func seedCodexConfig(codexHome string) error {
	p := filepath.Join(codexHome, "config.toml")
	if _, err := os.Stat(p); err == nil {
		return nil
	} else if !os.IsNotExist(err) {
		return err
	}
	return fsutil.WriteFileAtomic(p, []byte("cli_auth_credentials_store = \"file\"\n"), 0600)
}

// execCommand is overridden in tests to run a fake CLI instead of the real
// provider binary.
var execCommand = exec.Command

// runIdentityCheck runs argv with env applied and treats any failure to run
// it (missing binary, non-zero exit — both mean "not authenticated" for
// every provider's own status command) as LoggedIn: false rather than an
// error, since a fresh, never-logged-in profile is the expected common
// case, not a fault. parse, if non-nil, extracts a label from stdout; a
// nil result falls back to stdout's first line.
func runIdentityCheck(env []string, argv []string, parse func([]byte) string) (Identity, error) {
	cmd := execCommand(argv[0], argv[1:]...)
	cmd.Env = env
	out, err := cmd.Output()
	if err != nil {
		return Identity{LoggedIn: false, Label: "not logged in"}, nil
	}
	label := "logged in"
	if parse != nil {
		if l := parse(out); l != "" {
			label = l
		}
	} else if line := firstLine(out); line != "" {
		label = line
	}
	return Identity{LoggedIn: true, Label: label}, nil
}

func parseClaudeIdentity(out []byte) string {
	var m map[string]any
	if err := json.Unmarshal(out, &m); err != nil {
		return ""
	}
	email, _ := m["email"].(string)
	org, _ := m["orgId"].(string)
	switch {
	case email != "" && org != "":
		return fmt.Sprintf("%s (%s)", email, org)
	case email != "":
		return email
	default:
		return ""
	}
}

func firstLine(b []byte) string {
	s := strings.TrimSpace(string(b))
	if i := strings.IndexByte(s, '\n'); i >= 0 {
		s = s[:i]
	}
	return s
}

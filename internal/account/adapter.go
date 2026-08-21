package account

import (
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"time"

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
// supports one — currently all four. A provider absent from this map could
// only be used via legacy per-provider fields.
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
			// claude's auto-updater installs to a global location
			// (~/.local/share/claude/versions + the ~/.local/bin/claude
			// symlink) regardless of CLAUDE_CONFIG_DIR, so two profiles
			// launched concurrently race over one shared install and the
			// binary can drift out from under aide's version check between
			// runs. Disabling it per profile keeps that check meaningful,
			// same as COPILOT_AUTO_UPDATE=false below.
			return []string{"CLAUDE_CONFIG_DIR=" + dir, "DISABLE_AUTOUPDATER=1"}, nil
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
	"copilot": {
		Provider: "copilot",
		Dirs:     []string{"copilot", "gh"},
		// COPILOT_GITHUB_TOKEN/GH_TOKEN/GITHUB_TOKEN all short-circuit
		// copilot's own credential store ahead of GH_CONFIG_DIR in its
		// 5-tier identity-resolution chain; scrubbing all three (rather
		// than blanking them) keeps GH_CONFIG_DIR authoritative when no
		// acc.Token is set, and keeps acc.Token authoritative when one is.
		Scrub: []string{"COPILOT_GITHUB_TOKEN", "GH_TOKEN", "GITHUB_TOKEN"},
		Env: func(profileRoot string, acc Account) ([]string, error) {
			copilotHome, err := filepath.Abs(filepath.Join(profileRoot, "copilot"))
			if err != nil {
				return nil, err
			}
			ghConfigDir, err := filepath.Abs(filepath.Join(profileRoot, "gh"))
			if err != nil {
				return nil, err
			}
			env := []string{
				"COPILOT_HOME=" + copilotHome,
				"GH_CONFIG_DIR=" + ghConfigDir,
				"COPILOT_AUTO_UPDATE=false",
			}
			// With no acc.Token/acc.Command, identity falls through to gh's
			// own store under GH_CONFIG_DIR (seeded via LoginArgv's
			// `gh auth login`).
			token, err := ResolveToken(acc)
			if err != nil {
				return nil, err
			}
			if token != "" {
				env = append(env, "COPILOT_GITHUB_TOKEN="+token)
			}
			return env, nil
		},
		Identity: func(profileRoot string, env []string, acc Account) (Identity, error) {
			return copilotIdentity(env)
		},
		LoginArgv: func(profileRoot string, acc Account) []string {
			// Not scriptable: copilot's own login is a device-code flow, and
			// GH_CONFIG_DIR is the only directory-scoped lever gh exposes,
			// so seed this profile's identity through gh instead.
			return []string{"gh", "auth", "login", "--insecure-storage"}
		},
	},
	"opencode": {
		Provider: "opencode",
		Dirs:     []string{"opencode"},
		// OPENCODE_AUTH_CONTENT lets auth be supplied inline instead of from
		// $XDG_DATA_HOME/opencode/auth.json; an ambient value would silently
		// override this profile's isolated credentials.
		Scrub: []string{"OPENCODE_AUTH_CONTENT"},
		Env: func(profileRoot string, acc Account) ([]string, error) {
			dir, err := filepath.Abs(filepath.Join(profileRoot, "opencode"))
			if err != nil {
				return nil, err
			}
			return []string{"XDG_DATA_HOME=" + dir}, nil
		},
		Identity: func(profileRoot string, env []string, acc Account) (Identity, error) {
			return opencodeIdentity(env)
		},
		LoginArgv: func(profileRoot string, acc Account) []string {
			return []string{"opencode", "auth", "login"}
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

func envValue(env []string, key string) string {
	prefix := key + "="
	for _, e := range env {
		if strings.HasPrefix(e, prefix) {
			return e[len(prefix):]
		}
	}
	return ""
}

// copilotHTTPClient and copilotUserURL are overridden in tests so
// copilotIdentity can be exercised against an httptest.Server instead of the
// real GitHub API.
var (
	copilotHTTPClient = &http.Client{Timeout: 10 * time.Second}
	copilotUserURL    = "https://api.github.com/user"
)

// copilotIdentity predicts who copilot WILL run as by replicating the first
// two steps of its own token-resolution chain (COPILOT_GITHUB_TOKEN, then
// `gh auth token` under GH_CONFIG_DIR) and probing GitHub's unbilled
// GET /user endpoint with whatever token that yields — copilot itself has no
// "auth status" subcommand, and calling it would cost a billed request.
func copilotIdentity(env []string) (Identity, error) {
	token := envValue(env, "COPILOT_GITHUB_TOKEN")
	if token == "" {
		token = ghAuthToken(env)
	}
	if token == "" {
		return Identity{LoggedIn: false, Label: "not logged in"}, nil
	}
	return copilotProbeUser(token)
}

// ghAuthToken runs `gh auth token` with env applied, returning "" for any
// failure (missing gh, no stored credential under GH_CONFIG_DIR) rather than
// an error — mirroring tier 5 of copilot's fail-open resolution chain.
func ghAuthToken(env []string) string {
	cmd := execCommand("gh", "auth", "token")
	cmd.Env = env
	out, err := cmd.Output()
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(out))
}

func copilotProbeUser(token string) (Identity, error) {
	req, err := http.NewRequest(http.MethodGet, copilotUserURL, nil)
	if err != nil {
		return Identity{}, err
	}
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Accept", "application/vnd.github+json")
	resp, err := copilotHTTPClient.Do(req)
	if err != nil {
		return Identity{LoggedIn: false, Label: "not logged in"}, nil
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return Identity{LoggedIn: false, Label: "not logged in"}, nil
	}
	var body struct {
		Login string `json:"login"`
	}
	_ = json.NewDecoder(resp.Body).Decode(&body)
	label := body.Login
	if label == "" {
		label = "logged in"
	}
	return Identity{LoggedIn: true, Label: label}, nil
}

// opencodeIdentity treats $XDG_DATA_HOME/opencode/auth.json's existence as
// the primary logged-in signal — opencode login is an interactive
// (@clack/prompts) TTY flow with no scriptable status command guaranteed
// stable across versions — and upgrades the label via `opencode auth list`
// when that happens to run cleanly.
func opencodeIdentity(env []string) (Identity, error) {
	dataHome := envValue(env, "XDG_DATA_HOME")
	if dataHome == "" {
		return Identity{LoggedIn: false, Label: "not logged in"}, nil
	}
	if _, err := os.Stat(filepath.Join(dataHome, "opencode", "auth.json")); err != nil {
		return Identity{LoggedIn: false, Label: "not logged in"}, nil
	}
	if id, err := runIdentityCheck(env, []string{"opencode", "auth", "list"}, nil); err == nil && id.LoggedIn {
		return id, nil
	}
	return Identity{LoggedIn: true, Label: "logged in"}, nil
}

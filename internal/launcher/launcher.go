package launcher

import (
	"fmt"
	"os"
	"os/exec"
	"strings"

	"aide/internal/account"
	"aide/internal/installer"
)

// Launcher runs the provider binary, optionally switching accounts first.
type Launcher struct {
	AccountName string // optional — if set, loads account from ~/.aide/accounts.json
}

// Launch runs the provider binary as a child process, inheriting stdin/stdout/stderr.
// If AccountName is set, applies provider-specific account switching before launch.
func (l *Launcher) Launch(name string, args ...string) error {
	return l.LaunchWithEnv(name, nil, args...)
}

// LaunchWithEnv launches provider with a custom PATH (prepended before system PATH).
// If env is set, it is used as the child process environment.
// If env is nil, the current process environment is used as base.
func (l *Launcher) LaunchWithEnv(name string, env []string, args ...string) error {
	base := env
	if base == nil {
		base = os.Environ()
	}

	// Account env must be computed before PATH resolution: an account's
	// environment plan can itself influence which binary gets resolved
	// (e.g. isolated mode's shim PATH combined with a per-account PATH
	// addition), so lookPathInEnv has to see the final env, not the
	// pre-account one.
	if l.AccountName != "" {
		var err error
		base, err = l.applyAccount(name, base)
		if err != nil {
			return fmt.Errorf("applying account %q: %w", l.AccountName, err)
		}
	}

	path, err := lookPathInEnv(name, base)
	if err != nil {
		return fmt.Errorf("provider %q not found in PATH", name)
	}

	cmd := exec.Command(path, args...)
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	cmd.Env = base

	return cmd.Run()
}

// lookPathInEnv resolves name against the PATH found in env, rather than the
// ambient process environment. If env is nil, it falls back to exec.LookPath's
// normal ambient-PATH resolution.
func lookPathInEnv(name string, env []string) (string, error) {
	if env == nil {
		return exec.LookPath(name)
	}

	pathValue := ""
	for _, e := range env {
		if strings.HasPrefix(strings.ToUpper(e), "PATH=") {
			pathValue = e[len("PATH="):]
			break
		}
	}

	origPath, hadPath := os.LookupEnv("PATH")
	os.Setenv("PATH", pathValue)
	defer func() {
		if hadPath {
			os.Setenv("PATH", origPath)
		} else {
			os.Unsetenv("PATH")
		}
	}()

	return exec.LookPath(name)
}

// IsolatedEnv returns the environment for an isolated launch: prepends .aide/shims to PATH.
func IsolatedEnv(projectDir string) []string {
	shimDir := installer.ShimDir(projectDir)
	env := os.Environ()
	for i, e := range env {
		if strings.HasPrefix(strings.ToUpper(e), "PATH=") {
			env[i] = "PATH=" + shimDir + string(os.PathListSeparator) + e[5:]
			return env
		}
	}
	// PATH not found in environment — add it
	return append(env, "PATH="+shimDir)
}

// applyAccount loads the account, verifies it matches the provider being
// launched, and returns the environment to launch it with. providerName is
// the binary the caller is about to exec (cfg.Provider) — a claude account
// bound to a copilot launch (or vice versa) is a config error, not
// something to silently ignore.
func (l *Launcher) applyAccount(providerName string, base []string) ([]string, error) {
	acc, err := account.Get(l.AccountName)
	if err != nil {
		return nil, err
	}
	if acc.Provider != providerName {
		return nil, fmt.Errorf("account %q is configured for provider %q, but aide.yaml provider is %q", l.AccountName, acc.Provider, providerName)
	}

	return accountEnv(l.AccountName, acc, base)
}

// accountEnv returns the environment for launching acc's provider on top of
// base. It is pure — no exec calls, no global mutation — so it is
// table-testable without a real provider CLI. Profile-based accounts (an
// on-disk credential profile exists for name) go through their provider's
// Adapter; everything else falls back to the pre-profile legacy fields, so
// behavior for existing accounts.json entries is unchanged.
func accountEnv(name string, acc account.Account, base []string) ([]string, error) {
	if account.IsProfileBased(name, acc) {
		adapter, ok := account.Adapters[acc.Provider]
		if !ok {
			return nil, fmt.Errorf("provider %q does not support credential profiles", acc.Provider)
		}
		root, err := account.ProfileDir(name, acc)
		if err != nil {
			return nil, err
		}
		return account.BuildEnv(adapter, root, acc, base)
	}

	switch acc.Provider {
	case "copilot":
		// The old lever here was `gh auth switch --user <name>`, a global
		// mutation with no environment-variable equivalent — it has been
		// removed in favor of the copilot Adapter's GH_CONFIG_DIR/
		// COPILOT_HOME isolation. acc.User was a pointer into gh's global
		// state, not a credential, so there is nothing to migrate
		// automatically: the account must be re-authenticated once into a
		// profile.
		return nil, fmt.Errorf("account %q uses the removed 'gh auth switch' method — re-add it without --user (optionally with --token) to get an isolated credential profile, then run 'aide account login %s'", name, name)
	case "claude":
		apiKey, err := account.ResolveAPIKey(acc)
		if err != nil {
			return nil, err
		}
		return replaceEnv(base, "ANTHROPIC_API_KEY", apiKey), nil
	case "codex":
		return replaceEnv(base, "CODEX_HOME", acc.CodexHome), nil
	default:
		return nil, fmt.Errorf("unknown provider %q for account %q", acc.Provider, name)
	}
}

// replaceEnv builds a new env slice, replacing the target variable if it already exists.
// If env is nil, falls back to the current process environment.
func replaceEnv(env []string, key, value string) []string {
	if env == nil {
		env = os.Environ()
	}
	prefix := key + "="
	replaced := false
	for i, e := range env {
		if strings.HasPrefix(e, prefix) {
			env[i] = prefix + value
			replaced = true
			break
		}
	}
	if !replaced {
		env = append(env, prefix+value)
	}
	return env
}

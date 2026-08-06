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
	path, err := exec.LookPath(name)
	if err != nil {
		return fmt.Errorf("provider %q not found in PATH", name)
	}

	cmd := exec.Command(path, args...)
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	if env != nil {
		cmd.Env = env
	}

	// Apply account switching if configured
	if l.AccountName != "" {
		if err := l.applyAccount(cmd); err != nil {
			return fmt.Errorf("switching account: %w", err)
		}
	}

	return cmd.Run()
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
	cmd.Env = replaceEnv(cmd.Env, "ANTHROPIC_API_KEY", acc.APIKey)
	return nil
}

func applyCodexAccount(acc account.Account, cmd *exec.Cmd) error {
	cmd.Env = replaceEnv(cmd.Env, "CODEX_HOME", acc.CodexHome)
	return nil
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

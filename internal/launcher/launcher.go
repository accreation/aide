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

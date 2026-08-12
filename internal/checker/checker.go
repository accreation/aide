package checker

import (
	"fmt"
	"os"
	"os/exec"
	"strings"

	"aide/internal/account"
	"aide/internal/config"
	"aide/internal/display"
	"aide/internal/installer"
	"aide/internal/semver"
)

type Checker struct {
	cfg        *config.Config
	projectDir string
}

func New(cfg *config.Config) *Checker {
	return &Checker{cfg: cfg}
}

// NewWithProjectDir creates a Checker that knows the project root for isolated-mode PATH.
func NewWithProjectDir(cfg *config.Config, projectDir string) *Checker {
	return &Checker{cfg: cfg, projectDir: projectDir}
}

// CheckProvider checks if the configured provider binary is available in PATH.
func (c *Checker) CheckProvider() display.CheckResult {
	return c.checkBinary(c.cfg.Provider, "")
}

// CheckAccount verifies the account configured for this project (if any) is
// actually logged in, before the provider is launched. An accountless
// project returns CheckResult{Ok: true} with an empty Name, so it never
// gates the launch and display prints nothing for it — the pre-existing
// zero-account behavior must stay bit-identical.
//
// A legacy account (accounts.json fields, no on-disk credential profile)
// has no identity check available yet, so it is trusted as before rather
// than newly gated — introducing a fail-closed check here would be a
// behavior change for every account created before profiles existed.
func (c *Checker) CheckAccount() display.CheckResult {
	if c.cfg.Account == "" {
		return display.CheckResult{Ok: true}
	}

	acc, err := account.Get(c.cfg.Account)
	if err != nil {
		return display.CheckResult{Name: c.cfg.Account, Installed: err.Error()}
	}
	if acc.Provider != c.cfg.Provider {
		return display.CheckResult{
			Name:      c.cfg.Account,
			Installed: fmt.Sprintf("configured for provider %q, but aide.yaml provider is %q", acc.Provider, c.cfg.Provider),
		}
	}
	if !account.IsProfileBased(c.cfg.Account, acc) {
		// Unlike claude/codex's legacy fields, a copilot account with no
		// profile can no longer launch at all — the 'gh auth switch' lever
		// it relied on was removed. Fail the check here rather than
		// reporting Ok:true and letting the launcher's error be the first
		// sign of trouble.
		if acc.Provider == "copilot" {
			return display.CheckResult{
				Name:      c.cfg.Account,
				Installed: fmt.Sprintf("uses the removed 'gh auth switch' method — re-add without --user to get a credential profile, then run 'aide account login %s'", c.cfg.Account),
			}
		}
		return display.CheckResult{Name: c.cfg.Account, Ok: true, Installed: fmt.Sprintf("%s (legacy)", acc.Provider)}
	}

	adapter, ok := account.Adapters[acc.Provider]
	if !ok || adapter.Identity == nil {
		return display.CheckResult{Name: c.cfg.Account, Ok: true, Installed: acc.Provider}
	}
	root, err := account.ProfileDir(c.cfg.Account, acc)
	if err != nil {
		return display.CheckResult{Name: c.cfg.Account, Installed: err.Error()}
	}
	env, err := account.BuildEnv(adapter, root, acc, os.Environ())
	if err != nil {
		return display.CheckResult{Name: c.cfg.Account, Installed: err.Error()}
	}
	id, err := adapter.Identity(root, env, acc)
	if err != nil {
		return display.CheckResult{Name: c.cfg.Account, Installed: fmt.Sprintf("identity check failed: %v", err)}
	}
	if !id.LoggedIn {
		return display.CheckResult{
			Name:      c.cfg.Account,
			Installed: fmt.Sprintf("not logged in — run 'aide account login %s'", c.cfg.Account),
		}
	}
	return display.CheckResult{Name: c.cfg.Account, Ok: true, Installed: id.Label}
}

// CheckTools checks all configured tools for availability and version constraints.
func (c *Checker) CheckTools() []display.CheckResult {
	results := make([]display.CheckResult, len(c.cfg.Tools))
	for i, tool := range c.cfg.Tools {
		results[i] = c.checkBinary(tool.Name, tool.Version)
	}
	return results
}

func (c *Checker) checkBinary(name, constraint string) display.CheckResult {
	// Prepend .aide/shims to PATH when in isolated mode
	path := c.resolvePath()
	if path != "" {
		origPath := os.Getenv("PATH")
		os.Setenv("PATH", path+string(os.PathListSeparator)+origPath)
		defer os.Setenv("PATH", origPath)
	}

	result := display.CheckResult{Name: name, Required: constraint}
	binPath, err := exec.LookPath(name)
	if err != nil {
		return result
	}
	_ = binPath
	result.Found = true

	// If no version constraint, just existence is enough
	if constraint == "" {
		result.Ok = true
		return result
	}

	// Try to get version. Use CombinedOutput since some tools (e.g. `java
	// -version`) write their version string to stderr on a zero exit code.
	cmd := exec.Command(name, "--version")
	out, err := cmd.CombinedOutput()
	if err != nil {
		// --version failed, try -v
		cmd = exec.Command(name, "-v")
		out, err = cmd.CombinedOutput()
		if err != nil {
			// -v failed, try the "version" subcommand (e.g., "go version")
			cmd = exec.Command(name, "version")
			out, err = cmd.CombinedOutput()
			if err != nil {
				return result // can't determine version, treat as not ok
			}
		}
	}

	version, err := semver.ExtractVersion(strings.TrimSpace(string(out)))
	if err != nil {
		return result // can't parse version
	}
	result.Installed = version

	ok, err := semver.CheckConstraint(version, constraint)
	if err != nil {
		return result
	}
	result.Ok = ok
	return result
}

// resolvePath returns the PATH to use for checking: prepends .aide/shims if isolated.
func (c *Checker) resolvePath() string {
	if c.projectDir == "" || !c.cfg.IsIsolated() {
		return ""
	}
	return installer.ShimDir(c.projectDir)
}

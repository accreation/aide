package checker

import (
	"os"
	"os/exec"
	"strings"

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

	// If no version constraint, just existence is enough
	if constraint == "" {
		result.Ok = true
		return result
	}

	// Try to get version
	cmd := exec.Command(name, "--version")
	out, err := cmd.Output()
	if err != nil {
		// --version failed, try -v
		cmd = exec.Command(name, "-v")
		out, err = cmd.Output()
		if err != nil {
			// -v failed, try the "version" subcommand (e.g., "go version")
			cmd = exec.Command(name, "version")
			out, err = cmd.Output()
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

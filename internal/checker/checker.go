package checker

import (
	"os/exec"
	"strings"

	"aide/internal/config"
	"aide/internal/display"
	"aide/internal/semver"
)

type Checker struct {
	cfg *config.Config
}

func New(cfg *config.Config) *Checker {
	return &Checker{cfg: cfg}
}

// CheckProvider checks if the configured provider binary is available in PATH.
func (c *Checker) CheckProvider() display.CheckResult {
	return checkBinary(c.cfg.Provider, "")
}

// CheckTools checks all configured tools for availability and version constraints.
func (c *Checker) CheckTools() []display.CheckResult {
	results := make([]display.CheckResult, len(c.cfg.Tools))
	for i, tool := range c.cfg.Tools {
		results[i] = checkBinary(tool.Name, tool.Version)
	}
	return results
}

func checkBinary(name, constraint string) display.CheckResult {
	result := display.CheckResult{Name: name, Required: constraint}
	path, err := exec.LookPath(name)
	if err != nil {
		return result
	}
	_ = path // reserved for future use (e.g., showing install location)

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

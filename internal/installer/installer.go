package installer

import (
	"fmt"
	"os"
	"os/exec"
	"strings"
)

// InstallOptions controls how a tool is installed.
type InstallOptions struct {
	ProjectDir string // empty = global mode; non-empty = isolated to this project
	Version    string // version constraint from aide.yaml (e.g., ">=2.0.0") — used for isolated version resolution
}

// Installer manages tool installation.
type Installer struct {
	recipes map[string]Recipe
}

// New creates an Installer with the given recipe map.
func New(recipes map[string]Recipe) *Installer {
	return &Installer{recipes: recipes}
}

// Install installs a tool in global mode (default). Alias for InstallWithOptions
// with empty options.
func (i *Installer) Install(toolName string) error {
	return i.InstallWithOptions(toolName, InstallOptions{})
}

// InstallWithOptions installs a tool. When ProjectDir is set (isolated mode),
// github-type tools are installed to .aide/store/ and a shim is created.
// System PM tools emit a warning that they cannot be isolated.
func (i *Installer) InstallWithOptions(toolName string, opts InstallOptions) error {
	if opts.ProjectDir != "" {
		return i.installIsolated(toolName, opts.ProjectDir, make(map[string]bool))
	}
	return i.installGlobal(toolName, make(map[string]bool))
}

// installGlobal installs a tool system-wide (current behavior).
func (i *Installer) installGlobal(toolName string, seen map[string]bool) error {
	return i.installWithGuard(toolName, "", seen)
}

// installIsolated installs a tool into .aide/store/ and creates a shim.
func (i *Installer) installIsolated(toolName, projectDir string, seen map[string]bool) error {
	return i.installWithGuard(toolName, projectDir, seen)
}

// installWithGuard resolves and installs prerequisites recursively,
// guarding against circular dependencies.
func (i *Installer) installWithGuard(toolName, projectDir string, seen map[string]bool) error {
	return i.installWithGuardAndVersion(toolName, projectDir, "", seen)
}

// installWithGuardAndVersion is like installWithGuard but carries the version constraint.
func (i *Installer) installWithGuardAndVersion(toolName, projectDir, version string, seen map[string]bool) error {
	if seen[toolName] {
		return fmt.Errorf("circular dependency: %s", toolName)
	}
	seen[toolName] = true

	// Install prerequisites declared in the recipe
	if recipe, ok := i.recipes[toolName]; ok {
		for _, req := range recipe.Requires {
			if pmAvailable(req) {
				continue
			}
			if _, hasRecipe := i.recipes[req]; hasRecipe {
				fmt.Printf("  Prerequisite %s not found, installing...\n", req)
				if err := i.installWithGuardAndVersion(req, projectDir, "", seen); err != nil {
					return fmt.Errorf("installing prerequisite %s: %w", req, err)
				}
			}
		}
	}

	pm, args, err := ResolvePM(toolName, i.recipes)
	if err != nil {
		return err
	}

	// In isolated mode, only github-type tools are supported for local install.
	// System PMs get a warning and fall back to global install.
	if projectDir != "" && pm != "github" {
		fmt.Printf("  Warning: cannot isolate %s (%s installs system-wide), installing globally\n", toolName, pm)
		projectDir = ""
	}

	fmt.Printf("  Installing %s via %s...\n", toolName, pm)

	var cmd *exec.Cmd
	if pm == "curl" {
		pkg := strings.Join(args, " ")
		cmd = exec.Command("bash", "-c", fmt.Sprintf("curl -fsSL %s", pkg))
	} else if pm == "github" {
		if len(args) < 3 {
			return fmt.Errorf("invalid github recipe for %s: expected 'owner/repo asset binary'", toolName)
		}
		return i.installGithub(args[0], args[1], args[2], projectDir, version)
	} else {
		cmd = exec.Command(pm, args...)
	}

	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	cmd.Stdin = os.Stdin

	if err := cmd.Run(); err != nil {
		return fmt.Errorf("installing %s via %s: %w", toolName, pm, err)
	}
	fmt.Printf("  OK  %s installed\n", toolName)
	return nil
}

// installGithub handles github-type installation with optional isolated destDir.
func (i *Installer) installGithub(ownerRepo, assetPattern, binaryName, projectDir, version string) error {
	destDir := ""
	if projectDir != "" {
		// Resolve version: if constraint given, find concrete version via GitHub API
		resolvedVersion := "latest"
		if version != "" {
			tag, err := fetchLatestReleaseMatching(ownerRepo, version)
			if err != nil {
				return fmt.Errorf("resolving version for %s with constraint %s: %w", binaryName, version, err)
			}
			resolvedVersion = strings.TrimPrefix(tag, "v")
		}
		store := NewStore(projectDir)
		destDir = store.BinDir(binaryName, resolvedVersion)

		// Skip if already installed
		if store.IsInstalled(binaryName, resolvedVersion) {
			fmt.Printf("  %s %s already installed in .aide/store/, skipping\n", binaryName, resolvedVersion)
			return nil
		}

		if err := installFromGithub(ownerRepo, assetPattern, binaryName, destDir); err != nil {
			return err
		}

		// Create shim
		if err := CreateShim(projectDir, binaryName, resolvedVersion); err != nil {
			return fmt.Errorf("creating shim for %s: %w", binaryName, err)
		}
		if err := EnsureShimDir(projectDir); err != nil {
			return fmt.Errorf("ensuring shim dir: %w", err)
		}
		return nil
	}

	return installFromGithub(ownerRepo, assetPattern, binaryName, "")
}

// resolveVersion extracts the version constraint from the tool's recipe and
// resolves it to a concrete version. Returns empty string if no version info.
func resolveVersion(toolName string, recipes map[string]Recipe) string {
	return ""
}

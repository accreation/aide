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
		return i.installIsolated(toolName, opts.ProjectDir, opts.Version, make(map[string]bool))
	}
	return i.installGlobal(toolName, opts.Version, make(map[string]bool))
}

// installGlobal installs a tool system-wide (current behavior).
func (i *Installer) installGlobal(toolName, version string, seen map[string]bool) error {
	return i.installWithGuardAndVersion(toolName, "", version, seen)
}

// installIsolated installs a tool into .aide/store/ and creates a shim.
func (i *Installer) installIsolated(toolName, projectDir, version string, seen map[string]bool) error {
	return i.installWithGuardAndVersion(toolName, projectDir, version, seen)
}

// installWithGuardAndVersion resolves and installs prerequisites recursively,
// guarding against circular dependencies, and carries the version constraint
// (used for github-recipe tools) through to installGithub.
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
	} else if needsRoot(pm) && !isRoot() {
		// System PMs like apt/dnf need sudo when not running as root
		sudoArgs := append([]string{pm}, args...)
		cmd = exec.Command("sudo", sudoArgs...)
		fmt.Printf("  (using sudo for %s)\n", pm)
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

// installGithub handles github-type installation, in both global and isolated
// (projectDir != "") mode. If a version constraint is given, it is resolved to
// a concrete release via the GitHub API in either mode, and that same release
// is reused for the download so the resolved version and the downloaded bits
// can never diverge.
func (i *Installer) installGithub(ownerRepo, assetPattern, binaryName, projectDir, version string) error {
	resolvedVersion := "latest"
	var resolvedRelease *githubRelease
	if version != "" {
		release, err := fetchLatestReleaseMatching(ownerRepo, version)
		if err != nil {
			return fmt.Errorf("resolving version for %s with constraint %s: %w", binaryName, version, err)
		}
		resolvedRelease = release
		resolvedVersion = strings.TrimPrefix(release.TagName, "v")
	}

	if projectDir == "" {
		return installFromGithub(ownerRepo, assetPattern, binaryName, "", resolvedRelease)
	}

	store := NewStore(projectDir)
	destDir := store.BinDir(binaryName, resolvedVersion)

	// Skip if already installed
	if store.IsInstalled(binaryName, resolvedVersion) {
		fmt.Printf("  %s %s already installed in .aide/store/, skipping\n", binaryName, resolvedVersion)
		return nil
	}

	if err := installFromGithub(ownerRepo, assetPattern, binaryName, destDir, resolvedRelease); err != nil {
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

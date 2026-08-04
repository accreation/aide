package installer

import (
	_ "embed"
	"fmt"
	"os"
	"os/exec"
	"runtime"

	"github.com/goccy/go-yaml"
)

//go:embed recipes.yaml
var defaultRecipes []byte

type Recipe struct {
	Windows []PMEntry `yaml:"windows,omitempty"`
	MacOS   []PMEntry `yaml:"macos,omitempty"`
	Linux   []PMEntry `yaml:"linux,omitempty"`
}

type PMEntry map[string]string

// CurrentOS returns normalized OS name: "windows", "macos", or "linux".
func CurrentOS() string {
	switch runtime.GOOS {
	case "darwin":
		return "macos"
	case "windows":
		return "windows"
	default:
		return "linux"
	}
}

// LoadRecipes loads default embedded recipes, then merges external file if provided.
// External entries override embedded ones with the same tool name.
func LoadRecipes(externalFile string) (map[string]Recipe, error) {
	recipes := make(map[string]Recipe)
	if err := yaml.Unmarshal(defaultRecipes, &recipes); err != nil {
		return nil, fmt.Errorf("parsing embedded recipes: %w", err)
	}
	if externalFile != "" {
		data, err := os.ReadFile(externalFile)
		if err != nil {
			if os.IsNotExist(err) {
				return recipes, nil
			}
			return nil, fmt.Errorf("reading external recipes %s: %w", externalFile, err)
		}
		var external map[string]Recipe
		if err := yaml.Unmarshal(data, &external); err != nil {
			return nil, fmt.Errorf("parsing external recipes: %w", err)
		}
		for k, v := range external {
			recipes[k] = v
		}
	}
	return recipes, nil
}

// ResolvePM finds the first available package manager entry for the given tool on the current OS.
// Returns (pmName, pmArgs) suitable for os/exec.
func ResolvePM(tool string, recipes map[string]Recipe) (string, []string, error) {
	recipe, ok := recipes[tool]
	if !ok {
		return "", nil, fmt.Errorf("no recipe for %q on %s", tool, CurrentOS())
	}
	var entries []PMEntry
	switch CurrentOS() {
	case "windows":
		entries = recipe.Windows
	case "macos":
		entries = recipe.MacOS
	default:
		entries = recipe.Linux
	}
	if len(entries) == 0 {
		return "", nil, fmt.Errorf("no recipe for %q on %s", tool, CurrentOS())
	}
	for _, entry := range entries {
		for pm, pkg := range entry {
			if pmAvailable(pm) {
				return pm, buildInstallArgs(pm, pkg), nil
			}
		}
	}
	return "", nil, fmt.Errorf("no available package manager found for %q on %s", tool, CurrentOS())
}

// pmAvailable checks if a package manager is available in PATH.
func pmAvailable(pm string) bool {
	// curl and bash are special — check as regular commands
	_, err := exec.LookPath(pm)
	return err == nil
}

// buildInstallArgs builds the command-line arguments for a package manager.
func buildInstallArgs(pm, pkg string) []string {
	switch pm {
	case "winget":
		return []string{"install", "--accept-source-agreements", "--accept-package-agreements", pkg}
	case "scoop":
		return []string{"install", pkg}
	case "choco":
		return []string{"install", "-y", pkg}
	case "brew":
		return []string{"install", pkg}
	case "apt":
		return []string{"install", "-y", pkg}
	case "dnf":
		return []string{"install", "-y", pkg}
	case "curl":
		return []string{"-fsSL", pkg} // pipe to bash handled by shell wrapper
	default:
		return []string{"install", pkg}
	}
}

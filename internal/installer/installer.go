package installer

import (
	"fmt"
	"os"
	"os/exec"
	"strings"
)

type Installer struct {
	recipes map[string]Recipe
}

// New creates an Installer with the given recipe map.
func New(recipes map[string]Recipe) *Installer {
	return &Installer{recipes: recipes}
}

// Install installs a tool using the resolved package manager.
// It prints progress to stdout and returns any error.
// If the tool's recipe declares prerequisites (e.g. pipx), they are
// installed first when missing.
func (i *Installer) Install(toolName string) error {
	return i.installWithGuard(toolName, make(map[string]bool))
}

// installWithGuard resolves and installs prerequisites recursively,
// guarding against circular dependencies.
func (i *Installer) installWithGuard(toolName string, seen map[string]bool) error {
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
				if err := i.installWithGuard(req, seen); err != nil {
					return fmt.Errorf("installing prerequisite %s: %w", req, err)
				}
			}
		}
	}

	pm, args, err := ResolvePM(toolName, i.recipes)
	if err != nil {
		return err
	}

	fmt.Printf("  Installing %s via %s...\n", toolName, pm)

	var cmd *exec.Cmd
	if pm == "curl" {
		// curl recipes pipe to bash — need shell
		pkg := strings.Join(args, " ")
		cmd = exec.Command("bash", "-c", fmt.Sprintf("curl -fsSL %s", pkg))
	} else if pm == "github" {
		if len(args) < 3 {
			return fmt.Errorf("invalid github recipe for %s: expected 'owner/repo asset binary'", toolName)
		}
		return installFromGithub(args[0], args[1], args[2])
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

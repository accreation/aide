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
func (i *Installer) Install(toolName string) error {
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

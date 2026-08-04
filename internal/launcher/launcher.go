package launcher

import (
	"fmt"
	"os"
	"os/exec"
)

type Launcher struct{}

// Launch runs the provider binary as a child process, inheriting stdin/stdout/stderr.
// For providers that need to take over the terminal (like Claude CLI, Copilot).
func (l *Launcher) Launch(name string, args ...string) error {
	path, err := exec.LookPath(name)
	if err != nil {
		return fmt.Errorf("provider %q not found in PATH", name)
	}

	cmd := exec.Command(path, args...)
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	return cmd.Run()
}

package launcher

import (
	"fmt"
	"os"
	"os/exec"
	"strings"

	"aide/internal/installer"
)

type Launcher struct{}

// Launch runs the provider binary as a child process, inheriting stdin/stdout/stderr.
// For providers that need to take over the terminal (like Claude CLI, Copilot).
func (l *Launcher) Launch(name string, args ...string) error {
	return l.LaunchWithEnv(name, nil, args...)
}

// LaunchWithEnv launches provider with a custom PATH (prepended before system PATH).
// If shimDir is set, it's prepended to PATH for the child process.
// If env is nil, the current process environment is used as base.
func (l *Launcher) LaunchWithEnv(name string, env []string, args ...string) error {
	path, err := exec.LookPath(name)
	if err != nil {
		return fmt.Errorf("provider %q not found in PATH", name)
	}

	cmd := exec.Command(path, args...)
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	if env != nil {
		cmd.Env = env
	}

	return cmd.Run()
}

// IsolatedEnv returns the environment for an isolated launch: prepends .aide/shims to PATH.
func IsolatedEnv(projectDir string) []string {
	shimDir := installer.ShimDir(projectDir)
	env := os.Environ()
	for i, e := range env {
		if strings.HasPrefix(strings.ToUpper(e), "PATH=") {
			env[i] = "PATH=" + shimDir + string(os.PathListSeparator) + e[5:]
			return env
		}
	}
	// PATH not found in environment — add it
	return append(env, "PATH="+shimDir)
}

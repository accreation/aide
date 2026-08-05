package installer

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
)

// getBinDir returns the user-local bin directory path.
func getBinDir() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("getting home dir: %w", err)
	}
	return filepath.Join(home, ".local", "bin"), nil
}

// ensureInPath adds binDir to the user's PATH if not already present.
func ensureInPath(binDir string) error {
	if isInPath(binDir) {
		return nil
	}
	if runtime.GOOS == "windows" {
		return ensureInPathWindows(binDir)
	}
	return ensureInPathUnix(binDir)
}

func isInPath(binDir string) bool {
	if runtime.GOOS == "windows" {
		return isInPathWindows(binDir)
	}
	return isInPathUnix(binDir)
}

// isInPathUnix checks if the directory is in the current PATH environment variable.
func isInPathUnix(binDir string) bool {
	for _, p := range strings.Split(os.Getenv("PATH"), ":") {
		if p == binDir {
			return true
		}
	}
	return false
}

// isInPathWindows checks if the directory is in the current PATH (case-insensitive).
func isInPathWindows(binDir string) bool {
	lower := strings.ToLower(binDir)
	for _, p := range strings.Split(os.Getenv("PATH"), ";") {
		if strings.ToLower(p) == lower {
			return true
		}
	}
	return false
}

// ensureInPathUnix appends the binDir to ~/.bashrc and ~/.zshrc if they exist.
func ensureInPathUnix(binDir string) error {
	exportLine := fmt.Sprintf(`export PATH="%s:$PATH"`, binDir)
	home, err := os.UserHomeDir()
	if err != nil {
		return err
	}
	var errs []string
	for _, rc := range []string{".bashrc", ".zshrc"} {
		rcPath := filepath.Join(home, rc)
		if _, err := os.Stat(rcPath); err == nil {
			if err := appendToShellRc(rcPath, exportLine); err != nil {
				errs = append(errs, fmt.Sprintf("%s: %v", rc, err))
			}
		}
	}
	if len(errs) > 0 {
		return fmt.Errorf("updating shell config: %s", strings.Join(errs, "; "))
	}
	return nil
}

// appendToShellRc appends a line to a shell rc file if not already present.
func appendToShellRc(rcPath, line string) error {
	data, err := os.ReadFile(rcPath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil // file doesn't exist, skip
		}
		return err
	}
	content := string(data)
	for _, l := range strings.Split(content, "\n") {
		if strings.TrimSpace(l) == strings.TrimSpace(line) {
			return nil // already present
		}
	}
	f, err := os.OpenFile(rcPath, os.O_APPEND|os.O_WRONLY, 0644)
	if err != nil {
		return err
	}
	defer f.Close()
	_, err = fmt.Fprintln(f, "\n"+line)
	return err
}

// ensureInPathWindows adds binDir to user PATH via registry.
func ensureInPathWindows(binDir string) error {
	cmd := exec.Command("cmd", "/c", "setx", "PATH", os.Getenv("PATH")+";"+binDir)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("setx failed: %v: %s", err, string(output))
	}
	return nil
}

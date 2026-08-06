package installer

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime"
)

// ShimDir returns the .aide/shims directory path.
func ShimDir(projectDir string) string {
	return filepath.Join(projectDir, ".aide", "shims")
}

// CreateShim creates a shim for a tool binary.
// On Unix: symlink .aide/shims/<tool> → ../store/<tool>/<version>/bin/<tool>
// On Windows: .cmd wrapper that calls the real binary.
func CreateShim(projectDir, tool, version string) error {
	if err := os.MkdirAll(ShimDir(projectDir), 0755); err != nil {
		return fmt.Errorf("creating shims dir: %w", err)
	}

	if runtime.GOOS == "windows" {
		return createWindowsShim(projectDir, tool, version)
	}
	return createUnixShim(projectDir, tool, version)
}

// createUnixShim creates a relative symlink.
func createUnixShim(projectDir, tool, version string) error {
	shimPath := filepath.Join(ShimDir(projectDir), tool)
	// Relative: ../store/<tool>/<version>/bin/<tool>
	target := filepath.Join("..", "store", tool, version, "bin", tool)

	// Remove existing shim if present
	_ = os.Remove(shimPath)

	if err := os.Symlink(target, shimPath); err != nil {
		return fmt.Errorf("creating symlink %s -> %s: %w", shimPath, target, err)
	}
	return nil
}

// createWindowsShim creates a .cmd wrapper batch file.
func createWindowsShim(projectDir, tool, version string) error {
	shimPath := filepath.Join(ShimDir(projectDir), tool+".cmd")
	// @""%~dp0..\store\<tool>\<version>\bin\<tool>.exe" %*"
	content := fmt.Sprintf(`@"%%~dp0..\store\%s\%s\bin\%s.exe" %%*`, tool, version, tool)
	content += "\r\n"

	if err := os.WriteFile(shimPath, []byte(content), 0755); err != nil {
		return fmt.Errorf("creating shim %s: %w", shimPath, err)
	}
	return nil
}

// RemoveShim deletes a shim from .aide/shims/
func RemoveShim(projectDir, tool string) error {
	shimDir := ShimDir(projectDir)
	target := filepath.Join(shimDir, tool)
	if runtime.GOOS == "windows" {
		target += ".cmd"
	}
	if err := os.Remove(target); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("removing shim %s: %w", target, err)
	}
	return nil
}

// EnsureShimDir creates the shims directory and adds .aide/ to .gitignore.
func EnsureShimDir(projectDir string) error {
	if err := os.MkdirAll(ShimDir(projectDir), 0755); err != nil {
		return fmt.Errorf("creating shims dir: %w", err)
	}
	return ensureGitignore(projectDir)
}

// ensureGitignore adds ".aide/" to .gitignore if not already present.
func ensureGitignore(projectDir string) error {
	gitignorePath := filepath.Join(projectDir, ".gitignore")
	entry := ".aide/"

	data, err := os.ReadFile(gitignorePath)
	if err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("reading .gitignore: %w", err)
	}

	content := string(data)
	// Check if .aide/ is already in .gitignore
	for _, line := range splitGitignoreLines(content) {
		if line == entry {
			return nil // already present
		}
	}

	// Append .aide/ to .gitignore
	f, err := os.OpenFile(gitignorePath, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		return fmt.Errorf("opening .gitignore: %w", err)
	}
	defer f.Close()

	if len(data) > 0 && data[len(data)-1] != '\n' {
		if _, err := f.WriteString("\n"); err != nil {
			return err
		}
	}
	if _, err := f.WriteString(entry + "\n"); err != nil {
		return fmt.Errorf("writing .gitignore: %w", err)
	}
	return nil
}

func splitGitignoreLines(s string) []string {
	var lines []string
	start := 0
	for i := 0; i < len(s); i++ {
		if s[i] == '\n' {
			lines = append(lines, s[start:i])
			start = i + 1
		}
	}
	if start < len(s) {
		lines = append(lines, s[start:])
	}
	return lines
}

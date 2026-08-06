package installer

import (
	"fmt"
	"os"
	"path/filepath"
)

// Store manages project-local tool installations under .aide/store/
type Store struct {
	projectDir string
}

// NewStore creates a Store rooted at the given project directory.
func NewStore(projectDir string) *Store {
	return &Store{projectDir: projectDir}
}

// BinDir returns the directory for a specific tool version: .aide/store/<tool>/<version>/bin
func (s *Store) BinDir(tool, version string) string {
	return filepath.Join(s.StoreDir(), tool, version, "bin")
}

// IsInstalled checks if the binary exists in the store at BinDir.
// It checks for <tool>.exe on Windows and <tool> on Unix.
func (s *Store) IsInstalled(tool, version string) bool {
	binDir := s.BinDir(tool, version)
	binaryName := tool
	if os.PathSeparator == '\\' { // Windows
		binaryName += ".exe"
	}
	_, err := os.Stat(filepath.Join(binDir, binaryName))
	return err == nil
}

// Clean removes the store directory for a specific tool+version.
func (s *Store) Clean(tool, version string) error {
	dir := filepath.Join(s.StoreDir(), tool, version)
	if _, err := os.Stat(dir); os.IsNotExist(err) {
		return nil // already gone
	}
	if err := os.RemoveAll(dir); err != nil {
		return fmt.Errorf("removing store dir %s: %w", dir, err)
	}
	// Clean up tool directory if empty
	toolDir := filepath.Join(s.StoreDir(), tool)
	if isEmpty(toolDir) {
		_ = os.Remove(toolDir)
	}
	return nil
}

// StoreDir returns the .aide/store directory path.
func (s *Store) StoreDir() string {
	return filepath.Join(s.projectDir, ".aide", "store")
}

// isEmpty returns true if a directory is empty or doesn't exist.
func isEmpty(dir string) bool {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return true
	}
	return len(entries) == 0
}

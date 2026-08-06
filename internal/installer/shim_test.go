package installer

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func TestShimDir(t *testing.T) {
	expected := filepath.Join("/project", ".aide", "shims")
	if got := ShimDir("/project"); got != expected {
		t.Errorf("ShimDir = %q, want %q", got, expected)
	}
}

func TestCreateShimUnix(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("Unix-specific test")
	}
	dir := t.TempDir()

	if err := CreateShim(dir, "mytool", "1.0.0"); err != nil {
		t.Fatalf("CreateShim: %v", err)
	}

	shimPath := filepath.Join(ShimDir(dir), "mytool")
	target, err := os.Readlink(shimPath)
	if err != nil {
		t.Fatalf("Readlink: %v", err)
	}

	expectedTarget := filepath.Join("..", "store", "mytool", "1.0.0", "bin", "mytool")
	if target != expectedTarget {
		t.Errorf("symlink target = %q, want %q", target, expectedTarget)
	}
}

func TestCreateShimWindows(t *testing.T) {
	if runtime.GOOS != "windows" {
		t.Skip("Windows-specific test")
	}
	dir := t.TempDir()

	if err := CreateShim(dir, "mytool", "1.0.0"); err != nil {
		t.Fatalf("CreateShim: %v", err)
	}

	shimPath := filepath.Join(ShimDir(dir), "mytool.cmd")
	data, err := os.ReadFile(shimPath)
	if err != nil {
		t.Fatalf("reading shim: %v", err)
	}

	content := string(data)
	if !strings.Contains(content, `store\mytool\1.0.0\bin\mytool.exe`) {
		t.Errorf("shim content does not contain expected path: %s", content)
	}
}

func TestRemoveShim(t *testing.T) {
	dir := t.TempDir()

	if err := CreateShim(dir, "mytool", "1.0.0"); err != nil {
		t.Fatalf("CreateShim: %v", err)
	}

	if err := RemoveShim(dir, "mytool"); err != nil {
		t.Fatalf("RemoveShim: %v", err)
	}

	// Check that shim is gone
	shimName := "mytool"
	if runtime.GOOS == "windows" {
		shimName += ".cmd"
	}
	shimPath := filepath.Join(ShimDir(dir), shimName)
	if _, err := os.Stat(shimPath); !os.IsNotExist(err) {
		t.Error("shim should not exist after RemoveShim")
	}

	// Removing non-existent should not error
	if err := RemoveShim(dir, "mytool"); err != nil {
		t.Fatalf("RemoveShim on missing shim: %v", err)
	}
}

func TestEnsureShimDir(t *testing.T) {
	dir := t.TempDir()

	if err := EnsureShimDir(dir); err != nil {
		t.Fatalf("EnsureShimDir: %v", err)
	}

	// Check shims directory
	shimDir := ShimDir(dir)
	if fi, err := os.Stat(shimDir); err != nil || !fi.IsDir() {
		t.Error("shims dir not created")
	}

	// Check .gitignore contains .aide/
	gitignorePath := filepath.Join(dir, ".gitignore")
	data, err := os.ReadFile(gitignorePath)
	if err != nil {
		t.Fatalf("reading .gitignore: %v", err)
	}
	if !strings.Contains(string(data), ".aide/") {
		t.Errorf(".gitignore does not contain .aide/: %s", string(data))
	}

	// Second call should not duplicate .aide/ entry
	if err := EnsureShimDir(dir); err != nil {
		t.Fatalf("second EnsureShimDir: %v", err)
	}
	data2, _ := os.ReadFile(gitignorePath)
	count := strings.Count(string(data2), ".aide/")
	if count != 1 {
		t.Errorf(".aide/ appears %d times in .gitignore, want 1", count)
	}
}

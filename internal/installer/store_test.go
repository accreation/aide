package installer

import (
	"os"
	"path/filepath"
	"testing"
)

func TestStoreBinDir(t *testing.T) {
	store := NewStore("/project/a")
	expected := filepath.Join("/project", "a", ".aide", "store", "mytool", "1.0.0", "bin")
	if got := store.BinDir("mytool", "1.0.0"); got != expected {
		t.Errorf("BinDir = %q, want %q", got, expected)
	}
}

func TestStoreIsInstalled(t *testing.T) {
	dir := t.TempDir()
	store := NewStore(dir)

	// Not installed yet
	if store.IsInstalled("mytool", "1.0.0") {
		t.Error("expected IsInstalled = false for missing binary")
	}

	// Create binary
	binDir := store.BinDir("mytool", "1.0.0")
	os.MkdirAll(binDir, 0755)
	os.WriteFile(filepath.Join(binDir, "mytool"+exeExt()), []byte("binary"), 0755)

	if !store.IsInstalled("mytool", "1.0.0") {
		t.Error("expected IsInstalled = true after creating binary")
	}

	// Wrong version not installed
	if store.IsInstalled("mytool", "2.0.0") {
		t.Error("expected IsInstalled = false for different version")
	}
}

func TestStoreClean(t *testing.T) {
	dir := t.TempDir()
	store := NewStore(dir)

	// Create and clean
	binDir := store.BinDir("mytool", "1.0.0")
	os.MkdirAll(binDir, 0755)
	os.WriteFile(filepath.Join(binDir, "mytool"+exeExt()), []byte("binary"), 0755)

	if err := store.Clean("mytool", "1.0.0"); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if store.IsInstalled("mytool", "1.0.0") {
		t.Error("expected IsInstalled = false after clean")
	}

	// Clean non-existent should not error
	if err := store.Clean("mytool", "9.9.9"); err != nil {
		t.Fatalf("unexpected error on clean of missing: %v", err)
	}
}

func TestStoreStoreDir(t *testing.T) {
	store := NewStore("/project/b")
	expected := filepath.Join("/project", "b", ".aide", "store")
	if got := store.StoreDir(); got != expected {
		t.Errorf("StoreDir = %q, want %q", got, expected)
	}
}

// exeExt returns ".exe" on Windows, "" otherwise.
func exeExt() string {
	if os.PathSeparator == '\\' {
		return ".exe"
	}
	return ""
}

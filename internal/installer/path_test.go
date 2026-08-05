package installer

import (
	"os"
	"path/filepath"
	"runtime"
	"testing"
)

func TestGetBinDir(t *testing.T) {
	dir, err := getBinDir()
	if err != nil {
		t.Fatalf("getBinDir: %v", err)
	}
	if dir == "" {
		t.Fatal("expected non-empty bin dir")
	}

	home, _ := os.UserHomeDir()
	if runtime.GOOS == "windows" {
		expected := filepath.Join(home, ".local", "bin")
		if dir != expected {
			t.Errorf("bin dir = %q, want %q", dir, expected)
		}
	} else {
		expected := filepath.Join(home, ".local", "bin")
		if dir != expected {
			t.Errorf("bin dir = %q, want %q", dir, expected)
		}
	}
}

func TestIsInPathUnix(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("Unix-only test")
	}

	if !isInPathUnix("/usr/local/bin") {
		t.Error("expected /usr/local/bin to be in PATH")
	}
	if isInPathUnix("/nonexistent/path/xyz") {
		t.Error("expected /nonexistent/path/xyz to NOT be in PATH")
	}
}

func TestAppendToShellRc(t *testing.T) {
	tmp := t.TempDir()

	rcPath := filepath.Join(tmp, ".bashrc")
	exportLine := `export PATH="$HOME/.local/bin:$PATH"`

	// File doesn't exist — should still work (no-op)
	err := appendToShellRc(rcPath, exportLine)
	if err != nil {
		t.Fatalf("appendToShellRc (no file): %v", err)
	}

	// Create file with existing content
	os.WriteFile(rcPath, []byte("# existing config\n"), 0644)

	// First append
	err = appendToShellRc(rcPath, exportLine)
	if err != nil {
		t.Fatalf("appendToShellRc: %v", err)
	}

	data, _ := os.ReadFile(rcPath)
	if !containsLine(string(data), exportLine) {
		t.Errorf("expected file to contain %q, got:\n%s", exportLine, string(data))
	}

	// Second append — should be idempotent
	err = appendToShellRc(rcPath, exportLine)
	if err != nil {
		t.Fatalf("appendToShellRc (second): %v", err)
	}

	data2, _ := os.ReadFile(rcPath)
	if count := lineCount(string(data2), exportLine); count > 1 {
		t.Errorf("expected line to appear once, got %d times", count)
	}
}

func containsLine(content, line string) bool {
	for _, l := range splitLines(content) {
		if l == line {
			return true
		}
	}
	return false
}

func lineCount(content, line string) int {
	count := 0
	for _, l := range splitLines(content) {
		if l == line {
			count++
		}
	}
	return count
}

func splitLines(s string) []string {
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

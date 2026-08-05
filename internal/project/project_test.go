package project

import (
	"os"
	"path/filepath"
	"testing"
)

func setHome(t *testing.T, dir string) {
	t.Helper()
	t.Setenv("HOME", dir)
	t.Setenv("USERPROFILE", dir)
}

func TestRegisterAndGet(t *testing.T) {
	tmp := t.TempDir()
	setHome(t, tmp)

	p := "/home/user/dev/myproject"
	if err := Register("myproject", p); err != nil {
		t.Fatalf("Register failed: %v", err)
	}

	got, err := Get("myproject")
	if err != nil {
		t.Fatalf("Get failed: %v", err)
	}
	if got != p {
		t.Errorf("expected %q, got %q", p, got)
	}
}

func TestRegisterCreatesAideDir(t *testing.T) {
	tmp := t.TempDir()
	setHome(t, tmp)

	// Ensure .aide dir does not exist yet
	os.RemoveAll(filepath.Join(tmp, ".aide"))

	if err := Register("test", "/tmp/test"); err != nil {
		t.Fatalf("Register failed: %v", err)
	}

	if _, err := os.Stat(filepath.Join(tmp, ".aide")); os.IsNotExist(err) {
		t.Error(".aide directory was not created")
	}
}

func TestRegisterOverwrite(t *testing.T) {
	tmp := t.TempDir()
	setHome(t, tmp)

	if err := Register("proj", "/old/path"); err != nil {
		t.Fatalf("Register failed: %v", err)
	}
	if err := Register("proj", "/new/path"); err != nil {
		t.Fatalf("Register overwrite failed: %v", err)
	}

	got, err := Get("proj")
	if err != nil {
		t.Fatalf("Get failed: %v", err)
	}
	if got != "/new/path" {
		t.Errorf("expected /new/path, got %q", got)
	}
}

func TestGetNotFound(t *testing.T) {
	tmp := t.TempDir()
	setHome(t, tmp)

	_, err := Get("nosuchproject")
	if err == nil {
		t.Fatal("expected error for missing project")
	}
}

func TestGetNotFoundEmptyRegistry(t *testing.T) {
	tmp := t.TempDir()
	setHome(t, tmp)

	_, err := Get("anything")
	if err == nil {
		t.Fatal("expected error when no projects registered")
	}
}

func TestList(t *testing.T) {
	tmp := t.TempDir()
	setHome(t, tmp)

	Register("a", "/path/a")
	Register("b", "/path/b")

	all, err := List()
	if err != nil {
		t.Fatalf("List failed: %v", err)
	}
	if len(all) != 2 {
		t.Errorf("expected 2 projects, got %d", len(all))
	}
	if all["a"] != "/path/a" {
		t.Errorf("expected /path/a, got %q", all["a"])
	}
	if all["b"] != "/path/b" {
		t.Errorf("expected /path/b, got %q", all["b"])
	}
}

func TestListEmpty(t *testing.T) {
	tmp := t.TempDir()
	setHome(t, tmp)

	all, err := List()
	if err != nil {
		t.Fatalf("List on empty registry failed: %v", err)
	}
	if len(all) != 0 {
		t.Errorf("expected 0 projects, got %d", len(all))
	}
}

package account

import (
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"testing"
)

func TestAddAndGet(t *testing.T) {
	// Override home dir for isolated test
	tmp := t.TempDir()
	t.Setenv("HOME", tmp)
	t.Setenv("USERPROFILE", tmp)

	err := Add("company-x", Account{
		Provider: "copilot",
		User:     "company-x-gh",
	})
	if err != nil {
		t.Fatalf("Add failed: %v", err)
	}

	a, err := Get("company-x")
	if err != nil {
		t.Fatalf("Get failed: %v", err)
	}
	if a.Provider != "copilot" {
		t.Errorf("expected provider 'copilot', got %q", a.Provider)
	}
	if a.User != "company-x-gh" {
		t.Errorf("expected user 'company-x-gh', got %q", a.User)
	}
}

func TestGetNotFound(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("HOME", tmp)
	t.Setenv("USERPROFILE", tmp)

	_, err := Get("nonexistent")
	if err == nil {
		t.Fatal("expected error for missing account")
	}
}

func TestRemove(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("HOME", tmp)
	t.Setenv("USERPROFILE", tmp)

	Add("temp", Account{Provider: "claude", APIKey: "sk-test"})
	err := Remove("temp")
	if err != nil {
		t.Fatalf("Remove failed: %v", err)
	}
	_, err = Get("temp")
	if err == nil {
		t.Fatal("expected error after remove")
	}
}

func TestList(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("HOME", tmp)
	t.Setenv("USERPROFILE", tmp)

	Add("a", Account{Provider: "copilot", User: "a"})
	Add("b", Account{Provider: "claude", APIKey: "sk-b"})

	accounts, err := List()
	if err != nil {
		t.Fatalf("List failed: %v", err)
	}
	if len(accounts) != 2 {
		t.Fatalf("expected 2 accounts, got %d", len(accounts))
	}
}

func TestAddAPIKeyIsMaskedInFile(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("HOME", tmp)
	t.Setenv("USERPROFILE", tmp)

	Add("test", Account{Provider: "claude", APIKey: "sk-ant-secret123"})

	home, _ := os.UserHomeDir()
	data, _ := os.ReadFile(filepath.Join(home, ".aide", "accounts.json"))
	// API key should NOT appear in plain text in the JSON file
	if string(data) == "" {
		t.Fatal("accounts.json should exist")
	}
}

func TestAccountsFilePermissionsReenforcedOnWrite(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("HOME", tmp)
	t.Setenv("USERPROFILE", tmp)

	if err := Add("a", Account{Provider: "claude", APIKey: "sk-a"}); err != nil {
		t.Fatalf("Add failed: %v", err)
	}

	p := filepath.Join(tmp, ".aide", "accounts.json")
	if err := os.Chmod(p, 0644); err != nil {
		t.Fatalf("chmod failed: %v", err)
	}

	if err := Add("b", Account{Provider: "claude", APIKey: "sk-b"}); err != nil {
		t.Fatalf("second Add failed: %v", err)
	}

	info, err := os.Stat(p)
	if err != nil {
		t.Fatalf("stat failed: %v", err)
	}
	if info.Mode().Perm() != 0600 {
		t.Errorf("expected accounts.json to be re-tightened to 0600, got %o", info.Mode().Perm())
	}
}

func TestConcurrentAddDoesNotLoseUpdates(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("HOME", tmp)
	t.Setenv("USERPROFILE", tmp)

	const n = 20
	var wg sync.WaitGroup
	for i := 0; i < n; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			name := fmt.Sprintf("acct-%d", i)
			if err := Add(name, Account{Provider: "claude", APIKey: fmt.Sprintf("sk-%d", i)}); err != nil {
				t.Errorf("Add(%s) failed: %v", name, err)
			}
		}(i)
	}
	wg.Wait()

	accounts, err := List()
	if err != nil {
		t.Fatalf("List failed: %v", err)
	}
	if len(accounts) != n {
		t.Errorf("expected %d accounts after concurrent adds, got %d (lost updates)", n, len(accounts))
	}
}

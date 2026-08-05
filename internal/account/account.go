// Package account manages AI provider credentials stored in ~/.aide/accounts.json.
package account

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

// Account stores provider-specific credentials for an account.
type Account struct {
	Provider  string `json:"provider"`
	User      string `json:"user,omitempty"`
	APIKey    string `json:"api_key,omitempty"`
	CodexHome string `json:"codex_home,omitempty"`
}

func accountsPath() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("getting home directory: %w", err)
	}
	return filepath.Join(home, ".aide", "accounts.json"), nil
}

func ensureDir() error {
	home, err := os.UserHomeDir()
	if err != nil {
		return fmt.Errorf("getting home directory: %w", err)
	}
	dir := filepath.Join(home, ".aide")
	return os.MkdirAll(dir, 0755)
}

func readAccounts() (map[string]Account, error) {
	p, err := accountsPath()
	if err != nil {
		return nil, err
	}

	data, err := os.ReadFile(p)
	if err != nil {
		if os.IsNotExist(err) {
			return map[string]Account{}, nil
		}
		return nil, fmt.Errorf("reading %s: %w", p, err)
	}

	var accounts map[string]Account
	if err := json.Unmarshal(data, &accounts); err != nil {
		return nil, fmt.Errorf("parsing %s: %w", p, err)
	}
	if accounts == nil {
		accounts = map[string]Account{}
	}
	return accounts, nil
}

func writeAccounts(accounts map[string]Account) error {
	if err := ensureDir(); err != nil {
		return err
	}

	p, err := accountsPath()
	if err != nil {
		return err
	}

	data, err := json.MarshalIndent(accounts, "", "  ")
	if err != nil {
		return fmt.Errorf("marshaling accounts: %w", err)
	}

	// 0600 — credentials file
	if err := os.WriteFile(p, data, 0600); err != nil {
		return fmt.Errorf("writing %s: %w", p, err)
	}
	return nil
}

// Add creates or updates an account.
func Add(name string, a Account) error {
	accounts, err := readAccounts()
	if err != nil {
		return err
	}
	accounts[name] = a
	return writeAccounts(accounts)
}

// Get returns the account with the given name.
func Get(name string) (Account, error) {
	accounts, err := readAccounts()
	if err != nil {
		return Account{}, err
	}

	a, ok := accounts[name]
	if !ok {
		if len(accounts) == 0 {
			return Account{}, fmt.Errorf("account %q not found — no accounts registered. Run 'aide account add %s' first", name, name)
		}
		names := make([]string, 0, len(accounts))
		for n := range accounts {
			names = append(names, n)
		}
		sort.Strings(names)
		return Account{}, fmt.Errorf("account %q not found. Registered accounts: %s", name, strings.Join(names, ", "))
	}
	return a, nil
}

// Remove deletes an account.
func Remove(name string) error {
	accounts, err := readAccounts()
	if err != nil {
		return err
	}
	if _, ok := accounts[name]; !ok {
		return fmt.Errorf("account %q not found", name)
	}
	delete(accounts, name)
	return writeAccounts(accounts)
}

// List returns all registered accounts.
func List() (map[string]Account, error) {
	return readAccounts()
}

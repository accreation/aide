package cmd

import (
	"fmt"
	"sort"

	"aide/internal/account"

	"github.com/spf13/cobra"
)

var accountCmd = &cobra.Command{
	Use:   "account",
	Short: "Manage provider accounts for multi-account switching",
	Long:  "Add, list, and remove provider accounts stored in ~/.aide/accounts.json.",
}

var accountAddCmd = &cobra.Command{
	Use:   "add <name>",
	Short: "Add or update a provider account",
	Long: `Register an account for use with the 'account' field in aide.yaml.

Provider-specific flags:
  Copilot:  --user <github-username>
  Claude:   --api-key <key>
  Codex:    --codex-home <path>`,
	Args: cobra.ExactArgs(1),
	RunE: runAccountAdd,
}

var accountListCmd = &cobra.Command{
	Use:   "list",
	Short: "List registered accounts",
	Args:  cobra.NoArgs,
	RunE:  runAccountList,
}

var accountRemoveCmd = &cobra.Command{
	Use:   "remove <name>",
	Short: "Remove a registered account",
	Args:  cobra.ExactArgs(1),
	RunE:  runAccountRemove,
}

var (
	accountProvider  string
	accountUser      string
	accountAPIKey    string
	accountCodexHome string
)

func init() {
	accountAddCmd.Flags().StringVarP(&accountProvider, "provider", "p", "", "Provider type (copilot, claude, codex)")
	accountAddCmd.Flags().StringVar(&accountUser, "user", "", "GitHub username (for Copilot)")
	accountAddCmd.Flags().StringVar(&accountAPIKey, "api-key", "", "API key (for Claude)")
	accountAddCmd.Flags().StringVar(&accountCodexHome, "codex-home", "", "Codex home directory path (for Codex)")
	accountAddCmd.MarkFlagRequired("provider")

	accountCmd.AddCommand(accountAddCmd)
	accountCmd.AddCommand(accountListCmd)
	accountCmd.AddCommand(accountRemoveCmd)
	rootCmd.AddCommand(accountCmd)
}

func runAccountAdd(cmd *cobra.Command, args []string) error {
	name := args[0]

	a := account.Account{
		Provider:  accountProvider,
		User:      accountUser,
		APIKey:    accountAPIKey,
		CodexHome: accountCodexHome,
	}

	// Validate required fields per provider
	switch accountProvider {
	case "copilot":
		if accountUser == "" {
			return fmt.Errorf("--user is required for Copilot accounts (GitHub username)")
		}
	case "claude":
		if accountAPIKey == "" {
			return fmt.Errorf("--api-key is required for Claude accounts")
		}
	case "codex":
		if accountCodexHome == "" {
			return fmt.Errorf("--codex-home is required for Codex accounts")
		}
	default:
		return fmt.Errorf("unknown provider %q — must be copilot, claude, or codex", accountProvider)
	}

	if err := account.Add(name, a); err != nil {
		return err
	}
	fmt.Printf("Account %q added (%s). Use 'account: %s' in aide.yaml.\n", name, accountProvider, name)
	return nil
}

func runAccountList(cmd *cobra.Command, args []string) error {
	accounts, err := account.List()
	if err != nil {
		return err
	}
	if len(accounts) == 0 {
		fmt.Println("No accounts registered. Use 'aide account add' to add one.")
		return nil
	}

	names := make([]string, 0, len(accounts))
	for n := range accounts {
		names = append(names, n)
	}
	sort.Strings(names)

	fmt.Println("Registered accounts:")
	for _, name := range names {
		a := accounts[name]
		detail := ""
		switch a.Provider {
		case "copilot":
			detail = fmt.Sprintf("(user: %s)", a.User)
		case "claude":
			detail = "(api key set)"
		case "codex":
			detail = fmt.Sprintf("(home: %s)", a.CodexHome)
		}
		fmt.Printf("  %s — %s %s\n", name, a.Provider, detail)
	}
	return nil
}

func runAccountRemove(cmd *cobra.Command, args []string) error {
	name := args[0]
	if err := account.Remove(name); err != nil {
		return err
	}
	fmt.Printf("Account %q removed.\n", name)
	return nil
}

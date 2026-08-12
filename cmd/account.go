package cmd

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sort"

	"aide/internal/account"
	"aide/internal/config"

	"github.com/spf13/cobra"
)

var accountCmd = &cobra.Command{
	Use:   "account",
	Short: "Manage provider accounts for multi-account switching",
	Long:  "Add, list, remove, and authenticate accounts stored in ~/.aide/accounts.json.",
}

var accountAddCmd = &cobra.Command{
	Use:   "add <name>",
	Short: "Add or update a provider account",
	Long: `Register an account for use with the 'account' field in aide.yaml.

Omitting the legacy flags below creates a credential profile at
~/.aide/accounts/<name>/ instead — run 'aide account login <name>' next to
authenticate it (claude, codex, opencode) or pass --token up front (copilot).

Provider-specific flags:
  Copilot:  --user <github-username>   (legacy — omit for a credential profile)
            --token <github-pat>       (seeds COPILOT_GITHUB_TOKEN on a profile)
  Claude:   --api-key <key>            (legacy — omit for a credential profile)
  Codex:    --codex-home <path>        (legacy — omit for a credential profile)
  Any:      --dir <path>               (adopt an existing directory as the profile root)`,
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
	Long: `Removes an account's entry from ~/.aide/accounts.json.

If the account has a credential profile on disk, one of --force or
--keep-credentials is required: --force deletes the profile directory too,
--keep-credentials leaves it on disk (untracked by aide) but removes the
entry. A directory adopted via --dir is never deleted.`,
	Args: cobra.ExactArgs(1),
	RunE: runAccountRemove,
}

var accountLoginCmd = &cobra.Command{
	Use:   "login <name>",
	Short: "Authenticate a credential profile account",
	Long:  "Runs the provider's own login flow with this account's credential profile directory applied, so the resulting session is isolated to this account. Not available for legacy (pre-profile) accounts.",
	Args:  cobra.ExactArgs(1),
	RunE:  runAccountLogin,
}

var accountStatusCmd = &cobra.Command{
	Use:   "status [name]",
	Short: "Show whether an account is logged in",
	Long:  "Checks a credential profile account's identity. With no argument, uses the 'account' configured in the current project's aide.yaml.",
	Args:  cobra.MaximumNArgs(1),
	RunE:  runAccountStatus,
}

var accountPathCmd = &cobra.Command{
	Use:   "path <name>",
	Short: "Print an account's credential profile directory",
	Args:  cobra.ExactArgs(1),
	RunE:  runAccountPath,
}

var (
	accountProvider        string
	accountUser            string
	accountAPIKey          string
	accountCodexHome       string
	accountToken           string
	accountDir             string
	accountRemoveForce     bool
	accountKeepCredentials bool
)

func init() {
	accountAddCmd.Flags().StringVarP(&accountProvider, "provider", "p", "", "Provider type (copilot, claude, codex, opencode)")
	accountAddCmd.Flags().StringVar(&accountUser, "user", "", "GitHub username (legacy, for Copilot)")
	accountAddCmd.Flags().StringVar(&accountAPIKey, "api-key", "", "API key (legacy, for Claude)")
	accountAddCmd.Flags().StringVar(&accountCodexHome, "codex-home", "", "Codex home directory path (legacy, for Codex)")
	accountAddCmd.Flags().StringVar(&accountToken, "token", "", "Pre-provisioned credential to seed a profile with (e.g. a GitHub PAT for Copilot)")
	accountAddCmd.Flags().StringVar(&accountDir, "dir", "", "Adopt an existing directory as the credential profile root")
	accountAddCmd.MarkFlagRequired("provider")

	accountRemoveCmd.Flags().BoolVar(&accountRemoveForce, "force", false, "Also delete the credential profile directory")
	accountRemoveCmd.Flags().BoolVar(&accountKeepCredentials, "keep-credentials", false, "Remove the account entry but leave the credential profile on disk")

	accountCmd.AddCommand(accountAddCmd)
	accountCmd.AddCommand(accountListCmd)
	accountCmd.AddCommand(accountRemoveCmd)
	accountCmd.AddCommand(accountLoginCmd)
	accountCmd.AddCommand(accountStatusCmd)
	accountCmd.AddCommand(accountPathCmd)
	rootCmd.AddCommand(accountCmd)
}

func runAccountAdd(cmd *cobra.Command, args []string) error {
	name := args[0]

	a := account.Account{
		Provider:  accountProvider,
		User:      accountUser,
		APIKey:    accountAPIKey,
		CodexHome: accountCodexHome,
		Token:     accountToken,
	}
	if accountDir != "" {
		abs, err := filepath.Abs(accountDir)
		if err != nil {
			return fmt.Errorf("resolving --dir: %w", err)
		}
		a.Dir = abs
	}

	if err := account.ValidateProviderFields(a); err != nil {
		return err
	}

	if err := account.Add(name, a); err != nil {
		return err
	}

	profileCapable := a.Provider == "claude" || a.Provider == "codex" || a.Provider == "copilot" || a.Provider == "opencode"
	if profileCapable && !account.HasLegacyFields(a) {
		if err := account.CreateProfile(name, a); err != nil {
			return fmt.Errorf("creating credential profile: %w", err)
		}
		dir, err := account.ProfileDir(name, a)
		if err != nil {
			return err
		}
		fmt.Printf("Account %q added (%s, profile at %s).\n", name, a.Provider, dir)
		if a.Token != "" {
			fmt.Printf("Seeded with the provided --token. Use 'account: %s' in aide.yaml, or 'aide account status %s' to verify it first.\n", name, name)
		} else {
			fmt.Printf("Run 'aide account login %s' to authenticate, then use 'account: %s' in aide.yaml.\n", name, name)
		}
		return nil
	}

	fmt.Printf("Account %q added (%s). Use 'account: %s' in aide.yaml.\n", name, a.Provider, name)
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
		switch {
		case account.IsProfileBased(name, a):
			dir, _ := account.ProfileDir(name, a)
			detail = fmt.Sprintf("(profile: %s)", dir)
		case a.Provider == "copilot":
			detail = fmt.Sprintf("(user: %s — legacy, gh auth switch removed; re-add without --user)", a.User)
		case a.Provider == "claude":
			detail = "(api key set)"
		case a.Provider == "codex":
			detail = fmt.Sprintf("(home: %s)", a.CodexHome)
		}
		fmt.Printf("  %s — %s %s\n", name, a.Provider, detail)
	}
	return nil
}

func runAccountRemove(cmd *cobra.Command, args []string) error {
	name := args[0]
	opts := account.RemoveOptions{Force: accountRemoveForce, KeepCredentials: accountKeepCredentials}
	if err := account.Remove(name, opts); err != nil {
		return err
	}
	fmt.Printf("Account %q removed.\n", name)
	return nil
}

func runAccountLogin(cmd *cobra.Command, args []string) error {
	name := args[0]
	acc, err := account.Get(name)
	if err != nil {
		return err
	}
	if account.HasLegacyFields(acc) {
		return fmt.Errorf("account %q uses legacy credential fields, not a credential profile — remove them (or register a new account) to use 'aide account login'", name)
	}

	adapter, ok := account.Adapters[acc.Provider]
	if !ok {
		return fmt.Errorf("provider %q does not support 'aide account login' yet", acc.Provider)
	}

	if err := account.CreateProfile(name, acc); err != nil {
		return fmt.Errorf("preparing credential profile: %w", err)
	}
	root, err := account.ProfileDir(name, acc)
	if err != nil {
		return err
	}
	env, err := account.BuildEnv(adapter, root, acc, os.Environ())
	if err != nil {
		return err
	}

	argv := adapter.LoginArgv(root, acc)
	loginCmd := exec.Command(argv[0], argv[1:]...)
	loginCmd.Env = env
	loginCmd.Stdin = os.Stdin
	loginCmd.Stdout = os.Stdout
	loginCmd.Stderr = os.Stderr
	if err := loginCmd.Run(); err != nil {
		return fmt.Errorf("%s login failed: %w", acc.Provider, err)
	}

	if adapter.Identity != nil {
		if id, err := adapter.Identity(root, env, acc); err == nil && id.LoggedIn {
			fmt.Printf("Account %q is now logged in: %s\n", name, id.Label)
			return nil
		}
	}
	fmt.Printf("Account %q login flow completed.\n", name)
	return nil
}

func runAccountStatus(cmd *cobra.Command, args []string) error {
	name := ""
	if len(args) == 1 {
		name = args[0]
	} else {
		cwd, err := os.Getwd()
		if err != nil {
			return err
		}
		cfg, err := config.FindAndParse(cwd)
		if err != nil {
			return fmt.Errorf("no account name given and no aide.yaml found: %w", err)
		}
		if cfg.Account == "" {
			return fmt.Errorf("no account name given and aide.yaml has no 'account' configured")
		}
		name = cfg.Account
	}

	acc, err := account.Get(name)
	if err != nil {
		return err
	}

	if !account.IsProfileBased(name, acc) {
		fmt.Printf("%s — %s (legacy fields; no identity check available)\n", name, acc.Provider)
		return nil
	}

	adapter, ok := account.Adapters[acc.Provider]
	if !ok || adapter.Identity == nil {
		fmt.Printf("%s — %s (no identity check available for this provider)\n", name, acc.Provider)
		return nil
	}

	root, err := account.ProfileDir(name, acc)
	if err != nil {
		return err
	}
	env, err := account.BuildEnv(adapter, root, acc, os.Environ())
	if err != nil {
		return err
	}
	id, err := adapter.Identity(root, env, acc)
	if err != nil {
		return fmt.Errorf("identity check failed: %w", err)
	}

	if id.LoggedIn {
		fmt.Printf("%s — %s: logged in as %s\n", name, acc.Provider, id.Label)
		return nil
	}
	fmt.Printf("%s — %s: not logged in. Run 'aide account login %s'.\n", name, acc.Provider, name)
	return nil
}

func runAccountPath(cmd *cobra.Command, args []string) error {
	name := args[0]
	acc, err := account.Get(name)
	if err != nil {
		return err
	}
	dir, err := account.ProfileDir(name, acc)
	if err != nil {
		return err
	}
	fmt.Println(dir)
	return nil
}

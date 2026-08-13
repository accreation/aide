package cmd

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"aide/internal/checker"
	"aide/internal/config"
	"aide/internal/display"
	"aide/internal/launcher"
	"aide/internal/userconfig"

	"github.com/spf13/cobra"
)

// Version is set via ldflags at build time: -X aide/cmd.Version=1.0.0
var Version = "dev"

// DefaultRecipesURL is set via ldflags at build time: -X aide/cmd.DefaultRecipesURL=https://...
// When set, the CLI fetches remote recipes automatically without --recipes-url flag.
var DefaultRecipesURL = ""

var recipesURL string
var accountFlag string

var rootCmd = &cobra.Command{
	Use:     "aide",
	Version: Version,
	Short:   "AI environment manager — check and install tools for your AI provider",
	Long: `Aide reads aide.yaml and verifies that your AI provider and all required
tools are installed with correct versions.

Without subcommands, aide runs a check and exits with code 0 if everything is ok.`,
	RunE: runCheck,
}

func Execute() error {
	return rootCmd.Execute()
}

func init() {
	rootCmd.PersistentFlags().StringVar(&recipesURL, "recipes-url", "", "URL to fetch external recipes from (env: AIDE_RECIPES_URL)")
	rootCmd.PersistentFlags().StringVar(&accountFlag, "account", "", "Account name to use, overriding aide.yaml (env: AIDE_ACCOUNT)")
}

// resolveRecipesURL applies the documented precedence: --recipes-url flag >
// AIDE_RECIPES_URL env var > build-time DefaultRecipesURL > embedded recipes.
func resolveRecipesURL() string {
	if recipesURL != "" {
		return recipesURL
	}
	if envURL := os.Getenv("AIDE_RECIPES_URL"); envURL != "" {
		return envURL
	}
	return DefaultRecipesURL
}

// resolveAccountName applies the documented account-binding precedence:
// --account flag > AIDE_ACCOUNT env var > ~/.aide/config.yaml path bindings
// (longest-prefix match against projectDir and cfg.Provider) > aide.yaml's
// own account: field. A missing or unparsable ~/.aide/config.yaml is
// treated as "no bindings" rather than an error, so a typo in the user's
// own config can't brick an otherwise-working aide.yaml.
func resolveAccountName(cfg *config.Config, projectDir string) string {
	uc, _ := userconfig.Load()
	return userconfig.ResolveAccountName(accountFlag, os.Getenv("AIDE_ACCOUNT"), uc, projectDir, cfg.Provider, cfg.Account)
}

func runCheck(cmd *cobra.Command, args []string) error {
	cwd, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("getting current directory: %w", err)
	}

	cfgPath, err := config.FindPath(cwd)
	if err != nil {
		fmt.Fprintln(os.Stderr, "Error:", err)
		fmt.Fprintln(os.Stderr, "Tip: run 'aide init' to create aide.yaml")
		os.Exit(1)
	}

	cfg, err := config.FindAndParse(cwd)
	if err != nil {
		fmt.Fprintln(os.Stderr, "Error:", err)
		fmt.Fprintln(os.Stderr, "Tip: run 'aide init' to create aide.yaml")
		os.Exit(1)
	}

	projectDir := filepath.Dir(cfgPath)
	cfg.Account = resolveAccountName(cfg, projectDir)

	var chk *checker.Checker
	if cfg.IsIsolated() {
		chk = checker.NewWithProjectDir(cfg, projectDir)
	} else {
		chk = checker.New(cfg)
	}
	providerResult := chk.CheckProvider()
	accountResult := chk.CheckAccount()
	toolResults := chk.CheckTools()

	fmt.Println("Aide — environment check")
	fmt.Println("───────────────────────")
	display.PrintProviderResult(os.Stdout, providerResult)
	display.PrintAccountResult(os.Stdout, accountResult)
	display.PrintToolResults(os.Stdout, toolResults)

	if !display.AllOk(providerResult, accountResult, toolResults) {
		fmt.Println("\nSome items are missing or outdated. Run 'aide install' to fix.")
		os.Exit(1)
	}

	fmt.Println("\nAll checks passed! Launching provider...")
	return launchProvider(cfg, projectDir)
}

// launchProvider runs the standard launch tail shared by runCheck and
// runInstall: build a Launcher for cfg.Account, apply isolated-mode PATH if
// configured, and exec the provider.
func launchProvider(cfg *config.Config, projectDir string) error {
	l := &launcher.Launcher{AccountName: cfg.Account}
	extraArgs := strings.Fields(cfg.Args)
	if cfg.IsIsolated() {
		env := launcher.IsolatedEnv(projectDir)
		return l.LaunchWithEnv(cfg.Provider, env, extraArgs...)
	}
	return l.Launch(cfg.Provider, extraArgs...)
}

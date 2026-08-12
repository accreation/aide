package cmd

import (
	"fmt"
	"os"
	"path/filepath"

	"aide/internal/checker"
	"aide/internal/config"
	"aide/internal/display"
	"aide/internal/installer"

	"github.com/spf13/cobra"
)

var installCmd = &cobra.Command{
	Use:     "install",
	Aliases: []string{"i"},
	Short:   "Install missing tools and launch provider",
	Long:    "Checks the environment, installs any missing or outdated tools, then launches the provider.",
	RunE:    runInstall,
}

func init() {
	rootCmd.AddCommand(installCmd)
}

func runInstall(cmd *cobra.Command, args []string) error {
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

	// Load recipes (embedded + optional remote)
	recipes, err := installer.LoadRecipesWithRemote(resolveRecipesURL())
	if err != nil {
		return fmt.Errorf("loading recipes: %w", err)
	}

	projectDir := filepath.Dir(cfgPath)

	var chk *checker.Checker
	if cfg.IsIsolated() {
		chk = checker.NewWithProjectDir(cfg, projectDir)
	} else {
		chk = checker.New(cfg)
	}
	inst := installer.New(recipes)

	// Ensure shim dir exists for isolated projects
	if cfg.IsIsolated() {
		if err := installer.EnsureShimDir(projectDir); err != nil {
			return fmt.Errorf("setting up shim dir: %w", err)
		}
	}

	// Check provider
	providerResult := chk.CheckProvider()
	if !providerResult.Ok {
		fmt.Printf("Installing provider: %s\n", cfg.Provider)
		if err := inst.Install(cfg.Provider); err != nil {
			fmt.Fprintf(os.Stderr, "Failed to install provider %s: %v\n", cfg.Provider, err)
			os.Exit(1)
		}
		// Re-check after install
		providerResult = chk.CheckProvider()
	}

	// Check and install tools
	toolResults := chk.CheckTools()
	for i, tr := range toolResults {
		if !tr.Ok {
			tool := cfg.Tools[i]
			opts := installer.InstallOptions{Version: tool.Version}
			if cfg.IsIsolated() {
				opts.ProjectDir = projectDir
			}
			if err := inst.InstallWithOptions(tool.Name, opts); err != nil {
				fmt.Fprintf(os.Stderr, "Failed to install %s: %v\n", tool.Name, err)
			}
		}
	}

	// Re-check after all installs
	providerResult = chk.CheckProvider()
	accountResult := chk.CheckAccount()
	toolResults = chk.CheckTools()

	fmt.Println("\nAide — final check")
	fmt.Println("──────────────────")
	display.PrintProviderResult(os.Stdout, providerResult)
	display.PrintAccountResult(os.Stdout, accountResult)
	display.PrintToolResults(os.Stdout, toolResults)

	if !display.AllOk(providerResult, accountResult, toolResults) {
		fmt.Fprintln(os.Stderr, "\nSome items could not be installed. Check the errors above.")
		os.Exit(1)
	}

	fmt.Println("\nLaunching provider...")
	return launchProvider(cfg, projectDir)
}

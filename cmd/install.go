package cmd

import (
	"fmt"
	"os"

	"aion/internal/checker"
	"aion/internal/config"
	"aion/internal/display"
	"aion/internal/installer"
	"aion/internal/launcher"

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

	cfg, err := config.FindAndParse(cwd)
	if err != nil {
		fmt.Fprintln(os.Stderr, "Error:", err)
		fmt.Fprintln(os.Stderr, "Tip: run 'aion init' to create aion.yaml")
		os.Exit(1)
	}

	// Load recipes
	recipes, err := installer.LoadRecipes("")
	if err != nil {
		return fmt.Errorf("loading recipes: %w", err)
	}

	chk := checker.New(cfg)
	inst := installer.New(recipes)

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
			if err := inst.Install(cfg.Tools[i].Name); err != nil {
				fmt.Fprintf(os.Stderr, "Failed to install %s: %v\n", cfg.Tools[i].Name, err)
			}
		}
	}

	// Re-check after all installs
	providerResult = chk.CheckProvider()
	toolResults = chk.CheckTools()

	fmt.Println("\nAion — final check")
	fmt.Println("──────────────────")
	display.PrintProviderResult(os.Stdout, providerResult)
	display.PrintToolResults(os.Stdout, toolResults)

	if !display.AllOk(providerResult, toolResults) {
		fmt.Fprintln(os.Stderr, "\nSome items could not be installed. Check the errors above.")
		os.Exit(1)
	}

	fmt.Println("\nLaunching provider...")
	l := &launcher.Launcher{}
	return l.Launch(cfg.Provider)
}

package cmd

import (
	"fmt"
	"os"
	"path/filepath"

	"aide/internal/config"
	"aide/internal/installer"

	"github.com/spf13/cobra"
)

var cacheCmd = &cobra.Command{
	Use:   "cache",
	Short: "Manage the remote recipes cache and local tool store",
}

var cacheClearCmd = &cobra.Command{
	Use:   "clear",
	Short: "Clear the cached recipes, forcing a fresh download next time. With --store, also clears .aide/store/ for the current project.",
	RunE:  runCacheClear,
}

var cacheClearStore bool

func init() {
	cacheClearCmd.Flags().BoolVar(&cacheClearStore, "store", false, "Also clear the project-local .aide/store/ directory")
	cacheCmd.AddCommand(cacheClearCmd)
	rootCmd.AddCommand(cacheCmd)
}

func runCacheClear(cmd *cobra.Command, args []string) error {
	if err := installer.ClearRecipeCache(); err != nil {
		return fmt.Errorf("clearing cache: %w", err)
	}
	fmt.Println("Recipe cache cleared.")

	if cacheClearStore {
		cwd, err := os.Getwd()
		if err != nil {
			return fmt.Errorf("getting current directory: %w", err)
		}
		cfgPath, err := config.FindPath(cwd)
		if err != nil {
			return fmt.Errorf("finding aide.yaml: %w", err)
		}
		projectDir := filepath.Dir(cfgPath)
		storeDir := filepath.Join(projectDir, ".aide", "store")
		if err := os.RemoveAll(storeDir); err != nil && !os.IsNotExist(err) {
			return fmt.Errorf("clearing store: %w", err)
		}
		fmt.Println("Project-local .aide/store/ cleared.")
	}
	return nil
}

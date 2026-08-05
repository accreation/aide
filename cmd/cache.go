package cmd

import (
	"fmt"

	"aide/internal/installer"

	"github.com/spf13/cobra"
)

var cacheCmd = &cobra.Command{
	Use:   "cache",
	Short: "Manage the remote recipes cache",
}

var cacheClearCmd = &cobra.Command{
	Use:   "clear",
	Short: "Clear the cached recipes, forcing a fresh download next time",
	RunE:  runCacheClear,
}

func init() {
	cacheCmd.AddCommand(cacheClearCmd)
	rootCmd.AddCommand(cacheCmd)
}

func runCacheClear(cmd *cobra.Command, args []string) error {
	if err := installer.ClearRecipeCache(); err != nil {
		return fmt.Errorf("clearing cache: %w", err)
	}
	fmt.Println("Recipe cache cleared.")
	return nil
}

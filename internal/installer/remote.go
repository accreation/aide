package installer

import (
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"time"
)

// DefaultRecipesURL is the canonical GitHub Pages URL for community recipes.
const DefaultRecipesURL = "https://accreation.github.io/aide/recipes.yaml"

// cacheTTL is how long a cached recipes file is considered fresh.
const cacheTTL = 1 * time.Hour

// cacheDir returns the aide cache directory (~/.aide).
func cacheDir() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("finding home directory: %w", err)
	}
	return filepath.Join(home, ".aide"), nil
}

// FetchRemoteRecipes downloads recipes from url, caches them locally,
// and returns the file path to the cached copy. If the cache is fresh
// (less than cacheTTL old), it returns the cached path without network access.
// On any error (network, parse, etc.), it returns ("", nil) — meaning
// "no remote recipes available", which is not a fatal condition.
func FetchRemoteRecipes(url string) (string, error) {
	if url == "" {
		return "", nil
	}

	dir, err := cacheDir()
	if err != nil {
		return "", nil // non-fatal
	}

	if err := os.MkdirAll(dir, 0o755); err != nil {
		return "", nil
	}

	cacheFile := filepath.Join(dir, "recipes.yaml")

	// Check if cache is fresh
	if info, err := os.Stat(cacheFile); err == nil {
		if time.Since(info.ModTime()) < cacheTTL {
			return cacheFile, nil
		}
	}

	// Fetch from network
	data, err := download(url)
	if err != nil {
		// If we have a stale cache, use it as fallback
		if _, statErr := os.Stat(cacheFile); statErr == nil {
			return cacheFile, nil
		}
		return "", nil // no cache, no network — non-fatal
	}

	// Write to cache
	if err := os.WriteFile(cacheFile, data, 0o644); err != nil {
		// Couldn't cache — that's fine, we still have the data
		// Write to a temp file so LoadRecipes can use it
		tmpFile := filepath.Join(dir, "recipes.tmp.yaml")
		if err := os.WriteFile(tmpFile, data, 0o644); err != nil {
			return "", nil
		}
		return tmpFile, nil
	}

	return cacheFile, nil
}

// download fetches content from a URL and returns the body.
func download(url string) ([]byte, error) {
	client := &http.Client{Timeout: 15 * time.Second}

	resp, err := client.Get(url)
	if err != nil {
		return nil, fmt.Errorf("fetching %s: %w", url, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("fetching %s: HTTP %d", url, resp.StatusCode)
	}

	data, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20)) // 1 MB limit
	if err != nil {
		return nil, fmt.Errorf("reading response from %s: %w", url, err)
	}

	return data, nil
}

// ClearRecipeCache removes the cached recipes file, forcing a fresh download next time.
func ClearRecipeCache() error {
	dir, err := cacheDir()
	if err != nil {
		return err
	}
	cacheFile := filepath.Join(dir, "recipes.yaml")
	_ = os.Remove(cacheFile)
	return nil
}

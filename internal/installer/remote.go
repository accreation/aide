package installer

import (
	"crypto/sha256"
	"encoding/hex"
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

// cacheFileForURL returns a cache path unique to url, so different
// --recipes-url/AIDE_RECIPES_URL values don't collide on the same file.
func cacheFileForURL(dir, url string) string {
	sum := sha256.Sum256([]byte(url))
	return filepath.Join(dir, "cache", hex.EncodeToString(sum[:])+".yaml")
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

	cacheFile := cacheFileForURL(dir, url)

	if err := os.MkdirAll(filepath.Dir(cacheFile), 0o755); err != nil {
		return "", nil
	}

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
		tmpFile := cacheFile + ".tmp"
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

// ClearRecipeCache removes all cached recipes files, forcing a fresh download next time.
func ClearRecipeCache() error {
	dir, err := cacheDir()
	if err != nil {
		return err
	}
	if err := os.RemoveAll(filepath.Join(dir, "cache")); err != nil {
		return err
	}
	// Remove the pre-fix fixed-path cache file, if present.
	_ = os.Remove(filepath.Join(dir, "recipes.yaml"))
	return nil
}

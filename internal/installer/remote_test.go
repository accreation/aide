package installer

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"sync/atomic"
	"testing"
	"time"
)

// withHome points $HOME at a fresh temp dir for the duration of the test,
// so cacheDir() is isolated from the real ~/.aide.
func withHome(t *testing.T) string {
	t.Helper()
	home := t.TempDir()
	t.Setenv("HOME", home)
	// os.UserHomeDir() on Windows uses USERPROFILE; set both for safety.
	t.Setenv("USERPROFILE", home)
	return home
}

func TestFetchRemoteRecipes_DifferentURLsDoNotCollide(t *testing.T) {
	withHome(t)

	srvA := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprint(w, "org-a-recipes")
	}))
	defer srvA.Close()

	srvB := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprint(w, "org-b-recipes")
	}))
	defer srvB.Close()

	pathA, err := FetchRemoteRecipes(srvA.URL)
	if err != nil {
		t.Fatalf("FetchRemoteRecipes(A): %v", err)
	}
	pathB, err := FetchRemoteRecipes(srvB.URL)
	if err != nil {
		t.Fatalf("FetchRemoteRecipes(B): %v", err)
	}

	if pathA == pathB {
		t.Fatalf("expected distinct cache files for distinct URLs, got %q for both", pathA)
	}

	dataA, err := os.ReadFile(pathA)
	if err != nil {
		t.Fatalf("reading cache A: %v", err)
	}
	dataB, err := os.ReadFile(pathB)
	if err != nil {
		t.Fatalf("reading cache B: %v", err)
	}
	if string(dataA) != "org-a-recipes" {
		t.Errorf("cache A = %q, want org-a-recipes", dataA)
	}
	if string(dataB) != "org-b-recipes" {
		t.Errorf("cache B = %q, want org-b-recipes", dataB)
	}

	// Re-fetching B after A must still return org B's content — this is the
	// exact cross-project poisoning scenario from the bug report.
	pathB2, err := FetchRemoteRecipes(srvB.URL)
	if err != nil {
		t.Fatalf("FetchRemoteRecipes(B) again: %v", err)
	}
	dataB2, err := os.ReadFile(pathB2)
	if err != nil {
		t.Fatalf("reading cache B again: %v", err)
	}
	if string(dataB2) != "org-b-recipes" {
		t.Errorf("second fetch of B = %q, want org-b-recipes (got poisoned by A?)", dataB2)
	}
}

func TestFetchRemoteRecipes_ServesFreshCacheWithoutNetwork(t *testing.T) {
	withHome(t)

	var hits int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(&hits, 1)
		fmt.Fprint(w, "v1")
	}))
	defer srv.Close()

	if _, err := FetchRemoteRecipes(srv.URL); err != nil {
		t.Fatalf("first fetch: %v", err)
	}
	if _, err := FetchRemoteRecipes(srv.URL); err != nil {
		t.Fatalf("second fetch: %v", err)
	}

	if got := atomic.LoadInt32(&hits); got != 1 {
		t.Errorf("expected 1 network hit for a fresh cache, got %d", got)
	}
}

func TestFetchRemoteRecipes_RefetchesAfterTTLExpires(t *testing.T) {
	home := withHome(t)

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprint(w, "fresh")
	}))
	defer srv.Close()

	path, err := FetchRemoteRecipes(srv.URL)
	if err != nil {
		t.Fatalf("fetch: %v", err)
	}

	// Simulate the cache having aged past the TTL.
	stale := time.Now().Add(-2 * cacheTTL)
	if err := os.Chtimes(path, stale, stale); err != nil {
		t.Fatalf("chtimes: %v", err)
	}

	path2, err := FetchRemoteRecipes(srv.URL)
	if err != nil {
		t.Fatalf("second fetch: %v", err)
	}
	if path2 != path {
		t.Fatalf("expected same cache path across refetch, got %q vs %q", path, path2)
	}

	info, err := os.Stat(filepath.Join(home, ".aide", "cache"))
	if err != nil || !info.IsDir() {
		t.Fatalf("expected cache dir to exist: %v", err)
	}
}

func TestFetchRemoteRecipes_FallsBackToStaleCacheOnNetworkError(t *testing.T) {
	withHome(t)

	var up int32 = 1
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if atomic.LoadInt32(&up) == 0 {
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
		fmt.Fprint(w, "cached-content")
	}))
	defer srv.Close()

	path, err := FetchRemoteRecipes(srv.URL)
	if err != nil {
		t.Fatalf("fetch: %v", err)
	}

	stale := time.Now().Add(-2 * cacheTTL)
	if err := os.Chtimes(path, stale, stale); err != nil {
		t.Fatalf("chtimes: %v", err)
	}
	atomic.StoreInt32(&up, 0)

	path2, err := FetchRemoteRecipes(srv.URL)
	if err != nil {
		t.Fatalf("fetch during outage: %v", err)
	}
	data, err := os.ReadFile(path2)
	if err != nil {
		t.Fatalf("reading fallback cache: %v", err)
	}
	if string(data) != "cached-content" {
		t.Errorf("fallback cache = %q, want cached-content", data)
	}
}

func TestFetchRemoteRecipes_EmptyURL(t *testing.T) {
	withHome(t)

	path, err := FetchRemoteRecipes("")
	if err != nil {
		t.Fatalf("FetchRemoteRecipes(\"\"): %v", err)
	}
	if path != "" {
		t.Errorf("expected empty path for empty URL, got %q", path)
	}
}

func TestClearRecipeCache(t *testing.T) {
	home := withHome(t)

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprint(w, "v1")
	}))
	defer srv.Close()

	path, err := FetchRemoteRecipes(srv.URL)
	if err != nil {
		t.Fatalf("fetch: %v", err)
	}
	if _, err := os.Stat(path); err != nil {
		t.Fatalf("expected cache file to exist before clear: %v", err)
	}

	// Legacy pre-fix cache file should also be cleaned up if present.
	legacy := filepath.Join(home, ".aide", "recipes.yaml")
	if err := os.WriteFile(legacy, []byte("legacy"), 0o644); err != nil {
		t.Fatalf("writing legacy cache file: %v", err)
	}

	if err := ClearRecipeCache(); err != nil {
		t.Fatalf("ClearRecipeCache: %v", err)
	}

	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Errorf("expected cache file to be removed, stat err = %v", err)
	}
	if _, err := os.Stat(legacy); !os.IsNotExist(err) {
		t.Errorf("expected legacy cache file to be removed, stat err = %v", err)
	}
}

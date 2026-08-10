package installer

import (
	"archive/tar"
	"archive/zip"
	"compress/gzip"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"aide/internal/semver"
)

type githubRelease struct {
	TagName    string        `json:"tag_name"`
	Draft      bool          `json:"draft"`
	Prerelease bool          `json:"prerelease"`
	Assets     []githubAsset `json:"assets"`
}

type githubAsset struct {
	Name               string `json:"name"`
	BrowserDownloadURL string `json:"browser_download_url"`
}

// fetchLatestRelease fetches the latest GitHub release from the given API URL.
func fetchLatestRelease(url string, client *http.Client) (*githubRelease, error) {
	resp, err := client.Get(url)
	if err != nil {
		return nil, fmt.Errorf("fetching release: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("GitHub API returned %d", resp.StatusCode)
	}

	var release githubRelease
	if err := json.NewDecoder(resp.Body).Decode(&release); err != nil {
		return nil, fmt.Errorf("parsing release JSON: %w", err)
	}
	return &release, nil
}

// matchAsset finds an asset whose name matches the given pattern.
func matchAsset(assets []githubAsset, pattern string) (githubAsset, bool) {
	for _, a := range assets {
		if a.Name == pattern {
			return a, true
		}
	}
	return githubAsset{}, false
}

// githubAPIBaseURL is the base URL of the GitHub API. Tests override it to
// point at a mock server.
var githubAPIBaseURL = "https://api.github.com"

// installFromGithub downloads a binary from GitHub Releases, extracts if needed,
// and places it in destDir. If destDir is empty, defaults to ~/.local/bin (global mode).
// If release is nil, the latest release is fetched; otherwise the given release
// (typically resolved beforehand by fetchLatestReleaseMatching) is used as-is, so
// the same release is both version-checked and downloaded.
func installFromGithub(ownerRepo, assetPattern, binaryName, destDir string, release *githubRelease) error {
	client := &http.Client{Timeout: 30 * time.Second}

	if release == nil {
		parts := strings.SplitN(ownerRepo, "/", 2)
		if len(parts) != 2 {
			return fmt.Errorf("invalid owner/repo: %q", ownerRepo)
		}
		owner, repo := parts[0], parts[1]

		apiURL := fmt.Sprintf("%s/repos/%s/%s/releases/latest", githubAPIBaseURL, owner, repo)

		r, err := fetchLatestRelease(apiURL, client)
		if err != nil {
			return fmt.Errorf("fetching release for %s: %w", ownerRepo, err)
		}
		release = r
	}

	asset, ok := matchAsset(release.Assets, assetPattern)
	if !ok {
		return fmt.Errorf("no asset matching %q in latest release (%s)", assetPattern, release.TagName)
	}

	fmt.Printf("  Downloading %s from %s...\n", asset.Name, release.TagName)

	// Download asset to temp file
	tmpDir, err := os.MkdirTemp("", "aide-dl-*")
	if err != nil {
		return fmt.Errorf("creating temp dir: %w", err)
	}
	defer os.RemoveAll(tmpDir)

	assetPath := filepath.Join(tmpDir, asset.Name)
	if err := downloadFile(asset.BrowserDownloadURL, assetPath, client); err != nil {
		return fmt.Errorf("downloading asset: %w", err)
	}

	// Determine destination directory
	binDir := destDir
	isGlobal := binDir == ""
	if isGlobal {
		var err error
		binDir, err = getBinDir()
		if err != nil {
			return fmt.Errorf("getting bin dir: %w", err)
		}
	}

	if err := os.MkdirAll(binDir, 0755); err != nil {
		return fmt.Errorf("creating bin dir %s: %w", binDir, err)
	}

	if isArchive(asset.Name) {
		if err := extractArchive(assetPath, binDir, binaryName); err != nil {
			return fmt.Errorf("extracting archive: %w", err)
		}
	} else {
		// Plain binary — rename directly
		dest := filepath.Join(binDir, binaryName)
		if err := os.Rename(assetPath, dest); err != nil {
			// Fallback: copy
			if err := copyFile(assetPath, dest); err != nil {
				return fmt.Errorf("copying binary: %w", err)
			}
		}
		if err := os.Chmod(dest, 0755); err != nil {
			return fmt.Errorf("setting executable bit: %w", err)
		}
	}

	// Ensure bin dir in PATH (global mode only)
	if isGlobal {
		if err := ensureInPath(binDir); err != nil {
			fmt.Fprintf(os.Stderr, "  Warning: could not add %s to PATH: %v\n", binDir, err)
			fmt.Fprintf(os.Stderr, "  Add it manually to use %s from any terminal.\n", binaryName)
		}
	}

	fmt.Printf("  OK  %s installed to %s\n", binaryName, binDir)
	return nil
}

// isArchive returns true if the filename is a supported archive.
func isArchive(name string) bool {
	return strings.HasSuffix(name, ".zip") || strings.HasSuffix(name, ".tar.gz")
}

// downloadFile downloads a URL to a local file path.
func downloadFile(url, dest string, client *http.Client) error {
	resp, err := client.Get(url)
	if err != nil {
		return fmt.Errorf("downloading %s: %w", url, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("download returned %d", resp.StatusCode)
	}

	f, err := os.Create(dest)
	if err != nil {
		return fmt.Errorf("creating file %s: %w", dest, err)
	}
	defer f.Close()

	if _, err := io.Copy(f, resp.Body); err != nil {
		return fmt.Errorf("writing file: %w", err)
	}
	return nil
}

// extractArchive extracts a binary from a .zip or .tar.gz archive.
func extractArchive(src, destDir, binaryName string) error {
	if strings.HasSuffix(src, ".zip") {
		return extractZip(src, destDir, binaryName)
	}
	if strings.HasSuffix(src, ".tar.gz") {
		return extractTarGz(src, destDir, binaryName)
	}
	return fmt.Errorf("unsupported archive format: %s", src)
}

func extractZip(src, destDir, binaryName string) error {
	r, err := zip.OpenReader(src)
	if err != nil {
		return fmt.Errorf("opening zip: %w", err)
	}
	defer r.Close()

	for _, f := range r.File {
		if filepath.Base(f.Name) == binaryName || f.Name == binaryName {
			return extractZipFile(f, filepath.Join(destDir, binaryName))
		}
	}
	return fmt.Errorf("binary %q not found in zip archive", binaryName)
}

func extractZipFile(f *zip.File, dest string) error {
	rc, err := f.Open()
	if err != nil {
		return fmt.Errorf("opening zip entry: %w", err)
	}
	defer rc.Close()

	out, err := os.OpenFile(dest, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0755)
	if err != nil {
		return fmt.Errorf("creating output file: %w", err)
	}
	defer out.Close()

	if _, err := io.Copy(out, rc); err != nil {
		return fmt.Errorf("extracting: %w", err)
	}
	return nil
}

func extractTarGz(src, destDir, binaryName string) error {
	f, err := os.Open(src)
	if err != nil {
		return fmt.Errorf("opening tar.gz: %w", err)
	}
	defer f.Close()

	gz, err := gzip.NewReader(f)
	if err != nil {
		return fmt.Errorf("reading gzip: %w", err)
	}
	defer gz.Close()

	tr := tar.NewReader(gz)
	for {
		hdr, err := tr.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return fmt.Errorf("reading tar: %w", err)
		}
		if filepath.Base(hdr.Name) == binaryName || hdr.Name == binaryName {
			dest := filepath.Join(destDir, binaryName)
			out, err := os.OpenFile(dest, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, os.FileMode(hdr.Mode))
			if err != nil {
				return fmt.Errorf("creating output file: %w", err)
			}
			defer out.Close()

			if _, err := io.Copy(out, tr); err != nil {
				return fmt.Errorf("extracting: %w", err)
			}
			if err := os.Chmod(dest, 0o755); err != nil {
				return fmt.Errorf("chmod output file: %w", err)
			}
			return nil
		}
	}
	return fmt.Errorf("binary %q not found in tar.gz archive", binaryName)
}

// releasesPerPage is the page size used when paginating through release history.
const releasesPerPage = 100

// fetchReleases fetches all releases for ownerRepo, paginating through
// GET /repos/{owner}/{repo}/releases (newest first, per the GitHub API's default order).
func fetchReleases(ownerRepo string, client *http.Client) ([]githubRelease, error) {
	parts := strings.SplitN(ownerRepo, "/", 2)
	if len(parts) != 2 {
		return nil, fmt.Errorf("invalid owner/repo: %q", ownerRepo)
	}
	owner, repo := parts[0], parts[1]

	var all []githubRelease
	for page := 1; ; page++ {
		apiURL := fmt.Sprintf("%s/repos/%s/%s/releases?per_page=%d&page=%d", githubAPIBaseURL, owner, repo, releasesPerPage, page)

		resp, err := client.Get(apiURL)
		if err != nil {
			return nil, fmt.Errorf("fetching releases: %w", err)
		}
		if resp.StatusCode != http.StatusOK {
			resp.Body.Close()
			return nil, fmt.Errorf("GitHub API returned %d", resp.StatusCode)
		}

		var pageReleases []githubRelease
		err = json.NewDecoder(resp.Body).Decode(&pageReleases)
		resp.Body.Close()
		if err != nil {
			return nil, fmt.Errorf("parsing releases JSON: %w", err)
		}

		all = append(all, pageReleases...)
		if len(pageReleases) < releasesPerPage {
			break
		}
	}
	return all, nil
}

// fetchLatestReleaseMatching walks release history (newest first) to find the
// newest release whose tag satisfies the given semver constraint, skipping
// drafts, prereleases, and tags that aren't valid semver. Returns an error if
// no matching release is found in the entire release history.
func fetchLatestReleaseMatching(ownerRepo, constraint string) (*githubRelease, error) {
	client := &http.Client{Timeout: 30 * time.Second}

	releases, err := fetchReleases(ownerRepo, client)
	if err != nil {
		return nil, fmt.Errorf("fetching releases for %s: %w", ownerRepo, err)
	}

	for i := range releases {
		release := &releases[i]
		if release.Draft || release.Prerelease {
			continue
		}

		tag := strings.TrimPrefix(release.TagName, "v")
		ok, err := semver.CheckConstraint(tag, constraint)
		if err != nil {
			// Tag isn't valid semver (e.g. "nightly") — skip rather than abort the search.
			continue
		}
		if ok {
			return release, nil
		}
	}

	return nil, fmt.Errorf("no release found satisfying constraint %s", constraint)
}

// copyFile copies a file from src to dest.
func copyFile(src, dest string) error {
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()

	out, err := os.Create(dest)
	if err != nil {
		return err
	}
	defer out.Close()

	_, err = io.Copy(out, in)
	return err
}

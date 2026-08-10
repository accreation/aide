# Package Manager Publishing — Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Automatically publish `aide` to Homebrew, Scoop, Winget, Chocolatey, APT, and DNF on every tagged release.

**Architecture:** Shell scripts in `packaging/` render templates with version + SHA256 from `checksums.txt`, then push/publish to each manager. The release workflow gains a `publish` job that runs all scripts in parallel after GitHub Release creation.

**Tech Stack:** bash, envsubst, git, gh CLI, wingetcreate (dotnet tool), choco CLI, apt-ftparchive, createrepo_c, gpg

## Global Constraints

- All publish scripts must be idempotent (re-running with same version is safe)
- Each manager failure must not block other managers
- Tarball binary names: `aide-{goos}-{goarch}` (linux/mac) or `aide-{goos}-{goarch}.exe` (windows)
- Tag format: `vX.Y.Z`, version variable strip `v` prefix from `$GITHUB_REF`
- Checksums file: `aide_vX.Y.Z_checksums.txt`, format: `<sha256>  ./<filename>`
- GPG key: RSA 4096, no expiry, stored as `GPG_PRIVATE_KEY` secret
- Winget PAT stored as `WINGET_GITHUB_TOKEN`, Chocolatey API key as `CHOCOLATEY_API_KEY`
- All artifacts downloaded from the GitHub Release created by the same workflow run

---

### Task 1: Infrastructure — Repos, GPG Key, Secrets

**Files:**
- None (manual setup via gh CLI + gpg)

**Interfaces:**
- Produces: repos `accreation/homebrew-tap`, `accreation/scoop-bucket`, `accreation/aide-repo`
- Produces: GPG key pair, public key at `aide-repo/gpg.key`
- Produces: GitHub Secrets `GPG_PRIVATE_KEY`, `GPG_PASSPHRASE`, `WINGET_GITHUB_TOKEN`, `CHOCOLATEY_API_KEY`, `HOMEBREW_TAP_DEPLOY_KEY`, `SCOOP_BUCKET_DEPLOY_KEY`, `AIDE_REPO_DEPLOY_KEY`

- [ ] **Step 1: Create the three new repos**

```bash
gh repo create accreation/homebrew-tap --public --description "Homebrew tap for Aide"
cd /tmp && git clone git@github.com:accreation/homebrew-tap.git
cd homebrew-tap && mkdir -p Formula
echo "# Accreation Homebrew Tap" > README.md
git add README.md Formula && git commit -m "Initial tap setup"
git push origin main && cd /tmp && rm -rf homebrew-tap

gh repo create accreation/scoop-bucket --public --description "Scoop bucket for Aide"
cd /tmp && git clone git@github.com:accreation/scoop-bucket.git
cd scoop-bucket && mkdir -p bucket
echo "# Accreation Scoop Bucket" > README.md
git add README.md bucket && git commit -m "Initial bucket setup"
git push origin main && cd /tmp && rm -rf scoop-bucket

gh repo create accreation/aide-repo --public --description "APT/DNF repository for Aide"
# Enable GitHub Pages: Settings → Pages → Source: Deploy from a branch → gh-pages
```

- [ ] **Step 2: Generate GPG key**

```bash
gpg --batch --full-generate-key <<EOF
Key-Type: RSA
Key-Length: 4096
Name-Real: Aide Package Signing
Name-Email: aide@accreation.com
Expire-Date: 0
%no-protection
%commit
EOF
```

- [ ] **Step 3: Export keys and initialize gh-pages**

```bash
KEY_ID=$(gpg --list-keys --with-colons "aide@accreation.com" | grep '^pub' | cut -d: -f5)
gpg --armor --export-secret-keys "$KEY_ID" > aide-gpg-private.asc
gpg --armor --export "$KEY_ID" > gpg.key

cd /tmp && git clone git@github.com:accreation/aide-repo.git
cd aide-repo && git checkout --orphan gh-pages && git rm -rf .
cp /path/to/gpg.key . && echo "<h1>Aide Package Repository</h1>" > index.html
git add gpg.key index.html && git commit -m "Initialize gh-pages with GPG public key"
git push origin gh-pages && cd /tmp && rm -rf aide-repo
```

- [ ] **Step 4: Create deploy keys for all three repos**

```bash
# Homebrew tap
ssh-keygen -t ed25519 -f /tmp/homebrew-tap-key -N "" -C "aide-ci"
# Add /tmp/homebrew-tap-key.pub to accreation/homebrew-tap → Settings → Deploy keys (write access)
gh secret set HOMEBREW_TAP_DEPLOY_KEY --repo accreation/aide < /tmp/homebrew-tap-key

# Scoop bucket
ssh-keygen -t ed25519 -f /tmp/scoop-bucket-key -N "" -C "aide-ci"
gh secret set SCOOP_BUCKET_DEPLOY_KEY --repo accreation/aide < /tmp/scoop-bucket-key

# Aide repo
ssh-keygen -t ed25519 -f /tmp/aide-repo-key -N "" -C "aide-ci"
gh secret set AIDE_REPO_DEPLOY_KEY --repo accreation/aide < /tmp/aide-repo-key
```

- [ ] **Step 5: Set remaining secrets**

```bash
gh secret set GPG_PRIVATE_KEY --repo accreation/aide < aide-gpg-private.asc
gh secret set GPG_PASSPHRASE --repo accreation/aide --body ""
gh secret set WINGET_GITHUB_TOKEN --repo accreation/aide --body "github_pat_..."
gh secret set CHOCOLATEY_API_KEY --repo accreation/aide --body "choco_..."

rm aide-gpg-private.asc gpg.key /tmp/*-key /tmp/*-key.pub
```

- [ ] **Step 6: Commit**

```bash
git add -A && git commit -m "chore: document package manager infrastructure setup"
```

### Task 2: Template Files

**Files:**
- Create: `packaging/homebrew/aide.rb.tmpl`
- Create: `packaging/scoop/aide.json.tmpl`
- Create: `packaging/chocolatey/aide.nuspec.tmpl`
- Create: `packaging/chocolatey/tools/chocolateyInstall.ps1.tmpl`
- Create: `packaging/apt/apt-release.conf`
- Create: `packaging/dnf/aide.repo`

**Interfaces:**
- Produces: Template files with `$VERSION`, `$SHA_*` variables consumed by `render.sh` in Task 3

- [ ] **Step 1: Create directory structure**

```bash
mkdir -p packaging/homebrew packaging/scoop packaging/chocolatey/tools packaging/apt packaging/dnf
```

- [ ] **Step 2: Write Homebrew formula template**

Create `packaging/homebrew/aide.rb.tmpl`:

```ruby
class Aide < Formula
  desc "The package.json for your AI development environment"
  homepage "https://github.com/accreation/aide"
  version "$VERSION"
  license "AGPL-3.0"

  if OS.mac?
    if Hardware::CPU.arm?
      url "https://github.com/accreation/aide/releases/download/v$VERSION/aide-darwin-arm64.tar.gz"
      sha256 "$SHA_DARWIN_ARM64"
    else
      url "https://github.com/accreation/aide/releases/download/v$VERSION/aide-darwin-amd64.tar.gz"
      sha256 "$SHA_DARWIN_AMD64"
    end
  elsif OS.linux?
    if Hardware::CPU.arm?
      url "https://github.com/accreation/aide/releases/download/v$VERSION/aide-linux-arm64.tar.gz"
      sha256 "$SHA_LINUX_ARM64"
    else
      url "https://github.com/accreation/aide/releases/download/v$VERSION/aide-linux-amd64.tar.gz"
      sha256 "$SHA_LINUX_AMD64"
    end
  end

  def install
    bin.install Dir["aide-*"].first => "aide"
  end

  test do
    system "#{bin}/aide", "--version"
  end
end
```

- [ ] **Step 3: Write Scoop manifest template**

Create `packaging/scoop/aide.json.tmpl`:

```json
{
  "version": "$VERSION",
  "description": "The package.json for your AI development environment",
  "homepage": "https://github.com/accreation/aide",
  "license": "AGPL-3.0",
  "architecture": {
    "64bit": {
      "url": "https://github.com/accreation/aide/releases/download/v$VERSION/aide-windows-amd64.exe.zip",
      "hash": "sha256:$SHA_WINDOWS_AMD64"
    },
    "arm64": {
      "url": "https://github.com/accreation/aide/releases/download/v$VERSION/aide-windows-arm64.exe.zip",
      "hash": "sha256:$SHA_WINDOWS_ARM64"
    }
  },
  "bin": [["aide-windows-amd64.exe", "aide"]],
  "checkver": "github",
  "autoupdate": {
    "architecture": {
      "64bit": {
        "url": "https://github.com/accreation/aide/releases/download/v$version/aide-windows-amd64.exe.zip"
      },
      "arm64": {
        "url": "https://github.com/accreation/aide/releases/download/v$version/aide-windows-arm64.exe.zip"
      }
    }
  }
}
```

- [ ] **Step 4: Write Chocolatey nuspec template**

Create `packaging/chocolatey/aide.nuspec.tmpl`:

```xml
<?xml version="1.0" encoding="utf-8"?>
<package xmlns="http://schemas.microsoft.com/packaging/2015/06/nuspec.xsd">
  <metadata>
    <id>aide</id>
    <version>$VERSION</version>
    <title>Aide</title>
    <authors>Accreation</authors>
    <projectUrl>https://github.com/accreation/aide</projectUrl>
    <license type="expression">AGPL-3.0</license>
    <description>The package.json for your AI development environment</description>
    <releaseNotes>https://github.com/accreation/aide/releases/tag/v$VERSION</releaseNotes>
    <requireLicenseAcceptance>false</requireLicenseAcceptance>
  </metadata>
  <files>
    <file src="tools\**" target="tools" />
  </files>
</package>
```

- [ ] **Step 5: Write Chocolatey install script template**

Create `packaging/chocolatey/tools/chocolateyInstall.ps1.tmpl`:

```powershell
$ErrorActionPreference = 'Stop'
$packageName = 'aide'
$url64 = 'https://github.com/accreation/aide/releases/download/v$VERSION/aide-windows-amd64.exe.zip'
$checksum64 = '$SHA_WINDOWS_AMD64'

$binDir = "$env:USERPROFILE\.local\bin"
New-Item -ItemType Directory -Force -Path $binDir | Out-Null

$packageArgs = @{
  packageName    = $packageName
  url64bit       = $url64
  checksum64     = $checksum64
  checksumType64 = 'sha256'
  unzipLocation  = $binDir
}

Install-ChocolateyZipPackage @packageArgs

$extracted = Get-ChildItem "$binDir\aide-windows-*.exe" | Select-Object -First 1
Move-Item $extracted.FullName "$binDir\aide.exe" -Force

Install-ChocolateyPath $binDir 'User'
```

- [ ] **Step 6: Write APT release config**

Create `packaging/apt/apt-release.conf`:

```
APT::FTPArchive::Release::Origin "Accreation";
APT::FTPArchive::Release::Label "Aide";
APT::FTPArchive::Release::Suite "stable";
APT::FTPArchive::Release::Codename "stable";
APT::FTPArchive::Release::Architectures "amd64 arm64";
APT::FTPArchive::Release::Components "main";
APT::FTPArchive::Release::Description "Aide — AI environment manager";
```

- [ ] **Step 7: Write DNF repo file**

Create `packaging/dnf/aide.repo`:

```ini
[aide]
name=Aide — AI environment manager
baseurl=https://accreation.github.io/aide-repo/aide
enabled=1
gpgcheck=1
gpgkey=https://accreation.github.io/aide-repo/gpg.key
```

- [ ] **Step 8: Commit**

```bash
git add packaging/
git commit -m "feat: add package manager templates"
```

### Task 3: Shared Render Script

**Files:**
- Create: `packaging/render.sh`

**Interfaces:**
- Consumes: `checksums.txt` path + `$VERSION`
- Produces: Exported env vars `$VERSION`, `$SHA_DARWIN_AMD64`, `$SHA_DARWIN_ARM64`, `$SHA_LINUX_AMD64`, `$SHA_LINUX_ARM64`, `$SHA_WINDOWS_AMD64`, `$SHA_WINDOWS_ARM64`, `$SHA_DEB_AMD64`, `$SHA_DEB_ARM64`, `$SHA_RPM_AMD64`, `$SHA_RPM_ARM64`
- Produces: Function `render_template <tmpl> <out>` using `envsubst`

- [ ] **Step 1: Write render.sh**

Create `packaging/render.sh`:

```bash
#!/usr/bin/env bash
set -euo pipefail
# render.sh — Parse checksums.txt and provide template rendering.
# Usage: source packaging/render.sh <checksums.txt> <version>

CHKSUM_FILE="${1:-}"
VERSION="${2:-}"

if [[ -z "$CHKSUM_FILE" || -z "$VERSION" ]]; then
  echo "Usage: source render.sh <checksums.txt> <version>" >&2
  exit 1
fi

parse_checksum() {
  local hash
  hash=$(grep -F "$1" "$CHKSUM_FILE" | awk '{print $1}')
  if [[ -z "$hash" ]]; then
    echo "ERROR: checksum not found for $1 in $CHKSUM_FILE" >&2
    exit 1
  fi
  echo "$hash"
}

SHA_DARWIN_AMD64=$(parse_checksum "aide-darwin-amd64.tar.gz")
SHA_DARWIN_ARM64=$(parse_checksum "aide-darwin-arm64.tar.gz")
SHA_LINUX_AMD64=$(parse_checksum "aide-linux-amd64.tar.gz")
SHA_LINUX_ARM64=$(parse_checksum "aide-linux-arm64.tar.gz")
SHA_WINDOWS_AMD64=$(parse_checksum "aide-windows-amd64.exe.zip")
SHA_WINDOWS_ARM64=$(parse_checksum "aide-windows-arm64.exe.zip")
SHA_DEB_AMD64=$(parse_checksum "aide_${VERSION}_amd64.deb")
SHA_DEB_ARM64=$(parse_checksum "aide_${VERSION}_arm64.deb")
SHA_RPM_AMD64=$(parse_checksum "aide-${VERSION}-1.amd64.rpm")
SHA_RPM_ARM64=$(parse_checksum "aide-${VERSION}-1.arm64.rpm")

export VERSION SHA_DARWIN_AMD64 SHA_DARWIN_ARM64 SHA_LINUX_AMD64 SHA_LINUX_ARM64
export SHA_WINDOWS_AMD64 SHA_WINDOWS_ARM64 SHA_DEB_AMD64 SHA_DEB_ARM64 SHA_RPM_AMD64 SHA_RPM_ARM64

render_template() {
  envsubst < "$1" > "$2"
  echo "  Rendered $2"
}
```

- [ ] **Step 2: Test locally**

```bash
cat > /tmp/test-checksums.txt << 'EOF'
abc123  ./aide-darwin-amd64.tar.gz
def456  ./aide-darwin-arm64.tar.gz
111aaa  ./aide-linux-amd64.tar.gz
222bbb  ./aide-linux-arm64.tar.gz
333ccc  ./aide-windows-amd64.exe.zip
444ddd  ./aide-windows-arm64.exe.zip
555eee  ./aide_0.5.0_amd64.deb
666fff  ./aide_0.5.0_arm64.deb
777ggg  ./aide-0.5.0-1.amd64.rpm
888hhh  ./aide-0.5.0-1.arm64.rpm
EOF

source packaging/render.sh /tmp/test-checksums.txt "0.5.0"
echo "VERSION=$VERSION"
echo "SHA_DARWIN_AMD64=$SHA_DARWIN_AMD64"
# Expected: VERSION=0.5.0, SHA_DARWIN_AMD64=abc123
```

- [ ] **Step 3: Commit**

```bash
git add packaging/render.sh
git commit -m "feat: add shared render script for package templates"
```

### Task 4: Homebrew Publish Script

**Files:**
- Create: `packaging/publish-homebrew.sh`

**Interfaces:**
- Consumes: `render.sh`, `packaging/homebrew/aide.rb.tmpl`
- Produces: Pushed commit to `accreation/homebrew-tap`
- Requires: `HOMEBREW_TAP_DEPLOY_KEY` secret

- [ ] **Step 1: Write publish-homebrew.sh**

Create `packaging/publish-homebrew.sh`:

```bash
#!/usr/bin/env bash
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
CHKSUM_FILE="$(ls ${CHKSUM_FILE:-artifacts/aide_*_checksums.txt} 2>/dev/null | head -1)"

if [[ ! -f "$CHKSUM_FILE" ]]; then
  echo "ERROR: checksums file not found" >&2
  exit 1
fi

source "$SCRIPT_DIR/render.sh" "$CHKSUM_FILE" "$VERSION"

echo "=== Publishing to Homebrew Tap ==="

TAP_DIR=$(mktemp -d)
git clone --depth 1 "https://x-access-token:${HOMEBREW_TAP_DEPLOY_KEY}@github.com/accreation/homebrew-tap.git" "$TAP_DIR"
mkdir -p "$TAP_DIR/Formula"
render_template "$SCRIPT_DIR/homebrew/aide.rb.tmpl" "$TAP_DIR/Formula/aide.rb"

cd "$TAP_DIR"
if git diff --quiet; then
  echo "  Formula unchanged (version $VERSION already published)"
else
  git config user.email "ci@accreation.com"
  git config user.name "Aide CI"
  git add Formula/aide.rb
  git commit -m "aide $VERSION"
  git push origin main
  echo "  OK  Homebrew formula updated to $VERSION"
fi

rm -rf "$TAP_DIR"
```

- [ ] **Step 2: Commit**

```bash
git add packaging/publish-homebrew.sh
git commit -m "feat: add Homebrew publish script"
```

### Task 5: Scoop Publish Script

**Files:**
- Create: `packaging/publish-scoop.sh`

**Interfaces:**
- Consumes: `render.sh`, `packaging/scoop/aide.json.tmpl`
- Produces: Pushed commit to `accreation/scoop-bucket`
- Requires: `SCOOP_BUCKET_DEPLOY_KEY` secret

- [ ] **Step 1: Write publish-scoop.sh**

Create `packaging/publish-scoop.sh`:

```bash
#!/usr/bin/env bash
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
CHKSUM_FILE="$(ls ${CHKSUM_FILE:-artifacts/aide_*_checksums.txt} 2>/dev/null | head -1)"

if [[ ! -f "$CHKSUM_FILE" ]]; then
  echo "ERROR: checksums file not found" >&2
  exit 1
fi

source "$SCRIPT_DIR/render.sh" "$CHKSUM_FILE" "$VERSION"

echo "=== Publishing to Scoop Bucket ==="

BUCKET_DIR=$(mktemp -d)
git clone --depth 1 "https://x-access-token:${SCOOP_BUCKET_DEPLOY_KEY}@github.com/accreation/scoop-bucket.git" "$BUCKET_DIR"
mkdir -p "$BUCKET_DIR/bucket"
render_template "$SCRIPT_DIR/scoop/aide.json.tmpl" "$BUCKET_DIR/bucket/aide.json"

cd "$BUCKET_DIR"
if git diff --quiet; then
  echo "  Manifest unchanged (version $VERSION already published)"
else
  git config user.email "ci@accreation.com"
  git config user.name "Aide CI"
  git add bucket/aide.json
  git commit -m "aide $VERSION"
  git push origin main
  echo "  OK  Scoop manifest updated to $VERSION"
fi

rm -rf "$BUCKET_DIR"
```

- [ ] **Step 2: Commit**

```bash
git add packaging/publish-scoop.sh
git commit -m "feat: add Scoop publish script"
```

### Task 6: Winget Publish Script

**Files:**
- Create: `packaging/publish-winget.sh`

**Interfaces:**
- Consumes: `$VERSION`, `$WINGET_GITHUB_TOKEN`
- Produces: PR to `microsoft/winget-pkgs`
- Note: First submission is manual; subsequent updates are automated

- [ ] **Step 1: Write publish-winget.sh**

Create `packaging/publish-winget.sh`:

```bash
#!/usr/bin/env bash
set -euo pipefail

# publish-winget.sh — Submit winget manifest update via wingetcreate.
# Required env: VERSION, WINGET_GITHUB_TOKEN

echo "=== Publishing to Winget ==="

# Install wingetcreate
dotnet tool install --global Microsoft.WingetCreate 2>/dev/null || true
export PATH="$HOME/.dotnet/tools:$PATH"

ZIP_URL="https://github.com/accreation/aide/releases/download/v${VERSION}/aide-windows-amd64.exe.zip"

wingetcreate update "Accreation.Aide" \
  --version "$VERSION" \
  --urls "$ZIP_URL" \
  --token "$WINGET_GITHUB_TOKEN" \
  --submit

echo "  OK  Winget PR submitted for $VERSION"
```

- [ ] **Step 2: Commit**

```bash
git add packaging/publish-winget.sh
git commit -m "feat: add Winget publish script"
```

### Task 7: Chocolatey Publish Script

**Files:**
- Create: `packaging/publish-choco.sh`

**Interfaces:**
- Consumes: `render.sh`, `$VERSION`, `$CHOCOLATEY_API_KEY`
- Consumes: `packaging/chocolatey/aide.nuspec.tmpl`, `packaging/chocolatey/tools/chocolateyInstall.ps1.tmpl`
- Produces: Pushed package to chocolatey.org

- [ ] **Step 1: Write publish-choco.sh**

Create `packaging/publish-choco.sh`:

```bash
#!/usr/bin/env bash
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
CHKSUM_FILE="$(ls ${CHKSUM_FILE:-artifacts/aide_*_checksums.txt} 2>/dev/null | head -1)"

if [[ ! -f "$CHKSUM_FILE" ]]; then
  echo "ERROR: checksums file not found" >&2
  exit 1
fi

source "$SCRIPT_DIR/render.sh" "$CHKSUM_FILE" "$VERSION"

echo "=== Publishing to Chocolatey ==="

# Build package in temp dir
BUILD_DIR=$(mktemp -d)
cp -r "$SCRIPT_DIR/chocolatey/"* "$BUILD_DIR/"
render_template "$SCRIPT_DIR/chocolatey/aide.nuspec.tmpl" "$BUILD_DIR/aide.nuspec"
render_template "$SCRIPT_DIR/chocolatey/tools/chocolateyInstall.ps1.tmpl" "$BUILD_DIR/tools/chocolateyInstall.ps1"

cd "$BUILD_DIR"
choco pack aide.nuspec --outputdirectory .

# Push (skip if dev/test)
if [[ "${DRY_RUN:-0}" = "1" ]]; then
  echo "  DRY_RUN: skipping choco push"
else
  choco push aide.${VERSION}.nupkg --api-key "$CHOCOLATEY_API_KEY" --source https://push.chocolatey.org/
  echo "  OK  Chocolatey package $VERSION pushed"
fi

rm -rf "$BUILD_DIR"
```

- [ ] **Step 2: Commit**

```bash
git add packaging/publish-choco.sh
git commit -m "feat: add Chocolatey publish script"
```

### Task 8: APT/DNF Publish Script

**Files:**
- Create: `packaging/publish-apt-dnf.sh`

**Interfaces:**
- Consumes: `$VERSION`, `$GPG_PRIVATE_KEY`, `$GPG_PASSPHRASE`, `$AIDE_REPO_DEPLOY_KEY`
- Consumes: `packaging/apt/apt-release.conf`, `packaging/dnf/aide.repo`
- Produces: Updated APT + DNF repo on `accreation/aide-repo` gh-pages branch
- Downloads: `.deb` and `.rpm` packages from the GitHub Release

- [ ] **Step 1: Write publish-apt-dnf.sh**

Create `packaging/publish-apt-dnf.sh`:

```bash
#!/usr/bin/env bash
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"

echo "=== Publishing to APT/DNF Repository ==="

# ── Setup GPG ───────────────────────────────────────────────────────
export GPG_TTY=$(tty)
if [[ -n "${GPG_PRIVATE_KEY:-}" ]]; then
  echo "$GPG_PRIVATE_KEY" | gpg --batch --import
fi

# ── Clone aide-repo gh-pages ────────────────────────────────────────
REPO_DIR=$(mktemp -d)
git clone --depth 1 --branch gh-pages \
  "https://x-access-token:${AIDE_REPO_DEPLOY_KEY}@github.com/accreation/aide-repo.git" "$REPO_DIR"

# ── Download .deb packages from GitHub Release ──────────────────────
DEB_AMD64_URL="https://github.com/accreation/aide/releases/download/v${VERSION}/aide_${VERSION}_amd64.deb"
DEB_ARM64_URL="https://github.com/accreation/aide/releases/download/v${VERSION}/aide_${VERSION}_arm64.deb"
RPM_AMD64_URL="https://github.com/accreation/aide/releases/download/v${VERSION}/aide-${VERSION}-1.amd64.rpm"
RPM_ARM64_URL="https://github.com/accreation/aide/releases/download/v${VERSION}/aide-${VERSION}-1.arm64.rpm"

# APT: pool structure
mkdir -p "$REPO_DIR/pool/main/a/aide"
curl -fsSL "$DEB_AMD64_URL" -o "$REPO_DIR/pool/main/a/aide/aide_${VERSION}_amd64.deb"
curl -fsSL "$DEB_ARM64_URL" -o "$REPO_DIR/pool/main/a/aide/aide_${VERSION}_arm64.deb"

# ── Generate APT metadata ───────────────────────────────────────────
mkdir -p "$REPO_DIR/dists/stable/main/binary-amd64"
mkdir -p "$REPO_DIR/dists/stable/main/binary-arm64"

apt-ftparchive packages "$REPO_DIR/pool/main" > "$REPO_DIR/dists/stable/main/binary-amd64/Packages"
cp "$REPO_DIR/dists/stable/main/binary-amd64/Packages" "$REPO_DIR/dists/stable/main/binary-arm64/Packages"
gzip -kf "$REPO_DIR/dists/stable/main/binary-amd64/Packages"
gzip -kf "$REPO_DIR/dists/stable/main/binary-arm64/Packages"

apt-ftparchive -c "$SCRIPT_DIR/apt/apt-release.conf" release "$REPO_DIR/dists/stable" > "$REPO_DIR/dists/stable/Release"

# GPG sign: InRelease = clearsigned Release
gpg --batch --yes --clearsign -o "$REPO_DIR/dists/stable/InRelease" "$REPO_DIR/dists/stable/Release"

# ── DNF: copy .rpm and generate metadata ────────────────────────────
mkdir -p "$REPO_DIR/aide/x86_64"
mkdir -p "$REPO_DIR/aide/aarch64"
curl -fsSL "$RPM_AMD64_URL" -o "$REPO_DIR/aide/x86_64/aide-${VERSION}-1.amd64.rpm"
curl -fsSL "$RPM_ARM64_URL" -o "$REPO_DIR/aide/aarch64/aide-${VERSION}-1.arm64.rpm"

createrepo_c "$REPO_DIR/aide"

# GPG sign repomd.xml
gpg --batch --yes --detach-sign --armor "$REPO_DIR/aide/repodata/repomd.xml"

# ── Copy static files (first time only, idempotent) ─────────────────
cp -n "$SCRIPT_DIR/dnf/aide.repo" "$REPO_DIR/aide.repo" 2>/dev/null || true
cp -n "$SCRIPT_DIR/dnf/aide.repo" "$REPO_DIR/" 2>/dev/null || true

# ── Commit and push ─────────────────────────────────────────────────
cd "$REPO_DIR"
git config user.email "ci@accreation.com"
git config user.name "Aide CI"
git add -A
if git diff --cached --quiet; then
  echo "  Repo unchanged (version $VERSION already published)"
else
  git commit -m "aide $VERSION"
  git push origin gh-pages
  echo "  OK  APT/DNF repo updated to $VERSION"
fi

rm -rf "$REPO_DIR"
```

- [ ] **Step 2: Commit**

```bash
git add packaging/publish-apt-dnf.sh
git commit -m "feat: add APT/DNF publish script"
```

### Task 9: Release Workflow Update

**Files:**
- Modify: `.github/workflows/release.yml`

**Interfaces:**
- Consumes: All `packaging/publish-*.sh` scripts from Tasks 4-8
- Produces: New `publish` job with parallel manager steps

- [ ] **Step 1: Add publish job to release.yml**

Insert after the `release` job in `.github/workflows/release.yml`:

```yaml
  # ── Publish to package managers ──────────────────────────────────
  publish:
    name: Publish to package managers
    runs-on: ubuntu-latest
    needs: release
    strategy:
      fail-fast: false
      matrix:
        manager: [homebrew, scoop, winget, chocolatey, apt-dnf]
    steps:
      - uses: actions/checkout@v4

      - name: Set version
        id: version
        run: echo "VERSION=${GITHUB_REF#refs/tags/v}" >> $GITHUB_OUTPUT

      - name: Download checksums
        uses: actions/download-artifact@v4
        with:
          name: checksums
          path: artifacts

      - name: Install dependencies (apt-dnf)
        if: matrix.manager == 'apt-dnf'
        run: |
          sudo apt-get update -qq
          sudo apt-get install -y -qq createrepo-c apt-utils

      - name: Install dependencies (winget)
        if: matrix.manager == 'winget'
        uses: actions/setup-dotnet@v4
        with:
          dotnet-version: '8.x'

      - name: Install dependencies (chocolatey)
        if: matrix.manager == 'chocolatey'
        run: |
          # Install Mono (required by choco on Linux)
          sudo apt-get install -y -qq mono-complete

      - name: Publish (Homebrew)
        if: matrix.manager == 'homebrew'
        env:
          VERSION: ${{ steps.version.outputs.VERSION }}
          CHKSUM_FILE: artifacts/aide_*_checksums.txt
          HOMEBREW_TAP_DEPLOY_KEY: ${{ secrets.HOMEBREW_TAP_DEPLOY_KEY }}
        run: bash packaging/publish-homebrew.sh

      - name: Publish (Scoop)
        if: matrix.manager == 'scoop'
        env:
          VERSION: ${{ steps.version.outputs.VERSION }}
          CHKSUM_FILE: artifacts/aide_*_checksums.txt
          SCOOP_BUCKET_DEPLOY_KEY: ${{ secrets.SCOOP_BUCKET_DEPLOY_KEY }}
        run: bash packaging/publish-scoop.sh

      - name: Publish (Winget)
        if: matrix.manager == 'winget'
        env:
          VERSION: ${{ steps.version.outputs.VERSION }}
          WINGET_GITHUB_TOKEN: ${{ secrets.WINGET_GITHUB_TOKEN }}
        run: bash packaging/publish-winget.sh

      - name: Publish (Chocolatey)
        if: matrix.manager == 'chocolatey'
        env:
          VERSION: ${{ steps.version.outputs.VERSION }}
          CHKSUM_FILE: artifacts/aide_*_checksums.txt
          CHOCOLATEY_API_KEY: ${{ secrets.CHOCOLATEY_API_KEY }}
        run: bash packaging/publish-choco.sh

      - name: Publish (APT/DNF)
        if: matrix.manager == 'apt-dnf'
        env:
          VERSION: ${{ steps.version.outputs.VERSION }}
          GPG_PRIVATE_KEY: ${{ secrets.GPG_PRIVATE_KEY }}
          GPG_PASSPHRASE: ${{ secrets.GPG_PASSPHRASE }}
          AIDE_REPO_DEPLOY_KEY: ${{ secrets.AIDE_REPO_DEPLOY_KEY }}
        run: bash packaging/publish-apt-dnf.sh
```

- [ ] **Step 2: Commit**

```bash
git add .github/workflows/release.yml
git commit -m "feat: add package manager publish job to release workflow"
```

### Task 10: README Update — Installation from All Managers

**Files:**
- Modify: `README.md`

**Interfaces:**
- Consumes: none
- Produces: Installation instructions for all 6 managers in README

- [ ] **Step 1: Update the Install Aide section with all managers**

Replace the "Install Aide" section in `README.md` (lines 45-70) with:

```markdown
### 1. Install Aide

#### macOS
```bash
brew install accreation/tap/aide
```

#### Linux
```bash
# APT (Debian / Ubuntu)
curl -fsSL https://accreation.github.io/aide-repo/gpg.key | sudo gpg --dearmor -o /etc/apt/trusted.gpg.d/aide.gpg
echo "deb https://accreation.github.io/aide-repo stable main" | sudo tee /etc/apt/sources.list.d/aide.list
sudo apt update && sudo apt install aide-cli

# DNF (Fedora / RHEL)
sudo dnf config-manager --add-repo https://accreation.github.io/aide-repo/aide.repo
sudo dnf install aide-cli

# Portable binary (any distro)
curl -fsSL https://github.com/accreation/aide/releases/latest/download/aide-linux-amd64.tar.gz | tar -xz
sudo mv aide-linux-amd64 /usr/local/bin/aide
```

#### Windows
```powershell
# Winget
winget install Accreation.Aide

# Chocolatey
choco install aide

# Scoop
scoop bucket add accreation https://github.com/accreation/scoop-bucket
scoop install aide

# One-liner installer (recommended if no package manager)
powershell -c "irm https://raw.githubusercontent.com/accreation/aide/main/install.ps1 | iex"
```

#### Go Install
```bash
go install github.com/accreation/aide@latest
```
```

- [ ] **Step 2: Remove the npm-like mention for the old Linux instructions**

Verify no stale curl/dpkg instructions remain outside the new section.

- [ ] **Step 3: Commit**

```bash
git add README.md
git commit -m "docs: add install instructions for all package managers"
```

### Task 11: Winget First Submission (Manual)

**Files:**
- None (manual via `wingetcreate`)

**Interfaces:**
- Produces: Accepted winget manifest in `microsoft/winget-pkgs`

- [ ] **Step 1: Create initial winget manifest**

```bash
dotnet tool install --global Microsoft.WingetCreate
wingetcreate new \
  --id Accreation.Aide \
  --version 0.4.0 \
  --urls "https://github.com/accreation/aide/releases/download/v0.4.0/aide-windows-amd64.exe.zip" \
  --package-locale en-US \
  --publisher Accreation \
  --name Aide \
  --description "The package.json for your AI development environment"

# Submit the PR
wingetcreate submit --token $WINGET_GITHUB_TOKEN
```

- [ ] **Step 2: Wait for Microsoft moderation (1-3 days)**

Check: https://github.com/microsoft/winget-pkgs/pulls?q=Accreation.Aide

- [ ] **Step 3: Verify accepted, then future releases auto-update via Task 6/9**

---

## Task Summary

| # | Task | Dependencies | Est. time |
|---|------|-------------|-----------|
| 1 | Infrastructure setup | None | 30 min |
| 2 | Template files | None | 15 min |
| 3 | Shared render script | None | 10 min |
| 4 | Homebrew publish script | 2, 3 | 10 min |
| 5 | Scoop publish script | 2, 3 | 10 min |
| 6 | Winget publish script | None | 10 min |
| 7 | Chocolatey publish script | 2, 3 | 10 min |
| 8 | APT/DNF publish script | 2, 3 | 20 min |
| 9 | Release workflow update | 4-8 | 15 min |
| 10 | README update | None | 10 min |
| 11 | Winget first submission | None | 30 min (manual) |

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

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

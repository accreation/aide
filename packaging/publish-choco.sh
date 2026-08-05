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
render_template "$SCRIPT_DIR/chocolatey/tools/chocolateyInstall.ps1.tmpl" "$BUILD_DIR/tools/chocolateyInstall.ps1" '$VERSION $SHA_WINDOWS_AMD64'

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

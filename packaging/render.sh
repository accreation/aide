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
  local tmpl="$1" out="$2" fmt="${3:-}"
  if [[ -n "$fmt" ]]; then
    # SHELL-FORMAT restricts substitution to listed variables so
    # template-native variables (e.g. PowerShell $var) survive.
    envsubst "$fmt" < "$tmpl" > "$out"
  else
    envsubst < "$tmpl" > "$out"
  fi
  echo "  Rendered $out"
}

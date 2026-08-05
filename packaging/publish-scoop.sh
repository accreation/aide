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

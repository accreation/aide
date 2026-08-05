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
GIT_SSH_KEY=$(mktemp)
echo "$SCOOP_BUCKET_DEPLOY_KEY" > "$GIT_SSH_KEY"
chmod 600 "$GIT_SSH_KEY"
export GIT_SSH_COMMAND="ssh -i $GIT_SSH_KEY -o StrictHostKeyChecking=accept-new"
git clone --depth 1 "git@github.com:accreation/scoop-bucket.git" "$BUCKET_DIR"
mkdir -p "$BUCKET_DIR/bucket"
render_template "$SCRIPT_DIR/scoop/aide.json.tmpl" "$BUCKET_DIR/bucket/aide.json" '$VERSION $SHA_WINDOWS_AMD64 $SHA_WINDOWS_ARM64'

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

rm -f "$GIT_SSH_KEY"
rm -rf "$BUCKET_DIR"

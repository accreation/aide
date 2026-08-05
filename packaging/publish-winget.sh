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

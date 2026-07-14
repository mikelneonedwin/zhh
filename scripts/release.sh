#!/usr/bin/env bash
set -euo pipefail

APP="zhh"
REPO="mikelneonedwin/zhh"

usage() {
  echo "Usage: $0 <version>"
  echo "  version  Git tag / release version (e.g. v0.1.0)"
  exit 1
}

VERSION="${1:?$(usage)}"

# ---- Build matrix ----
builds=(
  "linux,amd64,linux,amd64"
  "linux,arm64,linux,arm64"
  "darwin,amd64,darwin,amd64"
  "darwin,arm64,darwin,arm64"
  "windows,amd64,windows,amd64"
)

workdir="$(mktemp -d)"
echo "Working in $workdir"

for entry in "${builds[@]}"; do
  IFS=',' read -r goos goarch plat arch <<<"$entry"

  ext=""
  if [ "$goos" = "windows" ]; then
    ext=".exe"
  fi

  binary="${APP}${ext}"
  archive="${APP}_${VERSION}_${plat}_${arch}.tar.gz"
  zip_archive="${APP}_${VERSION}_${plat}_${arch}.zip"

  echo "--- Building $goos/$goarch ---"
  GOOS="$goos" GOARCH="$goarch" CGO_ENABLED=0 go build -o "$workdir/$binary" .

  cd "$workdir"
  if [ "$goos" = "windows" ]; then
    zip "$zip_archive" "$binary" >/dev/null
    echo "  => $zip_archive"
  else
    tar czf "$archive" "$binary"
    echo "  => $archive"
  fi
  rm -f "$binary"
  cd - >/dev/null
done

echo ""
echo "=== Creating GitHub release $VERSION ==="
gh release create "$VERSION" \
  --repo "$REPO" \
  --title "$APP $VERSION" \
  --notes "Release $VERSION of $APP" \
  "$workdir"/*.tar.gz "$workdir"/*.zip

echo "=== Cleaning up ==="
rm -rf "$workdir"

echo "Done: https://github.com/$REPO/releases/tag/$VERSION"

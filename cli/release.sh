#!/usr/bin/env bash
# Package one standalone CLI target. The workflow supplies release metadata.
set -euo pipefail

: "${VERSION:?Set VERSION to the release tag}"
: "${COMMIT:?Set COMMIT to the full commit SHA}"
: "${GOOS:?Set GOOS to the target operating system}"
: "${GOARCH:?Set GOARCH to the target architecture}"

if [[ ! "$VERSION" =~ ^v[0-9][0-9A-Za-z.+-]*$ || ! "$COMMIT" =~ ^[0-9a-f]{40}$ ]]; then
  echo 'Invalid release tag or commit SHA.' >&2
  exit 1
fi
case "$GOOS/$GOARCH" in
  darwin/amd64|darwin/arm64|linux/amd64|linux/arm64|windows/amd64|windows/arm64) ;;
  *) echo "Unsupported CLI target: $GOOS/$GOARCH" >&2; exit 1 ;;
esac

repo_root=$(cd "$(dirname "$0")/.." && pwd)
output_dir=${1:-"$repo_root/.data/cli-release"}
mkdir -p "$output_dir"
output_dir=$(cd "$output_dir" && pwd)
stage=$(mktemp -d "${TMPDIR:-/tmp}/ircordctl-release.XXXXXX")
trap 'rm -rf "$stage"' EXIT

binary=ircordctl
if [[ "$GOOS" == windows ]]; then
  binary+=.exe
fi
(
  cd "$repo_root/cli"
  CGO_ENABLED=0 GOOS="$GOOS" GOARCH="$GOARCH" go build -mod=readonly -trimpath \
    -ldflags "-s -w -X ircord/cli/internal/buildinfo.Version=$VERSION -X ircord/cli/internal/buildinfo.Commit=$COMMIT" \
    -o "$stage/$binary" ./cmd/ircordctl
)
cp "$repo_root/LICENSE" "$stage/LICENSE"
chmod 755 "$stage/$binary"
chmod 644 "$stage/LICENSE"

asset="ircordctl_${VERSION}_${GOOS}_${GOARCH}"
if [[ "$GOOS" == windows ]]; then
  asset+=.zip
  (cd "$stage" && zip -X -q "$asset" "$binary" LICENSE)
else
  asset+=.tar.gz
  COPYFILE_DISABLE=1 tar -czf "$stage/$asset" -C "$stage" "$binary" LICENSE
fi
mv "$stage/$asset" "$output_dir/$asset"
echo "Packaged $asset"

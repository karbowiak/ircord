#!/usr/bin/env bash
# Publish only after the complete CLI matrix succeeds. Never creates a git tag.
set -euo pipefail

: "${RELEASE_TAG:?Set RELEASE_TAG to the existing version tag}"
: "${GH_REPO:?Set GH_REPO to owner/repository}"
if [[ ! "$RELEASE_TAG" =~ ^v[0-9][0-9A-Za-z.+-]*$ ]]; then
  echo 'Invalid release tag.' >&2
  exit 1
fi
# The user-facing notes of this version, from the changelog of the tagged commit.
changelog=${2:-CHANGELOG.md}
notes=$(node "$(dirname "$0")/../../frontend/scripts/release-changelog.mjs" --notes "$RELEASE_TAG" "$changelog")
cd "${1:?Pass the release artifact directory}"

assets=()
for goos in darwin linux windows; do
  extension=tar.gz
  if [[ "$goos" == windows ]]; then
    extension=zip
  fi
  for goarch in amd64 arm64; do
    asset="ircordctl_${RELEASE_TAG}_${goos}_${goarch}.${extension}"
    if [[ ! -s "$asset" ]]; then
      echo "Missing CLI release asset: $asset" >&2
      exit 1
    fi
    assets+=("$asset")
  done
done
checksums="ircordctl_${RELEASE_TAG}_checksums.txt"
sha256sum "${assets[@]}" > "$checksums"

if gh release view "$RELEASE_TAG" > /dev/null 2>&1; then
  # Preserve notes and draft/prerelease settings; replace only our named assets.
  gh release upload "$RELEASE_TAG" "${assets[@]}" "$checksums" --clobber
else
  flags=(--verify-tag --title "$RELEASE_TAG")
  if [[ -n "$notes" ]]; then
    # Keep the generated compare link under the hand-written notes; without it, the notes alone.
    compare=$(gh api -X POST "repos/$GH_REPO/releases/generate-notes" -f tag_name="$RELEASE_TAG" \
      --jq .body 2> /dev/null | grep -m1 '^\*\*Full Changelog\*\*' || true)
    notes_file=$(mktemp)
    printf '%s\n' "$notes" > "$notes_file"
    if [[ -n "$compare" ]]; then
      printf '\n%s\n' "$compare" >> "$notes_file"
    fi
    flags+=(--notes-file "$notes_file")
  else
    echo "No $RELEASE_TAG section in $changelog; using generated notes."
    flags+=(--generate-notes)
  fi
  version_without_metadata=${RELEASE_TAG%%+*}
  if [[ "$version_without_metadata" == *-* ]]; then
    flags+=(--prerelease)
  fi
  gh release create "$RELEASE_TAG" "${assets[@]}" "$checksums" "${flags[@]}"
fi

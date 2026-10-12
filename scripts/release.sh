#!/usr/bin/env bash
# Builds the image of one architecture and pushes it with the version from
# VERSION and the changes from CHANGES.md:
#   <image>:<version>-<arch>   (immutable, for rollbacks)
#   <image>:<arch>             (moving, what docker-compose.yml pulls)
#
#   scripts/release.sh amd64|arm64
#
# Needs docker login for the registry. IMAGE (default q25459768/filebrowser) and
# COMMIT (default: the current git commit) can be overridden; building from a
# source archive without .git needs COMMIT.
set -euo pipefail
cd "$(dirname "$0")/.."

arch="${1:?usage: scripts/release.sh amd64|arm64}"
case "$arch" in amd64 | arm64) ;; *) echo "unknown architecture: $arch" >&2; exit 1 ;; esac

image="${IMAGE:-q25459768/filebrowser}"
version="$(tr -d '[:space:]' < VERSION)"
commit="${COMMIT:-$(git rev-parse --short HEAD 2>/dev/null || echo unknown)}"
changes="$(scripts/changes.sh "$version")"

echo "Releasing $image:$version-$arch ($commit)"
echo "Changes: $changes"

docker buildx build -f Dockerfile.multiarch \
  --platform "linux/$arch" \
  --build-arg "VERSION=$version" \
  --build-arg "COMMIT=$commit" \
  --build-arg "CHANGES=$changes" \
  -t "$image:$version-$arch" \
  -t "$image:$arch" \
  --push .

docker buildx imagetools inspect "$image:$version-$arch" | grep -E '^(Name|Digest):'

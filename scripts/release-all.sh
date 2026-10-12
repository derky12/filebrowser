#!/usr/bin/env bash
# Releases both images and syncs git. Run on the amd64 machine, on branch main,
# after merging main into arm64 (so both branches carry the same VERSION and
# CHANGES.md):
#
#   DOCKERHUB_USER=... DOCKERHUB_TOKEN=... scripts/release-all.sh
#
# amd64 is built here, arm64 natively on ARM_HOST (default 100.64.0.30, ssh
# port ARM_PORT 10086) from a `git archive` of the arm64 branch. The token is
# passed on stdin, never on a command line, and both machines log out at the
# end. It also creates the multi-arch tags <image>:<version> and <image>:latest.
# Finally the git tag v<version> is created on main and pushed with both
# branches.
set -euo pipefail
cd "$(dirname "$0")/.."

: "${DOCKERHUB_USER:?set DOCKERHUB_USER}" "${DOCKERHUB_TOKEN:?set DOCKERHUB_TOKEN}"
arm_host="${ARM_HOST:-100.64.0.30}"
arm_port="${ARM_PORT:-10086}"
image="${IMAGE:-q25459768/filebrowser}"

[ "$(git branch --show-current)" = main ] || { echo "run this on branch main" >&2; exit 1; }
[ -z "$(git status --porcelain)" ] || { echo "the working tree is not clean" >&2; exit 1; }
git merge-base --is-ancestor main arm64 || { echo "merge main into arm64 first" >&2; exit 1; }

version="$(tr -d '[:space:]' < VERSION)"
[ "$(git show arm64:VERSION | tr -d '[:space:]')" = "$version" ] || { echo "VERSION differs on arm64" >&2; exit 1; }
changes="$(scripts/changes.sh "$version")"
git rev-parse -q --verify "refs/tags/v$version" > /dev/null && { echo "tag v$version already exists, bump VERSION" >&2; exit 1; }

logout() { docker logout > /dev/null 2>&1 || true; ssh -p "$arm_port" "$arm_host" 'docker logout > /dev/null 2>&1 || true; rm -rf /tmp/fb-release' || true; }
trap logout EXIT

printf '%s' "$DOCKERHUB_TOKEN" | docker login -u "$DOCKERHUB_USER" --password-stdin > /dev/null
COMMIT="$(git rev-parse --short main)" scripts/release.sh amd64

git archive arm64 | ssh -p "$arm_port" "$arm_host" 'rm -rf /tmp/fb-release && mkdir -p /tmp/fb-release && tar -x -C /tmp/fb-release'
printf '%s' "$DOCKERHUB_TOKEN" | ssh -p "$arm_port" "$arm_host" \
  "set -e; docker login -u '$DOCKERHUB_USER' --password-stdin > /dev/null; cd /tmp/fb-release && IMAGE='$image' COMMIT='$(git rev-parse --short arm64)' scripts/release.sh arm64"

# One tag for both architectures: Docker takes the image of the machine's (or
# the compose file's `platform`) architecture, so a machine cannot end up with
# the other one's image.
docker buildx imagetools create -t "$image:$version" -t "$image:latest" "$image:$version-amd64" "$image:$version-arm64"
docker buildx imagetools inspect "$image:latest" | grep -E '^(Name|  Platform):'

git tag -a "v$version" -m "$version"$'\n\n'"$(printf '%s\n' "$changes" | sed 's/; /\n- /g; s/^/- /')"
git push origin main arm64 "v$version"
echo "Released $version"

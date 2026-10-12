#!/usr/bin/env bash
# Prints the changes of a version from CHANGES.md as one line ("a; b; c").
# Fails when the version has no section or no bullets.
set -euo pipefail
cd "$(dirname "$0")/.."

version="${1:?usage: scripts/changes.sh <version>}"

changes="$(awk -v v="$version" '
  /^## / { active = ($2 == v); next }
  active && /^- / { sub(/^- /, ""); print }
' CHANGES.md | paste -sd';' - | sed 's/;/; /g')"

if [ -z "$changes" ]; then
  echo "CHANGES.md has no entry for version $version" >&2
  exit 1
fi

printf '%s\n' "$changes"

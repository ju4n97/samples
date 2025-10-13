#!/usr/bin/env bash

# "tree + cat" with simple positional ignores

# Usage: dumpfiles.sh [DIR] [IGNORE1 [IGNORE2 ...]]

set -euo pipefail

DIR="."
if [[ $# -gt 0 && -d "$1" ]]; then
  DIR="$1"
  shift
fi
IGNORES=("$@")

# Build prune expression: match basenames (files or dirs) anywhere
PRUNES=()
for pat in "${IGNORES[@]}"; do
  PRUNES+=( -name "$pat" -o -path "*/$pat/*" -o )
done
# drop trailing -o if any
if (( ${#PRUNES[@]} > 0 )); then unset 'PRUNES[${#PRUNES[@]}-1]'; fi

# Find files, respecting prunes
if (( ${#PRUNES[@]} > 0 )); then
  FIND_ARGS=( "$DIR" "(" "${PRUNES[@]}" ")" -prune -o -type f -print0 )
else
  FIND_ARGS=( "$DIR" -type f -print0 )
fi

# Traverse & print (null-safe paths)
find "${FIND_ARGS[@]}" | while IFS= read -r -d '' f; do
  rel=${f#"$DIR"/}
  echo "$rel"
  sed 's/^/    /' "$f"
  echo
done
#!/usr/bin/env bash
set -euo pipefail

repo_root="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
codeowners_file="$repo_root/.github/CODEOWNERS"
allow_placeholders="${ALLOW_CODEOWNERS_PLACEHOLDERS:-0}"

if [[ ! -f "$codeowners_file" ]]; then
  echo "CODEOWNERS file missing: $codeowners_file" >&2
  exit 1
fi

if [[ "$allow_placeholders" != "1" ]] && grep -q "@your-org/" "$codeowners_file"; then
  echo "CODEOWNERS still contains placeholder owners (@your-org/...)." >&2
  echo "Replace placeholders before enabling code-owner required reviews." >&2
  exit 1
fi

line_number=0
while IFS= read -r line || [[ -n "$line" ]]; do
  line_number=$((line_number + 1))
  trimmed="$(echo "$line" | sed 's/^[[:space:]]*//;s/[[:space:]]*$//')"
  if [[ -z "$trimmed" || "${trimmed:0:1}" == "#" ]]; then
    continue
  fi

  if [[ ! "$trimmed" =~ [[:space:]]+@ ]]; then
    echo "Invalid CODEOWNERS entry at line $line_number: missing owner handle" >&2
    echo "  $line" >&2
    exit 1
  fi
done <"$codeowners_file"

echo "CODEOWNERS verification passed."

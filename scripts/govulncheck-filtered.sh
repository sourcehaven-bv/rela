#!/usr/bin/env bash
# Run govulncheck and ignore vulnerabilities for which no fix is currently available.
#
# govulncheck has no built-in suppression mechanism (see https://go.dev/issue/61211),
# so we parse JSON output and filter out known-unfixable findings here. Each entry in
# IGNORED_OSVS must have an explanation; revisit when upstream releases a fix.

set -euo pipefail

# OSV IDs to ignore — keep this list small and documented.
# GO-2026-4923: bbolt index out-of-range panic; reached transitively via blevesearch.
#               No upstream fix as of 2026-04-07. Tracking: https://github.com/etcd-io/bbolt/pull/1171
IGNORED_OSVS=(
  "GO-2026-4923"
)

OUTPUT=$(govulncheck -format json ./...)

# Build a jq filter that drops findings whose OSV id is in the ignore list.
ids_json=$(printf '%s\n' "${IGNORED_OSVS[@]}" | jq -R . | jq -s .)

# Collect OSV ids that have a "finding" entry referencing user code (trace > 1).
called=$(printf '%s\n' "$OUTPUT" \
  | jq -rs --argjson ignored "$ids_json" '
      [ .[] | select(.finding) | .finding
        | select(.trace and (.trace | length > 1))
        | .osv
      ]
      | unique
      | map(select(. as $id | ($ignored | index($id)) | not))
      | .[]
    ')

if [[ -n "$called" ]]; then
  echo "govulncheck found vulnerabilities affecting this code:"
  echo "$called"
  echo
  echo "Re-running govulncheck for full report:"
  govulncheck ./... || true
  exit 1
fi

echo "govulncheck: no actionable vulnerabilities found."
echo "Ignored (no upstream fix): ${IGNORED_OSVS[*]}"

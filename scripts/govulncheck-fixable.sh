#!/usr/bin/env bash
# Given govulncheck JSON on stdin, print one line per fixable, called vulnerability:
#
#   <module-path> <fixed-version>
#
# "Called" means the vuln has a finding with a trace length > 1 (i.e., reachable
# from our code, not just present in the module graph). "Fixable" means the
# OSV has at least one `fixed` event in a SEMVER range. IGNORED_OSVS (same
# list as govulncheck-filtered.sh) are excluded.
#
# Exit 0 if at least one line printed; exit 1 if no fixable-and-called vulns.
#
# Usage:
#   govulncheck -format json ./... | scripts/govulncheck-fixable.sh
#
# Output is deduplicated (module,version) pairs, sorted.

set -euo pipefail

# Keep this list in sync with scripts/govulncheck-filtered.sh.
IGNORED_OSVS=(
  "GO-2026-4923"
)

ids_json=$(printf '%s\n' "${IGNORED_OSVS[@]}" | jq -R . | jq -s .)

# Collect called OSV ids (finding.trace length > 1) minus ignored.
# Then join to the matching .osv entry and emit module@fixed lines.
output=$(jq -rs --argjson ignored "$ids_json" '
  . as $all
  | [ $all[] | select(.finding) | .finding
      | select(.trace and (.trace | length > 1))
      | .osv
    ] | unique
  | map(select(. as $id | ($ignored | index($id)) | not))
  | . as $called
  | [ $all[] | select(.osv) | select(.osv.id as $id | $called | index($id))
      | .osv.affected[]?
      | . as $aff
      | $aff.ranges[]?
      | select(.type == "SEMVER")
      | .events[]?
      | select(.fixed)
      | "\($aff.package.name) \(.fixed)"
    ] | unique | .[]
')

if [[ -z "$output" ]]; then
  exit 1
fi

echo "$output"

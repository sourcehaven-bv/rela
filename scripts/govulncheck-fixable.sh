#!/usr/bin/env bash
# Given govulncheck JSON on stdin, print one line per module with a fixable,
# called vulnerability:
#
#   <module-path> <fixed-version>
#
# "Called" means the vuln has a finding with a trace length > 1 (i.e., reachable
# from our code, not just present in the module graph). "Fixable" means the
# finding has a `fixed_version`. govulncheck computes that for the version in
# use, so a stdlib finding on go1.26.x names the go1.26 fix rather than the fix
# on a newer release branch. The OSV ranges list a `fixed` event per branch;
# reading those instead jumped the toolchain a minor version (BUG-NVNPD8).
# IGNORED_OSVS (same list as govulncheck-filtered.sh) are excluded.
#
# When several vulns affect one module, the highest fixed version is printed.
# `sort -V` ranks a pre-release above its release; Go fixes are not
# pre-releases, so that does not arise in practice.
# Versions are printed without the leading "v" (stdlib: "1.26.10").
#
# Exit 0 if at least one line printed; exit 1 if no fixable-and-called vulns.
#
# Usage:
#   govulncheck -format json ./... | scripts/govulncheck-fixable.sh

set -euo pipefail

# Keep this list in sync with scripts/govulncheck-filtered.sh.
IGNORED_OSVS=(
  "GO-2026-4923"
)

ids_json=$(printf '%s\n' "${IGNORED_OSVS[@]}" | jq -R . | jq -s .)

# trace[0] is the vulnerable symbol, so its module is the one to upgrade.
pairs=$(jq -r --argjson ignored "$ids_json" '
  select(.finding) | .finding
  | select(.trace and (.trace | length > 1))
  | select(.fixed_version and .fixed_version != "")
  | select(.osv as $id | ($ignored | index($id)) | not)
  | "\(.trace[0].module) \(.fixed_version | ltrimstr("v"))"
')

if [[ -z "$pairs" ]]; then
  exit 1
fi

# Highest version per module: sort by module, then by version descending, and
# keep the first line of each module.
printf '%s\n' "$pairs" | sort -k1,1 -k2,2Vr | awk '!seen[$1]++'

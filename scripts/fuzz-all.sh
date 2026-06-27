#!/usr/bin/env bash
# fuzz-all.sh — run every Go fuzz target in the repo for a short budget.
#
# Discovers targets by scanning test files, so newly added Fuzz*
# functions are swept automatically — no hand-maintained list to go
# stale. Used by the weekly fuzz-sweep workflow and `just fuzz-all`.
#
# Usage:
#   FUZZTIME=25s scripts/fuzz-all.sh        # default budget per target
#   FUZZTIME=2s  scripts/fuzz-all.sh        # quick local smoke
#
# Behavior:
#   - Each target runs in isolation (`go test -fuzz` allows one target
#     per invocation).
#   - A failing target does not stop the sweep; failures are collected
#     and the script exits non-zero at the end.
#   - Failed targets are listed in fuzz-failures.txt (consumed by the
#     workflow's issue-filing step). Crashing inputs land in the
#     package's testdata/fuzz/<Target>/ directory as usual.
#   - Targets that skip themselves (e.g. pgstore without
#     RELA_TEST_DATABASE_URL) pass trivially.
#
# The discovery pattern matches `func FuzzX(f *testing.F)` exactly —
# one parameter — so shared fuzz helpers that take extra arguments
# (internal/store/storetest/fuzz.go) are not treated as targets; the
# per-backend wrappers that call them are.

# Exit codes:
#   0 — sweep clean
#   1 — at least one target failed (fuzz crash or test error; see summary)
#   2 — configuration/setup error (bad FUZZTIME, broken build, zero
#       targets discovered) — the workflow does NOT file an issue for
#       these, it just fails the run.

set -uo pipefail

FUZZTIME="${FUZZTIME:-25s}"
SUMMARY="fuzz-failures.txt"
rm -f "$SUMMARY"

if ! [[ "$FUZZTIME" =~ ^[0-9]+(\.[0-9]+)?(ns|us|µs|ms|s|m|h)$ ]]; then
  echo "ERROR: invalid FUZZTIME ${FUZZTIME@Q} (want a Go duration like 25s)"
  exit 2
fi

# Build gate: a broken tree must fail fast and unambiguously, not
# masquerade as 39 "fuzz failures" in the filed issue.
echo "==> build gate"
if ! go build ./...; then
  echo "ERROR: build broken — fix the tree before fuzzing"
  exit 2
fi

failed=0
total=0

while read -r file target; do
  pkg="./$(dirname "$file")"
  total=$((total + 1))
  echo ""
  echo "==> ${pkg} ${target} (${FUZZTIME})"
  out="$(go test -run='^$' -fuzz="^${target}\$" -fuzztime="$FUZZTIME" "$pkg" 2>&1)"
  status=$?
  echo "$out"
  if [ "$status" -ne 0 ]; then
    # Classify so the filed issue distinguishes a crashing input
    # (corpus file written) from an infrastructure/test error.
    kind="error"
    if grep -qE 'Failing input written to|--- FAIL: Fuzz' <<<"$out"; then
      kind="fuzz-crash"
    fi
    echo "${pkg} ${target} [${kind}]" >>"$SUMMARY"
    failed=$((failed + 1))
  fi
done < <(grep -rEH '^func Fuzz[A-Za-z0-9_]+\(f \*testing\.F\)' \
  --include='*_test.go' internal/ |
  sed -E 's|^([^:]+):func (Fuzz[A-Za-z0-9_]+)\(.*|\1 \2|' |
  sort)

echo ""
if [ "$total" -eq 0 ]; then
  echo "ERROR: no fuzz targets discovered — discovery pattern broken?"
  exit 2
fi
if [ "$failed" -gt 0 ]; then
  echo "FUZZ SWEEP FAILED: ${failed}/${total} targets"
  cat "$SUMMARY"
  exit 1
fi
echo "Fuzz sweep OK: ${total} targets, ${FUZZTIME} each."

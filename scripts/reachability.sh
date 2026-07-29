#!/usr/bin/env bash
# Reachability floor. A line counts as REACHED if it was executed at least once
# during ANY test — unit, cross-package, build-tagged (postgres), or e2e. The
# aim is 100%: every line runs at least once, or is explicitly dismissed with a
# reasoned `// coverage-ignore: <reason>`.
#
# This is a FLOOR, not a quality gate. "Reached" is strictly weaker than
# "tested" — a line an e2e flow merely walks past counts as reached. Its value
# is catching code that has NEVER run under any test (dead branches, an
# unreachable-in-practice path, a stray `1/0`) — unobserved until production.
# Test quality is a separate axis built ON TOP of this floor.
#
# Coverage from every source is collected as Go binary coverage (GOCOVERDIR /
# -cover) and merged with `go tool covdata`, which unions them (reached by ANY
# source = reached). Enforcement is via `scupper`, which reads the merged
# profile, applies `// coverage-ignore:` dismissals, and reports reachability.
#
# PR #1 is REPORT-ONLY: no threshold is enforced. It establishes the honest
# baseline. A rising per-package floor comes in later PRs.
#
# Usage: scripts/reachability.sh [--threshold N]   (N omitted => report only)
set -euo pipefail

THRESHOLD=""
while [ $# -gt 0 ]; do
  case "$1" in
    --threshold) THRESHOLD="$2"; shift 2 ;;
    *) echo "unknown arg: $1" >&2; exit 2 ;;
  esac
done

ROOT="$(cd "$(dirname "$0")/.." && pwd)"
cd "$ROOT"

SCUPPER_VERSION="v0.2.0"
DIRECTIVE_ARGS=(-d coverage-ignore --require-reason)

# Ensure scupper is available (pinned). Installed to GOBIN; fall back to `go run`.
SCUPPER="$(command -v scupper || true)"
if [ -z "$SCUPPER" ]; then
  echo "==> installing scupper@${SCUPPER_VERSION}"
  go install "github.com/sourcehaven-bv/scupper/cmd/scupper@${SCUPPER_VERSION}"
  SCUPPER="$(go env GOPATH)/bin/scupper"
fi

WORK="$(mktemp -d)"
trap 'rm -rf "$WORK"' EXIT
XPKG_DIR="$WORK/xpkg"
PG_DIR="$WORK/pg"
E2E_DIR="$WORK/e2e"
MERGED="$WORK/merged"
mkdir -p "$XPKG_DIR" "$PG_DIR" "$E2E_DIR" "$MERGED"

# All buildable packages (matches the justfile go_packages filter).
# NOTE: pass ./... directly to -coverpkg — a comma-joined list can inject
# newlines that break the flag.

echo "==> [1/4] unit + cross-package coverage (-coverpkg=./...)"
# -covermode=set: reachability is a boolean "did this line run" question; `set`
# answers exactly that, is cheaper than atomic, and ALL legs must share one mode
# for covdata merge to accept them.
go test -cover -covermode=set -coverpkg=./... \
  $(go list ./... | grep -v /frontend/node_modules/) \
  -args -test.gocoverdir="$XPKG_DIR" >/dev/null 2>&1 || {
    echo "    (some unit tests failed; coverage still collected from those that ran)" >&2
  }

echo "==> [2/4] postgres-tagged coverage"
if [ -n "${RELA_TEST_DATABASE_URL:-}" ]; then
  go test -tags postgres -cover -covermode=set -coverpkg=./... \
    ./internal/store/pgstore/... \
    -args -test.gocoverdir="$PG_DIR" >/dev/null 2>&1 || true
else
  echo "    skipped: RELA_TEST_DATABASE_URL not set (pgstore reads low without it)"
fi

echo "==> [3/4] e2e coverage (Playwright drives the instrumented rela-server)"
if [ "${RUN_E2E:-0}" = "1" ] && [ -d e2e ]; then
  mkdir -p bin
  go build -cover -covermode=set -o bin/rela-server ./cmd/rela-server
  ( cd e2e && RELA_E2E_COVERDIR="$E2E_DIR" npx playwright test ) || true
else
  echo "    skipped: set RUN_E2E=1 to include e2e (slow; needs Playwright browsers)"
fi

echo "==> [4/4] merge all sources and report reachability"
# Merge every non-empty covdata dir. covdata unions counters: reached by ANY.
MERGE_INPUTS=""
for d in "$XPKG_DIR" "$PG_DIR" "$E2E_DIR"; do
  if ls "$d"/covmeta.* >/dev/null 2>&1; then
    MERGE_INPUTS="${MERGE_INPUTS:+$MERGE_INPUTS,}$d"
  fi
done
if [ -z "$MERGE_INPUTS" ]; then
  echo "no coverage data collected" >&2; exit 2
fi
go tool covdata merge -i="$MERGE_INPUTS" -o="$MERGED"
go tool covdata textfmt -i="$MERGED" -o=reachability.out

echo
echo "Merged sources: $MERGE_INPUTS"
echo

# scupper reads rela's `// coverage-ignore: <reason>` dismissals and reports
# reachability. With --threshold it also gates (exit 1 below N); without it,
# report-only (exit 0). PR #1 runs report-only.
if [ -n "$THRESHOLD" ]; then
  exec "$SCUPPER" -i reachability.out -o reachability.filtered.out \
    "${DIRECTIVE_ARGS[@]}" -threshold "$THRESHOLD"
else
  "$SCUPPER" -i reachability.out -o reachability.filtered.out "${DIRECTIVE_ARGS[@]}"
fi

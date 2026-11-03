#!/usr/bin/env bash
# Tests for govulncheck-fixable.sh.
#
# The script decides which `go get` the daily Security Scan runs, and that
# workflow only runs on a schedule, so a wrong answer surfaces as a stream of
# issues rather than a red PR. BUG-NVNPD8: an OSV with a fix per Go release
# branch made it pick the newest branch (go1.27.3) instead of the fix for the
# version in use (go1.26.10).
set -uo pipefail

SCRIPT_DIR=$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)
SCRIPT="$SCRIPT_DIR/govulncheck-fixable.sh"

pass=0
fail=0

# check <description> <expected-rc> <expected-stdout> <json-on-stdin>
check() {
  local desc="$1" want_rc="$2" want_out="$3" input="$4"
  local out rc
  out=$(printf '%s\n' "$input" | "$SCRIPT" 2>&1)
  rc=$?
  if [[ "$rc" -eq "$want_rc" && "$out" == "$want_out" ]]; then
    echo "  ok   $desc"
    pass=$((pass + 1))
  else
    echo "  FAIL $desc"
    echo "       want rc=$want_rc out=$(printf '%q' "$want_out")"
    echo "       got  rc=$rc out=$(printf '%q' "$out")"
    fail=$((fail + 1))
  fi
}

# finding <osv> <fixed_version> <module> <trace-length>
finding() {
  local osv="$1" fixed="$2" mod="$3" n="$4"
  local frames='{"module":"'"$mod"'","package":"p","function":"F"}'
  for ((i = 1; i < n; i++)); do
    frames+=',{"module":"github.com/Sourcehaven-BV/rela","package":"q","function":"G"}'
  done
  printf '{"finding":{"osv":"%s","fixed_version":"%s","trace":[%s]}}\n' "$osv" "$fixed" "$frames"
}

# Two release branches fixed; the finding names the one for the version in use.
multi_branch_osv='{"osv":{"id":"GO-2026-6809","affected":[{"package":{"name":"stdlib"},"ranges":[{"type":"SEMVER","events":[{"introduced":"0"},{"fixed":"1.26.10"},{"introduced":"1.27.0-0"},{"fixed":"1.27.3"}]}]}]}}'

check "stdlib: uses the finding's fixed_version, not the newest branch" 0 \
  "stdlib 1.26.10" \
  "$multi_branch_osv
$(finding GO-2026-6809 v1.26.10 stdlib 3)"

check "module: highest fixed version wins, compared as versions" 0 \
  "golang.org/x/net 0.10.0" \
  "$(finding GO-2026-0001 v0.9.0 golang.org/x/net 2)
$(finding GO-2026-0002 v0.10.0 golang.org/x/net 2)"

check "one line per module, sorted by module" 0 \
  "golang.org/x/net 0.60.0
stdlib 1.26.10" \
  "$(finding GO-2026-0003 v1.26.10 stdlib 2)
$(finding GO-2026-0003 v1.26.10 stdlib 4)
$(finding GO-2026-0004 v0.60.0 golang.org/x/net 2)"

check "not called (trace length 1) is skipped" 1 "" \
  "$(finding GO-2026-0005 v1.2.3 example.com/m 1)"

check "no fixed_version is skipped" 1 "" \
  "$(finding GO-2026-0006 "" example.com/m 2)"

check "ignored OSV is skipped" 1 "" \
  "$(finding GO-2026-4923 v1.4.1 go.etcd.io/bbolt 2)"

check "ignored OSV next to a real one: only the real one" 0 \
  "example.com/m 1.2.3" \
  "$(finding GO-2026-4923 v1.4.1 go.etcd.io/bbolt 2)
$(finding GO-2026-0007 v1.2.3 example.com/m 2)"

check "one OSV in two modules: a line per module" 0 \
  "example.com/a 1.0.1
example.com/b 2.0.1" \
  "$(finding GO-2026-0008 v1.0.1 example.com/a 2)
$(finding GO-2026-0008 v2.0.1 example.com/b 2)"

check "no findings" 1 "" '{"config":{"scanner_name":"govulncheck"}}'

# Truncated output must not look like "nothing fixable" (exit 1).
truncated=$(printf '{"finding":{"osv":"GO-2026-0009"' | "$SCRIPT" 2>/dev/null)
rc=$?
if [[ "$rc" -ne 0 && "$rc" -ne 1 && -z "$truncated" ]]; then
  echo "  ok   truncated JSON fails with rc other than 1"
  pass=$((pass + 1))
else
  echo "  FAIL truncated JSON fails with rc other than 1 (got rc=$rc)"
  fail=$((fail + 1))
fi

echo "$pass passed, $fail failed"
[[ "$fail" -eq 0 ]]

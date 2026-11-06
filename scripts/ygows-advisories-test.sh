#!/usr/bin/env bash
# Tests for ygows-advisories.sh. The script only runs on a schedule in
# security.yml, so a filter bug would hide an advisory without failing
# anything; CI runs this test instead.
set -uo pipefail

SCRIPT_DIR=$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)
SCRIPT="$SCRIPT_DIR/ygows-advisories.sh"
TMP=$(mktemp -d)
trap 'rm -rf "$TMP"' EXIT

printf '# comment\nGO-2026-0003 patched in the fork\n\n' > "$TMP/ignore"

pass=0
fail=0

# check <description> <expected-rc> <expected-stdout> <json-on-stdin>
check() {
  local desc="$1" want_rc="$2" want_out="$3" input="$4"
  local out rc
  out=$(printf '%s\n' "$input" | "$SCRIPT" "$TMP/ignore" 2>/dev/null)
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

# vuln <id> <imports-json-or-empty> [extra-json-fields]
vuln() {
  local imports=""
  if [[ -n "$2" ]]; then
    imports=", \"ecosystem_specific\": {\"imports\": $2}"
  fi
  printf '{"id": "%s", "summary": "s-%s"%s, "affected": [{"package": {"ecosystem": "Go", "name": "github.com/reearth/ygo"}%s}]}' \
    "$1" "$1" "${3:-}" "$imports"
}

check "no advisories" 0 "" '{}'
check "empty list" 0 "" '{"vulns": []}'
check "relay package" 1 $'GO-2026-0001\ts-GO-2026-0001' \
  "{\"vulns\": [$(vuln GO-2026-0001 '[{"path": "github.com/reearth/ygo/provider/websocket"}]')]}"
check "forked internal package" 1 $'GO-2026-0002\ts-GO-2026-0002' \
  "{\"vulns\": [$(vuln GO-2026-0002 '[{"path": "github.com/reearth/ygo/internal/relaylane"}]')]}"
check "unforked package" 0 "" \
  "{\"vulns\": [$(vuln GO-2026-0004 '[{"path": "github.com/reearth/ygo/crdt"}]')]}"
check "prefix is not a match" 0 "" \
  "{\"vulns\": [$(vuln GO-2026-0005 '[{"path": "github.com/reearth/ygo/provider/websocketx"}]')]}"
check "no import paths counts" 1 $'GHSA-aaaa-bbbb-cccc\ts-GHSA-aaaa-bbbb-cccc' \
  "{\"vulns\": [$(vuln GHSA-aaaa-bbbb-cccc '')]}"
check "ignored" 0 "" \
  "{\"vulns\": [$(vuln GO-2026-0003 '[{"path": "github.com/reearth/ygo/provider/websocket"}]')]}"
check "withdrawn" 0 "" \
  "{\"vulns\": [$(vuln GO-2026-0006 '' ', "withdrawn": "2026-01-01T00:00:00Z"')]}"
check "pages are merged and deduplicated" 1 $'GO-2026-0001\ts-GO-2026-0001\nGO-2026-0002\ts-GO-2026-0002' \
  "{\"vulns\": [$(vuln GO-2026-0001 '')]} {\"vulns\": [$(vuln GO-2026-0002 ''), $(vuln GO-2026-0001 '')]}"
check "bad input" 2 "" 'not json'
check "other package spelling counts" 1 $'GO-2026-0007\ts-GO-2026-0007' \
  '{"vulns": [{"id": "GO-2026-0007", "summary": "s-GO-2026-0007", "affected": [{"package": {"name": "github.com/ReEarth/ygo"}}]}]}'
check "import without path counts" 1 $'GO-2026-0008\ts-GO-2026-0008' \
  "{\"vulns\": [$(vuln GO-2026-0008 '[{"symbols": ["X"]}]')]}"

out=$("$SCRIPT" </dev/null 2>/dev/null)
rc=$?
if [[ "$rc" -eq 2 ]]; then
  echo "  ok   missing argument"
  pass=$((pass + 1))
else
  echo "  FAIL missing argument: rc=$rc out=$out"
  fail=$((fail + 1))
fi

echo ""
echo "$pass passed, $fail failed"
[[ "$fail" -eq 0 ]]

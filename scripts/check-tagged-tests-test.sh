#!/usr/bin/env bash
# Tests for check-tagged-tests.sh.
#
# A guard that has never been observed FAILING is not a verified guard. The
# whole point of this one is that a tagged test file can rot without any other
# CI step noticing, so the negative case — a tagged file that does not compile
# must fail the build — is the test that matters.
#
# Runs against a throwaway Go module, not the real tree, so it is fast and
# cannot be affected by the repo's own tagged files.
set -uo pipefail

SCRIPT_DIR=$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)
GUARD="$SCRIPT_DIR/check-tagged-tests.sh"

TMP=$(mktemp -d)
trap 'rm -rf "$TMP"' EXIT

pass=0
fail=0

# want_rc <expected-rc> <description> <command...>
want_rc() {
  local want="$1" desc="$2"
  shift 2
  local out rc
  out=$("$@" 2>&1)
  rc=$?
  if [ "$rc" -eq "$want" ]; then
    echo "  ok   $desc"
    pass=$((pass + 1))
  else
    echo "  FAIL $desc (want rc=$want, got rc=$rc)"
    echo "$out" | sed 's/^/         /'
    fail=$((fail + 1))
  fi
}

# want_tags <expected-newline-separated> <description> <dir>
want_tags() {
  local want="$1" desc="$2" dir="$3"
  local got
  got=$(RELA_ROOT="$dir" "$GUARD" tags 2>&1)
  if [ "$got" = "$want" ]; then
    echo "  ok   $desc"
    pass=$((pass + 1))
  else
    echo "  FAIL $desc"
    echo "         want: [$want]"
    echo "         got:  [$got]"
    fail=$((fail + 1))
  fi
}

# make_module <dir> — a minimal module with one compiling, untagged package.
make_module() {
  local d="$1"
  mkdir -p "$d"
  cat >"$d/go.mod" <<'MOD'
module example.com/tagged

go 1.26
MOD
  cat >"$d/lib.go" <<'GO'
package tagged

// Add is the API a tagged test file will reference.
func Add(a, b int) int { return a + b }
GO
}

# add_test <dir> <name> <buildline> <body>
add_test() {
  local d="$1" name="$2" buildline="$3" body="$4"
  cat >"$d/${name}_test.go" <<GO
${buildline}

package tagged

import "testing"

func Test${name}(t *testing.T) {
${body}
}
GO
}

echo "discovery:"

make_module "$TMP/plain"
add_test "$TMP/plain" "Ok" "//go:build sometag" '	_ = Add(1, 2)'
want_tags "sometag" "plain tag discovered" "$TMP/plain"

# Negated constraints mark files that build by DEFAULT. They are already
# covered by the normal test run and must not be treated as opt-in tags.
make_module "$TMP/negated"
add_test "$TMP/negated" "Neg" "//go:build !postgres && !memorybackend" '	_ = Add(1, 2)'
want_tags "" "negated-only constraint yields no tags" "$TMP/negated"

# GOOS tags select a platform; `go vet -tags windows` type-checks against the
# HOST and collides with the real GOOS, so they must be skipped.
make_module "$TMP/platform"
add_test "$TMP/platform" "Win" "//go:build windows" '	_ = Add(1, 2)'
want_tags "" "platform (GOOS) tag skipped" "$TMP/platform"

make_module "$TMP/multi"
add_test "$TMP/multi" "A" "//go:build alpha" '	_ = Add(1, 2)'
add_test "$TMP/multi" "B" "//go:build beta" '	_ = Add(1, 2)'
add_test "$TMP/multi" "C" "//go:build alpha" '	_ = Add(3, 4)'
want_tags "alpha
beta" "multiple tags deduped and sorted" "$TMP/multi"

# An OR/AND expression contributes each of its plain terms.
make_module "$TMP/expr"
add_test "$TMP/expr" "E" "//go:build alpha || beta" '	_ = Add(1, 2)'
want_tags "alpha
beta" "or-expression contributes both terms" "$TMP/expr"

# A tag must not be picked up from a line that is not a real build constraint
# (e.g. one appearing after the package clause, which Go ignores).
make_module "$TMP/late"
cat >"$TMP/late/late_test.go" <<'GO'
package tagged

//go:build notarealconstraint

import "testing"

func TestLate(t *testing.T) { _ = Add(1, 2) }
GO
want_tags "" "constraint after package clause ignored" "$TMP/late"

echo
echo "vetting:"

# The positive case: a tagged file that compiles.
make_module "$TMP/good"
add_test "$TMP/good" "Good" "//go:build sometag" '	_ = Add(1, 2)'
want_rc 0 "compiling tagged file passes" env RELA_ROOT="$TMP/good" "$GUARD"

# THE case this guard exists for: a tagged file referencing an API that has
# since changed. This is exactly the shape of the real rot — Add takes two
# args, the tagged test passes three — and `go build ./...` and an untagged
# `go test ./...` both report success on this tree.
make_module "$TMP/rotted"
add_test "$TMP/rotted" "Rotted" "//go:build sometag" '	_ = Add(1, 2, 3)'
want_rc 1 "tagged file with stale call signature FAILS" \
  env RELA_ROOT="$TMP/rotted" "$GUARD"

# Prove the premise: the normal build/test path really is blind to it, so the
# guard is not duplicating an existing signal.
(cd "$TMP/rotted" && go build ./... >/dev/null 2>&1)
want_rc 0 "…while 'go build ./...' on the same tree passes" \
  bash -c "cd '$TMP/rotted' && go build ./..."
want_rc 0 "…and untagged 'go vet ./...' on the same tree passes" \
  bash -c "cd '$TMP/rotted' && go vet ./..."

# A syntactically broken tagged file must also fail.
make_module "$TMP/broken"
add_test "$TMP/broken" "Broken" "//go:build sometag" '	this is not go'
want_rc 1 "syntactically invalid tagged file FAILS" \
  env RELA_ROOT="$TMP/broken" "$GUARD"

# One bad tag among several good ones must still fail.
make_module "$TMP/mixed"
add_test "$TMP/mixed" "Fine" "//go:build alpha" '	_ = Add(1, 2)'
add_test "$TMP/mixed" "Rot" "//go:build beta" '	_ = Add(1, 2, 3)'
want_rc 1 "one rotted tag among several FAILS" \
  env RELA_ROOT="$TMP/mixed" "$GUARD"

echo
echo "empty match:"

# If every tagged test file is deleted the guard must go quiet, not fail. This
# is the state the repo is in for `e2e` after TKT-1CH5AX removed e2e_test.go.
make_module "$TMP/none"
add_test "$TMP/none" "Plain" "" '	_ = Add(1, 2)'
want_rc 0 "no tagged files at all exits 0" env RELA_ROOT="$TMP/none" "$GUARD"
want_tags "" "no tagged files yields no tags" "$TMP/none"

echo
echo "passed: $pass  failed: $fail"
[ "$fail" -eq 0 ]

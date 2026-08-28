#!/usr/bin/env bash
# Tests for check-embedded-spa.sh.
#
# A guard that has never been observed FAILING is not a verified guard. The
# regression this protects against (BUG-2YZ575, BUG-W144 before it) shipped for
# months precisely because nothing asserted the negative case, so the negative
# cases are the point of this file.
set -uo pipefail

SCRIPT_DIR=$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)
GUARD="$SCRIPT_DIR/check-embedded-spa.sh"

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

# make_good <dir> — a well-formed Vite output tree.
make_good() {
  local d="$1"
  mkdir -p "$d/assets"
  echo 'body{}' >"$d/assets/index-C0FFEE.css"
  echo 'console.log(1)' >"$d/assets/index-DEADBE.js"
  cat >"$d/index.html" <<'HTML'
<!doctype html><html><head>
<script type="module" crossorigin src="/assets/index-DEADBE.js"></script>
<link rel="stylesheet" href="/assets/index-C0FFEE.css">
</head><body><div id="app"></div></body></html>
HTML
}

echo "tree:"
make_good "$TMP/good"
want_rc 0 "populated build tree passes" "$GUARD" tree "$TMP/good"

want_rc 1 "missing dir fails" "$GUARD" tree "$TMP/nonexistent"

mkdir -p "$TMP/empty"
want_rc 1 "empty dir fails" "$GUARD" tree "$TMP/empty"

mkdir -p "$TMP/noassetsdir"
echo '<!doctype html>' >"$TMP/noassetsdir/index.html"
want_rc 1 "index.html with no assets/ dir fails" "$GUARD" tree "$TMP/noassetsdir"

mkdir -p "$TMP/emptyassets/assets"
echo '<!doctype html>' >"$TMP/emptyassets/index.html"
want_rc 1 "index.html referencing nothing fails" "$GUARD" tree "$TMP/emptyassets"

# The regression the first draft of this guard missed.
make_good "$TMP/zerobyte"
: >"$TMP/zerobyte/assets/index-DEADBE.js"
want_rc 1 "zero-byte entry asset fails" "$GUARD" tree "$TMP/zerobyte"

make_good "$TMP/missingasset"
rm "$TMP/missingasset/assets/index-DEADBE.js"
want_rc 1 "referenced asset absent from disk fails" "$GUARD" tree "$TMP/missingasset"

make_good "$TMP/emptyindex"
: >"$TMP/emptyindex/index.html"
want_rc 1 "zero-byte index.html fails" "$GUARD" tree "$TMP/emptyindex"

echo "binary:"
# A "binary" that embedded the bundle: contains the hashed asset names.
printf 'ELF junk /assets/index-DEADBE.js more junk /assets/index-C0FFEE.css\n' \
  >"$TMP/bin-good"
want_rc 0 "binary containing hashed assets passes" \
  "$GUARD" binary "$TMP/bin-good" "$TMP/good"

# The BUG-2YZ575 shape: the embed ERROR STRINGS are present, the bundle is not.
# A naive `grep static/v2` would pass this; the guard must not.
printf 'mount embedded SPA filesystem (static/v2): %%w\nstatic/v2\nstatic/v2\n' \
  >"$TMP/bin-bad"
want_rc 1 "binary with only embed error strings fails" \
  "$GUARD" binary "$TMP/bin-bad" "$TMP/good"

want_rc 1 "missing binary fails" "$GUARD" binary "$TMP/nope" "$TMP/good"

want_rc 1 "binary check with no index.html to derive from fails" \
  "$GUARD" binary "$TMP/bin-good" "$TMP/empty"

# A stale binary embedding a PREVIOUS build's hashes must not pass as current.
printf 'junk /assets/index-0LDBUILD.js junk\n' >"$TMP/bin-stale"
want_rc 1 "binary embedding only a stale build's assets fails" \
  "$GUARD" binary "$TMP/bin-stale" "$TMP/good"

echo "usage:"
want_rc 2 "no args exits 2" "$GUARD"
want_rc 2 "unknown subcommand exits 2" "$GUARD" bogus x
want_rc 2 "tree with wrong arity exits 2" "$GUARD" tree
want_rc 2 "binary with wrong arity exits 2" "$GUARD" binary "$TMP/bin-good"

echo
echo "passed: $pass  failed: $fail"
[ "$fail" -eq 0 ]

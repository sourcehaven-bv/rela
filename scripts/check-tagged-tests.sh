#!/usr/bin/env bash
# Compile every build-tag-gated _test.go file in the repo.
#
# `go build ./...` does not compile _test.go files, and `go test ./...` only
# compiles the files whose build constraints the default tag set satisfies. So a
# test file behind `//go:build e2e` is invisible to every other CI step: it can
# reference a function signature that changed three releases ago and nothing
# says a word.
#
# That is not hypothetical. internal/dataentry/e2e_test.go (tag `e2e`) stopped
# compiling when dataentry.NewApp gained store.VersionService,
# search.VisibleSearcher and state.KV, and sat broken until someone ran the vet
# by hand.
#
# `go vet` type-checks without running anything, which is what is wanted here:
# these tests need Postgres, a browser or a human reading narrated output, so
# CI compiles them rather than runs them.
#
# The tag list is DERIVED from the source, not hardcoded — a new tag is covered
# the day it is introduced, without anyone remembering to edit this file.
#
# Usage:
#   scripts/check-tagged-tests.sh            # vet every discovered tag
#   scripts/check-tagged-tests.sh tags       # print discovered tags, one per line
set -uo pipefail

SCRIPT_DIR=$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)
ROOT=${RELA_ROOT:-$(cd "$SCRIPT_DIR/.." && pwd)}

# GOOS/GOARCH values are build constraints too, but they select a platform
# rather than opt into a file, and `go vet -tags windows` is not how you check
# them (it type-checks against the HOST platform and collides with the real
# GOOS). Cross-platform files are covered by `GOOS=<x> go vet` instead, which
# is a different concern; skip them here so they are not mistaken for opt-in
# tags.
is_platform_tag() {
  case "$1" in
    aix | android | darwin | dragonfly | freebsd | hurd | illumos | ios | js | \
      linux | nacl | netbsd | openbsd | plan9 | solaris | wasip1 | windows | \
      386 | amd64 | arm | arm64 | loong64 | mips | mips64 | mips64le | mipsle | \
      ppc64 | ppc64le | riscv64 | s390x | wasm | \
      cgo | race | msan | asan | gc | gccgo | unix | purego)
      return 0
      ;;
  esac
  return 1
}

# discover_tags prints the sorted, unique set of opt-in build tags that appear
# on _test.go files.
#
# Only PLAIN tags count. A negated constraint (`//go:build !postgres`) marks a
# file that compiles by DEFAULT — it is already covered by the normal test run,
# and vetting `-tags postgres` correctly excludes it. Likewise an ANDed term in
# a negation is not an opt-in. So the parse keeps only bare identifiers from
# the constraint expression and drops anything carrying a `!`.
discover_tags() {
  # Find the //go:build line of every _test.go file. Exclude vendor/ and
  # node_modules/ — third-party tags are not ours to keep compiling.
  find "$ROOT" \
    -type d \( -name vendor -o -name node_modules -o -name .git \) -prune -o \
    -type f -name '*_test.go' -print |
    while IFS= read -r f; do
      # The //go:build line must appear before the package clause.
      sed -n '/^package /q; /^\/\/go:build /p' "$f"
    done |
    sed 's|^//go:build ||' |
    # Split the constraint expression into terms.
    tr '()' '  ' |
    tr ' ' '\n' |
    # Drop operators, empties, and any term with a negation.
    grep -Ev '^(\|\||&&)?$' |
    grep -v '!' |
    grep -Ev '^(\|\||&&)$' |
    grep -E '^[A-Za-z0-9_.]+$' |
    sort -u
}

mapfile -t ALL_TAGS < <(discover_tags)

TAGS=()
for t in "${ALL_TAGS[@]:-}"; do
  [ -n "$t" ] || continue
  if is_platform_tag "$t"; then
    continue
  fi
  TAGS+=("$t")
done

if [ "${1:-}" = "tags" ]; then
  printf '%s\n' "${TAGS[@]:-}"
  exit 0
fi

if [ "${#TAGS[@]}" -eq 0 ]; then
  # Not an error. If every tagged test file is deleted, there is nothing to
  # compile and the guard has simply gone quiet — it must not fail the build,
  # and it must not silently stop existing either, hence the message.
  echo "No build-tag-gated _test.go files found; nothing to compile."
  exit 0
fi

echo "Build tags found on _test.go files: ${TAGS[*]}"
echo

fail=0
for tag in "${TAGS[@]}"; do
  echo "==> go vet -tags $tag ./..."
  # `go vet` over ./... with a tag that matches no file is a no-op that exits
  # 0, so an empty match needs no special case.
  if ! (cd "$ROOT" && go vet -tags "$tag" ./...); then
    echo "ERROR: tagged test files do not compile under -tags $tag." >&2
    fail=1
  fi
done

echo
if [ "$fail" -ne 0 ]; then
  cat >&2 <<'MSG'
A build-tag-gated test file no longer compiles.

These files are not built by `go build ./...` or by the normal `go test` run,
so nothing else in CI would have caught this. Fix the test to match the current
API, or delete it if the coverage has moved elsewhere.

Reproduce locally with the failing command above.
MSG
  exit 1
fi

echo "OK: all build-tag-gated test files compile."

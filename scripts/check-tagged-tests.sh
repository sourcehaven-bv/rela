#!/usr/bin/env bash
# Compile every build-tag-gated Go file in the repo.
#
# `go build ./...` does not compile _test.go files, and neither it nor
# `go test ./...` compiles files whose build constraints the default tag set
# does not satisfy. So a file behind `//go:build e2e` is invisible to every
# other CI step: it can reference a function signature that changed three
# releases ago and nothing says a word.
#
# This covers PRODUCTION files too, not just tests. `//go:build sqlite` code is
# skipped by the default build for exactly the same reason, so scoping the
# guard to _test.go would leave half the rot uncovered.
#
# That is not hypothetical. internal/dataentry/e2e_test.go (tag `e2e`) stopped
# compiling when dataentry.NewApp gained store.VersionService,
# search.VisibleSearcher and state.KV, and sat broken until someone ran the vet
# by hand.
#
# `go vet` type-checks without running anything, which is what is wanted here:
# the tagged tests need Postgres, a browser or a human reading narrated output,
# so CI compiles them rather than runs them.
#
# The tag list is DERIVED from the source, not hardcoded — a new tag is covered
# the day it is introduced, without anyone remembering to edit this file.
# Discovery lives in scripts/tagged_build_tags.go, which uses Go's own
# go/build/constraint parser; see that file for why this is not done with grep.
#
# Usage:
#   scripts/check-tagged-tests.sh            # vet every discovered tag
#   scripts/check-tagged-tests.sh tags       # print discovered tags, one per line
set -uo pipefail

SCRIPT_DIR=$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)
ROOT=${RELA_ROOT:-$(cd "$SCRIPT_DIR/.." && pwd)}
DISCOVER="$SCRIPT_DIR/tagged_build_tags.go"

MODE=${1:-vet}
case "$MODE" in
  vet | tags) ;;
  *)
    echo "usage: $(basename "$0") [tags]" >&2
    exit 2
    ;;
esac

# Fail loudly on a broken environment. Without these checks a missing root or a
# missing toolchain would fall through to "no tagged files found" and exit 0 —
# a false negative arriving by the error path instead of the parse path, which
# is the very shape this guard exists to eliminate.
if [ ! -d "$ROOT" ]; then
  echo "ERROR: root is not a directory: $ROOT" >&2
  exit 2
fi
if [ ! -f "$DISCOVER" ]; then
  echo "ERROR: discovery program not found: $DISCOVER" >&2
  exit 2
fi
if ! command -v go >/dev/null 2>&1; then
  echo "ERROR: go is not on PATH" >&2
  exit 2
fi

# Discovery runs from the script's own directory, not $ROOT: `go run` needs a
# module context, and $ROOT may be an arbitrary tree (the guard's own tests
# point it at throwaway modules).
if ! TAG_LIST=$(cd "$SCRIPT_DIR" && go run "$DISCOVER" -root "$ROOT" -pattern '*.go' 2>&1); then
  echo "ERROR: build-tag discovery failed:" >&2
  echo "$TAG_LIST" >&2
  exit 2
fi

TAGS=()
while IFS= read -r tag; do
  [ -n "$tag" ] || continue
  TAGS+=("$tag")
done <<<"$TAG_LIST"

if [ "$MODE" = "tags" ]; then
  # Guarded: an unguarded `printf '%s\n' "${TAGS[@]:-}"` prints a blank line on
  # an empty array, which a caller piping to wc -l or xargs would count.
  if [ "${#TAGS[@]}" -gt 0 ]; then
    printf '%s\n' "${TAGS[@]}"
  fi
  exit 0
fi

if [ "${#TAGS[@]}" -eq 0 ]; then
  # Not an error. If every tagged file is deleted, there is nothing to compile
  # and the guard has simply gone quiet — it must not fail the build, and it
  # must not silently stop existing either, hence the message.
  echo "No build-tag-gated Go files found; nothing to compile."
  exit 0
fi

echo "Build tags found on Go files: ${TAGS[*]}"
echo

fail=0
for tag in "${TAGS[@]}"; do
  echo "==> go vet -tags $tag ./..."
  # Each tag is vetted on its own, matching how the files are actually built:
  # tags are additive, so combining mutually exclusive ones (`postgres` against
  # a `!postgres` file) would exclude the very files being checked. This does
  # not exercise tag COMBINATIONS; no file in this repo requires two opt-in
  # tags at once, and adding the power set would cost more than it catches.
  #
  # A failure does not break the loop: one rotted tag must not mask another.
  if ! (cd "$ROOT" && go vet -tags "$tag" ./...); then
    echo "ERROR: tagged files do not compile under -tags $tag." >&2
    fail=1
  fi
done

echo
if [ "$fail" -ne 0 ]; then
  cat >&2 <<'MSG'
A build-tag-gated file no longer compiles.

These files are not built by `go build ./...` or by the normal `go test` run,
so nothing else in CI would have caught this. Fix the file to match the current
API, or delete it if its coverage has moved elsewhere.

Reproduce locally with the failing command above.
MSG
  exit 1
fi

echo "OK: all build-tag-gated files compile."

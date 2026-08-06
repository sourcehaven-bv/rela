#!/usr/bin/env bash
# Guards against shipping a binary whose embedded Vue SPA is empty
# (BUG-2YZ575, and BUG-W144 before it).
#
# The failure is silent by construction: internal/dataentry/static/v2/ is
# gitignored, so a checkout without a frontend build embeds a tree with no
# index.html — and the Go build stays green, so the release publishes a dead
# UI. internal/dataentry/app_editor_dist/ is worse: a committed .gitkeep means
# the embed glob ALWAYS matches, so a missing editor bundle is not even a
# build error.
#
# Two independent checks:
#
#   tree <dir>            the Vite output about to be embedded: index.html and
#                         its referenced entry assets exist and are non-empty.
#                         Catches a build that silently produced nothing.
#
#   binary <bin> <dir>    a built binary contains the exact hashed asset names
#                         referenced by <dir>/index.html. This is the
#                         load-bearing check: it validates what really ships,
#                         so dropping or reordering the build step fails the
#                         release instead of quietly reverting the fix.
#
# Why the exact hashed name and not a fixed pattern: the binary legitimately
# contains the literal "static/v2" in embed ERROR MESSAGES, so a naive grep is
# a false pass. A hardcoded `index-` pattern would work today but is only a
# rollup default (vite.editor.config.ts already overrides output naming
# elsewhere in this repo) — a rename would cause a confusing false FAIL, and
# the tempting "fix" is to loosen the pattern until it matches those error
# strings again. Deriving the name from index.html cannot rot that way, and is
# strictly stronger: it proves the binary embeds THIS build, not an older one.
set -euo pipefail

usage() {
  echo "usage: $0 tree <dist-dir>" >&2
  echo "       $0 binary <binary-path> <dist-dir>" >&2
  exit 2
}

# Extract the entry asset paths (assets/<name>-<hash>.js|css) that index.html
# actually references. Vite fingerprints these, so they are unique to a build.
entry_assets() {
  local index_html="$1"
  grep -o -E '[A-Za-z0-9_./-]*assets/[A-Za-z0-9_.-]+\.(js|css)' "$index_html" |
    sed 's#^/##' | sort -u
}

check_tree() {
  local dir="$1"

  if [ ! -d "$dir" ]; then
    echo "FAIL: SPA build dir does not exist: $dir" >&2
    echo "      Run \`just build-frontend\` before building the binary." >&2
    return 1
  fi

  if [ ! -s "$dir/index.html" ]; then
    echo "FAIL: missing or empty $dir/index.html" >&2
    echo "      The Vite build did not produce an SPA entry point." >&2
    return 1
  fi

  if [ ! -d "$dir/assets" ]; then
    echo "FAIL: $dir/assets/ does not exist" >&2
    echo "      index.html is present but the Vite build emitted no bundle." >&2
    return 1
  fi

  local assets
  assets=$(entry_assets "$dir/index.html")
  if [ -z "$assets" ]; then
    echo "FAIL: $dir/index.html references no assets/ bundle" >&2
    echo "      Expected at least one <script>/<link> into assets/." >&2
    return 1
  fi

  # Every referenced asset must exist AND be non-empty: a truncated or
  # interrupted write leaves a 0-byte entry chunk that a existence-only
  # check would happily accept.
  local a
  while IFS= read -r a; do
    if [ ! -s "$dir/$a" ]; then
      echo "FAIL: referenced asset missing or empty: $dir/$a" >&2
      echo "      The Vite build did not complete." >&2
      return 1
    fi
  done <<<"$assets"

  echo "OK: SPA build tree populated ($dir; $(echo "$assets" | wc -l | tr -d ' ') entry assets)"
}

# grep_count echoes the match count, distinguishing "no match" (exit 1) from a
# real scan error (exit >1). Collapsing the two would report a read failure as
# "you forgot to build the frontend" and send the next reader down the wrong
# path.
grep_count() {
  local pattern="$1" file="$2" n rc
  set +e
  n=$(LC_ALL=C grep -c -F -a -- "$pattern" "$file" 2>/dev/null)
  rc=$?
  set -e
  if [ "$rc" -gt 1 ]; then
    echo "FAIL: could not scan $file (grep exit $rc)" >&2
    return 1
  fi
  echo "${n:-0}"
}

check_binary() {
  local bin="$1" dir="$2"

  if [ ! -f "$bin" ]; then
    echo "FAIL: binary does not exist: $bin" >&2
    return 1
  fi
  if [ ! -s "$dir/index.html" ]; then
    echo "FAIL: cannot derive expected assets: missing $dir/index.html" >&2
    return 1
  fi

  local assets
  assets=$(entry_assets "$dir/index.html")
  if [ -z "$assets" ]; then
    echo "FAIL: $dir/index.html references no assets to look for" >&2
    return 1
  fi

  local a n found=0
  while IFS= read -r a; do
    n=$(grep_count "$a" "$bin") || return 1
    if [ "$n" -gt 0 ]; then
      found=$((found + 1))
    fi
  done <<<"$assets"

  if [ "$found" -eq 0 ]; then
    echo "FAIL: $bin embeds none of the expected SPA assets" >&2
    echo "      Expected (from $dir/index.html):" >&2
    echo "$assets" | sed 's/^/        /' >&2
    echo "      The binary was built without running \`just build-frontend\`," >&2
    echo "      so //go:embed captured an empty tree and the web UI is dead." >&2
    return 1
  fi

  echo "OK: $bin embeds the SPA ($found of $(echo "$assets" | wc -l | tr -d ' ') entry assets matched)"
}

case "${1:-}" in
  tree)
    [ $# -eq 2 ] || usage
    check_tree "$2"
    ;;
  binary)
    [ $# -eq 3 ] || usage
    check_binary "$2" "$3"
    ;;
  *) usage ;;
esac

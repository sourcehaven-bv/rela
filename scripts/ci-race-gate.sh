#!/usr/bin/env bash
# Run the tests that race goroutines on purpose under -race.
#
# Usage: scripts/ci-race-gate.sh [-tags <tags>] [<dir>...]
#
# PRs and the merge queue run the full suites without -race, and a push to
# develop or main runs them with it (see the Test job in ci.yml). This script
# is the part of -race that still gates the merge. It selects tests by name,
# so a new concurrency test joins the gate when it is named to match:
#
#   1. top-level tests named Test*Concurren*, Test*Race* or Test*Parallel*,
#      in the packages under <dir> (default: internal cmd third_party);
#   2. subtests with those names inside the shared conformance suites. go test
#      matches -run against each level of a test name separately, so these
#      are selected by their full path.
set -euo pipefail

tags=()
if [ "${1:-}" = -tags ]; then
	tags=(-tags "${2:?-tags needs a value}")
	shift 2
fi
dirs=("$@")
if [ "${#dirs[@]}" -eq 0 ]; then
	dirs=(internal cmd third_party)
fi

name='(Concurren|Race|Parallel)'

packages_with() {
	grep -rlE --include='*_test.go' "$1" "${dirs[@]}" | xargs -r -n1 dirname | sort -u | sed 's|^|./|'
}

mapfile -t pkgs < <(packages_with "^func Test[A-Za-z0-9_]*${name}")
if [ "${#pkgs[@]}" -eq 0 ]; then
	echo "::error::No concurrency tests found under ${dirs[*]}; the race gate would test nothing."
	exit 1
fi
go test -race "${tags[@]}" -shuffle=on -run "^Test[A-Za-z0-9_]*${name}" "${pkgs[@]}"

# Paths of the concurrency subtests in commentstest, leasetest, pilestest,
# jobstest, visibilitytest (level 2) and storetest (CAS and VersionTags).
mapfile -t conf < <(packages_with "^func Test[A-Za-z0-9_]*Conformance")
if [ "${#conf[@]}" -gt 0 ]; then
	go test -race "${tags[@]}" -run "Conformance/${name}" "${conf[@]}"
	go test -race "${tags[@]}" -run "^TestConformance\$/^CAS\$/${name}" "${conf[@]}"
	go test -race "${tags[@]}" -run "^TestConformance\$/^VersionTags\$/^(MoveAndRemove|Lifecycle)\$/${name}" "${conf[@]}"
fi

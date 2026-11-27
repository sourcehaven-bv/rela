---
id: BUG-5CS40M2
type: bug
title: Pre-commit full race run times out on the sqlite packages
description: The pre-commit hook ran go test -race over all packages with output discarded; on a loaded machine it timed out in untouched sqlite packages and reported only "Tests failed". It now tests only the staged packages with a named timeout and prints what failed.
priority: medium
why1: The hook ran go test -race over every package with the default 10m per-package timeout and sent all output to /dev/null, so a timeout in an untouched sqlite package failed the commit with only "Tests failed".
why2: The hook was written when the full suite was small; it never scoped the run to the staged change, and nothing printed which package failed or why.
why3: The installed hook is a copy made by just install-hooks, so improvements to scripts/ never reached existing checkouts (the installed pre-push had already drifted from scripts/pre-push).
why4: Local hooks are treated as a convenience next to CI, so nobody owned their runtime budget as the race suite grew into multi-minute sqlite conformance packages.
why5: There was no rule that a local gate must be bounded by the size of the change and must report its own failure; a gate that hides its output and scales with the repo becomes a blocker on a loaded machine.
prevention: The hook tests only packages that contain staged .go files, with a named timeout (RELA_PRECOMMIT_TEST_TIMEOUT, default 5m), and prints the failing packages, the stuck tests on a timeout and the failure lines. just install-hooks installs stubs that run scripts/<hook> from the checked-out tree, so hook fixes take effect without a reinstall. Measure AM-pre-commit-names-failing-packages is active.
started: "2026-11-27"
completed: "2026-11-27"
status: done
---

## Description

The pre-commit hook runs `go test -race ./... > /dev/null` and reports only
"Tests failed". On a loaded machine (an M-series laptop with other sessions
running) a full race run on develop-based branch claude/r2r-TKT-UH2Q9MN failed
in three packages, none touched by the branch:

- `internal/channels/sqlchannels`: `TestConformance` hit the 10m package timeout (it passes alone in about 98s with -race).
- `internal/store/sqlitestore`: timed out after 10m in `TestGraphQueryExplainFlatWorldPageUsesDerivedListIndex`.
- `cmd/rela-desktop`: `TestChromeStyle_TargetsShippedClasses` fails with "no stylesheets in the embedded SPA" until `just build-frontend` has run in the worktree.

Because the hook discards the output, the cause is invisible and a commit of
ticket files alone is blocked.

## Reproduce

In a fresh worktree on develop, without a frontend build, run `go test -race
./...` while other builds load the machine.

## Ideas

Print the failing packages in the hook (for example `go test -race ./... 2>&1 |
grep -E "^(FAIL|panic)"`), raise or split the timeout of the sqlite conformance
packages, and skip or explain the desktop SPA test when the bundle is not built.

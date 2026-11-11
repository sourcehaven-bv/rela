---
id: BUG-5CS40M2
type: bug
title: Pre-commit full race run times out on the sqlite packages
description: The pre-commit hook's full go test -race run times out in sqlchannels and sqlitestore on a loaded machine and fails rela-desktop without a frontend build; it hides the output.
priority: medium
status: backlog
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

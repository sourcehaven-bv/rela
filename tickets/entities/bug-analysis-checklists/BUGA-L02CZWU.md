---
id: BUGA-L02CZWU
type: bug-analysis-checklist
title: 'Analysis: Pre-commit full race run times out on the sqlite packages'
started: "2026-11-27"
status: done
---

<!-- @managed: claude-workflow v1 -->

## Reproduction

- [x] Bug reproduced locally (by reading the hook: `go test -race ./... > /dev/null 2>&1` discards all output; a forced failing and a forced sleeping test confirm the new output paths)
- [x] Minimal reproduction steps documented (ticket body: full race run on a loaded machine)
- [x] Environment/conditions noted (loaded M-series laptop, no frontend build in the worktree)

## Root Cause

- [x] Immediate cause identified (why1)
- [x] Contributing factors found (why2-3)
- [x] Systemic cause explored (why4-5)

## Fix Planning

- [x] Fix approach determined (test only staged packages, named timeout, print failures; install stub hooks)
- [x] ~~Regression test planned~~ (N/A: shell hook with no test harness; verified by hand with forced failing and timing-out tests)
- [x] Related areas checked for similar issues (pre-push copy had drifted; fixed by the stub install. Lint and gofmt still run repo-wide, out of scope)

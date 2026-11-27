---
id: BUGA-0PJJ5PN
type: bug-analysis-checklist
title: 'Analysis: Two flaky tests, one root cause — fixed timeouts sized against uninstrumented, uncontended local timing'
started: "2026-11-27"
status: done
---

<!-- @managed: claude-workflow v1 -->

## Reproduction

- [x] ~~Bug reproduced locally~~ (intermittent under CI load only; the ticket already records a pass and a fail on identical bytes in one `just ci` run, and the fixed 2s wait is visible in the code)
- [x] Minimal reproduction steps documented (ticket body: `just ci` test pass vs. coverage pass)
- [x] Environment/conditions noted (-race, coverage instrumentation, contended CI runner)

## Root Cause

- [x] Immediate cause identified (why1)
- [x] Contributing factors found (why2-3)
- [x] Systemic cause explored (why4-5)

## Fix Planning

- [x] Fix approach determined (wait on the event with a 30s outer deadline instead of a fixed 2s wait)
- [x] Regression test planned (`go test -race -count=100` on the watcher tests)
- [x] Related areas checked for similar issues (all five 2s waits in `internal/fswatch/watcher_test.go`; instance 1 fixed by #1875; docscapture covered by the retry from #1618)

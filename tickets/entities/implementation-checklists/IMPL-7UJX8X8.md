---
id: IMPL-7UJX8X8
type: implementation-checklist
title: 'Implementation: Two flaky tests, one root cause — fixed timeouts sized against uninstrumented, uncontended local timing'
started: "2026-11-27"
status: done
---

<!-- @managed: claude-workflow v1 -->

## Development

- [x] ~~Unit tests written for new code~~ (N/A: the change is itself a test fix; no production code changed)
- [x] ~~Integration tests written (test full flow, not just units)~~ (N/A: test-only change; the watcher tests already exercise real fsnotify)
- [x] Happy path implemented (waitForEvent helper; all five fixed 2s waits replaced)
- [x] Edge cases from planning handled (unrelated or split debounce batches are skipped; the absence check keeps its positive .md sentinel)
- [x] Error handling in place (errors surfaced, not swallowed) (timeout message lists every event seen; modify/delete now fail instead of only logging)

## Test Quality

- [x] ~~Using fixture builders or factories for test data~~ (N/A: temp-dir files only)
- [x] No hardcoded values in assertions when object is in scope
- [x] Only specifying values that matter for the test
- [x] Interpolated values constructed from objects, not hardcoded
- [x] Property comparisons use original object, not hardcoded strings

## Manual Verification

- [x] Feature manually tested end-to-end (go test -race -count=100 on the flaky test)
- [x] Each acceptance criterion verified with test scenario from planning
- [x] Edge cases manually verified (go test -race -count=20 -run TestWatcher_ covers modify/delete and the sentinel test)

**Verification Evidence:**
`go test -race -count=100 -run TestWatcher_AutoWatchesNewDirectories ./internal/fswatch/`: ok (6.5s). `go test -race -count=20 -run TestWatcher_ ./internal/fswatch/`: ok. `go test -race ./internal/fswatch/`: ok.

## Quality

- [x] Code follows project patterns (check similar code) (mirrors the existing waitWatched helper)
- [x] Checked for DRY opportunities — repeated literals, expressions, or
patterns extracted to a helper / constant / type where it sharpens the contract
(don't extract for its own sake; CLAUDE.md "three similar lines is better than a
premature abstraction" still holds)
- [x] No security issues introduced
- [x] No silent failures (errors logged AND returned)
- [x] No debug code left behind

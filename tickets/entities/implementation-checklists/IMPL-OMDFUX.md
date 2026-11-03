---
id: IMPL-OMDFUX
type: implementation-checklist
title: 'Implementation: Job queue Close returns before running handlers stop; log-capture tests leave log output on a dead buffer'
started: "2026-10-18"
completed: "2026-10-18"
status: done
---

<!-- @managed: claude-workflow v1 -->

## Development

- [x] Unit tests written for new code (TestCapture_RestoresLogOutput, TestMemoryQueue_CloseGivesUpOnAStuckHandler)
- [x] Integration tests written (test full flow, not just units) (jobstest case CloseWaitsForRunningHandlers runs against the memory and postgres queues)
- [x] Happy path implemented
- [x] Edge cases from planning handled (handler that ignores cancellation: Close returns an error at its deadline; dispatch after Close returns ErrClosed; Close does not hold mu while waiting, so a handler that enqueues cannot deadlock it)
- [x] Error handling in place (errors surfaced, not swallowed) (Close returns the deadline error; appbuild's closeJobQueue logs it)

## Test Quality

- [x] Using fixture builders or factories for test data (existing startQueue and newQueue helpers)
- [x] No hardcoded values in assertions when object is in scope
- [x] Only specifying values that matter for the test
- [x] Interpolated values constructed from objects, not hardcoded
- [x] Property comparisons use original object, not hardcoded strings

## Manual Verification

- [x] ~~Feature manually tested end-to-end~~ (N/A: shutdown ordering and test helpers; covered by the tests and the probe below)
- [x] Each acceptance criterion verified with test scenario from planning
- [x] Edge cases manually verified

**Verification Evidence:**

- `go test -race -run 'TestMemoryQueue_Conformance/CloseWaits' -count=20 ./internal/jobs`: 20 of 20 fail on develop, 0 of 20 with the fix. Postgres queue (handler runs 1.5 s past the cancel, beyond neoq's 1 s grace): 3 of 3 fail without the fix, pass with it.
- Probe replaying the CI sequence (queue-mode job in a busy Lua loop, `Services.Close`, then the old capture pattern), 100 separate `-race -test.cpu=1` processes: 23 of 100 report the CI race on develop; 0 of 100 with the queue fix alone, with the capture fix alone, and with both.
- `go test -race -shuffle=on -count=2` over appbuild, jobs, scheduler and testutil: pass. `go test -race -shuffle=on` over the 15 other packages whose log swaps moved to logtest: pass. `go test -race -tags postgres ./internal/jobs/...` against a local database: pass.

## Quality

- [x] Code follows project patterns (check similar code)
- [x] Checked for DRY opportunities — repeated literals, expressions, or
patterns extracted to a helper / constant / type where it sharpens the contract
(don't extract for its own sake; CLAUDE.md "three similar lines is better than a
premature abstraction" still holds) (logtest.SetDefault and logtest.Capture replace
the hand-written default-logger swaps in 16 packages)
- [x] No security issues introduced
- [x] No silent failures (errors logged AND returned)
- [x] No debug code left behind (probe test removed)

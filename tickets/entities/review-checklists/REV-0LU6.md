---
id: REV-0LU6
type: review-checklist
title: 'Review: Wire workspace search backend as fsstore Observer (drop Subscribe goroutine)'
status: done
---

## Code Review

- [x] cranky-code-reviewer agent run on the diff
- [x] Critical findings addressed in-PR (#1 Close ordering)
- [x] Significant findings addressed in-PR (#3, #4, #5, #6)
- [x] Minor findings addressed (#7, #8, #9)
- [x] Deferred findings tracked (fsstore observer-error swallowing — separable ticket)
- [x] Tests pass under `-race`
- [x] `just ci` passes end-to-end

**Cranky-review disposition table:** see IMPL-RESP for the full table. Summary:

- 1 critical fix in-PR (Close ordering)
- 4 significant fixes in-PR (factory mutation hidden behind AddObserver, consumer-side interface, WithTestStore docs, idempotency comment)
- 3 minor fixes in-PR (test name, rename coverage, recordingObserver helper)
- 1 critical and 2 leverage findings deferred (fsstore error swallowing + test-builder + bridgePaths helper)

**Code Review Summary:**

Pre-PR review caught a real Close-ordering bug (search backend was being closed
while the store still held it as observer) and a CLAUDE.md violation (concrete
type assertion `factory.(*app.FSFactory)`). Both fixed via reordering Close + a
consumer-side `observerWiringFactory` interface in workspace. AddObserver method
on FSFactory cleanly hides the mutation that was previously a public-field
append. Net diff still a clean lift; the additional structure pays back the
cranky concerns without adding LOC.

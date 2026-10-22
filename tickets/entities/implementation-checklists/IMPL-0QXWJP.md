---
id: IMPL-0QXWJP
type: implementation-checklist
title: 'Implementation: Triage semgrep findings from scheduled security scan (GH #1923)'
started: "2026-10-22"
completed: "2026-10-22"
status: done
---

<!-- @managed: claude-workflow v1 -->

## Development

- [x] ~~Unit tests written for new code~~ (N/A: Go changes are comments only; the scanner run is the check)
- [x] ~~Integration tests written (test full flow, not just units)~~ (N/A: no behavior change)
- [x] Happy path implemented
- [x] Edge cases from planning handled
- [x] ~~Error handling in place (errors surfaced, not swallowed)~~ (N/A: no new code paths)

## Test Quality

- [x] ~~Using fixture builders or factories for test data~~ (N/A: no new tests)
- [x] ~~No hardcoded values in assertions when object is in scope~~ (N/A: no new tests)
- [x] ~~Only specifying values that matter for the test~~ (N/A: no new tests)
- [x] ~~Interpolated values constructed from objects, not hardcoded~~ (N/A: no new tests)
- [x] ~~Property comparisons use original object, not hardcoded strings~~ (N/A: no new tests)

## Manual Verification

- [x] Feature manually tested end-to-end
- [x] Each acceptance criterion verified with test scenario from planning
- [x] Edge cases manually verified

**Verification Evidence:**
- semgrep 1.181.0 `scan --config p/default --severity ERROR` on develop before the change: the same 10 findings as #1923. After: 7 results, each with a suppression; the 3 run-shell-injection results are gone. Unsuppressed count by the workflow's jq expression: 0.
- golangci-lint on touched packages: 0 issues. `just comment-lint`: clean.
- go test for cmdexec, docscapture, lock, piles, pgstore, dataentry: pass.

## Quality

- [x] Code follows project patterns (check similar code)
- [x] ~~Checked for DRY opportunities~~ (N/A: comment-only Go changes)
- [x] No security issues introduced
- [x] ~~No silent failures (errors logged AND returned)~~ (N/A: no new code paths)
- [x] No debug code left behind

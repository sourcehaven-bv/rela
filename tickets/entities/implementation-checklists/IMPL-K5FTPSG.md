---
id: IMPL-K5FTPSG
type: implementation-checklist
title: 'Implementation: Triage 2026-11-02 security scan findings (GH #2073, #2074, #2076)'
started: "2026-11-04"
completed: "2026-11-04"
status: done
---

<!-- @managed: claude-workflow v1 -->

## Development

- [x] ~~Unit tests written for new code~~ (N/A: no new product code; a rename, suppressions, a dependency override and demo-script requests)
- [x] Integration tests written (test full flow, not just units) (the seqtrace routes suite now requests both inbox routes end to end)
- [x] Happy path implemented
- [x] Edge cases from planning handled (realtime.yaml loads; the routes answer 200, not the realtime-off 404)
- [x] Error handling in place (errors surfaced, not swallowed)

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

1. semgrep 1.181.0 `p/default --severity ERROR` over the repo: 9 results,
0 without `suppressions`. `filecomments/file.go` results at the two returns
carry a suppression. `cmdexec.go:387` was already suppressed before this change;
alert #81 is open only because #2030 moved the line and GitHub opens a new alert
for a suppressed result (earlier ones were dismissed by hand).
2. `e2e/package-lock.json` resolves `minimatch` 11.0.4 (one copy).
`npm run lint` passes and lints 131 files; `npm run typecheck` passes.
3. `just archplan-security`: `route GET /api/v1/_inbox` and
`/_inbox/summary` as reader answer 200; steps.json names `handleV1Inbox` and
`handleV1InboxSummary` as handlers; `security.untraced-08311e3c50b2` is gone.
The two `untested_denial` findings this surfaced are accepted with a reason; no
other new finding.
4. `go test ./internal/dataentry -run 'Inbox|Mention'` and
`go test ./internal/comments/...` pass.

## Quality

- [x] Code follows project patterns (check similar code)
- [x] Checked for DRY opportunities
- [x] No security issues introduced
- [x] No silent failures (errors logged AND returned)
- [x] No debug code left behind

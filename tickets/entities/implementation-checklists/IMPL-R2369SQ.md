---
id: IMPL-R2369SQ
type: implementation-checklist
title: 'Implementation: seqtrace routes suite: trace the audit, changes and next-action routes'
started: "2026-11-21"
completed: "2026-11-21"
status: done
---

## Development

- [x] Unit tests written for new code
- [x] ~~Integration tests written (test full flow, not just units)~~ (N/A: trace scenarios and tooling, no behavior change)
- [x] Happy path implemented
- [x] ~~Edge cases from planning handled~~ (N/A: trace scenarios and tooling, no behavior change)
- [x] ~~Error handling in place (errors surfaced, not swallowed)~~ (N/A: trace scenarios and tooling, no behavior change)

## Test Quality

- [x] ~~Using fixture builders or factories for test data~~ (N/A: trace scenarios and tooling, no behavior change)
- [x] ~~No hardcoded values in assertions when object is in scope~~ (N/A: trace scenarios and tooling, no behavior change)
- [x] ~~Only specifying values that matter for the test~~ (N/A: trace scenarios and tooling, no behavior change)
- [x] ~~Interpolated values constructed from objects, not hardcoded~~ (N/A: trace scenarios and tooling, no behavior change)
- [x] ~~Property comparisons use original object, not hardcoded strings~~ (N/A: trace scenarios and tooling, no behavior change)

## Manual Verification

- [x] Feature manually tested end-to-end
- [x] Each acceptance criterion verified with test scenario from planning
- [x] ~~Edge cases manually verified~~ (N/A: trace scenarios and tooling, no behavior change)

**Verification Evidence:**
<!-- Document what you tested and the results -->

## Quality

- [x] Code follows project patterns (check similar code)
- [x] ~~Checked for DRY opportunities — repeated literals, expressions, or~~ (N/A: trace scenarios and tooling, no behavior change)
patterns extracted to a helper / constant / type where it sharpens the contract
(don't extract for its own sake; CLAUDE.md "three similar lines is better than a
premature abstraction" still holds)
- [x] No security issues introduced
- [x] ~~No silent failures (errors logged AND returned)~~ (N/A: trace scenarios and tooling, no behavior change)
- [x] No debug code left behind

**Verification Evidence:**

1. `SEQTRACE_SUITE=routes tools/seqtrace/demo/run.sh`: every new request
   answers its `EXPECT` status (audit 403/200, changes 200/404, next action
   GET 400, POST 204/400).
2. `archplan ci` on that trace: none of the five findings of #2369 remain, no
   ratchet metric is worse. Two new `security.untested_denial` findings remain
   (handleGitStatus, handleSSE). They are the accepted findings
   04f433aab7c3 and d40d5ae61e0c under new ids, because TKT-3KUIROC moved
   both handlers to new receivers; they appear without this change too.
3. `go test ./tools/seqtrace/... ./internal/dataentry/`, `just arch-lint`,
   golangci-lint and the comment-lint gate pass.

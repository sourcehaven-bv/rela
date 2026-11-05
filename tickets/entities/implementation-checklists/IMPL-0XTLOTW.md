---
id: IMPL-0XTLOTW
type: implementation-checklist
title: 'Implementation: Conflict resolve skips field rights and errors quote hidden values'
started: "2026-11-05"
completed: "2026-11-05"
status: done
---

<!-- @managed: claude-workflow v1 -->

## Development

- [x] Unit tests written for new code (TestConflictFieldWrite, TestValidatePropertyValue_MessageNeverQuotesValue)
- [x] Integration tests written (test full flow, not just units) (resolve, PATCH and git status handler tests)
- [x] Happy path implemented
- [x] Edge cases from planning handled (per-side verdicts, unset, enum filter, undeclared keys, config paths, deleted files)
- [x] Error handling in place (errors surfaced, not swallowed) (gate errors answer through writeGateError)

## Test Quality

- [x] Using fixture builders or factories for test data (ticketConflict, conflictReadGateApp)
- [x] No hardcoded values in assertions when object is in scope
- [x] Only specifying values that matter for the test
- [x] Interpolated values constructed from objects, not hardcoded
- [x] Property comparisons use original object, not hardcoded strings

## Manual Verification

- [x] ~~Feature manually tested end-to-end~~ (N/A: HTTP handler tests drive the full request path; no UI change)
- [x] Each acceptance criterion verified with test scenario from planning
- [x] Edge cases manually verified (each regression test checked to fail on origin/develop)

**Verification Evidence:** The regression tests fail on origin/develop and pass
on the branch. `go test ./internal/...`, `just arch-lint`, golangci-lint on the
changed packages, and `just comment-lint` are clean.

## Quality

- [x] Code follows project patterns (check similar code) (reuses validateFieldWrite, relationMetaDenial, conflictsReadable)
- [x] Checked for DRY opportunities (outOfRange shared by integer and decimal)
- [x] No security issues introduced (security review findings addressed)
- [x] No silent failures (errors logged AND returned)
- [x] No debug code left behind

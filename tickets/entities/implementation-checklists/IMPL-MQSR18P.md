---
id: IMPL-MQSR18P
type: implementation-checklist
title: 'Implementation: Remotes page asks update right before delete for remote_deleted entities'
started: "2026-11-14"
completed: "2026-11-14"
status: done
---

<!-- @managed: claude-workflow v1 -->

## Development

- [x] Unit tests written for new code (delete-without-update case in TestRemotes_RemoteDeletedNeedsDelete)
- [x] Integration tests written (test full flow, not just units) (the test drives GET /conflicts through the HTTP handler with a compiled ACL policy)
- [x] Happy path implemented
- [x] Edge cases from planning handled (update-only still refused; faced types still refused)
- [x] ~~Error handling in place~~ (N/A: boolean permission check, no new error paths)

## Test Quality

- [x] Using fixture builders or factories for test data (newRemoteEnv, syncRole, ticketEntry)
- [x] No hardcoded values in assertions when object is in scope
- [x] Only specifying values that matter for the test
- [x] ~~Interpolated values constructed from objects~~ (N/A: no interpolated values)
- [x] ~~Property comparisons use original object~~ (N/A: assertions compare booleans)

## Manual Verification

- [x] ~~Feature manually tested end-to-end~~ (N/A: gate-only change covered by the handler test; no UI change)
- [x] Each acceptance criterion verified with test scenario from planning
- [x] Edge cases manually verified (new case fails without the fix, passes with it)

**Verification Evidence:** go test ./internal/dataentry -run TestRemotes passes;
the new case fails on develop. golangci-lint on internal/dataentry: 0 issues.
just arch-lint: OK.

## Quality

- [x] Code follows project patterns (check similar code)
- [x] Checked for DRY opportunities (none; the branch was moved, not duplicated)
- [x] No security issues introduced (the delete check is the one the DELETE route asks; faced types stay refused)
- [x] ~~No silent failures~~ (N/A: no error paths)
- [x] No debug code left behind

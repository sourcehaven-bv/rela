---
id: IMPL-2VVRP3H
type: implementation-checklist
title: 'Implementation: List-context commands receive ungated, unredacted rows on stdin'
started: "2026-11-14"
completed: "2026-11-14"
status: done
---

<!-- @managed: claude-workflow v1 -->

The fix is commit 7b6c0c8ea in PR #2247 (TKT-F2EJQY8), with the test comment
trimmed in 53b89596d.

## Development

- [x] Unit tests written for new code (`TestCommandExec_ListContextIsGatedAndRedacted` in `internal/dataentry/commands_acl_test.go`)
- [x] Integration tests written (test full flow, not just units) (`TestGoldenParity` case `BUG-2TPO2F-command-list-input` drives the HTTP command endpoint for every principal and recording; the viewer recordings show the gated rows and the `redacted` sets)
- [x] Happy path implemented (`commandHandler.visibleListRows` in `internal/dataentry/commands.go`)
- [x] Edge cases from planning handled (fully hidden row; hidden field; per-row `when:` redaction in the golden fixture; deny-all verdict returns no rows; list `filters:` run on redacted rows)
- [x] Error handling in place (errors surfaced, not swallowed) (a failed read logs and returns a generic 500 and the command does not run; the old helper swallowed a store error and returned a partial list)

## Test Quality

- [x] Using fixture builders or factories for test data (`newHandlerTestApp`, `seedEntity`, `mustNewACL`; the golden fixture)
- [x] No hardcoded values in assertions when object is in scope
- [x] Only specifying values that matter for the test
- [x] ~~Interpolated values constructed from objects, not hardcoded~~ (N/A: assertions are substring checks on sentinel values)
- [x] Property comparisons use original object, not hardcoded strings

## Manual Verification

- [x] ~~Feature manually tested end-to-end~~ (N/A: the golden parity test drives the full HTTP and SSE flow for each principal)
- [x] Each acceptance criterion verified with test scenario from planning
- [x] Edge cases manually verified (reverted the fix in a scratch edit: both pinning tests fail with the leak; restored: both pass)

**Verification Evidence:** `go test -race ./internal/dataentry -run
'TestCommandExec_ListContextIsGatedAndRedacted|TestGoldenParity'` passes with
the fix. With `visibleListRows` replaced by the old raw `Store.ListEntities`
read, the unit test reports the hidden `status` value and the hidden row
`TKT-ACL1` in the payload, and `TestGoldenParity` fails.

## Quality

- [x] Code follows project patterns (check similar code) (same gated read as `GET /api/v1/<list>`: `ReadQuery`, `scopedEntities`, `loadRowContent`, `PrimeTraversals`, `redactEntity`)
- [x] Checked for DRY opportunities (reuses the shared scoped read; `listFromStoreByTypes` and `listAllFromStore` deleted)
- [x] No security issues introduced (rela-security-reviewer: no critical or significant findings)
- [x] No silent failures (errors logged AND returned)
- [x] No debug code left behind

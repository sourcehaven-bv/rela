---
id: IMPL-2H8OS6
type: implementation-checklist
title: 'Implementation: entity.Ref and one gated resolver in internal/visibility'
status: done
---

<!-- @managed: claude-workflow v1 -->

## Development

- [x] Unit tests written for new code
- [x] Integration tests written (test full flow, not just units)
- [x] Happy path implemented
- [x] Edge cases from planning handled
- [x] Error handling in place (errors surfaced, not swallowed)

## Test Quality

- [x] Using fixture builders or factories for test data
- [x] No hardcoded values in assertions when object is in scope
- [x] Only specifying values that matter for the test
- [x] Interpolated values constructed from objects, not hardcoded
- [x] Property comparisons use original object, not hardcoded strings

## Manual Verification

- [x] Feature manually tested end-to-end
- [x] Each acceptance criterion verified with test scenario from planning
- [x] Edge cases manually verified

**Verification Evidence:**

Delivered in the Stage 1 PRs against faces-intrinsic, each with its own
`/code-review`:

- #1712 `entity.Ref` with its text codec; replaces `rowKey`, `hitKey` and `stateKey`.
- #1716 `visibility.Resolver` (Ref, InWorld, Address, Family, batched `EndpointsReadable`), `resolver_test.go`, `batch_test.go`, `endpoints_test.go`.
- #1717 and #1718 MCP and Lua reads through the resolver, `FilterRelations` on `EndpointsReadable`, batched neighbour titles with budget tests.
- #1719 and #1720 dataentry single-entity reads through the resolver, write preflights with the face gate, the extended archguard zero-face guard, batched edge warnings.
- #1722 and #1723 per-face attachments and export (BUG-CTUW2N), `attachment_face_test.go`.
- #1721 frontend sub-resources on the served face (BUG-FYEEVX), e2e on the faced fixture.
- #1724 history, restore and purge per face (BUG-4SYAA6, TKT-7R0ABK), `history_face_test.go`, storetest version tests on pg and sqlite, e2e `faces-history.spec.ts`.
- The BUG-BZQQDP PR closes the remaining relation surfaces (command payloads, gantt).

Acceptance criteria 1 to 8 of PLAN-Y1JVSB map to those tests; the e2e specs
exercise the faced fixture end to end.

## Quality

- [x] Code follows project patterns (check similar code)
- [x] Checked for DRY opportunities — repeated literals, expressions, or
patterns extracted to a helper / constant / type where it sharpens the contract
(don't extract for its own sake; CLAUDE.md "three similar lines is better than a
premature abstraction" still holds)
- [x] No security issues introduced
- [x] No silent failures (errors logged AND returned)
- [x] No debug code left behind

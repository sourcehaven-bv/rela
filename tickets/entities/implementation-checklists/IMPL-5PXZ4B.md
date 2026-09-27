---
id: IMPL-5PXZ4B
type: implementation-checklist
title: 'Implementation: related() in views, next-action, CLI filter, validation, automation, state machine and ACL when:'
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

- [x] ~~Using fixture builders or factories for test data~~ (N/A: tests use small inline fixtures, e.g. seedTickets, relatedMeta; no builder exists for these shapes)
- [x] No hardcoded values in assertions when object is in scope
- [x] Only specifying values that matter for the test
- [x] Interpolated values constructed from objects, not hardcoded
- [x] Property comparisons use original object, not hardcoded strings

## Manual Verification

- [x] Feature manually tested end-to-end
- [x] Each acceptance criterion verified with test scenario from planning
- [x] Edge cases manually verified

**Verification Evidence:**
- CLI: `rela --project=tickets list ticket --filter "entity.status == 'in-progress' and related(entity, 'implements')"` returned TKT-205V2N; the negated form returned none; an unknown relation gave `related: unknown relation type "nope"`.
- Server: a copy of the tickets project with two list `condition:`s using `related(...)` and `not related(...)`; `GET /api/v1/tickets?list_id=...` returned [TKT-205V2N] and [] respectively, matching the CLI.
- Budget tests: validation, ACL `when:` list page and view condition each make 1 MatchingIDs call at 10 and 50 rows.
- Mutation checks: an Ungated binder fails the data-entry and MCP validator gate tests; removing ACL priming makes the budget test issue 10/50 calls; removing condition index derivation fails its test.

## Quality

- [x] Code follows project patterns (check similar code)
- [x] Checked for DRY opportunities — repeated literals, expressions, or
patterns extracted to a helper / constant / type where it sharpens the contract
(don't extract for its own sake; CLAUDE.md "three similar lines is better than a
premature abstraction" still holds)
- [x] No security issues introduced
- [x] No silent failures (errors logged AND returned)
- [x] No debug code left behind

---
id: IMPL-PRFBN1S
type: implementation-checklist
title: 'Implementation: Point-in-time (as-of) reads of a world'
started: "2026-11-17"
completed: "2026-11-18"
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

- `storetest.RunInstantTests` on pgstore and sqlitestore: created after, updated after, deleted after, renamed after, face published after, relations, ACL gate on today's edges, search at the instant, scale-free header page, writes refused.
- fsstore and memstore refuse the bind (`RunInstantUnsupportedTests`).
- dataentry `TestAsOf*` on sqlite: lists, detail, relations, views, dashboard card, search, gantt, documents, 400/422/501 codes, Lua binding guard.
- `TestGantt_FollowsTheWorld` and the world tests for kanban, calendar and dashboard cards.
- SPA `asOf.test.ts`: parsing, route carrying, write refusal.

## Quality

- [x] Code follows project patterns (check similar code)
- [x] Checked for DRY opportunities — repeated literals, expressions, or
patterns extracted to a helper / constant / type where it sharpens the contract
(don't extract for its own sake; CLAUDE.md "three similar lines is better than a
premature abstraction" still holds)
- [x] No security issues introduced
- [x] No silent failures (errors logged AND returned)
- [x] No debug code left behind

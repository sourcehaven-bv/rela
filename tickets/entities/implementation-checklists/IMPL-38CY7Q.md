---
id: IMPL-38CY7Q
type: implementation-checklist
title: 'Implementation: Relation-backed status: design boards and views for a single-valued status relation'
status: done
started: '2026-10-07'
completed: '2026-10-07'
---

<!-- @managed: claude-workflow v1 -->

## Development

- [x] ~~Unit tests written for new code~~ (N/A: design ticket; the code is in the follow-up tickets)
- [x] ~~Integration tests written (test full flow, not just units)~~ (N/A: design ticket; the code is in the follow-up tickets)
- [x] ~~Happy path implemented~~ (N/A: design ticket; the code is in the follow-up tickets)
- [x] ~~Edge cases from planning handled~~ (N/A: design ticket; the code is in the follow-up tickets)
- [x] ~~Error handling in place (errors surfaced, not swallowed)~~ (N/A: design ticket; the code is in the follow-up tickets)

## Test Quality

- [x] ~~Using fixture builders or factories for test data~~ (N/A: design ticket; the code is in the follow-up tickets)
- [x] ~~No hardcoded values in assertions when object is in scope~~ (N/A: design ticket; the code is in the follow-up tickets)
- [x] ~~Only specifying values that matter for the test~~ (N/A: design ticket; the code is in the follow-up tickets)
- [x] ~~Interpolated values constructed from objects, not hardcoded~~ (N/A: design ticket; the code is in the follow-up tickets)
- [x] ~~Property comparisons use original object, not hardcoded strings~~ (N/A: design ticket; the code is in the follow-up tickets)

## Manual Verification

- [x] Feature manually tested end-to-end: the demo in examples/relation-status-demo
- [x] Each acceptance criterion verified with test scenario from planning
- [x] Edge cases manually verified: the demo seed has a task whose status its initiative does not offer, placed under Other

**Verification Evidence:**
- RES-8CKUNJ is done and records options A to D, the recommendation and the outcome.
- The follow-ups TKT-65LVAK, TKT-KJ3Q07, TKT-JO8PN3 and TKT-CADCFX are done.
- Demo server: a drag re-points the status and the detail panel follows it, the grouped list by relation works, the relation field re-points leaving exactly one edge, and `style_from` badges show in the field and the menu.

## Quality

- [x] ~~Code follows project patterns (check similar code)~~ (N/A: design ticket; the code is in the follow-up tickets)
- [x] ~~Checked for DRY opportunities~~ (N/A: design ticket; the code is in the follow-up tickets)
- [x] ~~No security issues introduced~~ (N/A: design ticket; the code is in the follow-up tickets)
- [x] ~~No silent failures (errors logged AND returned)~~ (N/A: design ticket; the code is in the follow-up tickets)
- [x] ~~No debug code left behind~~ (N/A: design ticket; the code is in the follow-up tickets)

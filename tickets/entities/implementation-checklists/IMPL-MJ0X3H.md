---
id: IMPL-MJ0X3H
type: implementation-checklist
title: 'Implementation: Entity header: one Convert menu and overflow for actions that do not fit'
started: "2026-10-24"
completed: "2026-10-24"
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
- [x] Only specifying values that matter for the test

## Manual Verification

- [x] Feature manually tested end-to-end
- [x] Each acceptance criterion verified with test scenario from planning
- [x] Edge cases manually verified

**Verification Evidence:**

- Atlas topic with three conversions at 1600px: Convert, Onderdelen, Edit,
Export, Add to pile, Duplicate, Delete on one line.
- At 1100px: Onderdelen, Edit and "⋯". The menu lists the exports, piles, the
three conversions, Duplicate and Delete.
- The LongLabelInNarrowRow story fails without the RlButton change.

## Quality

- [x] Code follows project patterns (check similar code)
- [x] No security issues introduced
- [x] No debug code left behind

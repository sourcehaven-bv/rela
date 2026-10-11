---
id: IMPL-FJLPGE
type: implementation-checklist
title: 'Implementation: Configurable backlinks section with per-type columns'
started: "2026-10-08"
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

- Demo at /tmp/rela-demo with the prototype ticket view: `GET /api/v1/_views/ticket/TKT-002` returns `hideMentionedByPanel: true` and a grouped "Mentioned by" table. The Decisions group (DEC-001) has the fallback `title` column; the Tickets group (TKT-001, 003, 005, 006) has title, status and assignee. Every row carries `_self`.
- `rela validate` on prototypes/data-entry/project accepts the new example.
- Go tests: views_mentions_test.go (per-type groups, per-type fields, cap 200 and incomplete, where past the first page, panel switch, fault fails the view), querybudget_test.go (7 reads at 10 and 50 rows, one MentionSources call), validate_mentions_test.go (19 load cases, side panel, panel-only entity_views entry).
- Frontend: EntityDetail.mentions.test.ts (per-group headers, incomplete note, panel switch); vue-tsc clean.
- Visual check in a browser not done: the Chrome extension was not connected. The SPA behaviour is covered by the component tests.

## Quality

- [x] Code follows project patterns (check similar code)
- [x] Checked for DRY opportunities — repeated literals, expressions, or
patterns extracted to a helper / constant / type where it sharpens the contract
(don't extract for its own sake; CLAUDE.md "three similar lines is better than a
premature abstraction" still holds)
- [x] No security issues introduced
- [x] No silent failures (errors logged AND returned)
- [x] No debug code left behind

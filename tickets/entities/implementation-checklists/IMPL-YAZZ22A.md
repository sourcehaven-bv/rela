---
id: IMPL-YAZZ22A
type: implementation-checklist
title: 'Implementation: Options to hide type and ID on cards and mention lists'
started: "2026-11-26"
completed: "2026-11-26"
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

**Verification Evidence:** Ran rela-server from this branch on a copy of
prototypes/data-entry/project with `hide_type: true` and `hide_id: true` on the
ticket view's "Blocked By" cards section and `hide_id: true` on "Labels". On
TKT-002, `/api/v1/_views/ticket/TKT-002` returned `hide: ["type","id"]` and
`hide: ["id"]` for those sections and no `hide` elsewhere. The page drew the
four Blocked By cards with titles only, the Labels row with its type badge and
no ID, and the other sections unchanged. Blank and empty titles keeping their ID
are covered by EntityDetail.mentions.test.ts; misplaced keys by
validate_hideidentity_test.go; the mention panel by MentionedByPanel.test.ts and
views_mentions_test.go. archplan ci against develop reports 0 metrics worse.

## Quality

- [x] Code follows project patterns (check similar code)
- [x] Checked for DRY opportunities — repeated literals, expressions, or
patterns extracted to a helper / constant / type where it sharpens the contract
(don't extract for its own sake; CLAUDE.md "three similar lines is better than a
premature abstraction" still holds)
- [x] No security issues introduced
- [x] No silent failures (errors logged AND returned)
- [x] No debug code left behind

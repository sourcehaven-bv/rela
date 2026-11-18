---
id: IMPL-YBD50DA
type: implementation-checklist
title: 'Implementation: View section permission and hiding sections with only redacted fields'
started: "2026-11-19"
completed: "2026-11-19"
status: done
---

<!-- @managed: claude-workflow v1 -->

## Development

- [x] Unit tests written for new code
- [x] Integration tests written (test full flow, not just units)
- [x] Happy path implemented
- [x] Edge cases from planning handled
- [x] ~~Error handling in place (errors surfaced, not swallowed)~~ (N/A: the change adds no error path; a filter over already-built sections)

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

**Verification Evidence:** `TestWithheldSections_PerReader` (reader vs staff
over the real ACL and resolver), `TestWithheldSections_NoPolicyShowsAll` (no
policy, read-only), `TestAllFieldsRedacted`, `TestSectionPermission_Config`, and
the Playwright spec `view-section-permission.spec.ts` run against the built
server: a customer role sees neither the all-redacted "Classification" section
nor the `permission: staff.manage` section; staff see both.

## Quality

- [x] Code follows project patterns (check similar code)
- [x] Checked for DRY opportunities — repeated literals, expressions, or
patterns extracted to a helper / constant / type where it sharpens the contract
(don't extract for its own sake; CLAUDE.md "three similar lines is better than a
premature abstraction" still holds)
- [x] No security issues introduced
- [x] ~~No silent failures (errors logged AND returned)~~ (N/A: no error path added)
- [x] No debug code left behind

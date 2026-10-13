---
id: IMPL-200S0A
type: implementation-checklist
title: 'Implementation: Template picker as dropdown with readable variant labels'
started: "2026-10-12"
status: done
---

<!-- @managed: claude-workflow v1 -->

## Development

- [x] Unit tests written for new code
- [x] Integration tests written (test full flow, not just units)
- [x] Happy path implemented
- [x] Edge cases from planning handled
- [x] ~~Error handling in place (errors surfaced, not swallowed)~~ (N/A: no new error paths)

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
Ran a Playwright session against `rela-server` with nine `feature` templates
(Default, Compliance, Corrective action, Design, Development, Documentation,
Operations, Research, Spike). The form showed a labelled "Template" dropdown
of fixed width with all nine options and no horizontal overflow; choosing
"Corrective action" selected it and applied its body.
`npx vitest run` (4141 passed), `npx playwright test tests/templates.spec.ts`
(2 passed).

## Quality

- [x] Code follows project patterns (check similar code)
- [x] Checked for DRY opportunities — repeated literals, expressions, or
patterns extracted to a helper / constant / type where it sharpens the contract
(don't extract for its own sake; CLAUDE.md "three similar lines is better than a
premature abstraction" still holds)
- [x] No security issues introduced
- [x] No silent failures (errors logged AND returned)
- [x] No debug code left behind

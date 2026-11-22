---
id: IMPL-4L2WSDN
type: implementation-checklist
title: 'Implementation: Detail action order and danger style; hide Shortcuts on phones'
started: "2026-11-22"
completed: "2026-11-22"
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
- Go: TestValidateActions_Style, TestAction_StyleAndOrderServed, TestAction_OrderRejectsFraction, TestBuiltins_* incl. TestBuiltins_HiddenOnPhone(FromYAML) (dataentryconfig); TestSidebarBuiltins_HiddenOnPhone and _InSpace (dataentry handlers). All pass.
- vitest: detailActions.test.ts (order, label, id tiebreak; undeclared and false keys dropped), EntityDetail.actions.test.ts (button order and danger tone), usePhoneScreen.test.ts, Sidebar.builtins.test.ts (shortcuts hidden, hidden_on_phone on phone vs wide). Full suite 6710 pass.
- Playwright: e2e/tests/space-landing-builtins.spec.ts, 7 pass against the built rela-server: a space with `shortcuts: {hidden_on_phone: true}` shows the button wide and drops it at 375px; a space without it keeps it on a phone.

## Quality

- [x] Code follows project patterns (check similar code)
- [x] Checked for DRY opportunities — repeated literals, expressions, or
patterns extracted to a helper / constant / type where it sharpens the contract
(don't extract for its own sake; CLAUDE.md "three similar lines is better than a
premature abstraction" still holds)
- [x] No security issues introduced
- [x] No silent failures (errors logged AND returned)
- [x] No debug code left behind

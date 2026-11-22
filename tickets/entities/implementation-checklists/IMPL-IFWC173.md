---
id: IMPL-IFWC173
type: implementation-checklist
title: 'Implementation: Trigger placeholder for the current user'
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
- Go triggers: TestEngine_CurrentUserPlaceholder (resolved user, raw principal, RawUser equal to User, no principal, stamped identity not read; relate created or skipped, set left out), TestEngine_CurrentUserLeftOutOfCreate, TestInterpolate_CurrentUser (engine only; literal for copy/conversion; not in inline Lua), parse tests (known placeholder; `{{current_user}}` and `{{current_user.name}}` refused).
- Go appbuild: TestCheckTriggerACL, the exact target passes and a longer one is still refused.
- Go entitymanager: TestCreate_CurrentUserTriggerRelatesAuthor, a real create through the cascade writes note authored-by PERS-1 for a resolved principal and nothing for a raw or missing one.

## Quality

- [x] Code follows project patterns (check similar code)
- [x] Checked for DRY opportunities — repeated literals, expressions, or
patterns extracted to a helper / constant / type where it sharpens the contract
(don't extract for its own sake; CLAUDE.md "three similar lines is better than a
premature abstraction" still holds)
- [x] No security issues introduced
- [x] No silent failures (errors logged AND returned)
- [x] No debug code left behind

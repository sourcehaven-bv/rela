---
id: IMPL-NBECV2O
type: implementation-checklist
title: 'Implementation: Root opens the space home; config for built-in sidebar rows'
started: "2026-11-18"
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

**Verification Evidence:** Playwright `e2e/tests/space-landing-builtins.spec.ts`
against a built rela-server in Chromium: root opens the first space's home, the
last used space's home after a cold load, `/dashboard` still opens the
dashboard, a space without `builtins:` keeps Search..Analysis, a space hiding
every row has no pinned rows, and without spaces the root opens `/dashboard`.
`e2e/tests/spaces.spec.ts` still passes. Go: `TestSidebarBuiltins_*`,
`TestValidateBuiltins`, `TestBuiltins_*`, `TestDataEntryPermissions_PerSurface`.
Vitest: `Sidebar.builtins.test.ts`, `router/rootRedirect.test.ts`,
`stores/space.test.ts`.

## Quality

- [x] Code follows project patterns (check similar code)
- [x] Checked for DRY opportunities — repeated literals, expressions, or
patterns extracted to a helper / constant / type where it sharpens the contract
(don't extract for its own sake; CLAUDE.md "three similar lines is better than a
premature abstraction" still holds)
- [x] No security issues introduced
- [x] No silent failures (errors logged AND returned)
- [x] No debug code left behind

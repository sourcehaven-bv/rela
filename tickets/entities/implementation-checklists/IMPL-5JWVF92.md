---
id: IMPL-5JWVF92
type: implementation-checklist
title: 'Implementation: Two-tab autosave mutant test rejects one of the two blind-write reports'
started: "2026-11-12"
completed: "2026-11-12"
status: done
---

<!-- @managed: claude-workflow v1 -->

## Development

- [x] ~~Unit tests written for new code~~ (N/A: test-only change; the fix is to two existing tests)
- [x] ~~Integration tests written (test full flow, not just units)~~ (N/A: test-only change)
- [x] Happy path implemented (both mutant tests match either blind-write report)
- [x] Edge cases from planning handled (the property-merge test gets the same assertion)
- [x] ~~Error handling in place (errors surfaced, not swallowed)~~ (N/A: no product code)

## Test Quality

- [x] ~~Using fixture builders or factories for test data~~ (N/A: no new test data)
- [x] No hardcoded values in assertions when object is in scope
- [x] Only specifying values that matter for the test
- [x] ~~Interpolated values constructed from objects, not hardcoded~~ (N/A: the regex takes only the field kind)
- [x] ~~Property comparisons use original object, not hardcoded strings~~ (N/A: assertions match violation messages)

## Manual Verification

- [x] Feature manually tested end-to-end (the test file run 30 times)
- [x] Each acceptance criterion verified with test scenario from planning
- [x] Edge cases manually verified (15 instrumented runs logged the violation kinds of both mutant tests)

**Verification Evidence:** `npx vitest run
src/composables/useAutoSave.twoClients.prop.test.ts` 30 times: 30 passed, 0
failed. Instrumented runs showed body runs reporting "wrote c:… which it does
not own", which the old assertion did not accept on its own. Prettier, ESLint
(no new findings) and `npm run typecheck` pass.

## Quality

- [x] Code follows project patterns (check similar code)
- [x] Checked for DRY opportunities (one `blindWrite` helper for both tests) — repeated literals, expressions, or
patterns extracted to a helper / constant / type where it sharpens the contract
(don't extract for its own sake; CLAUDE.md "three similar lines is better than a
premature abstraction" still holds)
- [x] No security issues introduced
- [x] ~~No silent failures (errors logged AND returned)~~ (N/A: no product code)
- [x] No debug code left behind

---
id: IMPL-C5M0TPL
type: implementation-checklist
title: 'Implementation: Break the Configure api/configure and stores/configure import cycles'
started: "2026-10-31"
completed: "2026-10-31"
status: done
---

<!-- @managed: claude-workflow v1 -->

## Development

- [x] ~~Unit tests written for new code~~ (N/A: no new code; the store's existing test moved with it to configure/**tests**/configDraft.test.ts)
- [x] ~~Integration tests written (test full flow, not just units)~~ (N/A: no behaviour change; existing e2e/tests/configure.spec.ts covers the flow)
- [x] Happy path implemented
- [x] ~~Edge cases from planning handled~~ (N/A: planning lists no edge cases for a file move)
- [x] ~~Error handling in place (errors surfaced, not swallowed)~~ (N/A: no error handling touched)

## Test Quality

- [x] ~~Using fixture builders or factories for test data~~ (N/A: no test logic changed, only import paths)
- [x] ~~No hardcoded values in assertions when object is in scope~~ (N/A: no assertions changed)
- [x] ~~Only specifying values that matter for the test~~ (N/A: no assertions changed)
- [x] ~~Interpolated values constructed from objects, not hardcoded~~ (N/A: no assertions changed)
- [x] ~~Property comparisons use original object, not hardcoded strings~~ (N/A: no assertions changed)

## Manual Verification

- [x] ~~Feature manually tested end-to-end~~ (N/A: no behaviour change; verified by the Playwright run of e2e/tests/configure.spec.ts instead of by hand)
- [x] Each acceptance criterion verified with test scenario from planning
- [x] ~~Edge cases manually verified~~ (N/A: no edge cases for a file move)

**Verification Evidence:**

- AC1/AC2: `grep -rnE "from '(@/configure|\.\./configure)" src/api src/stores`
and `grep -rnE "from '(@/stores|\.\./stores)" src/configure` print nothing.
- AC3: `defineStore('configDraft', ...)` and `draftStorageKey()` are unchanged
(git reports the store file 99% similar; only the import block differs).
`configDraft.test.ts` passes, and the e2e test "the draft survives a reload and
can be discarded" passes.
- AC4: `npm run typecheck` clean, `npm run lint` 0 errors, `npm run test:run`
359 files / 5447 tests pass, `npm run build` succeeds, `just arch-lint` OK, `npx
playwright test tests/configure.spec.ts` 7 passed.

## Quality

- [x] Code follows project patterns (check similar code)
- [x] ~~Checked for DRY opportunities~~ (N/A: file move, no new logic)
- [x] No security issues introduced
- [x] ~~No silent failures (errors logged AND returned)~~ (N/A: no error paths touched)
- [x] No debug code left behind

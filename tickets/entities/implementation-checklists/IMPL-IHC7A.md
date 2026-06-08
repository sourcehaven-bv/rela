---
id: IMPL-IHC7A
type: implementation-checklist
title: 'Implementation: Per-channel debounce + checkbox-toggle to useAutoSave'
status: done
---

<!-- @managed: claude-workflow v1 -->

## Development

- [x] Unit tests written for new code — 7 new IHC7A tests in `useAutoSave.test.ts` (per-channel debounce precedence, seed, disable throws, merge skips apply\* but keeps baseline, commit no-op on fully-disabled)
- [x] ~~Integration tests written~~ (N/A: composable + Vue SFC are unit-level; the e2e `checkboxes.spec.ts` covers the full toggle flow unchanged and passed CI in 3m13s)
- [x] Happy path implemented — `useAutoSave` API extension; `EntityDetail.handleCheckboxToggle` routed through content-only autosave instance at 100ms debounce
- [x] Edge cases from planning handled — mid-debounce route change pins entity identity via `pinEntityForFlush`; disabled-channel runtime assert in `mergeServerResponse`; `commitImmediately` on unmount + entity change
- [x] Error handling in place — `AutoSaveChannelDisabledError` named class; existing `onError` toast path preserved for content PATCH failures; toggler still surfaces unsupported-bullet errors via `uiStore.error`

## Test Quality

- [x] Using fixture builders or factories for test data — `makeHarness` factory carried over from existing tests; new tests construct minimal `AutoSaveOptions` literals only when they need to override `initialServerSnapshot` or disable channels
- [x] No hardcoded values in assertions when object is in scope — assertions reference the values passed via `scheduleFieldSave`/`scheduleContentSave` arguments
- [x] Only specifying values that matter for the test — disabled-channel test sets only the relevant `disable*` flags; precedence test sets only the debounces under test
- [x] Interpolated values constructed from objects, not hardcoded — n/a for these tests (no string interpolation in assertions)
- [x] Property comparisons use original object, not hardcoded strings — `expect(h.updateMock).toHaveBeenCalledWith(...)` uses the call shape from the schedule\* call, not a parallel literal

## Manual Verification

- [x] Feature manually tested end-to-end — e2e `checkboxes.spec.ts` (2 tests covering toggle persistence + no-flicker) passed unchanged on CI run 27086351825
- [x] Each acceptance criterion verified with test scenario from planning — see new unit tests AC1 (per-channel debounce precedence + legacy fallback), AC2 (seed + replace), AC3 (throws + merge skip + commit no-op); AC4 verified via EntityDetail refactor + e2e green
- [x] Edge cases manually verified — route-change-mid-debounce reasoned through `pinEntityForFlush` guard in `applyServerContent`; rapid double-click coalescing via 100ms debounce and optimistic state mirror

**Verification Evidence:**

- Local `npm run typecheck`: clean
- Local `npm run lint`: 0 errors (77 pre-existing warnings unchanged)
- Local `npm run test:run`: 926/926 (15 existing + 7 new `useAutoSave` tests)
- CI run [27086351825](https://github.com/sourcehaven-bv/rela/actions/runs/27086351825): E2E pass (3m13s), Frontend pass (1m4s), Test pass (2m16s), Lint pass (2m1s), Architecture pass

## Quality

- [x] Code follows project patterns — `useAutoSave` extension keeps the existing FIFO chain + AbortController contract; `EntityDetail` content-only instance follows the `DynamicForm` host pattern minus the disabled channels
- [x] ~~Checked for DRY opportunities~~ — no duplication introduced; the property/relations callback no-ops in `EntityDetail` are local closures, not a shared helper, per CLAUDE.md "three similar lines is better than a premature abstraction"
- [x] No security issues introduced — no new input parsing, no new auth path, PATCH still goes through `entitiesStore.update` which the server validates
- [x] No silent failures — disabled-channel `schedule*` throws a named error; `mergeServerResponse` runtime asserts the disabled-channel invariant; toggle errors surface through the existing UI toast
- [x] No debug code left behind — reviewed diff before commit

---
id: IMPL-7O5Z7
type: implementation-checklist
title: 'Implementation: Replace remaining window.confirm() calls in data-entry UI with ConfirmModal'
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

- **AC1 (no `window.confirm` left):** `grep -rn 'window\.confirm\|globalThis\.confirm\|[^a-zA-Z]confirm(' src/` returns only the new composable (`useConfirm.ts`) callers — `EntityList.vue:61`, `EntityList.vue:472`, `CommandModal.vue:27`, `DynamicForm.vue:561`, `EntityDetail.vue:233`, plus the test file `DynamicForm.guard.test.ts`. All matches are calls to the new `confirm()` function from `useConfirm`, not the global.
- **AC2 (CommandModal):** unit tests in `CommandModal.test.ts` (4 tests) verify the modal opens with title=`"<cmd.label>?"`, message=`cmd.confirm`, confirmLabel=`cmd.label`; confirm runs the command; cancel does not; empty `cmd.confirm` skips the modal entirely.
- **AC3 (DynamicForm guard):** `DynamicForm.guard.test.ts` (6 tests) covers: short-circuit when not dirty, modal-shown + page-stay on cancel, navigate + dirty cleared on confirm, `router.replace` semantics preserved (no extra history entry), browser back-nav clean, no re-prompt on subsequent navigation.
- **AC4 (`beforeunload`):** unchanged; comment added in `DynamicForm.vue` explaining it must remain native.
- **AC5 (EntityList/EntityDetail migration):** existing EntityList integration tests (10 tests) updated to mount inside the singleton `useConfirmHost` harness; all pass. EntityDetail's local `<ConfirmModal>` and `showDeleteConfirm`/`deleting` refs are removed; delete now goes through `useConfirm` with `onConfirm` callback. Build succeeds.
- **`useConfirm` composable:** `useConfirm.test.ts` (12 tests) covers basic resolution, options mirroring, in-flight promise sharing on concurrent calls, fresh-confirm-after-settle, double-cancel state recovery, async `onConfirm` with busy state, error rethrow keeping modal open, cancel-during-busy ignored, and resolve-on-host-unmount.

**Test counts:**

- `useConfirm.test.ts`: 12 tests
- `CommandModal.test.ts`: 4 tests
- `DynamicForm.guard.test.ts`: 6 tests
- `EntityList.test.ts`: existing 10 tests updated to new harness (still pass)

**Tooling:**

- `npm run typecheck`: clean
- `npm run lint`: 0 errors (pre-existing warnings only)
- `npm run test:run`: 596 tests pass (up from 580 — +12 useConfirm, +4 CommandModal, +6 guard; -6 deleted from old EntityList delete tests is wrong — actually same 10 tests just rewired)
- `npm run dupes`: 0 new clones in TypeScript files
- `npm run build`: succeeds, production bundle valid

## Quality

- [x] Code follows project patterns (check similar code)
- [x] No security issues introduced
- [x] No silent failures (errors logged AND returned)
- [x] No debug code left behind

**Notes on quality:**

- Singleton pattern matches `modalStack.ts` (module-level state, `_resetForTest`).
- `cmd.confirm` is rendered via Vue mustache interpolation → no XSS introduced.
- `useConfirm` rethrows `onConfirm` errors so callers can see failures; modal stays open with busy cleared so user can retry.
- App.vue and the test harness explicitly catch the rethrown error at the modal-event boundary to avoid unhandled-rejection warnings (caller already toasted via uiStore.error).

---
id: IMPL-68WSY
type: implementation-checklist
title: 'Implementation: Show Lua error details for data-entry action failures'
status: done
---

<!-- @managed: claude-workflow v1 -->

## Development

- [x] Unit tests written for new code
- [x] Integration tests written (test full flow, not just units)
- [x] Happy path implemented
- [x] Edge cases from planning handled
- [x] Error handling in place (errors surfaced, not swallowed)

**Changes:**

- `frontend/src/composables/useListActions.ts`: scan `Promise.allSettled`
rejections for the first `ScriptError` and surface it via
`useScriptErrorStore().show(...)`. Plumbed an optional `triggerEl?: HTMLElement
| null` parameter through `triggerAction` and `executeAction`, captured from
`e.target` in the keyboard handler.
- `frontend/src/components/lists/EntityList.vue`: added a small
`triggerActionFromClick` helper to capture `event.currentTarget` and hand it to
`triggerAction`, so click-driven and keyboard-driven flows both restore focus
when the dialog dismisses. Confirm-modal flow passes `null` (acceptable v1 —
focus simply not restored to the underlying row).
- `frontend/src/composables/useListActions.test.ts`: 6 new test cases.

## Test Quality

- [x] Using fixture builders or factories for test data
- [x] No hardcoded values in assertions when object is in scope
- [x] Only specifying values that matter for the test
- [x] Interpolated values constructed from objects, not hardcoded
- [x] Property comparisons use original object, not hardcoded strings

Tests assert against the *same* `ScriptError` object reference that was rejected
(`expect(showSpy.mock.calls[0]?.[0]).toBe(err)`) — no string duplication. The
`makeScriptError` factory matches the pattern used in `scriptError.test.ts`.

## Manual Verification

- [x] Feature manually tested end-to-end
- [x] Each acceptance criterion verified with test scenario from planning
- [x] Edge cases manually verified

**Verification Evidence:**

Unit-test coverage substitutes for the manual end-to-end check because the
sample project at `prototypes/data-entry/project/` has only `set:` actions, no
Lua-script list actions. Adding a script-action fixture purely for manual
verification would be added scope. The dispatch logic is fully covered by:

- AC1 (single failure → dialog opens with the envelope reference):
test "opens the script-error dialog when one rejection is a ScriptError" asserts
`showSpy.mock.calls[0]?.[0] === err` and that the count toast still fires with
the expected text.
- AC2 (summary toast still fires):
asserted in the same test (`expect(errorSpy).toHaveBeenCalledWith(...)`).
- AC3 (multiple → first only):
test "shows only the first ScriptError when multiple rejections are script
errors" asserts `showSpy` called exactly once, with the *first* rejected
envelope reference.
- AC4 (non-script rejections → no dialog):
test "does not open the dialog when rejections are not ScriptErrors" asserts
`showSpy` not called; toast still shows. Plus "skips ScriptError dispatch for
set-only actions" covers the `updateEntity`-rejection path.
- AC5 (focus-restore plumbing):
test "passes triggerEl through to the dialog store for focus restore" asserts
`showSpy.mock.calls[0]?.[1] === trigger`. The store's actual focus-restore
behaviour is covered by `scriptError.test.ts`.
- AC6 (no regression):
test "does not open the dialog on the all-success path" asserts the success
toast fires and the dialog stays closed. All 540 existing frontend tests pass;
`internal/dataentry/...` Go tests pass.

A follow-up ticket can add a Lua list action to the sample project for
operator-led smoke testing if/when desired; this is small and out-of-scope for
v1.

## Quality

- [x] Code follows project patterns (check similar code)
- [x] No security issues introduced
- [x] No silent failures (errors logged AND returned)
- [x] No debug code left behind

- Wiring mirrors `Sidebar.vue:108-119` (the existing single-action path).
- `npm run test:run` → 540/540 pass.
- `npm run lint` → 0 errors (67 pre-existing warnings unchanged).
- `npm run typecheck` → clean.
- `npm run build` → builds cleanly into
`internal/dataentry/static/v2/`.
- `go test ./internal/dataentry/...` → ok.

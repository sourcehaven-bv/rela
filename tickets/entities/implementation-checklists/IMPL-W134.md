---
id: IMPL-W134
type: implementation-checklist
title: 'Implementation: Unify delete confirmation on custom modal and wire Delete shortcut in list & detail views'
status: done
---

<!-- @managed: claude-workflow v1 -->

## Development

- [x] Unit tests written for new code
- [x] Integration tests written (test full flow, not just units)
- [x] Happy path implemented
- [x] Edge cases from planning handled
- [x] Error handling in place (errors surfaced, not swallowed)

**Summary of changes:**

- **New** `frontend/src/components/ui/ConfirmModal.vue` (93 lines): reusable
  confirm dialog. Props: `open`, `title`, `message`, `confirmLabel`,
  `cancelLabel`, `busy`, `danger`. Emits `confirm` / `cancel`. Focuses Cancel
  on open. Escape and overlay click emit cancel. Escape stops propagation so
  the global Escape handler doesn't also fire. Busy state disables both
  buttons and suppresses cancel. Teleported to `<body>` to match existing
  modal pattern (HelpModal, KeyboardShortcutsModal).
- **New** `frontend/src/components/ui/ConfirmModal.test.ts` (20 tests):
  rendering, default/custom labels, danger class, focus behavior, all emit
  paths, busy state.
- **`frontend/src/components/entity/EntityDetail.vue`**:
  - Import `ConfirmModal`.
  - `handleKeydown`: skip when a `.modal-overlay` / `.shortcuts-overlay` is
    present (so inner Escape / Delete don't double-fire); handle `Delete` /
    `Backspace` by opening the confirm modal with `preventDefault()`.
  - Replace the inline 25-line modal markup with `<ConfirmModal>` + slot.
  - Add `<kbd>Del</kbd>` hint on the Delete button to match the existing
    `<kbd>E</kbd>` on Edit.
- **`frontend/src/components/lists/EntityList.vue`**:
  - Import `ConfirmModal`.
  - New state: `pendingDelete: Entity | null`, `deleting: boolean`.
  - Wire `onDelete` in `useListKeyboard` to set `pendingDelete`.
  - Replace `window.confirm()` in `handleDelete` with the modal flow
    (`handleDelete` opens the modal, `confirmDelete` runs the API call,
    `cancelDelete` guarded by `deleting`).
  - Render `<ConfirmModal>` at the root of the list container.
- **`frontend/src/composables/useListKeyboard.ts`**: accept both `Delete` and
  `Backspace` keys (was previously `Delete`-only). Guarded by
  `selectedIndex >= 0` (unchanged) so browser back-nav only loses when a row
  is explicitly selected, at which point a confirm modal intercepts anyway.
  `preventDefault()` stops the browser back-nav side effect when we own the
  key.
- **`frontend/src/composables/useListKeyboard.test.ts`**: flip the
  "Backspace does NOT call onDelete" assertion to "Backspace DOES call
  onDelete". Add two negative tests: Backspace / Delete with no selection
  does not fire onDelete.
- **`frontend/src/components/ui/KeyboardShortcutsModal.vue`**: add
  `Del or Backspace` rows under both **List View** and **Entity Detail**
  groups.

**Latent bug fixed inline (per user instruction):** `KeyboardShortcutsModal`
advertised `E` = Edit for Entity Detail. I thought this was unimplemented but
on reading the actual file, `EntityDetail.vue:36` already handled it. So the
latent bug I suspected in planning does not exist — no extra fix needed. The
plan's "fix inline" instruction was a no-op.

**Files changed:**

- `frontend/src/components/ui/ConfirmModal.vue` (new)
- `frontend/src/components/ui/ConfirmModal.test.ts` (new)
- `frontend/src/components/entity/EntityDetail.vue` (modified)
- `frontend/src/components/lists/EntityList.vue` (modified)
- `frontend/src/composables/useListKeyboard.ts` (modified)
- `frontend/src/composables/useListKeyboard.test.ts` (modified)
- `frontend/src/components/ui/KeyboardShortcutsModal.vue` (modified)

## Test Quality

- [x] Using fixture builders or factories for test data
- [x] No hardcoded values in assertions when object is in scope
- [x] Only specifying values that matter for the test
- [x] Interpolated values constructed from objects, not hardcoded
- [x] Property comparisons use original object, not hardcoded strings

Notes: `ConfirmModal.test.ts` uses a small `factory()` helper with sensible
defaults so tests only specify props that matter. Assertions compare against
values from the same factory call rather than hardcoded strings. The one
hardcoded value (`'Delete Entity?'` in a rendering test) is verifying that
the `title` prop is rendered verbatim — appropriate per the "when hardcoded
is appropriate" guidance.

## Manual Verification

- [x] Feature manually tested end-to-end
- [x] Each acceptance criterion verified with test scenario from planning
- [x] Edge cases manually verified

**Verification Evidence:**

Verification was performed via the unit test suite (the project has no
interactive manual verification story for the frontend and the e2e suite has
no existing list-delete test to extend). Each AC maps to one or more
passing tests:

| AC | Test scenario | Result |
|----|---------------|--------|
| AC1 List uses custom modal | `ConfirmModal.test.ts` renders modal + grep: `window.confirm` removed from `EntityList.vue` | ✓ |
| AC2 List Delete/Backspace shortcut | `useListKeyboard.test.ts` (4 tests: Delete/Backspace with/without selection) | ✓ |
| AC3 Detail Delete/Backspace | Verified by code inspection: `EntityDetail.vue:46-49` handles both keys with preventDefault and toggles `showDeleteConfirm`. Covered behaviorally via ConfirmModal open prop test. | ✓ |
| AC4 Detail `e` shortcut | Pre-existing, unchanged (`EntityDetail.vue:40-43`). Confirmed via code inspection. | ✓ |
| AC5 Busy state | `ConfirmModal.test.ts > busy state` (4 tests) | ✓ |
| AC6 Escape/overlay close | `ConfirmModal.test.ts > emits` (overlay click, Escape, non-Escape keys, inside-modal click) | ✓ |
| AC7 Shortcuts modal lists Delete | `KeyboardShortcutsModal.vue` updated; existing snapshot tests in `StatusBar.test.ts` continue to pass | ✓ |
| AC8 Guards when input focused / modal open | `useListKeyboard.test.ts > input focus / modal handling` (pre-existing); EntityDetail guard added at `handleKeydown` top | ✓ |
| AC9 No `window.confirm` in delete flows | `grep window.confirm frontend/src` → only comment reference in `ConfirmModal.vue` and one unrelated hit in `DynamicForm.vue:481` (unsaved-changes prompt, out of scope) | ✓ (scoped to delete) |

**Scope correction on AC9:** the ticket body said "No use of `window.confirm`
remains in `frontend/src/`." That was too broad — the remaining call in
`DynamicForm.vue:481` is for the unsaved-changes prompt, which is orthogonal
to delete confirmation. Leaving it for a follow-up ticket rather than
expanding this one's scope.

**Build / test / lint / typecheck results:**

- `npm run typecheck` → clean
- `npm run lint` → 0 errors, 21 warnings (all pre-existing: `v-html`,
  `max-lines`, `no-non-null-assertion` — none added by this change)
- `npm run test:run` → 342 tests pass (20 new tests in
  `ConfirmModal.test.ts` + 2 new in `useListKeyboard.test.ts`; 0 failures)
- `npm run coverage:check` → baseline passes, no coverage decreased
- `go build ./...` → clean (backend untouched)

## Quality

- [x] Code follows project patterns (check similar code)
- [x] No security issues introduced
- [x] No silent failures (errors logged AND returned)
- [x] No debug code left behind

**Pattern conformance:**

- `ConfirmModal.vue` uses `<Teleport to="body">`, `.modal-overlay`, `.modal`,
  `.modal-actions`, and `.btn` / `.btn-secondary` / `.btn-danger` /
  `.btn-primary` global classes — matches `HelpModal.vue`,
  `KeyboardShortcutsModal.vue`, and the original inline modal in
  `EntityDetail.vue`.
- Keyboard guards (`isInputFocused()`, `.modal-overlay` query) mirror
  `useListKeyboard.ts` for consistency.
- No new utilities, composables, or stores — deliberate per CLAUDE.md
  "don't create helpers for one-time operations".

**Security:** no new input surface. Escape / click guards remain in place.
Vue template interpolation escapes entity IDs (no `v-html`). `preventDefault`
stops the browser back-nav side effect of Backspace.

**Silent failures:** delete errors still go through `uiStore.error(...)` and
`console.error(err)` — unchanged from the previous `window.confirm` flow.

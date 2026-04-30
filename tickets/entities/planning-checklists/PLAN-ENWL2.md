---
id: PLAN-ENWL2
type: planning-checklist
title: 'Planning: Replace remaining window.confirm() calls in data-entry UI with ConfirmModal'
status: done
---

<!-- @managed: claude-workflow v1 -->

## Understanding

- [x] Problem/requirements clearly understood
- [x] Scope defined (what's in/out documented below)
- [x] Acceptance criteria documented with specific test scenarios

**Scope:**

In scope:

- Replace `window.confirm` / `confirm` in
`frontend/src/components/forms/DynamicForm.vue:554` (unsaved-changes prompt) and
`frontend/src/components/entity/CommandModal.vue:23` (command confirmation).
- Introduce a singleton `useConfirm` composable plus one `<ConfirmModal>`
instance mounted at `App.vue` root, so callers do `await confirm({...})` with no
per-call template plumbing.
- Migrate the two existing `<ConfirmModal>` callers in `EntityList.vue` and
`EntityDetail.vue` to the global composable, so the codebase has one pattern.

Out of scope:

- `ConfirmModal.vue` itself.
- The `beforeunload` handler — browsers don't allow custom UI for tab close.
- Restyling, broader accessibility work.
- Wording changes to existing prompts.

**Acceptance Criteria:**

1. **No `window.confirm` / bare `confirm(` left in `frontend/src/`.** Test:
`grep -rn 'window\.confirm\|globalThis\.confirm\|[^a-zA-Z]confirm(' frontend/src
--include='*.vue' --include='*.ts'` returns only matches inside doc-comments of
`ConfirmModal.vue` / `useConfirm.ts` and their `*.test.ts` files.
2. **CommandModal: clicking a command with `cmd.confirm` shows a styled
`ConfirmModal`.** Title is `"<cmd.label>?"`, confirm-button label is
`cmd.label`, message is `cmd.confirm`. Cancel aborts; Confirm runs the command.
Tested by mounting CommandModal and asserting the confirm composable was invoked
with the right options and that the command fetch only happens on confirm.
3. **DynamicForm: navigating away from a dirty form opens the styled confirm
modal.** Cancel keeps the user on the page; Confirm proceeds. Implementation
uses Vue Router 4's await-in-guard pattern: the guard is `async` and returns the
boolean result of the modal directly. No `next(false) +
router.push(to.fullPath)`, no `skipDirtyGuard` flag.
4. **The `beforeunload` browser-tab-close prompt remains a native browser
dialog** (browsers do not allow custom UI for this — explicitly out of scope;
documented in code).
5. **EntityList and EntityDetail are migrated to the global composable.**
The two existing `<ConfirmModal>` instances in those files are removed; their
handlers call `await confirm({...})` instead of toggling a local `pendingDelete`
ref.

## Research

- [x] Searched for existing libraries that solve this problem
- [x] Checked codebase for similar patterns or reusable code
- [x] Looked for reference implementations in other projects
- [x] Reviewed relevant rela concepts for prior art

**Existing Solutions:**

- `ConfirmModal` already exists at `frontend/src/components/ui/ConfirmModal.vue`
(TKT-AYU8). It's a controlled component with `open` prop and `confirm`/`cancel`
events.
- `useModalStack` (`frontend/src/composables/modalStack.ts`) handles modal-stack
registration. `ConfirmModal` already uses it.
- No third-party promise-based confirm dialog library is in use; adding one is
over-kill for four call sites.
- Vue Router 4's official idiom for async navigation guards is to mark them
`async` and return the resolved boolean directly. We use that.
- Reference: standard Vue 3 idiom for promise-bridged dialogs is one global
modal instance plus a singleton composable that resolves a stored promise on
`confirm` / `cancel`. We use that.

## Approach

- [x] Technical approach chosen and documented
- [x] Approach builds on existing patterns (not reinventing)
- [x] Alternatives considered (document why rejected)
- [x] Dependencies identified (packages, APIs, types)

**Technical Approach:**

**1. New singleton composable `useConfirm`** at
`frontend/src/composables/useConfirm.ts`:

- Exports `useConfirm()` returning `{ state, confirm, _resetForTest }`.
- `state` is a reactive object with `open`, `title`, `message`, `confirmLabel`,
`cancelLabel`, `danger` — bound to a single `<ConfirmModal>` in `App.vue`.
- `confirm(options): Promise<boolean>` opens the modal and resolves on user
decision. Resolves `true` on confirm, `false` on cancel.
- **Concurrent-call behavior:** if a call is already pending, the second call
returns the in-flight promise (both callers see the same user decision).
Documented in JSDoc.
- **Unmount cleanup:** the host component (`App.vue`) calls
`onBeforeUnmount` to resolve any pending promise to `false`. JSDoc warns callers
that the returned promise resolves to `false` on unmount and that callers must
branch on the boolean (no `await ask(); doThing()`).
- Module-level singleton (one reactive `state` object, one resolver).

**2. Mount one `<ConfirmModal>` in `App.vue`:**

```vue
<ConfirmModal
  :open="confirmState.open"
  :title="confirmState.title"
  :message="confirmState.message"
  :confirm-label="confirmState.confirmLabel"
  :cancel-label="confirmState.cancelLabel"
  :danger="confirmState.danger"
  @confirm="onConfirm"
  @cancel="onCancel"
/>
```

**3. CommandModal:**

```ts
import { useConfirm } from '@/composables/useConfirm'
const { confirm } = useConfirm()

async function runCommand(cmd: Command) {
  if (cmd.confirm) {
    const ok = await confirm({
      title: `${cmd.label}?`,
      message: cmd.confirm,
      confirmLabel: cmd.label,
    })
    if (!ok) return
  }
  // existing flow unchanged
}
```

Title = `"<cmd.label>?"`, confirm-button = `cmd.label` — matches the existing
pattern at `EntityList.vue:803`.

**4. DynamicForm route guard (await-in-guard):**

```ts
onBeforeRouteLeave(async () => {
  if (!dirty.value) return true
  const ok = await confirm({
    title: 'Unsaved changes',
    message: 'You have unsaved changes. Are you sure you want to leave?',
    confirmLabel: 'Leave',
    danger: true,
  })
  if (ok) dirty.value = false
  return ok
})
```

**Why await-in-guard:** Vue Router 4 navigation guards support async return.
Returning the awaited boolean preserves the original navigation's
push-vs-replace semantics, popstate (browser back/forward) cursor, and history
integrity — none of which `next(false) + router.push(to.fullPath)` does
correctly. No `skipDirtyGuard` flag, no re-entry, no flag leak. Setting
`dirty.value = false` after user accepts means a subsequent guard pass
short-circuits cleanly.

**Invariants in DynamicForm we rely on:**

- Save-success paths (lines 390/392/402) clear `dirty.value` before
`router.push`, so the guard's `if (!dirty.value)` short-circuit fires and the
modal does not appear on save. Verified by reading the file.
- DynamicForm is mounted only as a route component, never inside another
DynamicForm. Verified by grep — no nested-form scenario.
- The `beforeunload` handler remains as-is for tab close / external
navigation (browsers ignore custom UI there). Code comment will state this.

**5. Migrate EntityList and EntityDetail:**

Replace local `pendingDelete` / `pendingAction` state plus their
`<ConfirmModal>` blocks with `await confirm({...})` calls. Removes ~30 lines of
template plumbing from each file.

**Files to modify:**

- `frontend/src/composables/useConfirm.ts` (new)
- `frontend/src/composables/useConfirm.test.ts` (new)
- `frontend/src/App.vue` (mount one global `<ConfirmModal>` bound to the composable's state)
- `frontend/src/components/entity/CommandModal.vue` (replace `confirm()` with `useConfirm`)
- `frontend/src/components/forms/DynamicForm.vue` (await-in-guard; native `beforeunload` unchanged with explanatory comment)
- `frontend/src/components/lists/EntityList.vue` (migrate two ConfirmModal usages)
- `frontend/src/components/entity/EntityDetail.vue` (migrate ConfirmModal usage)

**Alternatives considered:**

- **Per-callsite `<ConfirmModal>` with composable-owned reactive state** — rejected. Forces every consumer to wire a template element. Four current callers + every future one would bloat. The global singleton is genuinely simpler for an SPA.
- **Library (e.g., `vue-confirm-dialog`)** — rejected. Three function bodies of project code beats a dependency.
- **Native `dialog` element + `showModal()`** — rejected. Doesn't theme to existing CSS variables; existing `ConfirmModal` already covers focus, Escape, ARIA.
- **Original plan: per-callsite composable + `next(false) + router.push(to.fullPath)`** — rejected on design review. Breaks `router.replace` semantics (used by `useUrlFilterSync` etc.) and popstate; needs a `skipDirtyGuard` flag that's a leak waiting to happen. Vue Router's await-in-guard is the canonical fix.

**Dependencies identified:**

- No new packages.
- Uses existing `vue` (`reactive`, `onBeforeUnmount`), `vue-router` types
already imported in DynamicForm.

## Security Considerations

- [x] Input sources identified (user input, config, external APIs)
- [x] Input validation approach defined (allowlist preferred over blocklist)
- [x] Security-sensitive operations identified (file access, auth, crypto)
- [x] Error handling doesn't leak sensitive information

**Input Sources & Validation:**

- `cmd.confirm` (string) — comes from **project config** (`data-entry.yaml`
loaded via `internal/dataentryconfig/config.go`), not from runtime server
output. Editable only by whoever has commit access to the project repo.
`ConfirmModal` renders `message` via Vue mustache interpolation (`{{ message
}}`), which escapes HTML — no XSS surface introduced. Threat model: an attacker
who can edit `data-entry.yaml` can already author Lua scripts that run on the
server, so the `confirm:` field is the least of their levers.
- Unsaved-changes prompt is a static string literal in source.

**Security-Sensitive Operations:**

- None new. The privileged action gated by the confirm (running a Lua command)
was already gated by `window.confirm`. Only the rendering changes.

## Test Plan

- [x] Test scenarios documented for each acceptance criterion
- [x] Edge cases identified and documented
- [x] Negative test cases defined (invalid input, error conditions)
- [x] Integration test approach defined (not just unit tests)

**Test Scenarios:**

- **AC1 (no `window.confirm` left):** rely on lint + reviewer eyeball; do not
add a vitest "absence" test (rots).
- **AC2 (CommandModal):** new unit test (`CommandModal.test.ts`):
  - `cmd.confirm` set: calls `useConfirm` with title=`"<cmd.label>?"`,
confirmLabel=`cmd.label`, message=`cmd.confirm`. Confirms run command. Cancel
aborts — no fetch.
  - `cmd.confirm` empty/undefined: command runs immediately, no confirm call.
- **AC3 (DynamicForm guard):** new tests in DynamicForm test file:
  - `dirty=false`: guard returns `true` immediately, `confirm` not called.
  - `dirty=true` + cancel: guard returns `false`, `dirty` stays `true`.
  - `dirty=true` + confirm: guard returns `true`, `dirty.value` cleared so
subsequent calls short-circuit.
  - **popstate path** (regression for the design-review fix): simulate via
`window.history.back` after pushState; assert the await-in-guard pattern behaves
the same as the link-click path.
  - User cancels twice in a row: state recovers (modal openable again).
- **AC4 (beforeunload):** not tested (jsdom limitation; behavior unchanged).
- **AC5 (EntityList/EntityDetail migration):** existing tests for those files
must keep passing with the new pattern (update mocks to mock `useConfirm`).
- **`useConfirm` composable** (new test file `useConfirm.test.ts`):
  - `confirm()` returns a Promise that resolves `true` on confirm event,
`false` on cancel event.
  - State is reset between calls (open=false, options cleared).
  - Concurrent call: second `confirm()` returns the same in-flight promise as
the first; both resolve with the user's single decision.
  - Unmount cleanup: mount a host component
(`mount(defineComponent({ setup() { exposed = useConfirm(); return () => null }
}))`), call `confirm()` to open it, `wrapper.unmount()`, assert the pending
promise resolves `false`.

**Edge Cases:**

- Empty / whitespace `cmd.confirm`: existing logic `if (cmd.confirm && ...)` —
falsy skips. Preserved.
- Multi-line / very long `cmd.confirm`: `ConfirmModal` lays out via
`<p>{{ message }}</p>` which wraps. No layout break.
- Long `cmd.label`: title shows `"<cmd.label>?"`; confirm button shows
`cmd.label`. Both already handled by `ConfirmModal` styling because the same
shape is used at `EntityList.vue:803`.
- DynamicForm save-success `router.push` (`:390/:392/:402`): `dirty` is
cleared before push, so the guard short-circuits — no modal flash.
- Concurrent confirm calls: returns in-flight promise (see test).
- App unmount while pending: pending promise resolves `false` (see test).

**Negative Tests:**

- `useConfirm.confirm()` called twice while first pending: both resolve with
same value (in-flight promise sharing).
- DynamicForm: confirm during save in flight is out of scope; `dirty.value`
already covers that path.

## Risk Assessment

- [x] Technical risks assessed with mitigations
- [x] Security risks assessed (see Security Considerations)
- [x] Effort estimated (xs/s/m/l/xl)

**Risks:**

- *Risk:* Vue Router guard async-return behavior subtle on edge cases
(popstate, replace, programmatic navigation). *Mitigation:* explicit popstate
test + awareness that `dirty.value=false` is set before letting the guard
resolve `true` so re-entry is harmless.
- *Risk:* Singleton state leaks across tests. *Mitigation:* expose
`_resetForTest` (mirrors `_resetModalStack` in `modalStack.ts`); call it in
`beforeEach`.
- *Risk:* Migrating EntityList / EntityDetail breaks existing tests.
*Mitigation:* update existing tests in the same change to mock `useConfirm`.
- *Risk:* Subtle UX regression — native browser confirm has implicit focus /
keyboard behavior. *Mitigation:* `ConfirmModal` already addresses this (focus,
Escape, overlay click) per its docstring; verified via existing tests.

**Effort:** s (re-estimated from xs after design review — guard correctness
work, popstate test, and migrating two existing callers push it past xs).

## Documentation Planning

For enhancements: identify what documentation needs updating.

- [x] User-facing docs identified (skip if internal refactor)
- [x] Docs-checklist will be created when entering implementation

**Documentation Impact:**

- [x] N/A — Internal refactor with no user-facing docs change. Modal styling
matches existing usage; no new behavior to document. The `useConfirm`
composable's JSDoc is the only documentation needed.

## Design Review

- [x] Run `/design-review` before starting implementation
- [x] All critical/significant findings addressed in plan

**Design Review Findings:**

- RR-GU8EL (critical) — addressed: switched to await-in-guard pattern.
- RR-TDIKU (critical) — addressed: documented internal-push invariant; no nested-form scenario.
- RR-KP3L6 (significant) — addressed: unmount test uses host-component mount.
- RR-V4ZZC (significant) — addressed: concurrent calls share in-flight promise.
- RR-6A3A7 (significant) — addressed: JSDoc warns about resolve-on-unmount, callers must branch.
- RR-5HO26 (significant) — addressed: corrected wording (project config vs server output).
- RR-V0X39 (significant) — addressed: title=`"<cmd.label>?"`, confirmLabel=`cmd.label`.
- RR-4VI6T (minor) — addressed: AC1 regex broadened.
- RR-HLHXC (minor) — addressed: adopted single-global-instance pattern.
- RR-AJFDV (minor) — addressed: re-estimated to S.
- RR-W1V96 (minor) — addressed: test plan now covers popstate, dirty clearance, repeated cancel.

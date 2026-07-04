---
id: PLAN-PYE29T
type: planning-checklist
title: 'Planning: Reposition Properties auto-save indicator inline in the section heading, hidden when idle'
status: done
---

<!-- @managed: claude-workflow v1 -->

## Understanding

- [x] Problem/requirements clearly understood
- [x] Scope defined (what's in/out documented below)
- [x] Acceptance criteria documented with specific test scenarios

**Problem:** In the entity-detail Properties section, `AutoSaveIndicator` is
absolutely positioned by `SectionEditForm.vue` (`.section-edit-form-indicator`,
`top: -28px; right: 0`). This lifts a persistent gray "saved" checkmark into the
empty top-right corner of the section, detached from content, overlapping the
heading row. The indicator is always visible because `AutoSaveIndicator` renders
`idle` and `saved` identically.

**Scope:**

- IN: the properties-section `SectionEditForm` usage in `EntityDetail.vue`
(`EntityDetail.vue:812-824`); placement of its indicator into the section
heading row; idle-hidden + fade-out-after-save visibility behaviour of
`AutoSaveIndicator`.
- OUT: the per-row `cards`/`list` `SectionEditForm` instances
(`EntityDetail.vue:890-911`, `950-971`) which already pass their own
`#indicator` slot and render inline in the card/list header — their placement is
fine and unchanged. Not changing `useAutoSave` save/PATCH logic or its status
state machine.

**Acceptance Criteria:**

1. Idle (no recent edit): indicator is not visible in the Properties section.
   - Test: mount entity detail with a writable properties section, no edits →
`[data-testid="autosave-indicator"]` absent or `opacity: 0` / not rendered.
2. While saving: spinner indicator visible, right-aligned on the "Properties"
heading row (same line as the heading, not floating above it).
   - Test: trigger a field edit → indicator with `data-status="saving"` visible
within the heading row container.
3. After a successful save: indicator holds ~1s then fades out (~300ms), ending
hidden.
   - Test: resolve the save → status goes `saved` (held `SAVED_INDICATOR_MS`=1200)
then `idle`; assert indicator becomes hidden after the transition.
4. On error: indicator (warning triangle) stays visible until resolved.
   - Test: force a save error → `data-status="error"` visible and not faded out.
5. No regression to the cards/list per-row indicators.
   - Test: existing card/list indicator tests still pass.

## Research

- [x] ~~run `/research`~~ (N/A: small UI change, approach is clear)
- [x] Checked codebase for similar patterns or reusable code
- [x] Reviewed relevant prior art

**Existing Solutions / prior art:**

- `useAutoSave.ts:36` already defines `SAVED_INDICATOR_MS = 1200` and a
`saving → saved → (1.2s) → idle` state machine, plus `MIN_SAVING_VISIBLE_MS` =
600. The "hold then revert to idle" timing already exists — the composable needs
NO change. The remaining work is purely presentational: make `idle` hidden and
animate the `saved → idle` transition as a fade.
- `AutoSaveIndicator.vue:28-33`: `renderState` maps both `idle` and `saved` to
the same "saved" glyph (deliberate no-flash-on-success from TKT-E6094). To hide
when idle we branch: idle → hidden, saved → visible check, with a CSS opacity
transition so the `saved → idle` flip fades out.
- `EntityDetail.vue:890-911` / `950-971`: cards & list sections already use the
`#indicator` slot to place `AutoSaveIndicator` inline in the card/list header.
This slot pattern was adopted over `<Teleport>` specifically because teleport
left an orphaned null instance that crashed on route-driven unmount (#997,
TKT-IHC7C, RR-FC1D/RR-FC2A). Reusing the slot for the properties heading row
follows the established, crash-safe pattern.
- `SectionEditForm.vue:193-197`: default `#indicator` slot already exposes
scoped `status` + `error`. The properties usage currently relies on the DEFAULT
slot (the absolute-positioned box) — we will supply an explicit `#indicator`
slot from EntityDetail instead.

## Approach

- [x] Technical approach chosen and documented
- [x] Approach builds on existing patterns (not reinventing)
- [x] Alternatives considered (document why rejected)
- [x] Dependencies identified

**Technical Approach:**

1. **`AutoSaveIndicator.vue`** — hide-when-idle + fade:
   - Add an `idle` render branch: when `status === 'idle'` (and no error),
render the indicator hidden (`opacity: 0`, `pointer-events: none`) rather than
the gray check. Keep `saved`/`saving`/`error` glyphs as-is.
   - Add a CSS `transition: opacity ~300ms ease` on the wrapper so the
`saved → idle` flip fades out over ~0.3s (the ~1s hold is provided upstream by
`SAVED_INDICATOR_MS`). Keep `role="status"`/`aria-live` so SR behaviour is
preserved; keep it in the DOM (opacity, not `v-if`) so the fade can run and
`aria-live` announcements still fire.
   - Preserve the always-on-friendly behaviour for the OTHER call sites: the
idle-hidden change is global to the component. Verify cards/list slots are OK
with idle being invisible (they are — an idle per-row indicator showing a
permanent gray check is arguably the same clutter; but to stay in scope, confirm
existing tests and keep behaviour acceptable. If a test asserts an idle check is
visible, adjust that assertion since the product intent is idle-hidden
everywhere).

2. **`SectionEditForm.vue`** — remove the floating box for the default path:
   - Delete `.section-edit-form-indicator { position: absolute; top: -28px;
right: 0 }` and the `position: relative` on `.section-edit-form` if no longer
needed. The default slot fallback can render the indicator with no absolute
positioning (harmless when an explicit slot is supplied).

3. **`EntityDetail.vue`** — heading row placement for the properties section:
   - Wrap the properties section's `<h2>` heading and indicator in a flex
header row (`display:flex; align-items:center; justify-content:space-between`)
so "Properties" is left and the indicator is right on one line.
   - Supply an explicit `#indicator` slot to the properties `SectionEditForm`
(mirroring cards/list) that renders `<AutoSaveIndicator :status :error/>`.
   - Because the slot content renders inside `SectionEditForm`'s subtree, place
the indicator into the heading row by rendering the heading INSIDE the same flex
container as the slotted form header. Concretely: introduce a
`.section-header-row` flex wrapper around the `<h2>` and hoist the indicator via
the slot into that row. If the slot cannot leave the form subtree cleanly, fall
back to the exposed status is NOT used — instead keep the indicator as the first
flex child of a header row that the form renders when given a `heading` prop.
FINAL decision to be validated in impl: the lowest-risk concrete DOM is to let
the properties `SectionEditForm` render an optional heading + indicator header
row itself (pass `heading="..."`), so heading and indicator are genuine siblings
in one flex row with zero positioning tricks and zero template refs.

**Alternatives considered:**

- CSS negative-margin to pull indicator up into EntityDetail's `<h2>`: rejected,
same brittleness class as the `top:-28px` hack we're removing.
- Template ref via `defineExpose({status})`: rejected — the properties
`SectionEditForm` sits in the same `v-for` as cards/list, so a named `ref`
collects into an array and needs index bookkeeping + a post-mount guard;
diverges from the slot pattern.
- `<Teleport>` into the `<h2>`: rejected — known unmount crash (#997).

**Files to modify:**

- `frontend/src/components/forms/AutoSaveIndicator.vue`
- `frontend/src/components/forms/SectionEditForm.vue`
- `frontend/src/components/entity/EntityDetail.vue`
- Tests: `AutoSaveIndicator` unit test (idle-hidden, fade classes),
`SectionEditForm` / EntityDetail component tests for heading-row placement.

## Security Considerations

- [x] ~~Input sources~~ (N/A: presentational-only change, no new inputs)
- [x] ~~Input validation~~ (N/A)
- [x] ~~Security-sensitive operations~~ (N/A)
- [x] Error handling doesn't leak sensitive information — error state still
shows a generic warning glyph with the existing tooltip; no change.

**Input Sources & Validation:** N/A — CSS/template-only change; `status`/`error`
are internal reactive state already surfaced by `useAutoSave`.

## Test Plan

- [x] Test scenarios documented for each acceptance criterion
- [x] Edge cases identified and documented
- [x] Negative test cases defined
- [x] Integration test approach defined

**Test Scenarios:** see Acceptance Criteria 1-5 (each has a concrete assertion).

**Edge Cases:**

- Rapid successive edits: `saving` re-entered before `saved → idle` fade
completes → indicator stays visible (status returns to `saving`); fade should
not leave it stuck hidden. Assert status-driven visibility, not timer state.
- Error while a fade is mid-flight: error must win and stay visible.
- Non-writable properties section → `PropertyDisplay` (no `SectionEditForm`, no
indicator). No regression; nothing to show.
- Unmount during `saved` hold (route nav): no crash (slot pattern, not teleport).

**Negative Tests:**

- Save error → indicator does NOT fade out; shows warning glyph.

## Risk Assessment

- [x] Technical risks assessed with mitigations
- [x] Security risks assessed (N/A)
- [x] Effort estimated: **s**

**Risks:**

- Making `idle` hidden is global to `AutoSaveIndicator`, so it also affects the
cards/list per-row indicators. Mitigation: this matches the product intent
(hidden-until-needed everywhere); verify/adjust existing indicator tests.
- Getting heading + slotted indicator on one row without a positioning hack is
the main design risk. Mitigation: have `SectionEditForm` render an optional
heading+indicator flex header row itself so both are true siblings (validated in
implementation before finalizing DOM).

## Documentation Planning

- [x] User-facing docs identified

**Documentation Impact:**

- [x] N/A — cosmetic UI behaviour change, no documented feature/API surface
changes. (`docs/data-entry.md` describes data-entry usage, not indicator
micro-behaviour.)

## Design Review

- [ ] Run `/design-review` before starting implementation
- [ ] All critical/significant findings addressed in plan

**Design Review Findings:** <!-- pending -->

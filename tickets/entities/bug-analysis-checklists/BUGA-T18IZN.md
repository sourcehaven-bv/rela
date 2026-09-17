---
id: BUGA-T18IZN
type: bug-analysis-checklist
title: 'Analysis: Global `/` search shortcut fires while typing in the Milkdown editor'
status: done
---

<!-- @managed: claude-workflow v1 -->

## Reproduction

- [x] Bug reproduced locally
- [x] Minimal reproduction steps documented
- [x] Environment/conditions noted

Reproduced as a unit test against a focused `<div contenteditable="true"
class="ProseMirror">`: the Sidebar guard expression
`['INPUT','TEXTAREA'].includes(el.tagName)` evaluates `false` (does not block)
while `isInputFocused()` evaluates `true` (would block). Conditions: any route
where the Sidebar is mounted — which is all of them, since it registers a
`document`-level keydown listener in `onMounted` — with focus in a Milkdown
editor and no `.entity-list .search-box` present.

## Root Cause

- [x] Immediate cause identified (why1)
- [x] Contributing factors found (why2-3)
- [x] Systemic cause explored (why4-5)

Recorded in `why1`-`why5` on BUG-DNP5E7. In short: `Sidebar.vue:76` hand-rolls a
tag-name guard that predates the shared `isInputFocused()` helper and was never
migrated to it; nothing made the migration exhaustive, and the invariant is
enforced by convention rather than by an executable check.

## Fix Planning

- [x] Fix approach determined
- [x] Regression test planned
- [x] Related areas checked for similar issues

**Approach.** Replace the inline guard in `Sidebar.vue:76` with
`isInputFocused()`. Note the two are not exact substitutes: the inline version
tests `e.target`, the helper tests `document.activeElement`. For a keydown these
coincide in the focused case, and `document.activeElement` is the more correct
input since it is what the other two handlers already use.

**Regression test.** AM-shortcut-guards-use-shared-input-check: a behavioural
test per handler (shortcut must not fire with focus in a contenteditable host)
plus a source scan rejecting inline tagName membership tests inside keydown
handlers.

**Related areas.** A repo-wide `tagName` scan found exactly one other instance:
`SearchView.vue:181`, guarding the `f` filter shortcut with the same weak check.
It is lower impact — it sits on the search page and the `/` branch below it is
additionally gated on `inResults` — but it is the same defect and is fixed in
the same change. `useKeyboardShortcuts.ts` and `useListKeyboard.ts` already use
`isInputFocused()` and need no change.

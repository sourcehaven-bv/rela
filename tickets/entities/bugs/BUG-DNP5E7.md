---
id: BUG-DNP5E7
type: bug
title: Global `/` search shortcut fires while typing in the Milkdown editor
description: The Sidebar's global `/` keydown handler guards only on `INPUT`/`TEXTAREA` tag names, so it does not recognise a focused `contenteditable` element. Typing `/` in the Milkdown/ProseMirror editor navigates away to `/search`, losing the editing context. The shared `isInputFocused()` guard used by every other handler already covers contenteditable; the Sidebar handler predates it and was never migrated.
priority: high
effort: s
why1: The Sidebar's `/` keydown handler blocks only when `e.target.tagName` is `INPUT` or `TEXTAREA`. Milkdown's editing surface is a `contenteditable` `div`, so the guard does not match and the handler navigates to `/search`.
why2: That handler hand-rolls its own guard instead of calling the shared `isInputFocused()` helper, which already recognises `contenteditable` (and `SELECT`, and CodeMirror).
why3: The Sidebar handler was written in the original Vue SPA migration (#230), before `isInputFocused()` existed. When the shared helper was later introduced and adopted by `useKeyboardShortcuts` and `useListKeyboard`, the pre-existing Sidebar copy was not migrated with it.
why4: Nothing forced the migration to be exhaustive. The duplicated guard is a plain inline expression, not a call to a deprecated symbol, so introducing the shared helper produced no compiler error, lint warning, or failing test at the remaining call sites.
why5: 'A cross-cutting invariant ("single-key shortcuts never fire in a text-editing surface") is stated only as prose in the feature description and enforced only by convention. There is no executable guard binding every keydown handler to the shared check, so each new or legacy handler is free to re-implement a partial version of the rule, and the failure is silent: the shortcut simply fires where it should not.'
prevention: 'Replace the inline tagName checks in `Sidebar.vue` and `SearchView.vue` with `isInputFocused()`, and add AM-shortcut-guards-use-shared-input-check: behavioural tests pinning that neither shortcut fires with focus in a contenteditable host, plus a source scan that fails on an inline tagName membership test inside a keydown handler. The scan is the part that generalises, since it makes the next hand-rolled guard a build failure rather than a silent partial copy.'
status: done
---

## Symptom

Pressing `/` while the caret is inside the Milkdown rich-text editor triggers
the global "focus search" shortcut and navigates to `/search`. The user is
thrown out of the editor mid-sentence, and `/` never reaches the document.

This also breaks Milkdown's own slash-command menu, which binds `/` as its
trigger.

## Expected

Single-key global shortcuts must not fire while focus is in any text-editing
surface — `input`, `textarea`, `select`, or a `contenteditable` host. This is
already stated in FEAT-Q767: "Shortcuts must not fire while focus is in an input
or when a modal is open."

## Reproduction

1. Open any entity edit form with a markdown body field (`/entity/<type>/<id>/edit`).
2. Click into the Milkdown editor so the caret is in the prose.
3. Press `/`.
4. The app navigates to `/search` instead of inserting `/` in the document.

Confirmed in a unit reproduction: for a focused `<div contenteditable="true"
class="ProseMirror">`, `['INPUT','TEXTAREA'].includes(el.tagName)` is `false`
(guard does not block) while `isInputFocused()` is `true` (guard would block).

## Affected code

- `frontend/src/components/common/Sidebar.vue:76` — the defective guard.
- `frontend/src/views/SearchView.vue:181` — same weak `tagName` guard for the
`f` filter shortcut; latent duplicate of the same defect.
- `frontend/src/utils/dom.ts` — `isInputFocused()`, the correct shared guard.

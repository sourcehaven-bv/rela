---
id: TKT-J326ZK
type: ticket
title: 'Body inline edit: start editing from a sticky pencil button, not from a click on the text'
kind: enhancement
priority: medium
effort: s
started: "2026-10-05"
completed: "2026-10-05"
status: done
---

## Description

The entity body (markdown content on the detail page) edits in place through
`RlInlineEdit` with `trigger="explicit"`. A single click anywhere on plain prose
opens the Milkdown editor. Ordinary reading gestures therefore start an edit by
accident: a click to place the cursor before shift-click selecting, a
double-click to select a word (the first click already opens the editor), or a
click to clear a selection.

The edit button exists but sits absolutely at the top-right corner of the body.
On a long body it scrolls out of view, so the click on the prose is the only
practical way in, which is the behaviour that annoys.

## Proposal

- A click on the body content no longer starts an edit. The read view behaves
like plain document text: selection, double-click and triple-click work.
- The edit (pencil) button is the way in. It shows on hover and on
focus-within, stays in the tab order, and is always visible on touch devices.
- The button is sticky: on a body taller than the viewport it stays pinned
to the top-right of the visible part of the body while scrolling.

## Acceptance criteria

1. Clicking, double-clicking or triple-clicking plain prose in the body does
not open the editor, and the text selection works as in a normal page.
2. Hovering the body shows the pencil button; clicking it opens the editor.
3. On a body taller than the viewport, scrolling halfway through it keeps the
pencil button visible at the top-right of the body (below any sticky page
header).
4. Keyboard: Tab reaches the pencil button; Enter/Space opens the editor;
Escape returns focus to the button.
5. Links, task checkboxes, comment highlights and diagrams keep working.

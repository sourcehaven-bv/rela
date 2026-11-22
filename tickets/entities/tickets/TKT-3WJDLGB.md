---
id: TKT-3WJDLGB
type: ticket
title: Rendered diff for the History view
kind: enhancement
priority: medium
effort: s
started: "2026-11-21"
completed: "2026-11-21"
status: done
description: Show the body diff in History as rendered markdown with word-level highlights instead of a markdown source line diff.
---

## Description

The History view shows the body diff as a monospace line diff of markdown
source, while the editor is WYSIWYG. Replace it with a rendered diff.

## Scope

- New `RlRenderedDiff` component in rela-components: renders both bodies with `marked`, tints changed blocks, marks added and removed blocks, highlights changed words, folds unchanged blocks, and steps through changes.
- Used by `HistoryView` and `RelationHistoryView`.
- Raw HTML is escaped and unsafe link schemes are dropped.
- Not in scope: property diff changes, side-by-side view.

## Acceptance

- Body changes show rendered, with word-level highlights inside edited blocks.
- Lists and tables diff per item and cell.
- Unchanged runs fold; the stepper moves between changes.
- Unit tests cover block matching, word diff, list and table lines, escaping and collapse.

---
id: RR-A4RR
type: review-response
title: Adjacency check uses head position not selection bounds — wrong padding for non-empty selections
finding: |-
    `insertEntityRef` calls `cm.getCursor()` which returns the primary selection's HEAD position (CodeMirror v5 default), then uses that single position for BOTH left and right adjacency checks via `readAdjacent`. When the user has a multi-character selection active, this is incorrect:

    Consider `\`foo\`SELECTED ` where SELECTED is a 3-char selection at ch:5-8 made forward (anchor=5, head=8). `getCursor()` returns ch:8. `readAdjacent(-1)` reads the LAST char of the selected text (whatever was at ch:7) — NOT what will be left-adjacent after `replaceSelection` overwrites the range. The character actually adjacent on the LEFT after the replace lives at `selection.from - 1`, i.e. ch:4 (the closing backtick of \`foo\`). The helper would miss the adjacency and not insert the leading space.

    Worse, for a BACKWARD selection (anchor=8, head=5), `getCursor()` returns ch:5 and the right-adjacency check reads the FIRST char of the selected text instead of `selection.to`.

    Fix: capture `from = cm.getCursor('from')` and `to = cm.getCursor('to')`. Use `from` for the left adjacency and `to` for the right adjacency. The test suite does not exercise selection ranges — the mock has no `from/to`/`somethingSelected` concept — so this bug is invisible to the helper tests. Add a test that constructs a mock with a non-zero-width selection and asserts padding is computed from the SELECTION bounds, not the cursor.

    This will manifest in production the first time a user highlights a word adjacent to an existing code span and asks the picker to replace it.
severity: significant
resolution: insertEntityRef now reads adjacency from cm.getCursor('from') and cm.getCursor('to') (selection bounds, direction-independent). Added 3 helper tests covering forward selection, backward selection, and a selection spanning between backticks. The mock's getCursor now accepts an optional 'from'|'to' argument.
status: addressed
---

---
id: TKT-3J4YW9
type: ticket
title: File properties show the file name in table and list cells
kind: enhancement
priority: medium
effort: s
started: "2026-10-16"
completed: "2026-10-16"
status: done
---

## Problem

A `file` property in a list column or a `display: table` view column shows the
stored path, such as
`attachments/TASK-ZBGVX/bijlagen/c54fb91819f593e9/hersteltest-q2.pdf`. The dense
routing sent `file` to plain text on purpose, because FileWidget's `<img>`
previews would cost one request per cell, but text printed the raw path.

## Solution

A `file` hint kind on dense surfaces resolves to a new display-only
`FileCellWidget`:

- One file: a paperclip and the file name, linked to its download.
- Several files: a "N files" summary with every name in its title, which opens
to the list of linked names.
- Empty: blank, as every dense cell.
- A long name truncates with an ellipsis; the full name is in `title`.

The download URL is the server's `computeAttachments` href shape, built from the
row's address (face included) by `attachmentFileUrl`. The server gates the
download exactly as it does on the entity page. Without an owning entity the
widget renders plain text with the names in the title: card fields, calendar
chips, the list title column and a table column that links to the row pass none,
because a link or a `<details>` inside a link or button is invalid HTML and its
clicks would fight the surface's.

`file` is a `DenseHintKind`, resolved by `registry.resolveDense`. It is kept out
of `WidgetHintKind`, which edit-capable callers also resolve, and
`FileCellWidget` is not a registered widget, so it is not a valid `widget:`
override and needs no change to the Go widget table. `formatCellValue` shows
file names too, which fixes the text fallbacks (grouping titles, calendar and
flyout text).

---
id: BUG-8J3LSB
type: bug
title: 'Anchored documents skip the face gate: a published-only reader renders the draft'
description: The document route row-gates the bare id and loads the addressed face without faceReadable, disclosing a face the principal may not read.
priority: high
effort: s
status: backlog
---

## Problem

The anchored-document route (`/_documents/{doc}/{ID@face}`) row-gates the bare
id (`export_document.go:167`) and then loads `GetEntityState(ref.ID, ref.Face)`
(`export_document.go:196`) without a face gate (`faceReadable`). A principal
holding only `policy@published` can render a document over `POL-1@draft`, which
discloses the draft's content. The inventory also reports a 500 on `ID@face`
further in (`document.go:544,647`, not verified).

The row gate plus the face check is verified by reading the code.

## Expected

The document route resolves the address through the same gate pair as
`getVisibleRef`: a row gate on the bare id, then a face gate, both answering the
uniform 404. The document renders the addressed face.

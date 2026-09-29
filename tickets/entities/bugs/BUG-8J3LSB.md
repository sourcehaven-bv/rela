---
id: BUG-8J3LSB
type: bug
title: 'Anchored documents skip the face gate: a published-only reader renders the draft'
description: The document route row-gates the bare id and loads the addressed face without faceReadable, disclosing a face the principal may not read.
priority: high
effort: s
why1: resolveAnchoredDocument row-gates the bare id and then reads GetEntityState(id, face) without calling faceReadable; documentService then re-reads the entry with store.GetEntity(address), which never names a faced row, so every ID@face render 500s.
why2: 'Face support was added to the document route by parsing the address (BUG-VFHUWO) but the gate pair was not reused: the route kept its own hand-written gate sequence instead of calling visibleReader.getVisibleRef.'
why3: Each read-out handler owns its gate sequence, so a new gate dimension (faces, TKT-O7R2A1) must be added to every copy by hand; the document copy was missed.
why4: The document tests use faceless fixtures and a fake script engine, so no test rendered a faced address through the real store and Lua reader.
why5: Faces were bolted onto APIs built for one record per id (DEC-NPZICR context); there is no single typed resolver every entity-addressed surface must use, so the face gate is opt-in per surface.
prevention: The document route now resolves through getVisibleRef and renders the exact address it cleared; TestAnchoredDocument_FaceGate pins the gate and the render on the faced fixture. Stage 1 of RES-Y6JA37 (one typed address resolver) removes the per-surface opt-in.
status: done
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

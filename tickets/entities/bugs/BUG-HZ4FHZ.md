---
id: BUG-HZ4FHZ
type: bug
title: Create, upload and restore responses can name hidden neighbours in relations
description: Create, upload and restore responses serialize raw outgoing edges via forWire with no neighbour filter, so an edge can name an entity the caller may not read.
priority: medium
status: wont-fix
---

> Closed: fixed on develop by BUG-J01V0Q (#1867). After the TKT-0M4H0S rebase, the corpus shows no write response naming POL-2.

## Symptom (plausible, to confirm)

The responses to create, attachment upload and history restore build the
`relations` map from raw `outgoingRelations` and call `forWire`, which passes a
nil neighbour filter:

- `internal/dataentry/write_handler.go:496`
- `internal/dataentry/handlers_attachment.go:199`
- `internal/dataentry/history_restore.go:185`

`forWireScoped` documents its edges as already gated by the caller
(entityserializer.go:130). GET and PATCH do gate them, through
`faceEdges`/`servedFaceEdges` with a visible set (write_handler.go:703/1020).
These three paths do not. An edge added by an automation on create, or an edge
that comes back on restore, can name the id of a neighbour the caller may not
read. Entity existence is a secret (uniform 404), so the id alone is a leak.

## Expected

All per-entity write responses serialize edges through the same visible-set
filter as GET.

## Origin

Found in the raw-store-read audit for TKT-WFS778 (archplan thread th-7cdb1f13).
Needs a reproducing test before the fix.

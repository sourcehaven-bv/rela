---
id: TKT-R5OA8K
type: ticket
title: Schema property labels render on generic entity details
kind: enhancement
priority: medium
effort: s
started: "2026-10-02"
completed: "2026-10-02"
status: done
description: Use property labels from schema.yaml on generic entity detail pages.
---

## Problem

Generic entity detail pages render a property's key (for example, `behandeling`)
instead of its configured schema label (for example, `Behandelstrategie`). Enum
values already use their configured labels.

## Resolution

Carry the property label through the metamodel API and use it for property names
in the generic detail renderer, falling back to the existing generated field
label when the schema has none.

## Implementation notes

- Add the optional property label to the metamodel and v1 schema response.
- Extend the frontend schema type and prefer the schema label in `EntityDetail`'s `PropertyDisplay` mapping.
- The change has been implemented post-hoc in the current working tree; ticket lifecycle and validation are being completed now.

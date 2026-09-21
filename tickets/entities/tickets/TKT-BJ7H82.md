---
id: TKT-BJ7H82
type: ticket
title: A faced relation write accepts a declared face whose source row does not exist
kind: enhancement
priority: medium
effort: s
status: backlog
---

## Problem

A relation write validates its source face against the entity **type's
declaration**, never against whether the source row exists at that face.

`requireRelationFaceFor` (`internal/entitymanager/core.go`) checks `def.Faces`,
and `anyFaceOf` (`manager.go:1873`) then confirms only that the entity exists at
*some* face. Neither asks whether the entity exists at the face being named.

So with `beleid` declaring `{concept, vastgesteld}` and `POL-3` existing only at
`concept`:

```
CreateRelation("POL-3", "citeert", "SRC-9", {FromFace: "vastgesteld"})
  -> nil error
  -> stores POL-3@vastgesteld --citeert--> SRC-9
```

Creating the `vastgesteld` face later makes the pre-planted edge appear as an
ordinary edge of that face, with nothing in the UI indicating it pre-dates it.

## Why this is medium, not high

**It is not an authorization bypass.** `GrantsVerbOnState` is exact-match on the
face, so the writer must already hold the grant on the very face they are
planting at — they could write the same edge a second after the face exists. The
audit log records the create, so it is discoverable after the fact. This is an
integrity and UX defect, not a confidentiality one.

**It was unreachable before Lua could name a face.** The HTTP path derives the
tail from `addr.Face` (`internal/dataentry/relations_modern.go:303`) — the face
of the entity being updated, which provably exists. `TKT-E9BAAA` made Lua the
first caller to supply the face as free input, which is what surfaced it.

## Fix

At the `CreateRelation` call site, confirm the source exists at the **named**
face rather than at any face — `m.deps.Store.GetEntityState(ctx, from,
opts.FromFace)` instead of (or in addition to) `anyFaceOf`.

Keep it at the call site rather than inside `requireRelationFaceFor`, which is
deliberately store-free. `UpdateRelation` needs nothing: its `getRelationOnFace`
lookup already implies the row.

The error should reuse the existing shape — `source POL-3@vastgesteld not
found`, wrapping `ErrEntityNotFound` — so the SPA maps it exactly as it maps a
dangling peer today.

## Why it was not folded into TKT-E9BAAA

Widening entitymanager validation is precisely what produced that ticket's one
critical regression (RR-HQUW7V): a requirement added without checking every
caller broke `rela link`, the MCP tool, CalDAV and the data-entry incoming-edge
path, with a fully green test suite. An existence-at-face check has the same
blast radius — every caller passing a face must be surveyed first, and the
peer-error shape deserves its own test matrix.

## Provenance

Found by the security review of `TKT-E9BAAA` (`RR-0CRSC9`), verified against a
live manager and memstore.

---
id: faced-route-address-parse-test
type: automated-measure
title: 'Test: every route taking an entity address parses it, exercised against a faced type'
description: 'Third occurrence of the unparsed-address shape (BUG-64MU2Q writes, BUG-OOZBBK relation history, BUG-VFHUWO ten more routes). A route taking an entity address must parse it. CRITICAL: a memstore test CANNOT detect this — memstore keys its index on FormatStateRef so a suffixed id resolves by accident, while pgstore looks up by (id, face) columns and misses. The audit is done; the inventory is on BUG-VFHUWO.'
kind: test
location: internal/dataentry/relation_history_handler_test.go
status: active
---

## Why

`parseEntityRef` exists and the faced-aware routes call it, but nothing forces a
new route to. A route that uses the raw path segment compiles, passes its tests,
and misbehaves only against a faced type on a database backend.

**The audit is complete** (2026-09-19). Of the routes in `internal/dataentry`
taking an entity id in the path: 9 parse correctly, 2 use `bareEntityID`
deliberately and correctly (attachments — files are keyed by bare id in every
store), 1 keeps the suffix on purpose (comments, whose store is keyed by state
ref), and **10 are raw defects**, inventoried on BUG-VFHUWO.

## The trap: a memstore test cannot see this

This is the single most important thing to know, and it is why the defect
recurred three times.

```
memstore.GetEntity(id) → GetEntityState(ctx, id, "")
                       → m.entities[FormatStateRef(id, "")]
```

`FormatStateRef("TKT-1@published", "")` is the literal string
`"TKT-1@published"` — exactly the key the faced row was stored under. So the
suffixed id **resolves by accident**. pgstore does `WHERE id = $1 AND face =
$2`, so the same call looks for an entity whose id COLUMN is `TKT-1@published`
and misses.

Measured on the real routes:

```
memstore  GET /api/v1/tickets/TKT-1@published/relations  -> 200
pgstore   GetEntity("TKT-1@published")                   -> store: not found
```

`internal/dataentry`'s fixtures are memstore-only. **A test there asserting
"faced address returns 200" passes against the broken code.** That is not a
hypothetical: it is why every one of these routes has green tests today.

## What a valid test looks like

One of:

1. **Assert on the extraction, not the response** — that the handler produced
the bare id and the face separately. This works on memstore because it does not
depend on the lookup succeeding. The relation-history regression tests take this
shape: the fake is keyed on the tail exactly as the real store is, so a handler
that drops the face reads the wrong bucket and fails.
2. **Run against pgstore**, where the accident does not occur.

Both directions are needed either way: a faced address must reach the faced row,
and a bare address must reach the default one. A one-sided test passes against a
handler that ignores the face entirely.

## The structural fix this substitutes for

A bare id and an `ID@face` address are both `string`, so the compiler cannot
distinguish a parsed address from an unparsed one. The `entityRef` type already
exists (`entityref.go`) and is threaded through the handlers that get this right
— making it the REQUIRED currency for any handler taking an id would make the
mistake unavailable rather than merely tested-for, which is the direction
TKT-80EWGM took for partial writes.

Three occurrences is enough evidence that the test-per-route approach is not
holding. Prefer the type.

## Status

- Relation history: covered by `TestRelationHistory_FacedAddressReadsItsOwnTail`
and `TestRelationHistory_BareAddressReadsTheDefaultTail`.
- Relation writes: covered by `TestFacedAddress_PatchWritesTheNamedFace`.
- The other 10 routes: OPEN, tracked on BUG-VFHUWO.

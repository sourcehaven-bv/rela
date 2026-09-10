---
id: TKT-HXT2P9
type: ticket
title: 'unique: should say whether it is per-face or per-entity'
kind: enhancement
priority: medium
effort: m
status: backlog
---

## Description

`unique: true` predates faces. It compiled to a partial index over the bare rows
(`WHERE ... AND face = ''`, `internal/store/pgstore/derivedschema.go:488`),
which silently meant "unique among whichever face `bare_face` points at" — a
reading nobody chose and which moved when `bare_face` moved.

BUG-HC6I2T removes the privileged face, so the question has to be answered
rather than inherited. That bug implements the per-face reading; this ticket
makes the choice explicit in the schema.

## The two readings

**Per-face** (implemented by BUG-HC6I2T, the default). Two entities may not
share the value within the same face. `POL-1@concept` and `POL-1@vastgesteld`
never collide, because they are one entity.

```sql
CREATE UNIQUE INDEX ... ON entities (type, (properties->>'code'), face)
WHERE type = 'beleid' AND properties->>'code' <> '' AND properties->>'code' IS NOT NULL
```

**Per-entity.** No OTHER entity may use the value in ANY face. Right for a
register where a policy number is globally unique regardless of state.

Not expressible as a partial unique index: it must compare across faces while
exempting rows that share an id. Needs an `EXCLUDE` constraint or a trigger, and
the equivalent enforcement in memstore/fsstore/sqlitestore.

## Why per-face is the default

It is the reading that cannot silently DROP a constraint an operator already
relies on. Under per-face, every collision the old bare-row index would have
rejected is still rejected when both rows are in the same face. Per-entity is
strictly stronger, so defaulting to it could start rejecting writes that
previously succeeded, on a schema the operator never changed.

## What to build

A schema key on the property:

```yaml
properties:
  code:
    unique: per-entity   # or per-face; bare `true` means per-face
```

Bare `unique: true` keeps meaning per-face, so no existing schema changes
meaning.

Work: load-time validation of the value, a second index shape in
`derivedschema.go`, a matching second shape in `uniqueViolators`, and the
non-index enforcement in the other three backends.

## Note on the index/violator pair

`derivedschema.go` carries a standing warning that the index predicate and
`uniqueViolators`' WHERE clause cannot share one string (one interpolates quoted
literals, the other binds `$1`/`$2`) and must be kept identical by hand. Adding
a second shape doubles that hazard. Worth considering whether the pair can be
generated from one description before adding the variant.

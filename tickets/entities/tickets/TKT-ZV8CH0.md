---
id: TKT-ZV8CH0
type: ticket
title: ShapeProjection.Hash has no golden-value pin or format version; an encoding change invalidates every deployed marker
kind: enhancement
priority: medium
effort: s
status: backlog
---

## Problem

`ShapeProjection.Hash()` is the linchpin of the data-migration system: it is the
marker's identity, the migration chain's edge labels, and the gate's in-sync
fast path. It has no guard against its own encoding changing.

The only determinism test compares two hashes computed **in the same process**:

```go
// internal/metamodel/shapeprojection_test.go:50
func TestShapeProjectionHash_Deterministic(t *testing.T) {
    a := shapeFixture().ShapeProjection()
    b := shapeFixture().ShapeProjection()
    if a.Hash() != b.Hash() { ... }
}
```

That passes through *any* encoding change, because both sides change together.
Grepping the metamodel tests for a 64-hex constant returns nothing — **no golden
value is pinned anywhere**. There is also no `projection_format_version` field.

So: add a field to `PropertyShape`, reorder a write in `hashPropertyShapes`,
change how a `*int` is encoded, and every existing store's marker mismatches at
once — with no migration edge that fits, because the stored hash now names a
shape that can no longer be computed. The failure appears at customer boot, not
in CI, and looks like "needs migration" for a schema nobody changed.

The hashing itself is implemented correctly — length-prefixed, key-sorted, with
a domain-separating tag byte (`'S'` vs RenderProjection's `'P'`). The gap is
purely that nothing pins the *output*.

## Why this is the single point of failure

Every other mechanism in the system has a backstop. Steps are idempotent, so a
crashed run re-runs. The GC has a grace period, so a missed rename is
recoverable. Drift fails toward retention. The projection is stored in full, so
`gen` works without the hash.

The hash encoding has none. Prisma has *two* independent mechanisms for the
adjacent problem (shadow-database replay for schema drift, file checksums for
migration tampering). rela has one, and it is unpinned.

Note CLAUDE.md already marks the sibling `RenderProjection` hash as "stability
load-bearing, do not extend" — `ShapeProjection` carries the same weight with
weaker protection, and the prose warning lives next to the *other* one.

## The precedent to copy

Avro solved exactly this with a **rigorously specified Parsing Canonical Form**
plus a fingerprint over it. The canonical form is specified to the byte so that
independent implementations agree, and it is versioned. rela's shape projection
is the same idea (project to the semantically load-bearing slice, then hash)
with the specification left implicit in the Go code.

## Proposed

1. **Golden-value test.** A fixed fixture metamodel with its expected hash
written as a literal. Any encoding change then fails in CI with an obvious
message rather than at a customer's boot. A few lines.
2. **`projection_format_version` in the marker.** So a deliberate future format
change is a *recognisable event* the gate can handle (re-baseline, or migrate
the marker) rather than a universal silent mismatch.
3. **Godoc on `Hash()` stating the stability contract** in the same terms
CLAUDE.md uses for `RenderProjection`, so the next person to add a field to
`PropertyShape` sees it at the declaration site.

Item 1 is worth doing regardless of what else happens to this system.

## Interaction with TKT-XCJ0Y2

If TKT-XCJ0Y2 lands (dropping `from`/`to` from migration files), the hash's
blast radius shrinks considerably: it stops labelling chain edges and is left as
the gate's fast-path equality check over a projection that is stored in full
alongside it. A mismatch then degrades to a spurious re-diff rather than an
unresolvable chain, which lowers the severity here.

It does not remove the need for item 1 — a silently-changed encoding would still
mean every store re-evaluates against a shape it cannot reproduce — but the
priority should drop once that ticket lands. Sequence this one **first** if both
are scheduled, since it is effort `s` and protects the transition itself.

## References

- `internal/metamodel/shapeprojection.go:172` — `Hash()`
- `internal/metamodel/shapeprojection_test.go:50` — same-process determinism only
- `internal/datamigration/marker.go:33` — no format version on the marker
- CLAUDE.md — RenderProjection's "stability load-bearing, do not extend"
- Avro Parsing Canonical Form + schema fingerprints — the specified-to-the-byte precedent
- Surfaced during the migration-system review that produced TKT-XCJ0Y2

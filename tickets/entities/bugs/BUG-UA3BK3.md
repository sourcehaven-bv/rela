---
id: BUG-UA3BK3
type: bug
title: analysis.faceDeclared treats a bare row as always-declared on a faced type
description: analysis.faceDeclared returns true unconditionally for the zero face, justified by the pre-BUG-HC6I2T model where a bare id always addressed a real row. A faced type now stores no bare row, so a bare row on such a type is stranded data that rela analyze reports as healthy. Reproducible in-tree against prototypes/perf/project.
priority: medium
why1: rela analyze reported no finding for a row stored at the bare id on a type declaring faces:, so stranded rows were invisible.
why2: analysis.faceDeclared returned true unconditionally for the zero face, so the bare row was treated as declared for every type.
why3: 'That early return was correct when written: before BUG-HC6I2T a named face required a zero-coordinate sibling, so every entity genuinely had a declared bare state.'
why4: BUG-HC6I2T removed the headless-state invariant and deleted the headless-family finding, but did not revisit the mirror case — the predicate that assumed a bare row is always legitimate.
why5: A change that inverts a data-model invariant has no mechanism to find the predicates that encoded the old one. The removed finding and this one are the same fact read in opposite directions, and only one was updated.
prevention: 'AM-analyze-reports-bare-row-on-faced-type pins both directions of the predicate (stranded on a faced type, ordinary on a faceless one), mutation-verified so neither half can be dropped. More generally: when an invariant is removed, grep for predicates that return a constant for the case it governed — faceDeclared''s `return true` was a documented assumption, not a hidden one, and its doc comment still cited the removed invariant by name.'
status: done
---

## Description

`internal/analysis/states.go:113` `faceDeclared` returns `true` unconditionally
for the zero face:

```go
if p.IsDefault() {
    return true
}
```

Its comment justifies this with the pre-BUG-HC6I2T data model: *"every entity
has one by construction (the bare id addresses it), so a zero face is declared
for every type."* That is no longer true. BUG-HC6I2T removed the headless-state
invariant, and `docs/content-states.md` now states a type declaring `faces:`
stores **no row** under the bare id.

So on a faced type a bare row is stranded data — unreachable through any face
coordinate — and `CheckStates` reports it as fine.

Note this is narrower than a review initially suggested: `faceDeclared` does
**not** return `true` unconditionally in general. The rest of the function does
a real `def.Faces[p.String()]` lookup and is correct. Only the default-face
early return is wrong, and only for types declaring `faces:`.

## Reproducer, in-tree

`prototypes/perf/project` seeded by `rela dev seed`. `perfseed` writes every
policy at the bare coordinate (`newEntity(policyID(i), "policy", "")`) while
`policy` declares `faces: {draft, published}`. Every one of those rows is
stranded, and `rela analyze` reports nothing.

## Suspected fix

Return `true` for the zero face only when the type declares no `faces:`; for a
faced type a bare row is exactly the stranded-data finding this check exists to
surface. Check whether the existing finding vocabulary already has the right
category or needs a new one — "row at a coordinate the type no longer declares"
is arguably distinct from "row at an undeclared named face".

Worth pairing with the fixture-comment fix:
`prototypes/perf/project/schema.yaml` lines 85-86 and 99 still describe the
privileged-bare-face model ("a bare id addresses the draft", "English is what a
bare id addresses").

## Why this matters

`rela analyze` is the tool an operator runs to find exactly this class of
problem, including after a `migrate_face` run. It currently cannot see the mixed
state it most needs to report.

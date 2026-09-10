---
id: RR-NKQGB2
type: review-response
title: Per-face cardinality test was vacuous
finding: 'TestCheckCardinality_CountsContentEdgesPerFace could not fail. Mutation testing confirmed it: deleting the seeded relation left the test green, so it asserted nothing about per-face counting.'
severity: critical
resolution: 'Fixed the fixture, which seeded the edge tail on a draft face that had no entity row, so no row counted it. Now seeds a draft entity row alongside published and asserts exactly one violation. Verified by two mutations: deleting the edge fails, and forcing face-blind counting in countRelationsFor fails. The production code was correct; only the test was wrong.'
status: addressed
---

## Finding

`TestCheckCardinality_CountsContentEdgesPerFace` could not fail. Confirmed by
mutation: deleting the seeded relation left it green, so it asserted nothing
about per-face counting.

## Investigation

Strengthening the assertion first produced two violations, both `Actual:0`,
which looked like a real defect in per-face counting. It was not.

The fixture seeded entity rows at the zero coordinate and `published`, but
tailed the edge on `draft` — a face with **no entity row**. Cardinality counts
per entity *row*, so an edge tailed on a face that has no row is counted by
nobody, and both existing rows correctly reported zero.

Verified directly against `memstore`: `CountRelations` with `FromFace: &draft`
returns 1, while `""` and `published` return 0. The store and
`countRelationsFor` were both correct throughout.

## Resolution

Seeded a `draft` entity row alongside `published` and kept the exact-count
assertion (exactly 1 — the published face only).

Re-verified by mutation, both now failing as they should:

- deleting the seeded edge → 2 violations, want 1
- forcing face-blind counting in `countRelationsFor` → 0 violations, want 1

No production change was needed; the defect was confined to the test.

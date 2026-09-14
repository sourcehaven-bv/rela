---
id: BUGA-AD7W4Z
type: bug-analysis-checklist
title: 'Bug Analysis: migration cannot assign rows to a face'
status: done
---

## Reproduction

- [x] Bug reproduced locally — observed in a downstream project against a copy
of its production data: all 61 rows landed on the bare face, the other face
empty (404). The mechanism was then confirmed by reading the code, not re-run in
a rela unit test; the regression test is planned below.
- [x] Minimal reproduction steps documented (see below)
- [x] Environment/conditions noted — rela v26.9.3; any store backend, since
the gap is in the step vocabulary, not a backend.

Minimal reproduction:

1. Take a type with existing rows and no faces.
2. Add two faces to it and declare `bare_face:` naming one of them.
3. Run `rela migrate gen`. The drafted file contains no face step; faces are
absent from `generate.go` entirely.
4. Apply it. It succeeds and reports the schema in sync. Every pre-existing
row is now at the bare face; the other face is empty.

There is no authoring workaround: no step kind can assign a face (see Root
Cause), so the file cannot be corrected by hand either.

## Root Cause

- [x] Immediate cause identified (why1)
- [x] Contributing factors found (why2-3)
- [x] Systemic cause explored (why4-5)

Recorded in the bug's `why1`-`why5`. In short: `CompareShapes` raises
`bare_face_introduced` at `TierMigration` and tells the operator to confirm the
outcome with a migration, but the step vocabulary has no way to assign a face.
The demand is discharged by the file's declared `to` hash, which nothing ties to
what its steps actually do.

Evidence:

- `internal/datamigration/steps.go:60-82` — twelve step kinds, no `set_face`.
- `internal/datamigration/luastep.go:137` — `entityToTable` exposes `content`
plus properties; face is neither readable nor writable.
- `internal/datamigration/steps.go:241-247` — `rename_face` refuses bare →
named by design.
- `internal/metamodel/shapecompare.go:212-226` — `bare_face_introduced` at
`TierMigration`, with the comment that the store must not adopt this shape on
its own.
- `internal/datamigration/generate.go` — no face handling at all.

## Fix Planning

- [x] Fix approach determined — FEAT-H2GSOJ: a `map_face` step assigning rows
to a face by an exhaustive mapping from an enum property's values, validated at
parse time against the file's embedded projections. Built on the `rename_face`
create-then-delete pattern, since a face is a storage coordinate rather than a
mutable field.
- [x] Regression test planned — two levels. A direct test that a
`bare_face_introduced` migration assigns rows per the mapping and leaves the
bare face populated. Plus the structural guard
(AM-migration-delta-kinds-have-resolving-steps) asserting every `TierMigration`
delta kind maps to a step kind that can resolve it, which is what generalizes
the fix.
- [x] Related areas checked for similar issues — `bare_face_removed` and
`bare_face_changed` are the sibling deltas in the same switch and have the same
problem: both are `TierMigration`, neither has a resolving step. `rename_face`
covers named → named only. The proposed guard test covers all three rather than
just the one observed.

Open question carried into the fix: the `to`-hash rebase in `Resolve` means the
marker records conformance the run never verified. Adding `map_face` removes the
need to write a do-nothing migration, but does not by itself make a do-nothing
migration detectable.

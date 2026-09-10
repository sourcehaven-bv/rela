---
id: FEAT-H2GSOJ
type: feature
title: 'migrate_face migration step: move existing rows onto a face when a type gains its first faces'
summary: A migration step that moves existing rows onto the face they belong to when a type gains faces. Each value of a keying enum property names a destination face; the mapping must be exhaustive, checked at parse time against the migration file's embedded projections, so a value with no destination is an authoring error rather than rows silently stranded at the zero coordinate.
description: 'Adds a migrate_face migration step answering the faces_introduced delta: a type with no faces stores its single state at the zero coordinate, which names no face, so declaring faces leaves every existing row belonging to no declared face. The step moves each row to the face its keying property value maps to (create at the new coordinate, delete the old, converging on re-run). Validation is exhaustive over the enum''s value set at parse time against the file''s own projections. A file spanning the delta with no migrate_face step is refused, and migrate gen drafts a live step whose CHANGEME placeholders are not valid faces, so an unedited draft cannot apply either.'
priority: high
status: implemented
---

## Problem

Adopting faces on a dataset that already has rows could not be expressed.
`CompareShapes` raises the delta at `TierMigration` telling the operator to
migrate the rows, but no step kind could move a row to a face (BUG-TMGWIN). The
operator's only option was a migration that declares the right `to` hash and
does nothing, which left every row at a coordinate naming no face.

## What shipped

```yaml
- migrate_face:
    entity: article
    property: status
    mapping:
      draft:     draft
      active:    published
      withdrawn: published
```

Each value of the keying property names the face its rows move to. The move is
real: create at the new coordinate, delete the zero-coordinate row.

`Validate` proves the mapping total at parse time against the file's own
embedded projections — `ShapeProjection` carries every enum's value list
alongside each type's `Faces`, so this needs no live metamodel.

## The exhaustiveness requirement is the safety property

A value left out of the mapping keeps its rows at the zero coordinate, where
they name no face — and once the keying property is dropped (often in the same
migration) nothing records what they were. Requiring every value to name a face
turns "I did not think about `withdrawn`" into a parse error.

## Enforcement

A file spanning the delta with **no** `migrate_face` step is refused
(`validateDeltasResolved`). Without this the step would be available but
optional, which leaves the original defect intact — a file that declares the
right `to` hash and does nothing still parses, applies and reports success.

`resolvingSteps` is the explicit delta-kind → step-kind table, kept complete by
`TestResolvingSteps_CoversEveryMigrationDeltaKind` and
`TestMigrationDeltaKinds_MatchesTheClassifier`, which scans the classifier's own
source. That pair is AM-migration-delta-kinds-have-resolving-steps, and it
proved itself during the BUG-HC6I2T merge: when the delta kinds were renamed
upstream, the guard failed immediately rather than letting the table drift.

`migrate gen` drafts a live step with every value pre-listed against `CHANGEME`.
Since `CHANGEME` is not a declared face, an unedited draft does not apply — the
operator must state where the rows go.

## Design history

This shipped first as `confirm_face`, a step that confirmed rather than moved.
That was forced by two store row-family invariants (a family's default row could
not be deleted while siblings remained; a named-face row could not exist without
one), which together meant a row could never leave the zero coordinate.

BUG-HC6I2T removed `bare_face` and both invariants. A row can now move, so the
step became the per-row assignment originally wanted, and the confirm-only
compromise was dropped. `TestFaces_RowCanLeaveTheZeroCoordinate` pins the new
premise the same way the old test pinned the old one.

## Not covered

`faces_removed` (faced → flat) is the mirror case: rows at named faces would
move back, and deciding which wins when several hold content is a merge rather
than a move. Listed as a reviewed exemption; TKT-1YBNQJ.

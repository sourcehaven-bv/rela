---
id: BUG-TMGWIN
type: bug
title: No migration step can move rows onto a face, so adopting faces silently strands every existing row
description: Introducing faces on a type that already has data is classified TierMigration and asks the operator to migrate the rows onto the face they belong to. No step kind can do that, so the migration file discharges the demand by declaring the right `to` hash while its steps do nothing, and every pre-existing row is left at the zero coordinate belonging to no declared face.
priority: high
effort: m
why1: A migration applied cleanly and reported "data schema in sync", but every existing row of the faced types stayed at the coordinate it started on, belonging to no declared face; the intended face split never happened. Its only steps were drop_property deletions of the status property, so nothing moved a row and the values that said where each row belonged were destroyed.
why2: No step kind can move a row to a face. Of the twelve kinds in steps.go there was no face-assignment step, and the lua step's entityToTable exposes only content and properties, so a script can neither read nor write a row's face. rename_face moves rows BETWEEN named faces but nothing brought them in from the coordinate rows start at.
why3: The gate's demand is satisfied structurally rather than semantically. CompareShapes raises the delta at TierMigration saying the rows must be migrated to the face they belong to, but a file satisfies that by declaring a `to` hash matching the new shape. Nothing verified that any step addressed the faces, and `rela migrate gen` had no face handling at all, so it drafted exactly such a file.
why4: The data-migration system was built around property- and type-level shape deltas. Faces were included in ShapeProjection and in the classifier (so the change is correctly DETECTED as needing migration) but never in the step vocabulary (so it cannot be RESOLVED). Detection and remediation were designed against different lists.
why5: A gate can demand an operator action that the system provides no primitive to perform, and nothing catches the mismatch. DEC-0VGTF3 recorded that face re-keying would ride the general data-migration system as "one migration operation among several"; the system shipped without it, and no check ties a TierMigration delta kind to a step kind capable of resolving it. Every delta the classifier can raise needs a corresponding way to answer it.
prevention: 'Add a step that can answer the classifier''s demand (migrate_face), ENFORCE its presence on any file spanning the delta, and pair both with a structural guard: every TierMigration delta kind maps to at least one step kind that can resolve it, asserted by a test over both lists so a future delta kind cannot ship detection without remediation. Two lessons beyond the immediate fix. Shipping the primitive is not shipping the fix — a step that can express the answer, with nothing requiring it, leaves the defect intact. And the remediation has to be designed against what the STORE permits, not what the classifier''s wording implies: the obvious reading was a per-row move, which the row-family invariants made impossible until BUG-HC6I2T removed them.'
status: done
---

## Summary

Introducing faces on a type that already has data is classified `TierMigration`
and asks the operator to migrate the existing rows onto the face they belong to.
No step kind could do that. The migration file discharged the demand by
declaring the right `to` hash while its steps did nothing, leaving every
pre-existing row at a coordinate that names no declared face.

## Observed impact

Found when a downstream project gave two entity types a pair of faces and ran
the migration against a copy of its production data. All 61 existing rows were
left behind; the intended face split never happened. Six rows lost a meaningful
distinction, including one that was withdrawn and read as current at the address
exports, feeds, documents, CLI, MCP and Lua all serve. The migration's only
steps were `drop_property` deletions of the status property, so the values that
could reconstruct the intended faces were destroyed. There is no in-place undo.

## The gap

`CompareShapes` raises the delta at `TierMigration` and states plainly that the
rows must be migrated; the comment above it says the store must not adopt this
shape on its own. That demand was satisfied *structurally*, not semantically: a
file satisfied it by declaring a `to` hash matching the new shape, and nothing
checked that any step addressed the faces. `rela migrate gen` had no face
handling at all, so it drafted exactly such a file.

Faces were present in `ShapeProjection` and in the classifier, so the change was
correctly *detected* as needing migration. They were absent from the step
vocabulary, so it could not be *resolved*. Detection and remediation were
designed against different lists.

## Fix

FEAT-H2GSOJ adds `migrate_face`: each value of a keying enum names the face its
rows move to, checked exhaustively at parse time against the migration file's
embedded projections. A value with no destination is an authoring error rather
than rows silently stranded.

Two things make it a fix rather than a primitive:

- A file spanning the delta with **no** `migrate_face` step is refused
(`validateDeltasResolved`). Without that the do-nothing file still applied.
- `migrate gen` drafts a live step whose `CHANGEME` placeholders are not valid
faces, so an unedited draft does not apply either.

The structural guard is AM-migration-delta-kinds-have-resolving-steps:
`resolvingSteps` maps every `TierMigration` delta kind to what resolves it, and
a test scans the classifier so the table cannot drift. It proved itself during
the BUG-HC6I2T merge — when the delta kinds were renamed upstream, the guard
failed immediately instead of letting the mapping go stale.

## Design history

The fix first shipped as `confirm_face`, which confirmed rather than moved,
because two store row-family invariants meant a row could never leave the
coordinate it started at. BUG-HC6I2T (#1557) removed `bare_face` and both
invariants, so the step became the per-row move originally wanted.
`TestFaces_RowCanLeaveTheZeroCoordinate` pins the premise it now rests on.

## Note on the `from` hash

The migration that exposed this declared a `from` hash that did not match the
store's and applied anyway. That is by design: `Resolve` takes a "free edge"
when the gap is compatible, so multi-tenant stores at differing hashes can catch
up without no-op migrations. What compounded the bug is what follows — taking
the free edge rebases the walk onto the file's projections, so the marker is
written as fully conformant and the run reports success even though the face
question was never answered.

---
id: BUG-TMGWIN
type: bug
title: No migration step can assign rows to a face, so bare_face adoption silently relabels every existing row
description: 'Introducing bare_face on a type that already has data is classified TierMigration and asks the operator to confirm the outcome with a migration. No step kind can express that confirmation: there is no set_face, the lua step cannot read or write a face, and rename_face refuses bare to named by design. The migration file discharges the demand by declaring the right `to` hash while its steps do nothing about faces, and every pre-existing row silently becomes the bare face.'
priority: high
effort: m
why1: A migration applied cleanly and reported "data schema in sync", but every existing row of the faced types landed on the bare face and the other face was left empty. Its only steps were drop_property deletions of the status property; nothing assigned a face.
why2: 'No step kind can assign or confirm a face. Of the twelve kinds in steps.go there was no face-assignment step; the lua step''s entityToTable exposes only content and properties; and rename_face deliberately refuses bare to named. That last refusal turned out not to be a rename-specific policy but the store''s row-family contract: an entity always occupies its bare coordinate, so no step could have moved rows even if one had existed.'
why3: The gate's demand is satisfied structurally rather than semantically. CompareShapes raises bare_face_introduced at TierMigration saying "confirm with a migration that this is the state they belong to", but a file satisfies that by declaring a `to` hash matching the new shape. Nothing verifies that any step addressed the faces, and `rela migrate gen` has no face handling at all, so it drafts exactly such a file.
why4: The data-migration system was built around property- and type-level shape deltas. Faces were included in ShapeProjection and in the classifier (so the change is correctly DETECTED as needing migration) but never in the step vocabulary (so it cannot be RESOLVED). Detection and remediation were designed against different lists.
why5: A gate can demand an operator action that the system provides no primitive to perform, and nothing catches the mismatch. DEC-0VGTF3 recorded that face re-keying would ride the general data-migration system as "one migration operation among several"; the system shipped without it, and no check ties a TierMigration delta kind to a step kind capable of resolving it. Every delta the classifier can raise needs a corresponding way to answer it.
prevention: 'Add a confirm_face step so the classifier''s demand can actually be answered, and pair it with a structural guard: every TierMigration delta kind must map to at least one step kind that can resolve it, asserted by a test over both lists so a future delta kind cannot ship detection without remediation. The deeper lesson is that the remediation had to be designed against what the STORE permits, not against what the classifier''s wording implied: the obvious reading of ''confirm which state they belong to'' is a per-row assignment, which the row-family invariants make impossible.'
status: done
---

## Summary

Introducing `bare_face:` on a type that already has data is classified
`TierMigration` and asks the operator to confirm the outcome with a migration.
No step kind can express that confirmation. The migration file discharges the
demand by declaring the right `to` hash, while its steps do nothing about faces,
and every pre-existing row silently becomes the bare face.

## Observed impact

Found when a downstream project gave two entity types a pair of faces with
`bare_face:` naming one of them, and ran the migration against a copy of its
production data.

- All 61 existing rows landed on the bare face; the other face was empty (404).
- Six rows lost a meaningful distinction, including one that was withdrawn and
now read as current at the bare address.
- That bare address is what exports, feeds, documents, CLI, MCP and Lua all
serve, so the wrong state is what every consumer sees.
- The migration's only steps were `drop_property` deletions of the status
property, so the values that could reconstruct the intended faces were
destroyed. There is no in-place undo.

## Why it cannot be fixed in the migration file

No step kind of the twelve declared in `internal/datamigration/steps.go` assigns
a row to a face:

- `set_face` does not exist. The kinds are `rename_property`,
`rename_entity_type`, `rename_face`, `rename_relation_type`, `map_values`,
`set_default`, `recompute_computed`, `convert`, `drop_property`,
`drop_entities`, `drop_relations`, `lua`.
- The `lua` step patches properties, unset and content only.
`entityToTable` (`luastep.go:137`) exposes `content` plus properties; the row's
face is neither readable nor writable from a script.
- `rename_face` moves rows between coordinates but deliberately refuses
bare → named (`steps.go:241-247`): the bare face is the family's identity row
and stores refuse to delete it while sibling faces remain. Its error directs the
operator to change `bare_face:` in the schema instead.

## The gap

`CompareShapes` raises `bare_face_introduced` at `TierMigration`
(`internal/metamodel/shapecompare.go:217`) with the text "every existing row is
stored at the zero coordinate and would silently become that face. Confirm with
a migration that this is the state they belong to". The comment above it states
the store must not adopt this shape on its own.

That demand is satisfied structurally, not semantically. A migration file
satisfies it by declaring a `to` hash matching the new shape; nothing checks
that any step addressed the faces. `rela migrate gen` has no face handling at
all, so it drafts exactly such a file. The gate correctly refuses to adopt the
change silently, then hands off to a system with no primitive to carry it out.

Faces are present in `ShapeProjection` and in the classifier, so the change is
correctly *detected* as needing migration. They are absent from the step
vocabulary, so it cannot be *resolved*. Detection and remediation were designed
against different lists.

## Fix

FEAT-H2GSOJ adds a `confirm_face` step: the operator states, value by value,
which face the existing rows become, checked exhaustively at parse time against
the migration file's embedded projections.

It confirms rather than moves, and that is forced by the store rather than
chosen. Two row-family invariants (TKT-DOFYR1) hold in every backend — a
family's default row cannot be deleted while a sibling face remains, and a
named-face row cannot exist without a default row — so an entity ALWAYS occupies
its bare coordinate and no step could ever have moved rows off it. `rename_face`
refuses bare to named for this reason, which turns out to be the store's
contract rather than a rename-specific policy. Pinned by
`TestFaces_RowCannotLeaveTheBareCoordinate`.

Giving different entities different faces needs a second row per entity, which
is a copy with its own semantics: FEAT-JZCGZW.

Alongside it, a structural guard: every `TierMigration` delta kind should map to
at least one step kind that can resolve it, asserted by a test over both lists
(AM-migration-delta-kinds-have-resolving-steps). That is what would have caught
this before release, and what stops the next delta kind shipping detection
without remediation.

## Note on the `from` hash

The migration that exposed this declared a `from` hash that did not match the
store's, and applied anyway, reporting "data schema in sync". That part is by
design, not a second defect: `Resolve` (`resolve.go`) takes a "free edge" when
`CompareShapes` classifies the gap as compatible, so multi-tenant stores at
differing hashes can catch up without no-op migrations.

What compounds this bug is what follows. Taking the free edge rebases the walk
onto the file's projections, so the marker is written as fully conformant and
the run reports success, even though the face question was never answered. The
"in sync" report is the system confirming a state nobody confirmed.

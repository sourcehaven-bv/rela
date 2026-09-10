---
id: FEAT-H2GSOJ
type: feature
title: 'confirm_face migration step: confirm which face existing rows become when a type adopts bare_face'
summary: 'A migration step that confirms which content state already-existing rows take on when a type gains bare_face. Every value of a keying enum property must name the face its rows become, checked exhaustively at parse time against the migration file''s embedded projections, so a value with no sensible destination is an authoring error rather than a silent default. The step writes nothing: rows cannot be reassigned between faces, because the store keeps every entity on its bare coordinate.'
description: 'Adds a confirm_face migration step answering the question a bare_face delta asks: which face do the rows that already exist belong to? Validation is exhaustive over the keying enum''s value set and runs at parse time against the file''s embedded projections. Every value must map to the declared bare_face, since that is provably where those rows land; naming a different face is refused with guidance to change the schema instead. The step writes nothing, because two store row-family invariants (no headless named rows; a family''s default row cannot be deleted while siblings exist) mean an entity always occupies its bare coordinate. Includes migrate gen support emitting a commented skeleton, and a parse-time guard against a drop_property preceding the confirm_face that reads it. Per-row face assignment needs copy semantics and is tracked separately.'
priority: high
status: implemented
---

## Problem

Adopting faces on a dataset that already has rows could not be expressed.
`CompareShapes` raises `bare_face_introduced` at `TierMigration` and instructs
the operator to "confirm with a migration that this is the state they belong
to", but no step kind could express that confirmation (BUG-TMGWIN). The
operator's only option was a migration that declares the right `to` hash and
does nothing, which silently relabels every row.

## What shipped

```yaml
- confirm_face:
    entity: article
    property: status
    mapping:
      draft:     published
      active:    published
      withdrawn: published
```

Every value of the keying enum must name the face its rows become. `Validate`
proves the mapping total at parse time against the file's own embedded
projections — `ShapeProjection` carries `Types` (each enum's value list)
alongside per-type `Faces` and `BareFace`, so this needs no live metamodel.

A value with no entry is an error, which is the safety property the step exists
for: a value whose rows do not belong in the new bare face gets caught at
authoring time rather than discovered later.

## The constraint that shaped the design

The original plan was per-row assignment: each row moves to the face its status
implies. **The store does not permit this**, and deliberately.

Two row-family invariants (TKT-DOFYR1) hold in every backend:

- a family's default row cannot be deleted while a sibling face remains, and
- a named-face row cannot exist without a default row ("headless").

Together they mean an entity **always** occupies the bare coordinate. A row
cannot move off it in either order: create-then-delete leaves the bare row
undeletable, delete-then-create is refused as headless. This is the same wall
`rename_face` reports when it refuses bare → named — not a policy choice
specific to renames, but the store's row-family contract.

So repointing `bare_face:` moves nothing. It relabels every bare row in place,
which is precisely why the classifier flags it: nothing looks wrong afterwards.
The migration's job is to confirm the relabel, not to perform a move.

Pinned by `TestFaces_RowCannotLeaveTheBareCoordinate`, which asserts both
directions. If that test ever fails, this design is back in play.

## Why a mapping rather than a bare confirmation

The confirmation could have been `- confirm_face: {entity: article}`. Requiring
the operator to write out, value by value, which face each existing row lands on
is what makes it informed rather than ceremonial. Every value must map to the
declared bare face, since that is where those rows provably end up; naming a
different face is refused with the reason and the fix (make that face the
`bare_face:`, or separate those rows first).

Getting that error at parse time, against a value set written out in full, is
the point — it is the moment the operator discovers the schema does not fit the
data.

## Guards

- **Non-bare targets are refused** with guidance, rather than accepted and
silently not honoured.
- **`drop_property` before the `confirm_face` that reads it is refused** at
parse time (`validateStepOrder`), since it would leave the step confirming
nothing.
- **Rows outside the declared value set** (unset, or a stale value) are counted
and reported in `StepResult.Notes`, never folded into the confirmation.

## Generator support

`draftActiveStep` gained a `bare_face_introduced` / `bare_face_changed` case
emitting a `confirm_face` skeleton with every value pre-listed against the
declared bare face, under a `TODO` banner explaining what to check.

The skeleton is emitted **commented**. `Generate` round-trips its own output
through `ParseFile`, and an unedited draft must not silently confirm a state
nobody looked at. Pinned by `TestGenerate_FaceDraftRoundTripsThroughParse`.

When no enum property can key the confirmation, the generator declines rather
than guessing and says what to check by hand.

## Follow-up

Assigning *different* rows to *different* faces needs a second row per entity
rather than a move — copy semantics, with its own questions about what content
each row carries. Tracked separately.

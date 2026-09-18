---
id: TKT-RJV4C0
type: ticket
title: Docs and analyze remedy still say faces cannot be adopted on populated types; migrate_face does it
kind: docs
priority: medium
effort: s
status: backlog
---

## Description

`docs/data-migration.md` tells operators that adopting faces on a populated type
cannot be done with a migration step, two paragraphs before the section that
documents the step which does it.

The stale passage is under "Adding or removing faces on a type that holds data"
(`docs/data-migration.md:136-141`):

> `rename_face` cannot express this move. It requires a declared face name on
> both sides, and the zero coordinate is not one, so a project crossing this
> boundary with data in it needs the rows rewritten out of band before the new
> schema is adopted. Plan the change on an empty type where you can, and treat a
> populated one as a data-export-and-reimport rather than a step in a migration
> file.

That was true before FEAT-H2GSOJ. `migrate_face` now moves exactly these rows,
and the very next section ("Adopting content states on data you already have",
`:160-207`) documents it in full, including that a file spanning
`faces_introduced` without the step is **rejected**. The two sections contradict
each other, and the wrong one comes first.

The advice is not merely out of date — it is worse than the tool. A
data-export-and-reimport on a populated type is a high-risk manual operation
with no exhaustiveness check, no idempotence, and no version capture, all of
which the step provides.

## Also wrong: the analyze remedy text

`internal/analysis/states.go:300-302` tells the operator to fix a
`bare-row-on-faced-type` finding by adopting the rows "with a `migrate_face`
migration step for that type". For a type that already declares faces there is
no shape change, so no migration file can carry that step and its `Validate`
would refuse it (`steps.go:380-434`). The remedy names a tool that will not do
the job. TKT-FTOENU proposes the surface that would make such advice true; this
ticket covers saying something accurate in the meantime.

## Scope

- Rewrite `docs/data-migration.md:136-141` to point at `migrate_face` and drop
the export-and-reimport advice. Keep the mirror sentence about `faces_removed`
(`:143-145`), which remains accurate — that direction genuinely has no step
(TKT-1YBNQJ).
- Correct the `bare-row-on-faced-type` remedy string in
`internal/analysis/states.go` so it does not prescribe a step that cannot apply.
The finding itself is right and stays.
- While in the file: the `store.EntityWriter` doc comment
(`internal/store/store.go:418-426`) still claims `DeleteEntityState` "refuses
(ErrInvalidQuery) to delete the DEFAULT face while non-default faces remain".
BUG-HC6I2T removed that invariant; no implementation refuses, and
`TestFaces_RowCanLeaveTheZeroCoordinate` (`migrateface_test.go:387`) asserts the
opposite as the premise `migrate_face` rests on.

Docs in this repo are generated from `docs-project/entities/`, so the edit
belongs there rather than in `docs/` directly.

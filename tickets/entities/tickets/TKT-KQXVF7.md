---
id: TKT-KQXVF7
type: ticket
title: Store API takes entity.Ref and RelationKey; queries must select their faces
kind: enhancement
priority: high
effort: xl
status: done
description: 'Stage 2 of RES-Y6JA37: store reads take a typed address, entity-level methods are explicit, and a query must select its faces.'
---

## Description

Stage 2 of RES-Y6JA37 / DEC-NPZICR. Design: `.ignored/stage2-design.md` (rulings
in section 10, review amendments in section 11, RR-QUXMAF ruling in section 12).

As built, in nine PRs into `faces-intrinsic`:

- `store.Store.GetEntity(ctx, entity.Ref)` on all four backends and the tx
views; `GetEntityState`, `GetEntityAt` and `StateGetter` removed (#1735).
- Face-level and entity-level methods are named apart: `DeleteFace(Ref)`,
`DeleteFamily`, `RenameFamily`, `AttachFamilyFile`, `DeleteFamilyAttachment`,
`ListFamilyAttachments`, plus the `store.FamilyHeaders` / `store.Family` helpers
(#1735).
- Relation methods take `entity.RelationKey`; `UpdateRelationState`,
`DeleteRelationState` and `RelationData.FromFace` removed from the store;
relation history and purge take the key (#1726, #1733).
- `EntityQuery` and `GraphQuery` require a `store.FaceSelection`
(`InWorld`, `AllFaces`, `AtFaces`); the zero value is `ErrInvalidQuery` on every
backend (#1734). This replaces the `AllStates` flag the first draft named.
- `AttachFamilyFile` checks existence over every face on all backends (#1735).
- pg migration `0018_face_read_indexes.sql` and sqlite rung 8 replace the
indexes partial on `face = ''` (#1725).
- `HistoryReader.ListVersions` / `GetVersion` take a `Ref` (#1735).
- `worlds.Compiled.Default()` seam; address readers renamed `GetAddress` (#1726).
- Guards: the Stage 0 zero-face allowlist is deleted. It is replaced by the
`bareref` guard (bare `entity.Ref` literals, `store.DefaultWorld()` calls), the
`tailless` guard (`RelationKey` literals without `FromFace`), the `faceselect`
guard (`EntityQuery` literals without `Faces`) and the retargeted `directread`
guard, each with a shrink-only allowlist.

Bugs fixed on top of this: BUG-J3PBFN (#1730), BUG-95W7MV (#1729). Also merged
under this ticket: remaining reads (#1728) and TKT-5LW875 (#1732).

Follow-ups filed: TKT-OQ7MDF, TKT-EFOQCX, TKT-1A2KE3, TKT-JAD5M9, TKT-5W4ISW.
The default world for faced types stays with TKT-7IZHP0.

## Acceptance

- `storetest` passes on fs, mem, pg and sqlite.
- No store method reads the zero face implicitly: `GetEntity` takes a `Ref`,
relation methods take a `RelationKey`, and a query without a `FaceSelection` is
`ErrInvalidQuery`.
- `internal/archguard` bareref, tailless, faceselect and directread guards
pass with shrink-only allowlists.

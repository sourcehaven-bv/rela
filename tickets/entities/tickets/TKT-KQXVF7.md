---
id: TKT-KQXVF7
type: ticket
title: Store API takes entity.Ref; zero-value EntityQuery is invalid
kind: enhancement
priority: high
effort: xl
status: ready
description: 'Stage 2 of RES-Y6JA37: store reads take a typed address, entity-level methods are explicit, and a query must select its faces.'
---

## Description

Stage 2 of RES-Y6JA37 / DEC-NPZICR.

- Replace `store.Store.GetEntity(id)` with a read taking `entity.Ref`, in all four backends and the tx views.
- Give `GetRelation`, `RenameEntity` and the attachment methods explicit signatures: face-level ones take `Ref`, entity-level ones are named as covering every face.
- Make the zero-value `EntityQuery` invalid: a query must select a world, `AllStates` or explicit faces.
- Align the `AttachFile` existence check across backends (fs/mem check by key today, pg/sqlite accept any face).
- Replace the pg read indexes that are partial on `face = ''` (migration 0014) so faced rows use them too.
- `HistoryReader.ListVersions`/`GetVersion` take a `Ref`.
- Delete the Stage 0 guard allowlist; the compiler now enforces the rule.

Bugs fixed on top of this: BUG-J3PBFN, BUG-95W7MV.

## Acceptance

- `storetest` passes on every backend.
- No store method reads "the zero face" implicitly.

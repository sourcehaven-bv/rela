---
id: TKT-FKF11BK
type: ticket
title: Break the Configure api/configure and stores/configure import cycles
kind: refactor
priority: medium
effort: xs
started: "2026-10-31"
completed: "2026-10-31"
status: done
---

## Description

The Configure frontend code had two import cycles between folders (archplan
finding `structure.tangle-b1bf2e78506f`, handover ho-22c49f92, batch 1):

- `api/configure.ts` imported the wire tree types (`TreeScalar`, `TreeMap`,
`TreeValue`) from `configure/tree.ts`, while `configure` imports `api`.
- The config draft store lived in `stores/configDraft.ts` and imported
`configure`, while `configure/useReview.ts` and `configure/useDiscardDraft.ts`
imported the store.

Origin: archplan threads th-74f9afc1 and th-6ac8a582.

## Approach

1. Move the three tree types into `api/configure.ts`, next to the wire types
that use them. `configure/tree.ts` keeps `TreePath` and the edit helpers and
imports the types from api. Importers take the types from `@/api/configure`
directly; there is no re-export.
2. Move `stores/configDraft.ts` and its test into `configure/`. Keep the Pinia
store id `configDraft` and the localStorage key, so drafts saved before the
change still load.

No behaviour change.

## Acceptance criteria

- No file under `frontend/src/api` imports `frontend/src/configure`.
- No file under `frontend/src/configure` imports `frontend/src/stores`, and no
file under `frontend/src/stores` imports `frontend/src/configure`.
- The store id and `draftStorageKey()` are unchanged.
- type-check, lint, unit tests, build, `just arch-lint` and
`e2e/tests/configure.spec.ts` pass.

---
id: BUG-NSGD6OE
type: bug
title: SPA edit form saves an edit onto a different entity after in-form navigation
description: 'An edit made in the SPA edit form of one entity can be autosaved onto another entity of the same form after a same-route navigation (/form/:id/X to /form/:id/Y). Reported on Atlas: an edit to procedure X landed on procedure Y.'
priority: critical
effort: m
why1: DynamicForm's autosave sent the PATCH for an edit made on X to Y. It reads the target as props.entityId when the debounce fires, and by then the prop had changed from X to Y.
why2: 'The router reuses DynamicForm when only :entityId changes (App.vue RouterView has no key, FormView passes the prop through). DynamicForm guards only onBeforeRouteLeave, which a param change does not fire, and it has no watcher on entityId. So it neither flushes X''s pending edits nor reloads: it keeps X''s values and merge bases under Y''s id.'
why3: 'The server did refuse the write: per-field tokens include the entity id, so X''s preconditions failed on Y with 412. useAutoSave then refetched Y and three-way merged X''s base, X''s edit and Y''s value. Where Y''s value equals X''s base (same status, empty field, template text) the merge says write, and the retry stores X''s edit on Y with Y''s fresh tokens.'
why4: useAutoSave takes the write target as a late-bound closure (getEntityType/getEntityId) and keeps no record of which entity its bases came from. Nothing ties a pending edit or a merge base to the entity it was made against, so a changed target is indistinguishable from a concurrent edit by someone else.
why5: 'Entity identity for a write is treated as ambient component state rather than as part of the edit. Each host must guard route reuse by hand: EntityDetail pins the id, SectionEditForm freezes it, DynamicForm forgot. No test drives a same-route param change on an editing component, and the conflict merge was designed for one entity only.'
prevention: 'Make the write target part of the autosave instance: a fixed {type, id} set at construction, no closure. Key the edit form per entity and flush on onBeforeRouteUpdate. In useAutoSave, fail closed when a snapshot or a 412 refetch names a different entity than the target. Regression tests drive a same-route entityId switch on the real DynamicForm.'
started: "2026-10-30"
completed: "2026-10-30"
status: done
---

## Description

In Atlas, an edit made in the SPA edit form of procedure X was saved onto
procedure Y. The user edited X; the stored change appeared on Y.

## Mechanism (from code reading)

1. The edit form route is `/form/:id/:entityId`. `App.vue` renders a bare `<RouterView />` with no `:key`, and `FormView` passes `entityId` straight to `DynamicForm`. Navigating from `/form/procedure/X` to `/form/procedure/Y` (side-panel link, browser back/forward, a pushed edit route) reuses the same `DynamicForm` instance.
2. `DynamicForm` builds `useAutoSave` once in `onMounted` with `getEntityId: () => props.entityId!`. The id is read when the save fires, not when the edit was made.
3. `DynamicForm` only flushes in `onBeforeRouteLeave`. A param change in the same route record fires `onBeforeRouteUpdate` instead, so nothing flushes or cancels the pending debounce (800 ms) for X.
4. `DynamicForm` has no watcher on `props.entityId`, so it does not reload either. After the switch the form still shows X's values and X's merge bases while the header and the save target are Y.
5. The PATCH to Y carries X's per-field precondition tokens. These include the id, so the server answers 412. `useAutoSave.sendPatch` then refetches **Y**, runs a three-way merge with X's base, X's edit and Y's current value, and retries with Y's fresh tokens. When Y's value equals X's base (same status, same empty field, same template line), the merge says "write", and X's edit is stored on Y. The conflict protection turns a cross-entity mismatch into a clean merge.

`EntityDetail` already guards the same reuse with `pinEntityForFlush` and a
watch on the id. `SectionEditForm` freezes its `owner` on mount. `DynamicForm`
has neither.

## Expected

An edit is only ever written to the entity whose state the user edited. A save
target that does not match the loaded entity must fail closed.

---
id: TKT-ZEKO4
type: ticket
title: Migrate RelationCards + RelationPicker to unified PATCH-with-relations wire format
kind: enhancement
priority: medium
effort: m
status: done
---

## Problem

Today's `DynamicForm.savePendingRelationCards` (DynamicForm.vue:474) and the
`RelationPicker` widget both use the **per-edge** REST endpoints to save
relation changes:

- `POST /api/v1/{plural}/{id}/relations/{relType}` — create one edge
- `PATCH /api/v1/{plural}/{id}/relations/{relType}/{targetId}` — update one edge's meta
- `DELETE /api/v1/{plural}/{id}/relations/{relType}/{targetId}` — delete one edge

A form with 5 added + 3 removed + 2 updated relation rows fires **10 separate
HTTP requests**, in parallel via `Promise.all`. None of them are atomic with
respect to each other or with the entity write that runs first.

TKT-6WLSW shipped the **unified PATCH wire format** that supports per-edge meta
+ content in a single request:

```http
PATCH /api/v1/tickets/TKT-001
{
  "relations": {
    "tagged": {
      "data": [
        {"type": "label", "id": "L-001", "meta": {"weight": 5}, "meta_unset": ["added_by"]},
        {"type": "label", "id": "L-002"}
      ]
    }
  }
}
```

This ticket migrates the form widgets to use the new shape.

## Why now

- **Single round-trip per save**. 1 request instead of N. Faster, less flaky.
- **Atomicity**: the unified PATCH validates everything before any write
(DEC-HWZHA + TKT-6WLSW's validation-first ordering); per-edge calls have no such
guarantee.
- **Unblocks autosave** (TKT-E6094): `formAllowsAutosave` currently excludes
RelationCards/RelationPicker forms. After this ticket, the same `useAutoSave`
composable can serialize relation edits through the same FIFO queue as property
edits, just by adding a `scheduleRelationsChange()` method.
- **Per-edge endpoints can be retired** (out of scope for this PR; separate
deprecation ticket once nothing uses them).

## What this ticket delivers

- `DynamicForm.savePendingRelationCards`: replace the per-edge `Promise.all`
with a single `updateEntity(type, id, {relations: {...}})` call using the modern
wire shape from TKT-6WLSW.
- `RelationCards.vue`: emit changes in the shape the modern wire format wants
(`{added: [{type, id, meta?, content?}], removed: [...], updated: [...]}`). Same
as today plus per-edge `meta` and `content` flowing through.
- `RelationPicker.vue` + the incoming-direction bridge in DynamicForm
(`updateIncomingPicker`): same migration. The `direction: "incoming"` shape
doesn't fit the unified PATCH directly (the PATCH targets a single source
entity); for incoming relations, we still need the per-edge endpoints OR a
second PATCH targeting the peer. **Decide in planning.**
- Frontend types in `entities.ts`: ensure the `RelationsUpdate` and
`ResourceIdentifier` types from TKT-6WLSW are exported and usable. Add them now
if not already.
- Tests: e2e tests for the form save flow with mixed adds/removes/meta-updates,
asserting one PATCH request fires (not N).

## Open questions for planning

- **Incoming-direction relations**: the unified PATCH only handles outgoing
relations of the source entity. To handle "incoming pickers" (where the user
manages who points AT this entity), we either (a) keep using the per-edge
endpoints for incoming, or (b) issue one PATCH per peer entity whose outgoing
edge changes. Option (a) is simpler; option (b) is more consistent. Plan in
PLAN-_.
- **Mixed-shape body when both incoming and outgoing changes exist**: if (a)
above, the form save fires 1 unified PATCH for outgoing + N per-edge for
incoming. Acceptable but documents the asymmetry.

## Out of scope

- Retiring the per-edge endpoints. They stay for backwards compat; a
deprecation ticket is separate.
- Frontend autosave integration — that's TKT-E6094.
- Symmetric / inverse relation propagation on writes — known gap, separate.

## Relation to other work

- **Depends on**: TKT-6WLSW (merged) — the unified PATCH wire format
- **Independent of**: TKT-QETTR (validation softening) — they touch
different code paths
- **Unblocks**: TKT-E6094 (autosave) — after this ticket, RelationCards forms
become autosave-eligible

## Acceptance criteria sketch

(Full ACs in PLAN-_ after planning.)

1. Saving a form with 3 added + 2 removed + 1 meta-updated `tagged` relations
fires **exactly one** `PATCH /api/v1/tickets/TKT-001` with body `{relations:
{tagged: {data: [...]}}}`. **Test**: e2e + vitest with network mock.
2. Per-edge `meta` from RelationCards (e.g., `weight`) lands on the relation
correctly via the modern wire shape. **Test**: e2e.
3. Per-edge `content` body (markdown) from RelationCards lands correctly.
**Test**: e2e.
4. Removing a row sends the relation type with `data: [...]` containing only
the kept rows; the removed row is absent. **Test**: vitest.
5. Clearing all rows sends `data: []` (full clear). **Test**: vitest.
6. Forms with only outgoing relations: ALL relations save in one PATCH.
**Test**: vitest.
7. Forms with mixed incoming + outgoing (per the planning decision):
outgoing batched in one PATCH, incoming via TBD. **Test**: e2e.
8. The DynamicForm `savePendingRelationCards` function is removed or
reduced to a thin wrapper. **Test**: code review + vitest assertions.
9. Warnings from the unified PATCH (e.g., `target_not_found`,
`target_type_mismatch`) flow back to the form and render inline next to the
offending row. **Test**: vitest.

## Effort

m. Frontend-heavy refactor with a few open design questions on incoming
relations. 1–2 days plus design review.

---
id: TKT-IHC7D
type: ticket
title: View wire-shape — typed _props + _fields per cards/list row entity
kind: enhancement
priority: medium
effort: s
status: done
---

## Goal

Extend `V1ViewEntity` (the row shape used by cards / list / properties-of-collection view sections) so each row carries:

1. `_props: map[string]any` — the entity's typed property values (currently the row only carries `Fields[].Values: []string`, display-stringified).
2. `_fields: map[string]V1FieldAffordance` — per-cell writability verdict for THIS row's entity (currently only entry-level GETs carry `_fields`; rows skip it).

Prerequisite for TKT-IHC7C (cards/list inline edit) — `useAutoSave`'s no-op suppression needs typed initial values, and per-cell writability gating needs the verdict on the row.

## Scope

### Backend (Go)

- Extend `V1ViewEntity` in `internal/dataentry/api_v1.go` with `_props` and `_fields` (with the existing pointer-to-map semantics used by `V1Entity.FieldAffordances` so "absent" vs "present-but-empty" stays distinguishable).
- Populate from the entity in `internal/dataentry/sections.go` and the legacy templates path:
  - `_props`: copy from `e.Properties` (the typed map already on the entity).
  - `_fields`: invoke the existing `App.computeFieldAffordances(ctx, e)` per row.
- Introduce hidden-field stripping to the cards/list row surface via `App.hiddenProperties(ctx, e)`. This path does NOT today go through `stripHiddenProperties` (which targets `V1Entity`, not `V1ViewEntity`) — see PLAN AC 5 / RR-FD1A.
- Update the wire-format docs in `docs/data-entry/api-reference.md`.

### Frontend (TypeScript types only)

- Extend `ViewEntity` in `frontend/src/api/views.ts` with `_props?: Record<string, unknown>` and `_fields?: Record<string, FieldAffordance>` to match the new wire shape.
- **No UI consumer yet.** TKT-IHC7C wires SectionEditForm to consume these.

### Tests

- Go-side: extend the existing view-builder tests to assert `_props` matches `e.Properties` and `_fields` reflects the per-entity affordance verdict (mirror the entry-level `_fields` tests).
- Frontend-side: no new tests; the existing wire-decoding tests should still pass with the additive fields.

## Non-goals

- **No frontend rendering change.** This ticket only ships the wire shape and TS types. SectionEditForm wrapping per row is TKT-IHC7C.
- **No `_relations` per row.** Out of scope; row-level relation affordances are predicate territory (deferred per UD7YR comments).
- **No table cells.** `table` display uses `V1ViewRow.cells` with display-stringified values too, but that's a larger surface (column-driven) — punt to a separate ticket if/when needed.

## Why this is its own ticket

The IHC7B round-3 reviewer noted that absorbing the wire-shape change into the parent IHCY7 ticket was the wrong choice. Wire-shape changes touch backend Go code, require backend tests, may have migration concerns, and don't naturally compose with frontend-only inline-edit work.

Shipping this prerequisite solo is defensive: the new fields are additive, sparse, optional. Old SPAs ignore them. New SPAs (post-IHC7C) consume them. The contract evolves cleanly.

## Verification gate

1. `V1ViewEntity` carries `_props` and `_fields` on a cards/list view response (smoke via `curl /api/v1/_views/<type>/<id>`).
2. Affordance verdict on a row matches the verdict on a per-entity GET of that row entity (consistency check; same `computeFieldAffordances` path).
3. `_props` is hidden-field-stripped: properties marked hidden in the metamodel do not appear in the row's `_props`.
4. Existing view-builder tests pass unchanged; new tests cover the additions.
5. TS typecheck on the frontend passes after the `ViewEntity` shape update.

## Inherited findings

Resolves:

- **RR-UE3B** (cards/list wire-shape gap) — typed `_props` per row.

Required by:

- **TKT-IHC7C** (cards/list inline edit) — depends on this ticket.

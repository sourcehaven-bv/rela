---
id: TKT-IHC7C
type: ticket
title: Cards/list inline edit (requires typed _props per entity on the wire)
kind: enhancement
priority: medium
effort: l
status: backlog
---

## Goal

Make cards and list view sections inline-editable per-row, using `SectionEditForm` (built in TKT-IHC7B) wrapped around each row. Requires a wire-shape change to include typed `_props` per row entity (cards/list rows currently only carry display-stringified values).

This is the third slice of the split TKT-IHCY7.

## Blocker / dependency

**RR-UE3B** identified a wire-shape gap: cards/list `ViewEntity.fields` carries `values: string[]` (display-stringified) and does not carry typed property values. `useAutoSave`'s no-op suppression needs typed initial values to work correctly.

**Resolution:** TKT-IHC7D files the prerequisite wire-shape change — `V1ViewEntity` gains `_props` (typed values) and `_fields` (per-cell writability verdict). This ticket depends on TKT-IHC7D and becomes a frontend-only PR.

## Scope (sketch — refine in planning)

- Apply `SectionEditForm` (built in TKT-IHC7B) to each card row in `EntityDetail`'s cards section. Same for list rows.
- Each row passes its entity's typed `_props` as `initialValues` (requires the wire change).
- Per-cell writability gating via `_fields`.
- Indicator placement: one `AutoSaveIndicator` per row (in the row chrome), not per cell.

## Why this is its own ticket

The split's round-3 reviewer noted that absorbing the wire-shape change into the parent TKT-IHCY7 was the wrong choice. Wire shape changes touch backend Go code, require backend tests, may have migration concerns, and don't naturally compose with the frontend-only inline-edit work that ships in IHC7A and IHC7B.

This ticket may itself split into a wire-shape prerequisite + the frontend cards/list integration. Decision deferred to its own planning phase.

## Inherited findings

Resolves:
- **RR-UE3B** (cards/list wire-shape gap) — the headline problem of this ticket.

Depends on:
- **TKT-IHC7B** (SectionEditForm)
- **TKT-IHC7D** (typed `_props` + `_fields` per cards/list row entity on the wire)

## Out of scope

- The wire-shape change itself — shipped by TKT-IHC7D.
- View-config `editable: true` overrides (TKT-HOIX1).

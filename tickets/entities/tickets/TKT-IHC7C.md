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

**RR-UE3B** identified a wire-shape gap: cards/list `ViewEntity.fields` carries `values: string[]` (display-stringified) and does not carry typed property values. `useAutoSave`'s no-op suppression needs typed initial values to work correctly. Either:

- The cards/list wire shape must be extended to include `_props: Record<string, unknown>` per entity (backend change in `internal/dataentry/sections.go` and `api_v1.go`)
- OR `SectionEditForm` needs to be modified to handle a "no baseline yet" mode where no-op suppression is disabled until the first server response lands. This weakens the no-op story and bleeds back into `useAutoSave`.

Likely outcome: a separate prerequisite ticket for the wire-shape change, with this ticket depending on it. May split further depending on the scope of the wire change.

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
- A yet-to-be-filed wire-shape change ticket (or TBD: do that work in this ticket).

## Out of scope (and TBD)

- The wire-shape change itself: may be in-ticket or filed as a prerequisite. Decide during this ticket's planning phase.
- View-config `editable: true` overrides (TKT-HOIX1).

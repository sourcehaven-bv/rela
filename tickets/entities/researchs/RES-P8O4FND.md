---
id: RES-P8O4FND
type: research
title: Render-target-neutral presentation model
summary: A versioned server-built model of semantic nodes plus intents between the semantic API and rendered output, adopted incrementally starting with existing resolved payloads and kanban
status: done
---

## Problem

Can rela add a layer between the semantic API and fully rendered output: a
presentation model that does not depend on the render target? One server-built
model would drive the SPA, a TUI, native apps and MCP clients, so no client
reimplements the meaning of `data-entry.yaml`.

## Context

Rela already answers at three levels, applied unevenly:

- Semantic: entities, `_schema`, `_actions`, `_fields`.
- Resolved view model: `_views` sections (labels, values, render mode, span,
affordances), `_dashboard` cards, next-action bands, nav status, gantt roll-ups.
Gating and folding happen on the server.
- Fully rendered: export through `internal/transform`, mail through
`mailrender`, iCal and CalDAV.

Kanban, list, form and calendar have no resolved level. The SPA fetches
`/_config` and the list API, then groups cards, resolves `columns_from` and
swimlanes, and builds the PATCH for a card drop (`KanbanView.vue`). That logic
sits in about 11,000 lines of Vue views and would be duplicated by any second
client.

The resolved payloads are inconsistent: `ViewSectionField.render` is
server-resolved, but `widget` is not (`frontend/src/api/views.ts`).

Existing rules already push presentation logic to the server: gate before
folding, fail closed without identity, field verdicts computed server-side.
Every client-assembled view is a place where a new client can get that order
wrong.

The desktop app wraps the SPA through Wails, so it gains little directly.

## Options

A. Status quo. Keep the semantic API plus per-feature resolved endpoints, and
let each client interpret the config. No migration cost. Every new client
reimplements kanban, form and list semantics, and the field-by-field boundary
stays arbitrary.

B. Presentation model of semantic nodes plus intents. A versioned, typed tree:
`page`, `section`, `field {label, type, value, affordance, hint}`, `collection
{rows, pagination}`, `board {columns, cards}`, `timeline`, `form {steps,
fields}`, `action {id, label, confirm}`. Interactions are named intents ("card
can move to column X") that the server translates to writes. Layout hints such
as span or emphasis are optional, and clients may ignore them. Built by one Go
package (for example `internal/present`) that both the HTTP handlers and an
in-process `rela tui` call. Costs: rewrite of the kanban, form and list views;
protocol versioning with capability negotiation and an unknown-node fallback;
rich interactions (gantt drag, reorder, calendar drag, rich-text editing) still
need client-side behaviour and a predicted result to avoid round-trip latency.
Effort xl, but incremental.

C. Server-rendered component tree. The server sends widgets and layout. Thin
clients, but it is HTML in another syntax, browser habits leak into the
protocol, and non-browser targets fit poorly.

D. Server-rendered HTML with a hypermedia library (htmx-style). Fast for web, no
help for TUI, native or MCP. Rejected for this goal.

## Recommendation

B, adopted incrementally, with the semantic API kept for scripts, custom apps
(`_apps` and the bridge) and power clients:

1. Unify the existing resolved payloads (`_views`, dashboard cards, next
actions, nav status) into one versioned Go model with generated TypeScript
types, so a rule decides what the server resolves.
2. Move kanban to the model first: columns, cards and per-card move intents.
3. Then forms: steps, fields, widgets and affordances in one payload.
4. Build a throwaway TUI against the model early as the second client that
proves render neutrality.

Open questions: which client justifies the work (a TUI needs steps 1 and 2;
native mobile needs a versioning and interaction-model decision first), and
whether every interaction goes through intents or rich views keep direct writes.

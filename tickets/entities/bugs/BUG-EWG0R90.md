---
id: BUG-EWG0R90
type: bug
title: Flow runs are missing from tenant data export
description: A project or tenant export copies entities, relations and attachments but not the flow run store. Open runs lose their state after an export and import.
priority: medium
status: backlog
---

## Description

A tenant or project export copies entities, relations and attachments
(`ExportMarkdownData`, `copyData` in `projectdata_sqlite.go`). It does not copy
the flow run store: the run state of unfinished runs (instance, pending mark,
job attempt, timeline, branches) and the stored flow definitions.

After an export and import, every `_flow-run` header still says the run is open,
but no run state exists behind it. The runs cannot advance, be retried or be
cancelled through the engine.

Found while planning TKT-PY2G11W (flow job steps), which adds job attempt state
to the run store.

## Scope

- Include the run store (runs and definitions) in the export for every
backend (kv, sqlite, postgres).
- Restore it on import, or document that an import closes open runs.
- Related: TKT-TWLN8UB (`db load` of runs).

---
id: BUG-EWG0R90
type: bug
title: Flow runs are missing from tenant data export
description: A project or tenant export copies entities, relations and attachments but not the flow run store. Open runs lose their state after an export and import.
priority: medium
why1: ExportMarkdownData and copyData copied entities, relations and attachments, but not the flow run store.
why2: The flow run store is a separate store outside the graph, added after the export code was written.
why3: Adding a project store did not require extending export and import; nothing checked that the export covers every store.
why4: Export and import were written as a list of graph parts, not as a copy of every store the project owns.
why5: There is no inventory of project stores that export and import are checked against; AM-export-covers-every-project-store is meant to provide it.
prevention: flowrun.CopyRun copies a run with its definition; the sqlite db dump, db load and import-fs paths carry runs. TestMarkdownData_FlowRunsRoundTrip pins the round trip. Measure AM-export-covers-every-project-store tracks the general rule.
started: "2026-11-16"
completed: "2026-11-16"
status: done
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

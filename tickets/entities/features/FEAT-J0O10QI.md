---
id: FEAT-J0O10QI
type: feature
title: 'Flows: declared processes on one engine'
summary: flows.yaml definitions run on the extracted session engine with _flow-run runs and provenance
description: Declare processes in flows.yaml and run them on one engine extracted from realtime channel sessions. Runs are rela-managed _flow-run entities; every write records its run and step.
priority: medium
status: proposed
---

## Summary

Operators declare processes in `flows.yaml`. Rela runs them on one engine, the
same one that drives channel sessions, enum transitions and automations. Every
write records the run and step that produced it.

## Settled decisions

- Definitions live in `flows.yaml` (config, not secret), versioned by content hash.
- Runs are entities of the reserved, rela-managed type `_flow-run`, with
relations `_carries` (subject) and `_produced` (created entities). The `_`
prefix is reserved for types, relations and properties. Nobody writes runs
directly through the SPA, API, MCP or CLI.
- Execution state (active state, timers, reminders) lives in a run store,
not in the graph.
- One engine, extracted from the realtime session engine. Existing features
compile onto it; no duplicate code.
- Edits outside a flow follow a per-flow policy: `allow`, `deny` or
`permission`, per state and per property. It compiles into the ACL ceiling.
- Compaction is built in: runtime state drops on completion; headers archive
or delete after a configured age.
- fs gets a good-enough version; history of runs is not supported on fs.

## Reused from realtime

States and outcome status, waiting roles, guarded branches, timers that fire
once across processes, outputs (`set`, `create` + `relate`), predicate triggers,
reminders, handoff, the `_inbox`, retention, stranded-state handling, and the
session store on file, pg and sqlite.

## Milestones

- M0: extract the session engine into `internal/flow`, no behavior change.
- M1: flows without channels (`flows.yaml`, `_flow-run`, run store, start
triggers, provenance, retention, CLI).
- M2: people (step forms, entity-page panel, run list, MCP).
- M3: policy for edits outside the flow.
- M4: parallel branches (state becomes a marking) and `job` steps.
- M5: compile enum transitions and automations onto the engine; delete the
old paths; dogfood the tickets workflow.

## Out of the MVP (not ruled out)

BPMN import/export, process mining and conformance, subflows and `for_each`,
compensation, a visual editor, migrating running runs to a new definition
version.

## Open questions

- Keep `realtime.yaml` session syntax permanently, or migrate it to
`flows.yaml` with a data migration?
- Package name: `flow` or `process`.

Not to be confused with FEAT-YIOF (interactive Lua flows), which is about
coroutine-driven UI in scripts.

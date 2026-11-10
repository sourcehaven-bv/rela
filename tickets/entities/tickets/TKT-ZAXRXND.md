---
id: TKT-ZAXRXND
type: ticket
title: 'Flows M2: step forms and run UI'
kind: enhancement
priority: medium
effort: l
started: "2026-11-10"
completed: "2026-11-10"
status: done
---

## Description

Let people do their steps.

- Step forms limited to the step's `writes`; completing a step is an
`_actions` affordance and goes through `AuthorizeUpdate`.
- Entity page panel: active run, state, deadline, diagram with current
position, timeline from the audit log. Reuse the session panel.
- Run list with operator retry and cancel.
- MCP tools: list my tasks, complete a step, show a run.

## Acceptance

- The run list and inbox are row-gated, batched per page and pinned by a
budget test.
- E2E test completes a sample flow through the SPA.

## Why a run is waiting

Inspired by Waymark's run detail view. The run panel states why a run is not
moving: which role it waits for, which timer and when it fires, which job is in
flight, or which guard is false. The timeline is built from the audit provenance
(`flow`, `run`, `step`) and shows gaps between events. The MCP "show a run" tool
returns the same reason, so an agent knows whether to act, wait or stop.

## Decisions

- The timeline is kept in the run's state envelope, at most 32 entries
(the start and the newest moves), not read from the audit log.
- A step writes through the field-gated manager with the flow step on ctx
and then moves the run with the generation it saw. The change event still fires;
TKT-FWQZGAM skips writes that carry a flow step.
- Tasks match the current flows.yaml version only and stop at 500
candidates with `truncated: true`.
- The `flowview` values are the JSON wire shape for the API and MCP.
- Retry needs the manual-start permissions, like cancel.
- List paging never multiplies the caller's page and page size: a page
past the end is empty, however large (RR-5N62IL5).

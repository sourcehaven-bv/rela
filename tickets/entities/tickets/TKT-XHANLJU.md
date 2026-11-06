---
id: TKT-XHANLJU
type: ticket
title: 'Flows M1: run flows without channels'
kind: enhancement
priority: medium
effort: xl
started: "2026-11-06"
completed: "2026-11-06"
status: done
---

## Description

Run flows that are not tied to a channel.

- `flows.yaml` parser and load-time checks: references resolve, guards
compile, every state reachable, every waiting role can answer, outputs checked
against the ACL. Definition version is a content hash; a snapshot is stored at
first use.
- Reserve the `_` prefix for types, relations and properties (load error).
- Implicit, rela-managed type `_flow-run` (flow, version, status, active
state, started, completed, deadline) and relations `_carries` and `_produced`.
Only the engine principal writes it. Visibility follows the subject through a
`related(_carries, ...)` grant.
- Run store keyed by run id on file, pg and sqlite.
- Start triggers: on create, on predicate, manual (with permission), on
schedule; `per_subject: one | many`. Deleting the subject cancels the run.
- Events from step completion and entity changes.
- Provenance: audit entries carry `flow`, `run` and `step`; writes outside
any flow carry an implicit source.
- Retention: runtime state drops on completion; headers archive or delete
after a configured age.
- CLI: `rela flow validate | show | start | runs | cancel`; `show` renders
Mermaid.
- Waiting runs appear in `_inbox`.

## Acceptance

- A sample flow runs end to end on file, pg and sqlite.
- A `flowtest` conformance harness covers the run store on every backend.
- Run history is not required on fs.

## Split

M1 is too large for one reviewable PR. It ships as four successive PRs, each
passing CI on its own. This ticket covers the first; the acceptance above is met
when the last one lands.

1. TKT-XHANLJU (this ticket): `flows.yaml` parser and load-time checks,
version hash, reserved `_` prefix, `flow` limits, `rela flow validate` and `rela
flow show`, `rela validate` checks `flows.yaml`, docs.
2. TKT-PV9R0GK: run store on file, pg and sqlite, `flowtest` harness.
3. TKT-BC5G8RP: `_flow-run`, `_carries`, `_produced`, ACL, provenance.
4. TKT-9IT3LN8: runtime, CLI start/runs/cancel, `_inbox`, end-to-end
acceptance on all backends.

Acceptance for this ticket: the sample flow loads; every invalid construct is a
load problem with its key; a comment edit keeps the version; a schema name
starting with `_` does not load; `rela flow show` and `rela validate` report
flows.

## Archplan

`flowcfg.State` mirrors `realtimecfg.SessionState` on purpose: both compile
onto the shared `internal/flow` engine. The finding is accepted in
`.archplan/baseline.json`. Template variables carry the subject as an
`entity.Ref` and the run as a typed `flowcfg.RunID`, not raw id strings.

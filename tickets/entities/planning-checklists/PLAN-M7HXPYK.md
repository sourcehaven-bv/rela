---
id: PLAN-M7HXPYK
type: planning-checklist
title: 'Planning: Flows M1d: run flows end to end'
started: "2026-11-07"
status: done
---

<!-- @managed: claude-workflow v1 -->

## Understanding

- [x] Problem/requirements clearly understood
- [x] Scope defined (what's in/out documented below)
- [x] Acceptance criteria documented with specific test scenarios

**Scope:**

The ticket is too large for one PR, so it is split. This PR (part 1) is the
runtime core and the manual path:

- In: the flow engine (`internal/flowengine`): manual start, outputs
written as run_as with templates read through the run_as-gated reader, `done`
branches, final states, timers and pending-output recovery through `Engine.Due`,
cancel, runs on an older version through the stored definition, stranded runs
reported and skipped.
- In: wiring. `metamodel.WithFlowRuns` when flows.yaml declares flows,
the run store per backend (fs KV, sqlite table, pg table), an invalid flows.yaml
fails the boot.
- In: `rela flow start|runs|cancel`, gated by the manual permission and
the header's read gate.
- In: `rela db import-fs` copies unfinished runs.
- In: the carried-over findings listed on the ticket, fixed or decided.
- Out (follow-up ticket): create, predicate and schedule triggers;
`change` events; a flow worker that calls `Engine.Due`; subject delete cancels
the run and rename rekeys it; retention; `_inbox`; RR-LU55NIL.

**Acceptance Criteria:**

1. The sample flow starts on fs, sqlite and postgres: header as
system:flow, `_carries`, the `set` output on the subject and the `create` output
with `about` and `_produced` - `TestFlowEngine_SampleRunsOnEveryBackend` under
each build tag.
2. The sample runs end to end through its timers to a final state -
`TestStart_SampleFlowEndToEnd` (flowengine).
3. `rela flow start` and `cancel` need the manual permission; a hidden
subject answers not found; `rela flow runs` follows the header gate -
`TestFlowControl_GatesStartCancelAndRuns`.
4. `db import-fs` copies an unfinished run with its definition and
pending mark, and refuses when the target cannot take runs -
`TestRun_CopiesFlowRuns`, `TestRun_RefusesFlowRunsWithoutRunStore`.

## Research

- [x] ~~For larger features: run `/research` to create a structured research doc~~ (N/A: the design was settled in TKT-XHANLJU, TKT-PV9R0GK and TKT-BC5G8RP)
- [x] ~~Searched for existing libraries that solve this problem~~ (N/A: builds on the in-repo flow core, run store and entity manager)
- [x] Checked codebase for similar patterns or reusable code
- [x] ~~Looked for reference implementations in other projects~~ (N/A: the design tickets did this)
- [x] Reviewed relevant rela concepts for prior art

**Research Doc:** N/A

**Existing Solutions:**

- `internal/flow` (Decide, Enter, NextDue), `internal/flowcfg`
(compiled flows, templates, guards), `internal/flowrun` (run store per backend).
- `entitymanager.CreateLinked`, `PatchEntity` and `FieldGated` for the
writes; `scriptReads` for the gated reader; `audit.WithFlowStep`.
- `internal/appbuild/leases.go` for a per-backend store chosen at
wiring, and `cli/lease.go` for a nil-checked CLI bundle.

## Approach

- [x] Technical approach chosen and documented
- [x] Approach builds on existing patterns (not reinventing)
- [x] Alternatives considered (document why rejected)
- [x] Dependencies identified (packages, APIs, types)

**Technical Approach:**

- `flowengine.Engine` takes three narrow consumer-side interfaces
(Headers, Outputs, Subjects) that appbuild fills. Headers write as system:flow,
outputs as run_as through the field-gated manager, and templates read the
subject as run_as through the gated reader.
- A pending mark is stored with each move into a state with outputs
(RR-U30RNUZ) and makes the run due, so `Engine.Due` finishes the outputs after a
crash. A `create` output is claimed before it is written (at most once); a `set`
output is written and then marked (at least once, idempotent).
- A run on an older version compiles its own stored definition with
`flowcfg.ParseFlow` (RR-7E3BWVM). A definition that no longer compiles is
`ErrStranded`: Due logs and skips it, Cancel still works.
- `appbuild.FlowControl` is the access layer for the CLI: gated reads,
`HoldsPermissionForEntity` for the manual permission.

Alternatives rejected: running outputs inside the store transaction (no slow
work inside `Tx`); a per-row ACL callback for the run list (the header gate
already folds into the store query).

**Files to modify:**

`internal/flowengine/*` (new), `internal/flowrun/*` (pending mark),
`internal/flowcfg/{load,states}.go`, `internal/appbuild/{appbuild,piles,
flowruntime,flowcontrol,flowruns_*,configloader_sqlite, sqlitedata_sqlite}.go`,
`internal/cli/{flow,flow_runs,cli_wiring,kong, identity,db_importfs*}.go`,
`internal/fsimport/*`, `internal/realtime*` (limits), `.go-arch-lint.yml`, the
flows and CLI guides.

## Security Considerations

- [x] Input sources identified (user input, config, external APIs)
- [x] Input validation approach defined (allowlist preferred over blocklist)
- [x] Security-sensitive operations identified (file access, auth, crypto)
- [x] Error handling doesn't leak sensitive information

**Input Sources & Validation:**

- flows.yaml: operator config, compiled and refused at boot when invalid;
managed names (`_flow-run`, `_carries`, `_produced`) refused as subject, create
type or relate type.
- CLI arguments (flow name, subject address, run id): the flow must
exist, the subject is resolved through the gated reader, the run id through the
header gate.
- Stored run state and definitions: checked by `flowrun.CheckState` on
every write and decode; a definition that fails to compile strands the run
instead of crashing.

**Security-Sensitive Operations:**

- Header writes as system:flow: config-derived values only (RR-K4KY2I8).
- Output writes as run_as through the field-gated manager, so acl.yaml
must grant every write; `run_as` may not be a rela identity.
- Template reads as run_as through the gated reader (RR-JIX6EZR).
- Start and cancel need the manual permission for the subject.

### Threat model (POLICY-015 §3, PROCEDURE-j5mq step 1)

| STRIDE | Threat | Mitigation |
| ------ | ------ | ---------- |
| Spoofing | A caller starts or cancels a run as someone else, or a flow writes as system:flow | Identity is bound once at the CLI edge; FlowControl reads the bound Request and fails closed without one. Outputs run as `run_as`, which may not be a rela identity; only header writes use system:flow. |
| Tampering | A run writes data its run_as may not write, or a crash leaves outputs half done | Outputs go through the field-gated entity manager under the ACL. The pending mark is stored with the move and recovered by Due; `create` is claimed first so it is never written twice. |
| Repudiation | A flow write cannot be traced to its run | Every step write carries `flow`, `run`, `step` and `source: flow` in the audit log; headers are written as system:flow with tool `flow`. |
| Information disclosure | A template copies a hidden value into an output, or the run list reveals a hidden subject | Templates read the subject through the run_as-gated reader. `rela flow runs` lists headers through the visibility gate; a run is readable exactly when its subject is. A hidden subject or run answers not found. RR-EIRWL5J (copy into a wider audience) stays a documented author choice. |
| Denial of service | A stranded run blocks the timers loop, or many runs flood a round | A stranded run is logged and skipped; Due advances at most 64 runs per round; the store caps runs and page sizes. |
| Elevation of privilege | A caller without the permission starts a flow that writes as a stronger run_as | Start and cancel need the manual permission for the subject (`HoldsPermissionForEntity`); without acl.yaml the operator shell is trusted. A flow without a manual trigger cannot be started by hand. |

## Test Plan

- [x] Test scenarios documented for each acceptance criterion
- [x] Edge cases identified and documented
- [x] Negative test cases defined (invalid input, error conditions)
- [x] Integration test approach defined (not just unit tests)

**Test Scenarios:** see Acceptance Criteria; the appbuild test runs on every
backend build tag.

**Edge Cases:**

- A subject deleted between the start check and the header write.
- A failed output write, recovered by Due without a duplicate create.
- A run on a version that is no longer current; a stored definition that
no longer compiles.
- `per_subject: one` with an unfinished run.

**Negative Tests:**

- Unknown flow, missing or hidden subject, missing permission, run not
found, cancel twice, invalid flows.yaml at boot, import without a run store.

## Risk Assessment

- [x] Technical risks assessed with mitigations
- [x] Security risks assessed (see Security Considerations)
- [x] Effort estimated (xs/s/m/l/xl)

**Risks:**

- Nothing calls `Engine.Due` in a server yet, so timers and pending
recovery only run once the worker lands (follow-up). Documented in the guide.
- An invalid flows.yaml now fails the boot. Documented as an upgrade
note.

Effort: l (split into two PRs).

## Documentation Planning

- [x] User-facing docs identified (skip if internal refactor)
- [x] Docs-checklist will be created when entering implementation

**Documentation Impact:** GUIDE-flows (running flows, status, upgrade note),
GUIDE-cli-reference (`rela flow start|runs|cancel`).

## Design Review

- [x] ~~Run `/design-review` before starting implementation~~ (N/A: the design was reviewed in TKT-XHANLJU, TKT-PV9R0GK and TKT-BC5G8RP, whose findings this ticket carries; /code-review with both reviewers covers the implementation)
- [x] All critical/significant findings addressed in plan

**Design Review Findings:** carried over: RR-EMIUGI8, RR-XC2WBH9, RR-JIX6EZR,
RR-EIRWL5J, RR-U30RNUZ, RR-7E3BWVM, RR-DTSDBII, RR-R5I7UE0, RR-K4KY2I8,
RR-LU55NIL.

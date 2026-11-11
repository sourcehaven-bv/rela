---
id: PLAN-9PX91CT
type: planning-checklist
title: 'Planning: Flows M3: policy for edits outside a flow'
started: "2026-11-10"
completed: "2026-11-10"
status: done
---

<!-- @managed: claude-workflow v1 -->

## Understanding

- [x] Problem/requirements clearly understood
- [x] Scope defined (what's in/out documented below)
- [x] Acceptance criteria documented with specific test scenarios

**Scope:**

M2 lets people do a run's step through a form limited to the state's `writes:`.
Nothing stops anyone from editing the same subject through the ordinary entity
form, MCP or a scheduled script while the run is active. This ticket adds
`outside_edits:` to a flow: which subject properties stay writable outside the
flow while a run of the flow is unfinished on the subject.

```yaml
outside_edits:
  default: allow                 # allow | deny | {permission: P}
  properties:
    ernst: deny
    risico_betrokkenen: { permission: privacy.officer }
  states:
    melden-ap: { default: deny }   # also takes properties:
```

- In: `outside_edits:` in flows.yaml, compiled and checked at load.
- In: the rule for one (state, property): the state's property rule, else the
state's `default`, else the flow's property rule, else the flow's `default`,
else `allow`. A state entry therefore overrides the flow's rules for that state:
in `melden-ap` above `risico_betrokkenen` is denied even to a privacy officer.
- In: the restriction is a new dimension of the write side of the field
verdicts (`affordances.FieldVerdicts.Writable`), applied after the role union:
`writable = grants ∩ flow ceiling`. The flow ceiling for a row is the
intersection, over every run of the row held in the flow run store, of the
properties the run's (flow, state) rule allows the caller. The run store is
authoritative: it holds exactly the runs that have not ended (a run is deleted
when it ends), so the `_flow-run` header, which can lag after a failed header
patch, is not consulted. A row with no stored run of a flow that declares
`outside_edits:` is not narrowed.
- In: "run in state S" is answered by one run-store query per page:
`flowrun.Query` gains `SubjectIDs` (at most `MaxPage` ids), implemented by
kvflowrun (filter) and sqlflowrun (`subject_id IN (...)`, sqlite and postgres),
and pinned by the flowruntest conformance suite on every backend. This replaces
the ticket's `related()` wording: a `related()` query over run headers would
read the lagging header and, on fs and mem, scan every header.
- In: every surface that already enforces `fields:` grants enforces the flow
ceiling, because both read the same verdict: the data-entry PATCH/PUT
(`validateFieldWrite`), the field-gated manager (MCP on rela-server, scheduled
Lua, flow step completion, flow outputs, channel promote and channel outputs)
and the SPA's `_fields` (a restricted property renders read-only).
`_transitions` reports a transition of a property that is not writable as not
allowed.
- In: writes of a run's own flow are not outside that run. A dedicated
capability, `flowedits.WithinRun(ctx, runID)` (unexported ctx key), is set only
around the step's `grant.PatchEntity` in `FlowControl.complete` (which accepts
only the state's `writes:`, after the waiting-role check) and around the output
`PatchEntity` in `flowOutputs.Set` (fixed by the definition). A write carrying
run R is not narrowed by R; every other stored run on the subject still narrows
it. `audit.FlowStep` is not used for this: it spreads into the automations a
step triggers. An archguard test pins the callers of `WithinRun`.
- In: `outside_edits:` requires acl.yaml (load error otherwise): without it no
surface gates fields, so the policy would be silently unenforced. The check
reads the policy `compileFlows` passes, which is the file policy even when the
CLI or stdio MCP injects NopACL.
- In: `RELA_AFFORDANCE_PROFILE=none|demo` is refused when a flow declares
`outside_edits:`, as it is for `fields:` grants.
- In: a run-store lookup error fails closed: every declared property is
read-only, with attribution `outside_edits:unavailable`, so the audit row does
not read as a user's denial.
- In: a subject with a stored run keeps its id. The manager asks a
RenameGuard inside the rename's store Tx, and the flow guard refuses with
ErrRenameBlocked on every surface (the CLI included). A lookup error refuses.
- In: on postgres and sqlite the check and the write are ordered. A field
patch on a governed type asks the flow edit ceiling again inside its persist Tx,
reading the runs on the transaction's handle, and the engine's run Create and
Update write inside a store Tx on the transaction's handle, so the store's write
lock (pg advisory lock, sqlite write mutex) serializes them. fs has no such lock
and stays check-then-write (documented).
- Out (documented): the entity body, relations and delete are not narrowed;
`outside_edits:` governs properties. Surfaces that act for the operator or the
system and do not gate fields today are not narrowed either: the CLI, stdio MCP,
operator-declared `_actions`, automation cascades, `rela.bypass_acl` and CalDAV
client edits. Creates are not narrowed (a new row carries no run). The channel
policy service (`compileDeclarative`) and `NewFromCollaborators` get no ceiling.
Follow-up ticket for body/relations/delete.
- Out (documented): a merge conflict on a property a run restricts cannot be
resolved through the API while the run holds it (the conflict resolve runs the
same field check). Resolve it after the run moves on, or cancel the run.
- Out: a separate "permission to override outside_edits" bypass. `permission:`
rules are the way to let a role edit.
- Faces: flowcfg refuses a faced subject type, so every governed row is
faceless; runs are matched by subject id.

**Acceptance Criteria:**

1. Load checks - `TestParse_OutsideEdits` (flowcfg, table): `allow`, `deny`,
`{permission: P}` for `default` and per property; `states:` override; unknown
property, derived property, reserved property, unknown state, final state, bad
value, unknown key, empty permission are errors; a permission no role grants is
a warning; `outside_edits:` without acl.yaml is an error.
2. Rule precedence - `TestOutsideEdits_Rule` (flowcfg, table).
3. Ceiling evaluation - `TestCeiling_Withheld` (flowedits, table): allow,
deny, permission held and not held, per-state override, two overlapping runs of
different flows intersect, two runs of one flow (`per_subject: many`) in
different states intersect, a run's own writes are exempt from that run only, a
stranded state or a final state falls back to the flow-level rules, a run of a
flow that no longer exists restricts nothing, a `WithinRun` ctx set for another
run exempts nothing.
4. Run lookup - `TestCeiling_ActiveRuns` (flowedits, kvflowrun): runs on
other subjects do not count; more than `MaxPage` ids are split into batches;
more than `MaxPage` runs are paged with `After`; one List call per batch of ids.
5. "Run in state S" on every backend - flowruntest `SubjectIDs` subtests
(several ids, ids with no runs, combined with `Flow`, paging with `After`,
`CheckQuery` rejects more than `MaxPage` ids or a bad id) run on kv, sqlite and
postgres through the existing conformance harness.
6. Field verdicts - `TestFieldVerdicts_FlowEdits` (affordances): the
ceiling only removes (a property a `fields:` grant denies stays denied; an
`allow` rule does not open it); `Visible` and `CreateFieldVerdicts` are
untouched; a lookup error makes every property read-only with attribution
`outside_edits:unavailable`; no bound identity still fails closed; attribution
names the flow and state (audit only); the redactor's visible read does not look
up runs.
7. Writes end to end - `TestOutsideEdits_*` (appbuild, real services on
memstore with flows.yaml and acl.yaml): a field-gated PatchEntity of a denied
property is refused while the run is in the state and accepted after it moves on
or finishes; a permission holder may write a `permission:` property; the step
completion writes a denied property of its own run; a second flow's `deny`
refuses the first flow's step write; a flow output writes a property its own
`outside_edits` denies.
8. HTTP - `TestOutsideEdits_DataEntry` (dataentry): PATCH answers 403
`field-affordance:read-only:<property>`; GET `_fields` marks the property not
writable; `_transitions` reports a transition of a restricted property not
allowed; `RELA_AFFORDANCE_PROFILE=none` is refused when a flow declares
`outside_edits:`; a view section with editable rows of N governed rows makes the
same number of run-store List calls for N=5 and N=25 (budget).

## Research

- [x] ~~For larger features: run `/research` to create a structured research doc~~ (N/A: the shape is settled in FEAT-J0O10QI and the ticket; this is one more allowlist dimension on an existing verdict)
- [x] ~~Searched for existing libraries that solve this problem~~ (N/A: builds on the in-repo field verdicts and store graph queries)
- [x] Checked codebase for similar patterns or reusable code
- [x] Looked for reference implementations in other projects
- [x] Reviewed relevant rela concepts for prior art

**Research Doc:** N/A

**Existing Solutions:**

- Client field ceiling (TKT-IAC8TX): `affordances.applyClientCeiling`
removes fields from a dimension after the role union (`dimension.restrictTo` /
`redact`). The flow ceiling is the same operation on the writable dimension.
- `affordances.PrimeTraversals` + ctx memo: the pattern for answering a
per-row graph question once per page.
- `acl.flowRunReadQuery` and `storetest` `Any_HasOutbound_branches`: the run
is matched from its own side through `HasOutbound _carries`, on every backend.
- `acl.Request.HoldsPermissionForEntity`: permission on one subject, global
and conferred roles, with the client ceiling.
- `flowcfg` `writes:` checks (declared, not derived, not reserved).
- Camunda/Flowable "task-scoped edits" and ServiceNow "read-only while in
approval" UI policies: both lock fields while a process is in a state.

## Approach

- [x] Technical approach chosen and documented
- [x] Approach builds on existing patterns (not reinventing)
- [x] Alternatives considered (document why rejected)
- [x] Dependencies identified (packages, APIs, types)

**Technical Approach:**

- `flowcfg`: `rawFlow.OutsideEdits` (yaml node); `OutsideEdits` compiled
type with `Rule(state, property) EditRule`; `EditRule{Kind allow|deny|
permission, Permission string}`; `Flow.OutsideEdits()` (nil when absent). The
block is part of the flow's declaration, so it is in the version hash. At most
64 property entries per level.
- `flowrun`: `Query.SubjectIDs []string` (at most `MaxPage`, each checked like
`SubjectID`); kvflowrun filters on it, sqlflowrun adds an `IN` clause;
flowruntest conformance subtests.
- New package `internal/flowedits` (deps: affordances, entity, flowcfg,
flowrun, metamodel): `New(flows, runs)` returns nil when no flow declares
`outside_edits:`. It implements the consumer-side `affordances.FlowEditCeiling`:
  - `Governs(entityType) bool` - some flow on the type declares it.
  - `ActiveRuns(ctx, ids)` - `flowrun.Store.List` with `SubjectIDs`, in
batches of `MaxPage` ids, paged with `After`. Raw store read: an authorization
input, and a run the caller cannot read must still restrict.
  - `Withheld(ctx, e, runs, holds)` - per run of a flow with `outside_edits:`
(current flows.yaml, by flow name), except the run `WithinRun` names: each
declared, non-derived property whose rule is `deny`, or `permission: P` with
`holds(P)` false, is withheld with attribution `outside_edits:flow=F state=S`.
  - `WithinRun(ctx, runID)` and the unexported reader.
- `affordances`: `WithFlowEdits(FlowEditCeiling)` option. In `fieldVerdicts`,
for the update verb on a live (non-historical) row of a governed type: get the
runs (primed memo, or one live lookup), `holds =
req.HoldsPermissionForEntity(ctx, e.ID, P)` memoized per call, and remove the
withheld properties from the writable dimension with a new `dimension.readOnly`.
A lookup error removes every declared property with attribution
`outside_edits:unavailable`. Package functions, not methods, because
`PolicyResolver` is at its plimsoll cap:
  - `PrimeFlowRuns(ctx, r, rows)` primes the runs of a page in one lookup.
  - `VisibleFields(ctx, r, e)` returns the visible map without the flow
ceiling. `visibility.PolicyRedactor` and the data-entry `hiddenProperties` use
it, so reads that only redact do not look up runs.
- `dataentry`: `PrimeFlowRuns` is called only where per-row `_fields` are
built (view sections and editable table rows), not from
`policyResolver.PrimeTraversals`. `ResolverFromProfile` and
`ResolverFromServices` take the ceiling: the policy resolver is built when
acl.yaml has affordance grants or the ceiling is non-nil, and the profile
override is refused in both cases. `computeTransitions` reports a transition of
a non-writable property as not allowed.
- `appbuild`: the flow run store is created before
`resolveACLAndFieldPolicy`, so the ceiling has its run reader.
`buildFieldPolicy` takes the ceiling: with field grants the redactor and the
write gate use the policy resolver as today; with only the ceiling the write
gate uses the policy resolver and the redactor stays `NopRedactor`. Package
function `FlowEdits(s)` for the cmd entry points (Services is at its plimsoll
cap). `FlowControl.complete` and `flowOutputs.Set` wrap their `PatchEntity` ctx
in `flowedits.WithinRun`.
- `archguard`: pin non-test callers of `flowedits.WithinRun` to the two
appbuild sites.
- `.go-arch-lint.yml`: add `flowedits`.

Alternatives rejected:

- Lowering the ceiling into each role's `fields:` `when:` as
`not related(...)`: it is a runtime deny inside the grant evaluator, and
`fields:` grants are a union, so a negated condition on one role is undone by
another role. The ceiling must be an intersection applied after the union.
- Reading "unfinished" from the `_flow-run` header with a `related()`
GraphQuery: the header can lag the run store after a failed header patch (a
denied edit would pass), and on fs and mem the query reads every header ever
written. The run store is authoritative and already indexed by subject.
- Exempting step writes through `audit.FlowStep`: it is provenance with
several setters and spreads into the automations a step triggers. A dedicated
capability set at two sites is narrower and reviewable.
- Using each run's own definition version for the policy: needs a per-version
compile on the field-verdict path. Policy follows the current flows.yaml, like
acl.yaml does.

**Files to modify:**

`internal/flowcfg/{raw,compiled,load,outside_edits}.go` (+ tests),
`internal/flowrun/{flowrun.go,kvflowrun,sqlflowrun,flowruntest}`,
`internal/flowedits/*` (new), `internal/affordances/{resolver,dimension,
flowedits}.go` (+ tests), `internal/visibility/adapters.go`,
`internal/dataentry/{affordances,affordances_policy,affordances_stub,
sections}.go` (+ tests), `internal/appbuild/{appbuild,piles,flowruntime,
flowsteps}.go` (+ tests), `internal/archguard/*`, `cmd/rela-server/serve.go`,
`cmd/rela-desktop/main.go`, `.go-arch-lint.yml`, GUIDE-flows.

## Security Considerations

- [x] Input sources identified (user input, config, external APIs)
- [x] Input validation approach defined (allowlist preferred over blocklist)
- [x] Security-sensitive operations identified (file access, auth, crypto)
- [x] Error handling doesn't leak sensitive information

**Input Sources & Validation:**

- flows.yaml `outside_edits:`: strict decode, rule values from a closed set,
properties declared on the subject and not derived or reserved, states declared
and not final, permission non-empty; refused without acl.yaml.
- Runs in the flow run store (flow, subject, state): read raw; an unknown
flow is ignored, an unknown or final state falls back to the flow rules.
- ctx `flowedits.WithinRun`: set only by Go code at two sites (archguard
pin); no transport or Lua binding sets it.

**Security-Sensitive Operations:**

- The property write gate. The ceiling only removes from the writable set;
the compile step has direct unit tests.

### Threat model (POLICY-015 §3, PROCEDURE-j5mq step 1)

| STRIDE | Threat | Mitigation |
| ------ | ------ | ---------- |
| Spoofing | A caller makes an outside edit look like a step of the run, to escape the ceiling | The exemption reads `flowedits.WithinRun` from ctx, an unexported key. Only two Go sites set it: `FlowControl.complete` (after the waiting-role check, with only the state's `writes:`) and `flowOutputs.Set` (values fixed by the definition). No HTTP, MCP or Lua input reaches it; an archguard test pins the callers. It exempts only from the named run; every other run on the subject still narrows. |
| Tampering | A person changes a property the flow owns while a run is in a state that denies it | The field verdict removes the property from the writable set on every field-gated surface (data-entry PATCH/PUT, MCP on rela-server, scheduled Lua, flow steps and outputs, channel promote and outputs). The SPA shows it read-only. The run store is the source, so a lagging header does not open a property. A rename of a subject with a stored run is refused inside the rename's Tx, so renaming does not lift the rules; `TestOutsideEdits_RenameRefusedWhileHeld` pins it. Surfaces that do not gate fields (CLI, stdio MCP, operator actions, cascades, bypass_acl, declarative webhooks, conversions, CalDAV) are not narrowed, as documented. On postgres and sqlite the flow part of the field check is asked again inside the write Tx and run writes take the same lock, so a run that starts between the first check and the write is seen. This covers the field-gated manager and, in the data-entry app, PATCH, attachment upload and history restore, which write through the update grant or the attachment owner with the recheck (`TestOutsideEdits_RunStartedDuringPatchFailsClosed`, `TestOutsideEdits_RunStartedDuringHTTPPatchFailsClosed`, `TestOutsideEdits_RunWriteTakesWriteLock`). Batch creates only, so it has no run to recheck. Residual, accepted (TKT-LU8PSKJ): on the fs and memory backends run writes do not take the store's lock, so a run can commit between the recheck and the write, and data-entry conflict resolution (fs only) writes the file directly. |
| Repudiation | A refused outside edit leaves no trace | The refusal is a field-gate denial: the manager records a `denied-write` audit row with attribution `outside_edits:flow=F state=S` (or `outside_edits:unavailable` on a lookup error); the data-entry PATCH records its denial the same way. |
| Information disclosure | The read-only verdict reveals a run's state to a caller who cannot read the run | The verdict is one bit per property, the same channel a `fields:` grant with `related()` has. The wire carries only "not writable"; the flow and state appear in the audit attribution only. Config (flow, state, property names) is not secret. |
| Denial of service | Field verdicts on a page query runs per row; a policy locks a subject for ever | Runs are looked up once per page (`PrimeFlowRuns`, one List per 500 ids), only where `_fields` are built; a nested section primes its whole tree once; redaction (list pages) does not look them up. Budget tests pin one lookup per section and zero per list page. Only stored runs count, and a run is deleted when it ends; cancelling the run or editing flows.yaml lifts the restriction. Per-level entries are capped at 64. |
| Elevation of privilege | The ceiling widens access, a lookup error opens fields, or an env var switches it off | The ceiling is applied after the role union and only removes; an `allow` rule cannot open a property a grant closed. A lookup error makes every property read-only. No identity still denies every field. `RELA_AFFORDANCE_PROFILE=none|demo` is refused when a flow declares `outside_edits:`. |

## Test Plan

- [x] Test scenarios documented for each acceptance criterion
- [x] Edge cases identified and documented
- [x] Negative test cases defined (invalid input, error conditions)
- [x] Integration test approach defined (not just unit tests)

**Test Scenarios:** see Acceptance Criteria.

**Edge Cases:**

- Two flows on one subject, both active, rules differ (intersection).
- `per_subject: many`: two runs of one flow in different states.
- A run in a final state, settling its outputs (flow-level rules).
- A run on a state the current flows.yaml no longer declares.
- More than 500 subject ids or runs in one lookup (batched and paged).
- A run of a flow removed from flows.yaml.
- A property removed with `unset` (judged like a set).
- A run that moves between page load and PATCH (PATCH judges live).

**Negative Tests:**

- Denied property refused; permission not held refused; lookup error makes
all properties read-only; no identity; invalid flows.yaml blocks refused.

## Risk Assessment

- [x] Technical risks assessed with mitigations
- [x] Security risks assessed (see Security Considerations)
- [x] Effort estimated (xs/s/m/l/xl)

**Risks:**

- Body, relations and delete are not narrowed; documented, follow-up ticket.
- A flow whose own outputs write a property another active flow denies: the
output fails and the run backs off. Documented as the intersection rule.

Effort: l.

## Documentation Planning

- [x] User-facing docs identified (skip if internal refactor)
- [x] Docs-checklist will be created when entering implementation

**Documentation Impact:** GUIDE-flows (new "Edits outside a flow" section,
`outside_edits` in the flow keys table, checks).

## Design Review

- [x] Run `/design-review` before starting implementation
- [x] All critical/significant findings addressed in plan

**Design Review Findings:** 15 findings (1 critical, 6 significant, 7 minor, 1
nit), recorded as review-responses on TKT-XI7IDBS and folded into this plan: the
profile override is refused under `outside_edits:`; the run store, not the
header, answers "run in state S"; a dedicated `WithinRun` capability replaces
`audit.FlowStep` for the exemption; CalDAV and channel writes are in the scope
list; priming happens only where `_fields` are built; the redactor stays Nop
without field grants; lookup errors carry their own attribution; `_transitions`
follows the ceiling; conflict resolution and faces are documented.

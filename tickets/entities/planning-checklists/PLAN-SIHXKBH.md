---
id: PLAN-SIHXKBH
type: planning-checklist
title: 'Planning: Flows M1c: _flow-run type, relations, ACL and provenance'
started: "2026-11-07"
status: done
---

<!-- @managed: claude-workflow v1 -->

## Understanding

- [x] Problem/requirements clearly understood
- [x] Scope defined (what's in/out documented below)
- [x] Acceptance criteria documented with specific test scenarios

**Scope:**

Third of the four M1 PRs (see TKT-XHANLJU, Split). In scope:

- The rela-managed type `_flow-run` (properties `flow`, `version`,
  `status`, `state`, `started`, `completed`, `deadline`) and the relations
  `_carries` (run to subject; one by convention, no cardinality bound,
  because a create and its edge are two writes) and `_produced` (run to an
  entity a step created). The names live in `internal/entity`, so `acl`
  can name them without importing `metamodel`.
- `metamodel.WithFlowRuns(m)`: returns a copy of a validated schema with
  the three definitions added, and validates the copy again. `_carries`
  and `_produced` target every faceless entity type, because flows refuse
  faced subjects and faced create types.
- ACL, compiled at load:
  - `Policy.Validate` refuses an operator grant, key, role name or
    assignment that names a reserved (`_`) type, relation or role, or that
    assigns the engine principal.
  - The `*` wildcard covers operator types only, never a reserved name,
    at every type-matching site in `internal/acl`.
  - `ValidateAgainstMetamodel`, when the schema carries `_flow-run`, adds
    the role `_flow-engine` (create, update and delete `_flow-run`; read
    `*` and `_flow-run`) and assigns it to the new engine principal
    `system:flow` (`principal.UserFlow`, a rela identity no caller may
    assert).
  - Read gate: for a principal without a global grant,
    `readQuery("_flow-run")` is a disjunction with one branch per faceless
    subject type the principal may read: `HasOutbound(_carries)` into an
    endpoint that carries that type's row gate (the `gateHop` fold that
    `related(...)` already uses). No readable subject type gives DenyAll.
    A run whose subject the reader cannot see is not found (uniform 404).
  - `store.GraphBranch` gains `HasOutbound`; the naive, pg and sqlite
    backends render it; storetest pins it on every backend.
- Write-path integrity in `entitymanager`: a write to `_flow-run`,
  `_carries` or `_produced` is accepted only from `system:flow`, also
  under `bypass_acl` and with no acl.yaml (NopACL). Renaming or converting
  a `_flow-run` entity is refused for everyone (RR-SVDODML, carried from
  TKT-PV9R0GK).
- Provenance: `audit.WithFlowStep(ctx, FlowStep{Flow, Run, Step})`.
  Entity and relation write records made under it carry `flow`, `run` and
  `step`. Every record carries a derived `source`: `flow` when a flow step
  is set, else the kind of `triggered_by` (`automation`, `schedule`,
  `cascade`, ...), else the principal's tool (`cli`, `mcp`, `data-entry`,
  ...).
- Docs: flows guide (runs, visibility, writes, provenance), audit-log
  reference.

Out of scope (TKT-9IT3LN8): calling `WithFlowRuns` from the wiring when
flows.yaml declares flows, the runtime that writes runs, subject delete,
`_inbox`, CLI start, runs and cancel. Issue #2106 was already fixed by
#2124 (BUG-GSCDO88) on the flows side. This PR closes the same alias gap
in acl.yaml: a `visible:` or `redact:` key that names a type through an
alias now fails to load, for every role and baseline, instead of
restricting nothing.

**Acceptance Criteria:**

1. `WithFlowRuns` adds the type and both relations; a schema declaring a
   `_` name still fails to load. Test: `TestWithFlowRuns`.
2. acl.yaml naming `_flow-run`, `_carries`, a `_` role or `system:flow`
   fails to load. Test: `TestValidate_ReservedNames`.
3. A role with `create`, `update`, `delete` or `read: ["*"]` gets no grant
   on `_flow-run`; the engine principal does. Test:
   `TestFlowRunWrite_OnlyEngine`.
4. A reader never sees a run whose subject it cannot see. A reader sees
   the runs of every subject it reads through a global grant or a plain
   role relation; a subject reachable only through `inherit_roles_through`
   or face-split grants drops its branch (fails closed, follow-up ticket).
   A run with no `_carries` edge, or whose subject was deleted, is hidden. Tests:
   `TestFlowRunReadQuery` (acl), storetest branch conformance, a
   visibility integration test.
5. With NopACL and with `bypass_acl`, a non-engine write to a managed type
   is refused; rename and convert of a run are refused for everyone. Test:
   `TestManagedTypeGuard`.
6. Records written under `WithFlowStep` carry flow, run and step; others
   carry the derived source. Tests: `TestRecordSource`,
   `TestFlowProvenance`.

## Research

- [x] ~~For larger features: run `/research` to create a structured research doc~~ (N/A: settled by the product owner in FEAT-J0O10QI.)
- [x] Searched for existing libraries that solve this problem
- [x] Checked codebase for similar patterns or reusable code
- [x] Looked for reference implementations in other projects
- [x] Reviewed relevant rela concepts for prior art

**Research Doc:** N/A.

**Existing Solutions:**

- Row gate through a relation: `acl.Request.gateHop` and `lowerTraversal`
  (`internal/acl/traversal.go`) already fold a type's read query into an
  endpoint for `related(...)`; the run gate reuses them.
- Per-branch disjunction: `store.GraphQuery.Any` (role relations with
  different faces).
- System identities: the `principal.UserProvisioner` pattern (fixed
  identity, explicit role); `IsRelaIdentity` refuses assertion.
- Audit attribution: `audit.WithTriggeredBy`.
- Camunda and Temporal keep process instances outside business data and
  attribute writes to the engine. rela keeps the run header in the graph,
  so it is gated like any row.

## Approach

- [x] Technical approach chosen and documented
- [x] Approach builds on existing patterns (not reinventing)
- [x] Alternatives considered (document why rejected)
- [x] Dependencies identified (packages, APIs, types)

**Technical Approach:** as in Scope.

Alternatives rejected:

- Inject `_flow-run` into every schema at load: changes the API schema,
  the MCP instructions and the shape hash of every project, for a type
  only flow projects use.
- Allow engine writes by identity in the evaluator: CLAUDE.md forbids
  inferring allow from identity; an explicit compiled role is reviewable.
- `_carries` pointing from subject to run, to reuse
  `GraphBranch.HasInbound`: reverses the settled meaning of the relation.
- Row gate by a Go post-filter over run pages: per-row lookups on a list
  path and pre-filter counts (collection-reads rule).
- Expand `*` into explicit type lists at load: changes how ceilings,
  aclaudit and docs read every policy.

**Files to modify:** `internal/entity/managed.go` (new),
`internal/metamodel/{reserved.go,flowrun.go}`, `internal/principal`,
`internal/acl` (policy, readquery, write grants, world grants, ceiling
compile, traversal, access, new managed.go),
`internal/store/graphquery.go`, `internal/store/graphquerynaive`,
`internal/store/pgstore/graphquery.go`,
`internal/store/sqlitestore/graphsql.go`, `internal/store/storetest`,
`internal/entitymanager` (guard, rename and convert, audit stamping),
`internal/audit`, docs-project guides.

## Security Considerations

- [x] Input sources identified (user input, config, external APIs)
- [x] Input validation approach defined (allowlist preferred over blocklist)
- [x] Security-sensitive operations identified (file access, auth, crypto)
- [x] Error handling doesn't leak sensitive information

**Input Sources & Validation:**

- acl.yaml (operator config): reserved names refused at load.
- Write requests from the SPA, API, MCP, CLI and Lua: the target type
  comes from the request; the allowlist decides and the write-path guard
  backs it.
- The principal: `system:flow` is a rela identity; request edges and
  `--run-as` already refuse every `system:` name or rela identity.

**Security-Sensitive Operations:** row gating of runs (existence is
secret), writes to rela-managed rows, audit attribution.

### Threat model (POLICY-015 §3, PROCEDURE-j5mq step 1)

Assets: entity values and existence (secret); integrity of run headers
and of the audit trail. Config is not secret.

Actors: a person with a role (SPA, API, MCP, CLI); the operator (acl.yaml,
scripts with `bypass_acl`); the flow engine (`system:flow`).

| Threat (STRIDE) | Vector | Mitigation |
| --- | --- | --- |
| Information disclosure | A run reveals that a hidden subject exists, or its flow and state | The run read gate is the subject's row gate through `_carries`; no readable subject means not found |
| Information disclosure | Run counts or search hits leak hidden runs | The gate is a store predicate, so lists, counts and search apply it before paging |
| Information disclosure | Gate evaluated at a face other than the reader's world | Subject types are faceless; `_carries` targets only faceless types |
| Tampering | A user creates, edits or deletes a run or its edges | Allowlist: `*` excludes reserved names and explicit grants are refused; only `_flow-engine` holds them. The entitymanager guard covers NopACL and `bypass_acl` |
| Tampering | A rename orphans a run's stored state | Rename and convert of `_flow-run` refused |
| Spoofing | A caller asserts `system:flow` to borrow the engine role | Listed as a rela identity: `--run-as`, `run_as` and request edges refuse it |
| Elevation of privilege | Operator assigns `system:flow` or declares `_flow-engine` with wider grants | Refused at load |
| Repudiation | A flow write looks like a person's | Records carry flow, run, step and `source`; the principal stays the genuine identity |

Residual: a reader under a client ceiling with a type allowlist cannot see
runs (fails closed). The runtime (TKT-9IT3LN8) must write run headers only
as `system:flow` and outputs as `run_as`.

## Test Plan

- [x] Test scenarios documented for each acceptance criterion
- [x] Edge cases identified and documented
- [x] Negative test cases defined (invalid input, error conditions)
- [x] Integration test approach defined (not just unit tests)

**Test Scenarios:** see the acceptance criteria.

**Edge Cases:** a schema with only faced types (no `_carries` targets: the
type still loads and every reader gets DenyAll); a subject read through a
role relation; a subject readable only on named faces (branch dropped); a
run with no `_carries` edge (hidden from all but the engine);
`WithFlowRuns` on a schema that already has the type (error); a policy
without roles.

**Negative Tests:** user create, update and delete of a run (403 or not
found); a user relation write on `_carries`; rename; convert; acl.yaml
with reserved names; assertion of `system:flow`.

Integration: visibility reader over memstore with a seeded graph; storetest
branch conformance on fs, memory, sqlite and pg.

## Risk Assessment

- [x] Technical risks assessed with mitigations
- [x] Security risks assessed (see Security Considerations)
- [x] Effort estimated (xs/s/m/l/xl)

**Risks:**

- A missed `*` match site grants users a run write. Mitigation: one
  helper, a guard test scanning `internal/acl` for bare `"*"`
  comparisons, and the entitymanager guard as a second layer.
- A backend ignores the new branch field. Mitigation: storetest on every
  backend.
- Nothing in production calls `WithFlowRuns` until TKT-9IT3LN8. Recorded
  on that ticket.

Effort: l.

## Documentation Planning

- [x] User-facing docs identified (skip if internal refactor)
- [x] Docs-checklist will be created when entering implementation

**Documentation Impact:**

- [x] docs/flows.md: runs, visibility, writes, provenance
- [x] docs/audit-log.md: `flow`, `run`, `step`, `source`
- [x] docs/server-security.md: reserved names in acl.yaml, `system:flow`

## Design Review

- [x] Run `/design-review` before starting implementation
- [x] All critical/significant findings addressed in plan

**Design Review Findings:** 15 findings, all addressed in the plan:
RR-YHDXSPL, RR-1QKRQPB (critical); RR-ZFWRLIS, RR-BLECTJJ, RR-U0T5WZ9,
RR-K4KY2I8, RR-1SYZD5J, RR-TR9PTMP, RR-ELB2WNT, RR-M6QBRC1 (significant);
RR-E32LDUP, RR-2U69WDN, RR-LU55NIL, RR-20DH1GB (minor); RR-5KG3MHB (nit).

Changes to the approach from the review:

- `_carries`/`_produced` targets are computed from operator types before
  the managed type is added; readQuery skips a reserved subject type.
- `principal.ValidateRunAs` refuses `system:flow` for every caller.
- An edge of a managed relation removed because its far endpoint is
  deleted is not authorized as a relation write of the deleting
  principal. Converting an entity with a managed edge is not refused:
  the edge is kept when the target type is still a valid endpoint of the
  relation, and dropped otherwise (`planManagedRelation`).
- Type-less automation triggers and validation rules skip rela-managed
  types and relations.
- The run header holds config-derived values only.
- For `_flow-run` the type-level ceiling check is skipped; each branch's
  subject read query applies the ceiling.
- `WithFlowRuns` clones the maps it writes and sets a private flag;
  `validateReservedNames` exempts exactly those definitions. The id
  prefix `RUN-` is checked against operator prefixes.
- The engine role is injected in normalized form, idempotently.
- The importer and data migration steps refuse rela-managed types.
- Every branch site handles `HasOutbound`; storetest pins it.
- When every target is fully readable the gate is one `HasOutbound`.
- `source` comes from one closed mapping with a table test.

---
id: PLAN-18YFYCT
type: planning-checklist
title: 'Planning: Flows M1: run flows without channels'
started: "2026-11-06"
status: done
---

<!-- @managed: claude-workflow v1 -->

## Understanding

- [x] Problem/requirements clearly understood
- [x] Scope defined (what's in/out documented below)
- [x] Acceptance criteria documented with specific test scenarios

**Scope:**

M1 is too large for one reviewable PR. It ships as four successive PRs, each
passing CI on its own. This ticket is the first; the rest have their own
tickets (see the ticket's Split section).

In scope for this ticket (PR 1, definitions):

- `internal/flowcfg`: parse `flows.yaml` strictly and compile it against the
  schema and acl.yaml. Load-time checks: references resolve, guards compile
  against an allowlist, every state is reachable, every waiting role can
  answer, outputs are checked against the `run_as` grants. Version is a
  content hash of the definition with comments stripped.
- Reserve the `_` prefix for entity types, aliases, properties, relation
  types, inverses and relation properties (schema load error).
- `internal/flow` owns its limits (`ReminderSlack`, state/timer caps)
  instead of `realtimelimits`.
- CLI `rela flow validate` and `rela flow show [name]` (Mermaid);
  `rela validate` checks `flows.yaml`; sqlite carries `flows.yaml` as config;
  conflict listing names it.
- Docs: new Flows guide, CLI reference, metamodel reserved names.

Out of scope here (later M1 PRs): run store and backends, `_flow-run` type
with `_carries`/`_produced` and its ACL, audit provenance, the runtime
(triggers, events, timers, outputs, retention, subject delete),
`rela flow start | runs | cancel`, `_inbox`. Out of M1 entirely: step forms
(M2), outside-edit policy (M3), parallel/job steps (M4), compiling
transitions/automations (M5).

**Acceptance Criteria:**

1. The sample onboarding flow loads. Test: `TestParse_Sample`.
2. Each invalid construct is a load problem with its key path. Test:
   `TestParse_Problems` (about 40 cases).
3. A comment edit keeps the version; a definition edit changes it. Test:
   `TestParse_Version`.
4. A schema name starting with `_` does not load. Test:
   `TestParse_ReservedPrefix`.
5. `rela flow show` lists and renders; `rela validate` reports flows.yaml.
   Tests: `TestFlowShow_Write`, `TestValidate_Flows`.

## Research

- [x] ~~For larger features: run `/research`~~ (N/A: FEAT-J0O10QI and its
  design notes settle the approach; the ticket lists the settled decisions.)
- [x] Searched for existing libraries that solve this problem
- [x] Checked codebase for similar patterns or reusable code
- [x] Looked for reference implementations in other projects
- [x] Reviewed relevant rela concepts for prior art

**Research Doc:** N/A (design settled in FEAT-J0O10QI).

**Existing Solutions:**

- `internal/realtimecfg` is the model: strict YAML, problems with key
  paths, guard allowlist, `acl.JobIdentityGrants` for `run_as` outputs.
- `internal/flow` (M0) supplies `Definition`, `Timer`, `Period`, `Status`.
- `internal/mermaid.StateDiagram` renders the diagram.
- No external library: a state-machine library would not know the schema
  or the ACL, which is where the checks live.

## Approach

- [x] Technical approach chosen and documented
- [x] Approach builds on existing patterns (not reinventing)
- [x] Alternatives considered (document why rejected)
- [x] Dependencies identified (packages, APIs, types)

**Technical Approach:**

`flowcfg.Parse(data, meta, policy)` decodes into raw structs with
`KnownFields`, then a loader compiles each flow: subject, per_subject,
deadline, retention, states (status defaults, waiting_for roles, branches
per event, timers, outputs), starts, run_as. Whole-flow checks run after:
reachability, waiting roles answerable, grants. The result is immutable
(`Compiled`, `Flow`, `State` with accessors) and exposes a
`flow.Definition` for the engine. `appbuild.LoadFlows` reads through the
config reader. The CLI `rela flow` group keeps the old script runner as its
default subcommand so `rela flow x.lua` and shebangs keep working.

Alternatives rejected: putting flows in `realtime.yaml` (flows are not
channel-bound); `flow` as a new top-level command name (taken by the Lua
runner; a group with a default keeps both).

**Files to modify:** `internal/flowcfg/*` (new), `internal/flow/limits.go`,
`internal/flow/timers.go`, `internal/realtimelimits/limits.go`,
`internal/metamodel/{reserved.go,loader.go}`, `internal/appbuild/flows.go`,
`internal/appbuild/projectfiles_sqlite.go`, `internal/cli/{flow,flows,
validate,validate_flows,kong,identity}.go`, `internal/dataentry/conflict_gate.go`,
`.go-arch-lint.yml`, docs-project guides.

## Security Considerations

- [x] Input sources identified (user input, config, external APIs)
- [x] Input validation approach defined (allowlist preferred over blocklist)
- [x] Security-sensitive operations identified (file access, auth, crypto)
- [x] Error handling doesn't leak sensitive information

**Input Sources & Validation:**

- `flows.yaml` (operator-authored config, not secret). Strict decode
  rejects unknown keys; size cap 1 MiB; caps on flows, states, starts,
  branches, outputs, timers; names match `^[a-z][a-z0-9_-]{0,63}$`.
- Guards (`when:`): compiled with the predicate engine and held to an
  allowlist (subject only as `entity.<prop>`, no `related`, functions
  `contains`, `date_add`, `days_between`, `len`, `today`, no property a
  `visible:` grant restricts). Invalid input is a load problem.
- Templates: only `subject.id`, `run.id`, `now`, `today`,
  `subject.<prop>`; unknown references and restricted properties refused.

**Security-Sensitive Operations:** none at runtime in this PR. The file
only loads and is shown. `run_as` must be a valid job identity and every
output write must be granted to it in acl.yaml, checked at load.

### Threat model (POLICY-015 §3, PROCEDURE-j5mq step 1)

Assets: entity values (secret), entity existence (secret). Config is not
secret.

Actors: operator (writes flows.yaml, trusted as config author); person
with a role (later: causes `change` events); job identity (`run_as`).

| Threat (STRIDE) | Vector | Mitigation in this PR |
| --- | --- | --- |
| Information disclosure | A guard over a hidden property makes a run's state an oracle for the value | Guards may not read a property any `visible:` grant restricts; no `related`; function allowlist |
| Information disclosure | A template copies a hidden subject property into a created entity readers can see | Templates may not reference a restricted property |
| Elevation of privilege | Outputs write as `run_as` beyond what acl.yaml grants it | `JobIdentityGrants` checks verb, field write, enum option and relation per output at load; `run_as` must pass `principal.ValidJobIdentity` |
| Elevation of privilege | Schema declares `_flow-run` or `_carries` to collide with rela's own types | `_` prefix reserved; schema fails to load |
| Denial of service | Huge or deeply branching flows.yaml slows boot | Byte cap and per-dimension caps; linear checks |
| Tampering | Changed definition applied to running runs silently | Version is a content hash; runs pin it (later PR) |
| Spoofing / repudiation | n/a in this PR | No writes; provenance lands with the runtime PR |

Residual: the later PRs carry the runtime threats (row visibility of
`_flow-run` through `_carries`, user writes to rela-owned types, provenance);
their tickets repeat this threat model for their surface.

## Test Plan

- [x] Test scenarios documented for each acceptance criterion
- [x] Edge cases identified and documented
- [x] Negative test cases defined (invalid input, error conditions)
- [x] Integration test approach defined (not just unit tests)

**Test Scenarios:** see the acceptance criteria above.

**Edge Cases:** empty file and `flows: {}`; nil metamodel; no acl.yaml
(roles refused, run_as grants skipped only when no outputs); comment-only
edit; branch list where an earlier branch shadows a later one (warning);
final initial state; unreachable state; template with only a placeholder
keeps type, text around it needs a text property; unset subject property.

**Negative Tests:** `TestParse_Problems` covers unknown keys, bad names,
undeclared subject/type/relation/role/state, faced types, manual ids,
derived properties, bad periods, below-minimum periods, guard traversal,
disallowed functions, restricted fields, missing run_as, ungranted writes.

Integration: CLI tests drive `rela validate` against a temp project; a
manual smoke run of the built binary.

## Risk Assessment

- [x] Technical risks assessed with mitigations
- [x] Security risks assessed (see Security Considerations)
- [x] Effort estimated (xs/s/m/l/xl)

**Risks:**

- The format may need changes once the runtime lands. Mitigation: the
  runtime is not shipped, the guide marks flows as check-only, and the
  format is not yet consumed by anything stored.
- Restructuring `rela flow` could break `rela flow script.lua` and shebangs.
  Mitigation: `default:"withargs"`; smoke-tested.
- Reserving `_` could break an existing schema. Mitigation: searched the
  in-tree projects; none uses a leading `_`.

Effort: xl overall for M1; this PR m.

## Documentation Planning

- [x] User-facing docs identified (skip if internal refactor)
- [x] Docs-checklist will be created when entering implementation

**Documentation Impact:**

- [x] docs/flows.md (new guide)
- [x] docs/metamodel.md - reserved `_` prefix
- [x] docs/cli-reference.md - `rela flow validate | show`, `rela validate`

## Design Review

- [x] Run `/design-review` before starting implementation
- [x] All critical/significant findings addressed in plan

**Design Review Findings:** see the review-responses linked to TKT-XHANLJU.

---
id: PLAN-2QU3YXM
type: planning-checklist
title: 'Planning: Flows M5b: compile automations onto entity-change triggers'
started: "2026-11-13"
completed: "2026-11-13"
status: done
---

<!-- @managed: claude-workflow v1 -->

## Understanding

- [x] Problem/requirements clearly understood
- [x] Scope defined (what's in/out documented below)
- [x] Acceptance criteria documented with specific test scenarios

Today `automation.Engine.Process` walks every automation, re-checks its trigger
keys, transpiles each `when:` clause through `CompileFilter` on every write
(cached by source), re-parses each `validate:` check, and turns the actions into
an `automation.Result` that `internal/autocascade` executes. M5b moves the
decision onto the flow engine without changing what `Result` holds.

**Scope:**

In scope:

- An automation compiles once, at load, to a trigger: the events that select
it (indexed by event type), its scope (`entity:`, `faces:`, `copy:`), and a
guard. The guard is the `when:` clauses and the `condition:`, compiled per
entity type through `internal/predicate` (`predicatefns.CompileFilter` and
`Compile`).
- A new pure decider `flow.Fire` in `internal/flow`, the counterpart of
`flow.Decide` for triggers: every trigger whose guard holds fires, in
declaration order, and a guard that fails to evaluate is reported and does not
fire. `Engine.Process` selects the triggers by event and scope, fires them
through `flow.Fire`, and renders the outputs.
- Actions compile to outputs in the flow vocabulary: `set` is a set output,
`create_entity` (with `relation:`) is a create output with a relate, and
`create_relation` is a relate output.
- Named exceptions stay as they are, now compiled values beside the outputs:
`add_to_pile`, `notify`, `lua`/`lua_file` (script, including `background:` jobs)
and `validate:` checks. There is no `delete` action; `if_exists: replace`
deletes inside `internal/autocascade` and stays there.
- `automation.Result`, `internal/autocascade`, `internal/entitymanager` and the
job service do not change. Writes, audit records, `triggered_by` attribution,
cascade depth and the `jobs.Queue` side effects therefore stay as they are.

Out of scope (M5c, TKT-ZRQSGCM): moving automations into `flows.yaml`, reusing
`flowcfg.Output` and its template language, deleting `internal/automation`, and
dogfooding the tickets workflow as a flow.

Split decision: one PR. The change is confined to `internal/automation` (the
compile step and `Process`) plus one pure function in `internal/flow`. The
consumers keep their API. No further split is needed.

**Acceptance Criteria:**

1. Every existing test in `internal/automation` and `internal/autocascade`
passes. Two test fixtures that set internal fields (`engine.automations`,
`engine.ev`) move to the public construction path; their assertions do not
change.
2. Two golden tests written against the old engine, in a commit before the
refactor. `TestSyntheticAutomations_Golden` runs a synthetic schema built to
reach every matcher branch (alias, unknown and case-variant types, undeclared
and untranspilable clauses, conditions that hold, are false or fail, faces,
copies, `validate:`, piles, notify, create, scripts, managed names) over four
engines (binder true, false, error, and string-only).
`TestSchemaAutomations_Golden` replays an event matrix (created, property change
from/becomes, updated, relation created and removed, renamed, deleted, copy)
over every automation of `tickets/`, `docs-project/`, `examples/basecamp/` and
the dataentry fixture schema. It compares the planned `Result` byte for byte and
passes unchanged after the refactor.
3. `flow.Fire` has unit tests: order kept, a false guard skips, a guard error
is reported and later triggers are still tried.
4. The `entitymanager`, `appbuild` and `dataentry` race tests pass. They cover
the cascade and jobs paths over the new engine.

## Research

- [x] For larger features: run `/research` to create a structured research doc
- [x] Searched for existing libraries that solve this problem
- [x] Checked codebase for similar patterns or reusable code
- [x] Looked for reference implementations in other projects
- [x] Reviewed relevant rela concepts for prior art

**Research Doc:** N/A: the approach follows M5a (TKT-12RC7JT, PLAN-CU7IFC1).

**Existing Solutions:**

- M5a compiled enum transitions to a `flow.Definition` plus branches and
decided through `flow.Decide` (`internal/statemachine/compile.go`,
`enforce.go`). M5b does the same for automations.
- `flow.Decide` picks one branch and stops on a guard error. Automations are
independent: every matching one fires, and one failing condition must not stop
the others. A sibling `flow.Fire` is therefore needed rather than a reuse of
`Decide`.
- `flowcfg.Output` (set, create, relate) cannot be reused as is. Its templates
resolve only `subject.*`, `run.id`, `now` and `today`, while automation
templates use `{{new.*}}`, `{{old.*}}` and `{{user.*}}`. Its relate goes from
the new entity to the subject, while `create_entity.relation` goes from the
trigger to the new entity. Reusing it would change behaviour; M5c reconciles the
two when automations move to `flows.yaml`.
- No external library applies (internal refactor).

## Approach

- [x] Technical approach chosen and documented
- [x] Approach builds on existing patterns (not reinventing)
- [x] Alternatives considered (document why rejected)
- [x] Dependencies identified (packages, APIs, types)

**Technical Approach:**

1. `internal/flow/fire.go`: `Fire[T](triggers []T, holds func(T) (bool, error)) []Fired[T]`,
which does no I/O itself (the guard callback may bind traversals in the store).
`Fired{Trigger, Err}` lists the fired triggers and the failed guards in order.
It is a short loop; it lives in `internal/flow` so the entity-change triggers of
`flows.yaml` share it in M5c (TKT-ZRQSGCM).
2. `internal/automation/compile.go`: the trigger table. Per automation it holds
the `Trigger` keys, an event-type index, the guard, the outputs per action, and
the `validate:` checks. In the guard, each `when:` clause compiles per entity
type to one of the three cases `matchTyped` chooses between today: a predicate
program, the legacy `filter.Match` verdict for a clause `CompileFilter` cannot
transpile, or the string matcher for an undeclared property. The `condition:`
compiles to one program per type. `validate:` checks compile the same way as
`when:` clauses. A type-less automation compiles against every declared type. An
entity whose type is an alias uses the canonical entry. An unknown type, or an
engine without a metamodel, uses the string matcher, as today.
3. `Engine.Process` looks up the triggers for the event type; the index is a
superset prefilter. It keeps those whose scope admits the entity (managed name,
`entity:` as an exact match on the raw type, `faces:`, `copy:`) and whose event
keys select it. One `selects(trigger, event)` function, today's switch, stays
the authority for property from/becomes, the updated hash, relation type, rename
and copy only. It calls `flow.Fire` with the guard, which evaluates the `when:`
clauses first and stops at the first that fails, then the `condition:`. It then
walks the `Fired` list in declaration order: a failed guard adds its warning in
place, a fired trigger renders its outputs, so `Warnings` keep today's order.
Programs get the raw `ent.Type`; only the compiled-clause lookup falls back to
the canonical type. Script outputs keep `InterpolateSafeOnly`.
4. `NewEngine` compiles string-only; `NewEngineFromMetamodel` compiles after
the options (clock, traversals) apply. The trigger table is immutable after
construction, so `Process` reads it without a lock. A `CompileFilter` failure
keeps the `filter.Match` verdict and an unparseable `validate:` check keeps its
runtime warning; neither becomes a load error. The dead branch in `matchesFace`
goes. The unused `SetMetamodel` is removed (no callers). `SetTemplateVars`
stays, because templates render at fire time.

Alternatives rejected:

- Keep the old matcher and add the compiled one beside it behind a switch.
That leaves two evaluation paths on the hot write path, which M5c would then
have to delete.
- Compile into `flowcfg` flows now. That changes templates and relation
direction (see Research), so behaviour would change; it is M5c.

**Files to modify:**

- `internal/flow/fire.go`, `internal/flow/fire_test.go` (new)
- `internal/automation/compile.go` (new), `engine.go`, `notify.go`, `types.go`
- `internal/automation/schema_golden_test.go` and `testdata/golden/*.jsonl` (new)
- `internal/automation/typed_comparison_test.go`,
`recurrence_condition_test.go` (fixture construction only)
- `.go-arch-lint.yml`: `automation` may depend on `flow` (a leaf).

## Security Considerations

- [x] Input sources identified (user input, config, external APIs)
- [x] Input validation approach defined (allowlist preferred over blocklist)
- [x] Security-sensitive operations identified (file access, auth, crypto)
- [x] Error handling doesn't leak sensitive information

**Input Sources & Validation:**

- `schema.yaml` `automations:` (operator config, not secret): validated at
load as today. A `when:` parse error, a condition compile error, a file-property
write or an unresolvable notify path fails the boot.
- The written entity (caller data): its type, face, old and new values select
and guard triggers, and are interpolated into output values as today. The
template-name allowlist for `create_entity.template` is kept.

**Security-Sensitive Operations:**

- Automation writes run as system policy through `entitymanager` (unchanged).
- `related(...)` in conditions is answered by the raw-store binder
(unchanged).
- Lua actions and their `allow_acl_bypass` and `capabilities:` are carried
into `LuaToExecute` unchanged.

### Threat model (POLICY-015 §3, PROCEDURE-j5mq step 1)

Assets: integrity of the graph (automation writes are privileged system writes),
confidentiality of data a condition reads, and the capability grants of Lua
actions.

Actors: a caller writing through data-entry, MCP, CLI or Lua (all through
entitymanager); an operator authoring `schema.yaml` (trusted).

| Threat (STRIDE) | Vector | Mitigation in this PR |
| --- | --- | --- |
| Elevation of privilege | Compilation drops a `when:` or `condition:`, so an automation fires on more entities | Every clause compiles or fails the load as today; golden test over the in-repo schemas and the existing when and condition tests |
| Elevation of privilege | A capability or `allow_acl_bypass` grant is lost or widened in the compile hop | Script outputs carry the action's fields unchanged; the golden test pins `Capabilities` and `AllowACLBypass` of the basecamp background script |
| Tampering | A caller-written value steers the template name into a path | `isValidTemplateName` allowlist kept at render time |
| Tampering | A managed record (`_flow-run`, `_carries`) fires an operator automation | The managed-name check stays first in the scope check; existing test |
| Information disclosure | A failing condition's error text leaks data | Same message as today; it names the condition and the evaluation error, not property values |
| Denial of service | More work per write | Less: clauses compile once at load, and triggers are selected by event before any guard runs, so a write evaluates and binds traversals only for automations it can fire |
| Repudiation | Audit and `triggered_by` lost | `Result` carries `AutomationName` as today; autocascade and entitymanager unchanged |
| Tampering (code injection) | A render step interpolates entity values into inline `lua:` source | Script outputs keep `InterpolateSafeOnly`; the synthetic golden has an inline script naming `{{new.title}}`, which stays uninterpolated |
| Elevation of privilege | `add_to_pile` with an owner template that renders empty pushes to the acting user's pile | `planPilePush` and its empty-owner skip are kept; pinned by the synthetic golden |
| Denial of service | Background scripts run inline | `Background` is carried as before; autocascade still enqueues them on `jobs.Queue` |
| Spoofing | n/a | No identity handling changes |

Residual: none new.

## Test Plan

- [x] Test scenarios documented for each acceptance criterion
- [x] Edge cases identified and documented
- [x] Negative test cases defined (invalid input, error conditions)
- [x] Integration test approach defined (not just unit tests)

**Test Scenarios:**

1. AC1: `go test -race ./internal/automation/... ./internal/autocascade/...`.
2. AC2: `TestSchemaAutomations_Golden` (golden files written before the
refactor and committed with it).
3. AC3: `internal/flow/fire_test.go` table test.
4. AC4: `go test -race` on `internal/entitymanager`, `internal/appbuild` and
`internal/dataentry`.

**Edge Cases:**

- Type-less automation on an entity type not in the metamodel: string
matcher.
- Alias entity type: canonical compiled clauses.
- `when:` clause on an undeclared property: string matcher, as today.
- Untranspilable clause (regex on an int): `filter.Match` verdict, as today.
- Engine without a metamodel (`NewEngine`): string matching; a condition is
still a load error.
- Several automations setting the same property: the later one wins (order
kept).
- `create_entity` with an empty type or an invalid interpolated template: an
error, and the rest of that action is skipped, as today.

**Negative Tests:**

- A failing condition (traversal bind error) produces the same warning and
does not fire; other automations still fire (existing traversal tests).
- An invalid `validate:` check produces the "Invalid validation check"
warning.

## Risk Assessment

- [x] Technical risks assessed with mitigations
- [x] Security risks assessed (see Security Considerations)
- [x] Effort estimated (xs/s/m/l/xl)

**Risks:**

- Behaviour drift on the hot write path. Mitigation: `Result` unchanged,
every existing test kept, golden test written before the refactor.
- One intended difference: a `condition:` is evaluated only when the event
selects its automation. Before, a condition that failed to evaluate added a
warning to every write of its entity type, even writes that could never fire it
(for example a created-only automation on an update). This is documented in the
PR and pinned by a test.

Effort: m.

## Documentation Planning

- [x] User-facing docs identified (skip if internal refactor)
- [x] Docs-checklist will be created when entering implementation

**Documentation Impact:**

- [x] N/A - Internal change, no user-facing docs needed. The schema syntax
and behaviour do not change. The package doc of `internal/automation` records
that automations compile to flow triggers.

## Design Review

- [x] Run `/design-review` before starting implementation
- [x] All critical/significant findings addressed in plan

**Design Review Findings:** RR-YI23YFP, RR-4DEP7S7, RR-2CG178Z, RR-P7H2HQ4 and
RR-AAQSM35 (significant); RR-P1GUQXB and RR-TURW9FJ (minor); RR-KT5LKCD (nit).
All are addressed in this plan.

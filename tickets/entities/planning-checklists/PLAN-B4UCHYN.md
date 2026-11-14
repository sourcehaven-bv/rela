---
id: PLAN-B4UCHYN
type: planning-checklist
title: 'Planning: Flows M5c: delete the old transition and automation paths and dogfood the tickets workflow'
started: "2026-11-13"
completed: "2026-11-13"
status: done
---

<!-- @managed: claude-workflow v1 -->

## Understanding

- [x] Problem/requirements clearly understood
- [x] Scope defined (what's in/out documented below)
- [x] Acceptance criteria documented with specific test scenarios

After M5a and M5b both old evaluation paths already decide on the flow engine:
enum transitions through `flow.Decide`, automations through `flow.Fire`. What
remains is where the code and the configuration live. `internal/statemachine`
still hosts the headless flows of schema transitions, so `rela flow` cannot show
them. `internal/automation` still hosts the trigger engine and its `schema.yaml`
vocabulary.

**Split.** The whole ticket does not fit one reviewable PR. Deleting
`internal/automation` is not a package move. Its piles, notify, script and
`validate:` exceptions need a home in the flows.yaml vocabulary. Its templates
(`{{new.*}}`, `{{old.*}}`, `{{user.*}}`) and its relation direction differ from
`flowcfg.Output`. `automations:` needs a schema migration into `flows.yaml`. The
tickets dogfood needs that vocabulary first, because the checklist workflow
creates entities on status changes and the tickets schema declares no
transitions. The work is split into three sequential tickets:

1. TKT-ZRQSGCM (this ticket): fold the transition machines into `flowcfg`,
delete `internal/statemachine`, show headless flows in `rela flow show`, and
record the unset-property decision.
2. TKT-GWOBH2U (depends on 1): entity-change triggers in `flows.yaml` on
the flowcfg output vocabulary, the named exceptions moved into it, a schema
migration from `automations:`, and the deletion of `internal/automation`.
3. TKT-XB4M29R (depends on 2): the tickets checklist workflow as a flow in
`tickets/flows.yaml`.

**Scope:**

In scope:

- `internal/statemachine` moves into `internal/flowcfg` as the headless
flows of schema transitions. `flowcfg.CompileTransitions(meta, opts...)` returns
a `*flowcfg.Transitions`. `compileMachine`, the enforcer (`EnforceUpdate`,
`EnforceCreate`), `Performable`, `MachineProps`, `TraversingWhens`, `EntryValue`
and the sentinel errors move with it. Renames: `Set` becomes `Transitions`,
`Machine` becomes `HeadlessFlow`, `EmptySet` becomes `NoTransitions`, `Compile`
becomes `CompileTransitions`, `Option` and `WithTraversals` become
`TransitionOption` and `WithTransitionTraversals`, and the `Guard` interface
becomes `PermissionGuard` (flowcfg already has a `Guard` type). Behaviour does
not change.
- Every caller (entitymanager, affordances, dataentry, appbuild, and the
attachment and remotesync tests) imports `flowcfg`. `.go-arch-lint.yml`, the
archplan config and baseline, `.claude/rules/predicate.md` and the comments
follow.
- `rela flow show` lists the headless flows beside the flows of
`flows.yaml`, with `kind: headless`, the entity properties that follow them, and
their states. `rela flow show schema.<type>` renders one as a Mermaid diagram.
`rela flow validate` counts them. A headless flow is named `schema.<custom
type>`. A flows.yaml name cannot contain a dot, so the two never collide.
- A decision record for the unset property: a headless flow does not read
an unset property as its initial state. Every move from unset is refused,
including the move to the entry value. This is today's behaviour.

Out of scope: `internal/automation`, the `automations:` vocabulary and the
tickets dogfood (TKT-GWOBH2U and TKT-XB4M29R). Transitions stay declared in
`schema.yaml`.

**Acceptance Criteria:**

1. `internal/statemachine` is deleted and nothing imports it. All its tests
pass in `internal/flowcfg`.
2. The write path is unchanged: the entitymanager, affordances, dataentry,
appbuild and remotesync tests pass with `-race`, and the M5b goldens pass.
3. `rela flow show` lists headless flows on a project with transitions and
no flows.yaml, and renders one. JSON output carries `kind`.
4. The unset decision is recorded and pinned by
`TestEnforceUpdate_UnsetIsNotInitial`, extended to the entry value.
5. The three analyze gates are clean for the new entities.

## Research

- [x] For larger features: run `/research` to create a structured research doc
- [x] Searched for existing libraries that solve this problem
- [x] Checked codebase for similar patterns or reusable code
- [x] Looked for reference implementations in other projects
- [x] Reviewed relevant rela concepts for prior art

**Research Doc:** N/A: a package fold that follows M5a (PLAN-CU7IFC1) and M5b
(PLAN-2QU3YXM).

**Existing Solutions:**

- `flowcfg.Flow.Mermaid` and `mermaid.StateDiagramAt` render a flow. The
headless flow reuses them.
- `internal/cli/flows.go` `FlowShowCmd` lists and renders flows. It gains
the headless flows from the metamodel.
- `appbuild.CompileTransitions` compiles the machines with the raw-store
binder. `rela flow show` uses the same compile, so a `related(...)` `when:`
loads as it does at boot.
- No external library applies.

## Approach

- [x] Technical approach chosen and documented
- [x] Approach builds on existing patterns (not reinventing)
- [x] Alternatives considered (document why rejected)
- [x] Dependencies identified (packages, APIs, types)

**Technical Approach:**

1. `git mv` the statemachine files into `internal/flowcfg` as
`transitions*.go` and `transitions*_test.go`, change the package clause, apply
the renames above, and rename the colliding unexported `definition` to
`headlessDefinition`.
2. `HeadlessFlow` gains read accessors for display: `Name()` (the custom
type name), `States()` and `Mermaid()`. `Transitions` gains `Headless()` (sorted
by name), `HeadlessFlow(name)` and `Uses(name)` (the `entity.property` pairs
that follow a flow). Every accessor is nil-safe. The unexported `branch` becomes
`headlessBranch`, beside flowcfg's `Branch`. `GuardError` keeps its name
(callers match it with `errors.As`); its doc says it names a permission.
3. Callers: sed the import path and the qualified names, then `goimports`.
4. One exported appbuild helper, `CompileHeadlessFlows(meta, st)`,
compiles with the raw-store binder. The boot path (`appbuild.go`),
`schemacheck.go`, `CompileTransitions` and the CLI call it.
5. `rela flow show` compiles the headless flows on their own, so a broken
flows.yaml or acl.yaml does not hide them, and lists both kinds. JSON entries
carry `kind` (`flow` or `headless`); a headless entry carries `uses` (the
`entity.property` pairs) instead of `subject`. The `schema.` prefix is one
exported constant (`flowcfg.HeadlessPrefix`) used only for presentation; it
never reaches the run store or flowview. `rela flow show schema.<type>` renders
the diagram. `rela flow validate` reports flows.yaml and the headless flows on
separate lines.
6. `.go-arch-lint.yml`: drop the `statemachine` component. Every former
statemachine dependent depends on flowcfg. A GUARD comment on flowcfg says it
takes no runtime dependency (store, lua, jobs, automation): headless moves
evaluate through injected collaborators only.
7. The flowcfg package doc names the two kinds of flows it compiles and
how the headless enforcer stays pure.

Alternatives rejected:

- Move the enforcer into `internal/flow`: flow is a dependency-free leaf,
and the enforcer needs entity, predicate and an ACL guard.
- Move it into `internal/flowengine`: entitymanager would then import the
run engine and the run store.
- Keep `internal/statemachine` and only add `rela flow show`: the compile
stays in a second package, which the ticket asks to fold.
- Do all three parts in one PR: see Split.

**Files to modify:**

- `internal/statemachine/*` → `internal/flowcfg/transitions*.go`
- `.claude/rules/predicate.md`: glob `internal/flowcfg/transitions*.go`
- `internal/flowcfg/doc.go`
- `internal/cli/flows.go`, `flows_test.go`
- `internal/appbuild/transitions.go`, `appbuild.go`, `schemacheck.go`,
`realtime_outputs.go` and tests
- `internal/entitymanager`, `internal/affordances`, `internal/dataentry`,
`internal/attachment`, `internal/remotesync` (imports and names)
- `.go-arch-lint.yml`, `.archplan/*`, `.claude/rules/predicate.md`
- docs-project GUIDE-flows (headless flows in `rela flow show`)

## Security Considerations

- [x] Input sources identified (user input, config, external APIs)
- [x] Input validation approach defined (allowlist preferred over blocklist)
- [x] Security-sensitive operations identified (file access, auth, crypto)
- [x] Error handling doesn't leak sensitive information

**Input Sources & Validation:**

- `schema.yaml` transitions (operator config, not secret): compiled at boot
as today. A malformed machine still fails the boot.
- The written entity (caller data): old and new values of the machine
property, on the unchanged path.
- `rela flow show <name>` (CLI argument): a map lookup. An unknown name is
an error naming the key. The output is configuration only.

**Security-Sensitive Operations:**

- Transition guards (`guard:` permissions) are checked through the injected
`PermissionGuard`, adapted from the ACL in appbuild, unchanged.
- `when:` traversals read the raw store, as today.

### Threat model (POLICY-015 §3, PROCEDURE-j5mq step 1)

Assets: integrity of governed status changes (guarded transitions), the
read-side affordance answer (`_transitions`), and the entity data a `when:`
reads.

Actors: callers writing through data-entry, MCP, CLI, Lua and remote sync (all
through entitymanager); an operator authoring schema.yaml (trusted); a CLI user
running `rela flow show` (has the project files).

| Threat (STRIDE) | Vector | Mitigation |
| --- | --- | --- |
| Elevation of privilege | The fold drops or reorders a check, so a guarded or illegal move is accepted | Code moves unchanged; every statemachine test moves with it; the entitymanager, affordances and dataentry transition tests pass |
| Elevation of privilege | A wiring site loses the enforcer after the rename | `entitymanager.New` still rejects a nil enforcer; `NoTransitions()` is the explicit opt-out |
| Elevation of privilege | Performable and enforcement drift apart | Both still call the one `decideMove` |
| Information disclosure | `rela flow show` prints entity data | It prints configuration only (states, guards, `when:` sources); the configuration is not a secret (CLAUDE.md) |
| Tampering | A crafted name in `rela flow show` | Map lookup only; no path or template use |
| Denial of service | More compile work at boot | None: the compile runs as often as before |
| Repudiation, Spoofing | n/a | No identity or audit change |

Residual: none new.

## Test Plan

- [x] Test scenarios documented for each acceptance criterion
- [x] Edge cases identified and documented
- [x] Negative test cases defined (invalid input, error conditions)
- [x] Integration test approach defined (not just unit tests)

**Test Scenarios:**

1. AC1: `go build ./...`, `go test -race ./internal/flowcfg/...`, and a
repository grep for `internal/statemachine` that finds nothing.
2. AC2: `go test -race` on entitymanager, affordances, dataentry, appbuild,
remotesync, attachment, automation and autocascade. A scratch-copy end-to-end
run of every ticket and bug status transition on `tickets/`.
3. AC3: `internal/cli/flows_test.go`: list with headless flows and no
flows.yaml, JSON `kind` for both kinds, render `schema.<type>`, unknown name
error. `transitions_show_test.go` for `Headless`, `HeadlessFlow`, `Uses` and
`Mermaid`.
4. AC4: `TestEnforceUpdate_UnsetIsNotInitial` covers the entry value.

**Edge Cases:**

- A project with no transitions and no flows.yaml: "No flows declared."
- A custom type shared by two entity properties: one headless flow, both
uses listed.
- A machine with a guarded and a `when:` edge: the Mermaid label names both.

**Negative Tests:**

- `rela flow show schema.missing`: an error naming the flow.
- Unset → entry value: refused with `ErrIllegalTransition`. The decision
names the recovery path: a data migration sets the value; it writes the store
directly, so the enforcer does not apply.
- `Transitions` accessors on a nil receiver.

## Risk Assessment

- [x] Technical risks assessed with mitigations
- [x] Security risks assessed (see Security Considerations)
- [x] Effort estimated (xs/s/m/l/xl)

**Risks:**

- A wide mechanical diff (about 100 files, mostly test imports).
Mitigation: `git mv` keeps the history; the renames are listed in the PR.
- Write-path consumers now import flowcfg. Mitigation: the GUARD comment
and the arch-lint allowlist keep flowcfg free of runtime components.

Effort: m.

## Documentation Planning

- [x] User-facing docs identified (skip if internal refactor)
- [x] Docs-checklist will be created when entering implementation

**Documentation Impact:**

- [x] docs-project GUIDE-flows: `rela flow show` lists headless flows.
- [x] CLI reference: the `rela flow show` help text.

## Design Review

- [x] Run `/design-review` before starting implementation
- [x] All critical/significant findings addressed in plan

**Design Review Findings:** RR-YXYX9EQ and RR-G4VKS89 (significant); RR-LNAV79T,
RR-3FUM5QZ, RR-U5V89VY, RR-0TLEFA1 and RR-N9UEIVX (minor); RR-NIBXSM1 (nit). All
are addressed in this plan.

---
id: PLAN-HQ5Y
type: planning-checklist
title: 'Planning: Define entitymanager.Manager (real implementation, not adapter)'
status: done
---

<!-- @managed: claude-workflow v1 -->

## Understanding

- [x] Problem/requirements clearly understood
- [x] Scope defined (what's in/out documented below)
- [x] Acceptance criteria documented with specific test scenarios

**Problem:** `internal/entitymanager` ships only the interface (`EntityManager`)
and the public result types. The real implementation is `wsEntityManager`
(`internal/workspace/manager.go:24-157`), a thin adapter that delegates to
Workspace's private methods (`w.createEntity`, `w.updateEntity`, etc.).
Workspace is the transitional shim being decomposed (FEAT-workspace) — Manager
needs to be liftable out of it so per-command tickets (TKT-KWAX / TKT-2IAC /
TKT-9JEI / TKT-0SP1) can wire their own Manager without going through Workspace.

This ticket builds the real `entitymanager.Manager` type with focused, typed
dependencies, satisfies `autocascade.Host` so it can drive cascades directly,
and keeps `wsEntityManager` alive as the transition path for callers that still
use Workspace.

### Audience and packaging

**Who uses `Manager`?** The wiring sites of the per-command tickets (TKT-KWAX
MCP, TKT-2IAC scheduler, TKT-9JEI dataentry, TKT-0SP1 CLI). Each command
constructs its own Manager from focused services and hands it (as the
`entitymanager.EntityManager` interface) to the consumers that already depend on
that interface (Lua bindings, HTTP handlers, MCP tools). Manager isn't called by
random code — it's called by *the same things wsEntityManager is called by
today*, just constructed differently.

**Why keep the `EntityManager` interface AND a `Manager` impl?** The interface
is load-bearing for the existing system:

- `lua.WriteDeps.EntityManager` is the interface type — Lua scripts
call into it without knowing the impl.
- HTTP handlers, MCP tools, and tests stub the interface for unit
testing.
- The migration itself is enabled by the split: consumers depend on
the interface; we swap wsEntityManager → Manager via wiring without touching
call sites.

Once Manager is the only production impl (post-TKT-64R3, when wsEntityManager
dies), the interface *could* be deletable if no consumer needs stubbing.
Long-term cleanup question; not this ticket's concern.

**Consumer-side-interface debt: acknowledged.** Today's
`entitymanager.EntityManager` is a producer-side interface — it publishes the
union of methods every consumer might call. CLAUDE.md's "Define interfaces at
the call site, not next to the implementation" rule says consumers should each
declare their own narrow interface. We don't do that today, and TKT-QTNX
deliberately *doesn't fix that now*.

Why not now: each consumer (lua bindings, dataentry handlers, MCP tools, CLI
commands) calls a slightly different subset. Narrowing properly means defining
four (or more) small interfaces in their respective packages, updating field
types, updating stubs in tests. That's the *per-command migration work* already
scoped into TKT-KWAX (MCP), TKT-2IAC (scheduler), TKT-9JEI (dataentry), TKT-0SP1
(CLI), and TKT-Y0JU (lua specifically). Each migration ticket will declare its
own consumer-side interface as part of that ticket's scope.

TKT-QTNX's job is to *make Manager exist* so those migrations have something to
wire to. Folding the consumer-interface narrowing into this ticket would
conflate "build the implementation" with "rewire every consumer" and bloat the
PR past usefully reviewable size.

The end state, after all migrations land + TKT-64R3 deletes wsEntityManager:
`entitymanager.EntityManager` becomes deletable (or, if any test still wants a
stub of "the union shape," it stays as a convenience but isn't on any consumer's
import surface).

**Two roles, two types.** Manager has *two* distinct responsibilities in the
original draft of this plan:

1. **Public write API** — satisfies `entitymanager.EntityManager`,
called by HTTP handlers / Lua bindings / MCP tools / CLI.
2. **Cascade callback surface** — satisfies `autocascade.Host`, called
by `autocascade.Runner` during automation cascades.

After reviewer pushback (c_d38635), these split into two types in the revised
plan:

- **`Manager`** holds the public write API. Satisfies `EntityManager`.
- **`cascadeHost`** holds the seven Host methods. Satisfies
`autocascade.Host`. Unexported.

Both share the same `Deps` (Store, Meta, Templater, etc.). Manager constructs
(or exposes) a `cascadeHost` internally and passes it to `runner.Process(ctx,
host, req)` during cascade dispatch. Public consumers never see `cascadeHost`;
readers of `Manager` see only the public surface.

The compile-time assertions split too:

```go
var _ entitymanager.EntityManager = (*Manager)(nil)
var _ autocascade.Host           = (*cascadeHost)(nil)
```

This makes the two responsibilities concrete, makes the test surfaces distinct,
and avoids the "is `CreateEntity` the cascading one or the non-cascading one?"
reader-confusion that two same-name methods on one type would invite. (They had
different signatures so wouldn't collide at the language level, but the
reader-level confusion is real.)

### Package home: `internal/entitymanager` (existing)

The package already exists with the interface and result types
(`internal/entitymanager/entitymanager.go:101-123`). Manager + cascadeHost land
as new files alongside. **`internal/entitymanager.mayDependOn`** stays minimal —
today it has zero internal deps (entity is commonComponent); after this ticket
it grows to include the deps Manager actually uses:

```yaml
entitymanager:
  mayDependOn:
    - autocascade
    - automation
    - metamodel
    - store
    - templating
```

Each is justified by a real call site in the survey. `lua` is intentionally
**not** added — see "ScriptRunner wiring" below.

**In scope:**

- New `internal/entitymanager/manager.go` with:
  - `Manager` struct.
  - `Deps` struct (typed required collaborators; see "Deps shape" below).
  - `New(d Deps) (*Manager, error)` constructor; rejects nil required fields per CLAUDE.md.
  - Implementations of the 7 `EntityManager` methods (`CreateEntity`, `UpdateEntity`, `DeleteEntity`, `RenameEntity`, `CreateRelation`, `UpdateRelation`, `DeleteRelation`).
  - A shared internal `runWriteCascade(ctx, trigger, oldTrigger, autoResult) (cascadeOutcome, error)` helper that encapsulates "build ScriptRunner, build Request, call runner.Process, return outcome." This is the single place a future audit / policy hook plugs in.
- New `internal/entitymanager/cascadehost.go` with:
  - Unexported `cascadeHost` struct holding the same `Deps`.
  - The 7 `autocascade.Host` methods (`CreateEntity`, `WriteEntity`, `GetEntity`, `WriteRelation`, `ValidateRelation`, `DeleteEntity`, `FindExistingRelationTarget`).
  - Constructed inside Manager (or exposed via a `Manager.host()` accessor) so the wiring of cascade-callback state is internal to entitymanager. Public consumers of Manager never see cascadeHost.
- Both `Manager` and `cascadeHost` call into a shared `createCore` helper (in `core.go`) for entity creation without automation — preserves the single source of truth for "the bare write path" so `Manager.CreateEntity` (with cascade) and `cascadeHost.CreateEntity` (without cascade) can't drift.
- Unit tests with stub deps (no Workspace) covering each method's pipeline shape.
- Behavior-pinning protocol: before deleting any wsEntityManager code, verify Manager produces the same `*CreateResult` / `*UpdateResult` / `*DeleteResult` / `*RenameResult` shape as wsEntityManager for the same inputs, on a few representative cases.

**Out of scope (subsequent phases):**

- Wiring Manager into production. Workspace continues to construct `wsEntityManager`; per-command tickets (TKT-KWAX et al.) will switch over.
- Removing `wsEntityManager`. It stays alive until the last consumer migrates (TKT-64R3 marker).
- Audit / principal / policy fields on `Deps`. Those land in their respective tickets (TKT-6YYM for audit; future tickets for principal/policy).
- Rename's metamodel re-validation. Workspace's `rename` doesn't currently re-validate the post-rename state (see survey #2); Manager preserves this behavior verbatim. Tightening this is a follow-up.
- Plumbing `CreateOptions.SkipAutomation`. The field exists on the public interface (`entitymanager.go:31`) but isn't honored by `wsEntityManager` today. Preserving the gap; revisit when a real use case appears.

### Pipeline shape

Survey #2 confirmed five distinct pipelines across the 7 methods. Manager
preserves each verbatim.

**Create entity** (`Workspace.createEntity` lines 790-853):
1. Precondition validation (ID rules, duplicate check).
2. `createEntityCore`: ID gen → template → defaults → metamodel-validate → store-write.
3. `automation.Engine.Process(EventEntityCreated)` → collect property changes.
4. If property changes: apply + store-write again (yes, two writes).
5. `runner.Process(...)` with the resulting autoResult.

**Update entity** (`Workspace.updateEntity` lines 865-909):
1. Metamodel-validate the post-update entity.
2. **Gated automation:** if `oldEntity != nil` AND engine present, `automation.Engine.Process(EventEntityUpdated)` → collect property changes. (If oldEntity is nil — e.g., update called from a Lua script with no prior-state context — automation is skipped. Preserve verbatim.)
3. Apply property changes.
4. Store-write.
5. `runner.Process(...)` with the autoResult.

**Delete entity** (`Workspace.deleteEntity` lines 921-958):
1. Lookup entity (existence check).
2. Collect incident relations (incoming + outgoing).
3. If cascade=false and relations exist: error.
4. Delete incident relations (no automation).
5. Delete entity (no automation, no cascade).

**Rename** (`Workspace.rename` lines 19-71 of rename.go):
1. Validate preconditions (old exists, new doesn't); collect incident relations.
2. Early return if dry-run.
3. Write entity at new ID.
4. Write all incident relations with new ID substituted.
5. Delete old relation files.
6. Delete old entity.
7. **No automation, no metamodel re-validation.**

**Relations** (createRelation / updateRelation / deleteRelation, lines
1092-1194):
- `CreateRelation`: fetch endpoints → validate-relation-type → check duplicates → template → write. **No automation.**
- `UpdateRelation`: fetch existing → merge properties → write. **No automation, no validation.**
- `DeleteRelation`: delete. **No automation.**

The shared `runWriteCascade` helper handles steps 3-5 of Create and steps 2-5 of
Update (the cascade-bearing operations); the other five methods skip it.

### Deps shape

```go
type Deps struct {
    // Store is the authoritative persistence layer.
    Store store.Store

    // Meta is the loaded metamodel. Manager uses it for
    // ValidateEntity (entity writes) and ValidateRelation (relation
    // writes).
    Meta *metamodel.Metamodel

    // Automations is the rule-evaluation engine. Manager calls it on
    // EventEntityCreated / EventEntityUpdated to discover side effects.
    // Optional: nil means no automation (metamodel without rules).
    Automations *automation.Engine

    // Cascade is the autocascade Runner that orchestrates automation
    // side effects after a write. Required iff Automations is non-nil
    // — there's no point evaluating rules without a Runner to apply
    // them.
    Cascade *autocascade.Runner

    // Templater applies entity-creation templates (default-property
    // sets per type, optional named variants).
    Templater templating.Templater

    // NewScriptRunner builds a per-cascade autocascade.ScriptRunner.
    // Called once per Manager method invocation that triggers a
    // cascade (Create + Update only). The factory pattern keeps
    // entitymanager from importing internal/lua: the wiring site
    // (workspace today; per-command bootstrap later) closes over
    // the per-call lua.WriteDeps assembly.
    //
    // Optional: nil means scripted automation actions produce errors
    // in the Outcome (via Runner's own missing-Scripts handling).
    NewScriptRunner func() autocascade.ScriptRunner
}
```

**Why a factory for ScriptRunner:**

The cleanest options (open during planning, lock at implementation):

1. **Factory (`func() autocascade.ScriptRunner`)** — wiring site closes over `lua.WriteDeps` assembly; Manager has no Lua import. **Recommended.**
2. **Direct `autocascade.ScriptRunner` field, late-bound via setter** — wiring needs two steps (`m := New(d); m.SetScripts(...)`) because `lua.WriteDeps` references Manager itself via `EntityManager` field. Cycle workaround that adds a setter — smells.
3. **Per-call argument** — every `Manager.CreateEntity` etc. signature gains a `ScriptRunner` parameter. Breaks `entitymanager.EntityManager` public contract.

Option 1 keeps the interface clean, keeps entitymanager Lua-free, and accepts
the minor cost of "the wiring site provides a thunk." Per CLAUDE.md
"Transport-specific types belong at adapter layers" — the lua.WriteDeps
construction is transport-specific; it stays at the adapter (wiring site), not
in entitymanager.

**Open question for review:** is the factory acceptable, or do you prefer the
late-bound setter? My recommendation is factory.

### autocascade.Host satisfaction (separate type)

A new unexported `cascadeHost` struct (in `cascadehost.go`) satisfies the
7-method `autocascade.Host` interface. cascadeHost and Manager share the same
`Deps` — they're peer collaborators, not parent/child.

| Host method | cascadeHost implementation |
|---|---|
| `CreateEntity(type, opts) (*entity.Entity, error)` | Calls the shared `createCore` helper (template + validate + write, no automation). Same code path Manager.CreateEntity uses for step 2 of its own pipeline. |
| `WriteEntity(e) error` | `h.deps.Store.UpdateEntity(ctx, e)` (or Upsert depending on store interface). |
| `GetEntity(ctx, id) (*entity.Entity, error)` | `h.deps.Store.GetEntity(ctx, id)`. |
| `WriteRelation(r) error` | Direct store relation upsert; no validation (cascade-relations are validated at the Runner layer via `ValidateRelation`). |
| `ValidateRelation(relType, fromType, toType) error` | `h.deps.Meta.ValidateRelation(relType, fromType, toType)`. |
| `DeleteEntity(ctx, entityType, id, cascade) error` | Same logic as `Manager.DeleteEntity` (collect incident relations, delete them, delete entity). Shared helper. |
| `FindExistingRelationTarget(sourceID, relType, targetType)` | Iterate `h.deps.Store.ListRelations(...)`, return first match of the right target type. |

**Wiring inside Manager:**

```go
// internal/entitymanager/manager.go
type Manager struct {
    deps Deps
    host *cascadeHost  // constructed at New() time; shared deps
}

func New(d Deps) (*Manager, error) {
    if err := d.validate(); err != nil { return nil, err }
    m := &Manager{deps: d}
    m.host = &cascadeHost{deps: d}  // peer collaborator
    return m, nil
}

// When Manager dispatches a cascade:
func (m *Manager) CreateEntity(ctx, e, opts) (*CreateResult, error) {
    // ... createCore, automation.Process, apply property changes, write ...
    outcome, err := m.deps.Cascade.Process(ctx, m.host, autocascade.Request{
        Trigger: e, OldTrigger: nil, Result: autoResult,
        Scripts: m.deps.NewScriptRunner(),
    })
    // ...
}
```

**Important contract:** `cascadeHost.CreateEntity` must NOT fire follow-up
cascades (per the contract documented in autocascade/host.go). Both methods
delegate to the same `createCore` helper; Manager's version wraps it with
automation + cascade, while cascadeHost's calls it directly and returns. The
shared helper is the single source of truth for "the bare write path."

**Why split into two types instead of one:**

- **Single responsibility per type.** Manager is what consumers call; cascadeHost is what Runner calls. Different audiences, different contracts.
- **Reader clarity.** A reader looking at Manager sees the public write API. The cascade-callback surface is in cascadehost.go, not interspersed with public methods.
- **No "which CreateEntity?" confusion.** The two `CreateEntity` methods (with different signatures) would technically not collide at the Go language level, but they'd confuse readers about which behavior is which. Splitting types removes the ambiguity.
- **Independent test surfaces.** cascadeHost can be unit-tested with a stub Manager-deps-equivalent and a recording Runner; Manager can be tested with a stub cascadeHost or a real one over a memstore.

### Files to modify

**New:**
- `internal/entitymanager/manager.go` — Manager type, Deps, New, the 7 EntityManager methods.
- `internal/entitymanager/host.go` — the 7 autocascade.Host methods. Separate file because they're the "called-into" surface, distinct from the "called-out-from" public methods.
- `internal/entitymanager/core.go` — private helpers (`createCore`, `runWriteCascade`, etc.).
- `internal/entitymanager/manager_test.go` — stub-Deps tests covering each pipeline shape.

**Modified:**
- `.go-arch-lint.yml` — extend `entitymanager.mayDependOn` to `[autocascade, automation, metamodel, store, templating]`.

**Untouched (out of scope):**
- `internal/workspace/manager.go` — wsEntityManager stays. The migration tickets (TKT-KWAX et al.) will swap their callers.
- `internal/workspace/workspace.go` — no changes; Workspace continues to construct wsEntityManager.

### Behavior-pinning protocol

Three behaviors that the survey called out as easy to miss:

1. **Two writes in CreateEntity.** When automation sets properties, the entity is written twice (once in `createCore`, once after property changes). Manager's CreateEntity test must capture this — pin via a recording store that asserts call count == 2 when automation runs, == 1 when it doesn't.
2. **Update-without-oldEntity skips automation.** `EntityManager.UpdateEntity` doesn't take oldEntity, so Manager has to fetch it. If `m.store.GetEntity(ctx, e.ID)` errors, the update should *fail* (entity not found is an error in this path). But if it returns nil for some reason (shouldn't happen, but defensively), automation must skip. Pin via a test.
3. **Delete has no automation.** Easy to "add cascade evaluation to all writes." Don't. Pin via a test that exercises a metamodel-with-on-deleted-automation (if engine supports it; if not, the absence is structural and lint-able).

## Acceptance Criteria

1. **AC1:** `entitymanager.Manager` exists; `New(Deps{}) (*Manager, error)` rejects nil required fields (Store, Meta, Templater minimum; Automations+Cascade as a pair).
2. **AC2:** Manager implements `entitymanager.EntityManager` (7 methods). Compile-time assertion: `var _ entitymanager.EntityManager = (*Manager)(nil)`.
3. **AC3:** `cascadeHost` satisfies `autocascade.Host` (7 methods). Compile-time assertion: `var _ autocascade.Host = (*cascadeHost)(nil)`. Manager wires it internally; public consumers don't see the host type.
4. **AC4:** Pipeline order matches Workspace for each of the 5 distinct shapes (create / update / delete / rename / relations). Pinned by tests:
   - `TestManagerCreate_WritesOnceWithoutAutomation` — no rules in engine; one store CreateEntity call.
   - `TestManagerCreate_WritesTwiceWithAutomationProperties` — automation sets a property; two store writes.
   - `TestManagerUpdate_SkipsAutomationOnNilOldEntity` — store returns "not found"; update errors and automation is never invoked.
   - `TestManagerDelete_NoAutomation` — engine present but automation events are not invoked on delete.
   - `TestManagerRename_NoMetamodelRevalidation` — rename doesn't re-validate the post-state.
5. **AC5:** `cascadeHost.CreateEntity` does NOT fire cascades. Pinned by a test using a stub Cascade.Runner that fails if invoked.
6. **AC6:** Workspace tests still pass unchanged. wsEntityManager continues to work; no production wiring touched.
7. **AC7:** `just ci` green: lint, arch-lint (entitymanager.mayDependOn extended), race-tests, coverage.

## Research

- [x] Reviewed survey output covering wsEntityManager + Workspace + entitymanager + validator + autocascade.Host.
- [x] CLAUDE.md rules: "Constructors reject nil required fields", "Define interfaces at the call site", three new sub-rules from TKT-6OMC review (narrow returns, contracts-in-docs, transport-at-adapter).
- [x] TKT-6OMC's autocascade.Host pattern (the worked example in CLAUDE.md).
- [x] No relevant external libraries — this is purely a code reorganization.

## Approach

(Detailed above. Summary: pipeline-preserving lift with focused deps, factory
for ScriptRunner to keep entitymanager Lua-free.)

## Security Considerations

- [x] Input sources unchanged from current Workspace path. Manager performs the same metamodel.ValidateEntity / ValidateRelation today's Workspace does.
- [x] No new privileged operations.
- [x] Errors are wrapped with context (preserving today's `fmt.Errorf("write entity: %w", err)` etc.). No information disclosure changes.

## Test Plan

Two layers:

**Manager-level unit tests** (`internal/entitymanager/manager_test.go`) using
stub Deps:
- One test per AC above.
- Plus: each EntityManager method exercised end-to-end (create through delete) with a minimal metamodel, memstore, and `automation.NewEngine(nil)` (no rules).
- Plus: `TestManager_CascadeRunsThroughHost` — uses a real autocascade.Runner with a stub ScriptRunner; verifies that Manager-driven creates trigger cascade evaluation when automation rules are present.

**No new workspace tests** — the existing 14 workspace cascade tests verify
wsEntityManager's behavior, which we're preserving. Those tests don't need to
know Manager exists.

**Coverage target:** entitymanager package should land above 80% statements
(Manager has clear branches, all of which are exercisable).

## Risk Assessment

| Risk | Mitigation |
|---|---|
| Pipeline drift — Manager misses a sequencing detail and a workspace test now passes against Manager-via-wsEntityManager-replacement but fails when migrated. | Workspace migration is *out of scope* for this ticket. Manager's tests are self-contained; existing workspace tests pass unchanged because wsEntityManager is untouched. Drift would surface in the per-command tickets. |
| Two-writes-on-create is a smell preserved by mandate. Someone reads the code and "fixes" it. | Pin via test `TestManagerCreate_WritesTwiceWithAutomationProperties`. Comment in createCore explains why the two writes exist. |
| Factory-for-ScriptRunner felt overengineered to a reviewer. | Open question above. Default (factory) and alternatives are documented. |
| `Host.CreateEntity` accidentally calls `Manager.CreateEntity` instead of `createCore` → infinite cascade. | Compile-time assertion + explicit test (AC5) catches this. |
| Effort estimate (m) is wrong. | Bumped up if implementation reveals more complication than the survey suggested. Survey was thorough; I'd be surprised by a 2x overrun. |

**Effort:** **m** (~3-4 days). Breakdown: ½ day Deps + skeleton + arch-lint; 1.5
days each of the 7 methods (with care for the pipeline shapes); ½ day unit
tests; ½ day stub Host implementations + assertions; ½ day docs + cleanup.

## Documentation Planning

- [x] ~~User guide — N/A (internal refactor).~~ (N/A: parent shipped; back-filled by TKT-5S8T)
- [x] ~~CLI help — N/A.~~ (N/A: parent shipped; back-filled by TKT-5S8T)
- [x] CLAUDE.md — minor update? The "Don't extend internal/workspace" rule and the consumer-side-interface section both apply. May not need explicit additions.
- [x] ~~README.md — N/A.~~ (N/A: parent shipped; back-filled by TKT-5S8T)
- [x] ~~API docs — N/A.~~ (N/A: parent shipped; back-filled by TKT-5S8T)

## Design Review

- [x] ~~`/crit` reviewers (Jeroen)~~ (N/A: parent shipped; back-filled by TKT-5S8T)
- [x] ~~cranky-code-reviewer + go-architect on the plan~~ (N/A: parent shipped; back-filled by TKT-5S8T)
- [x] ~~All critical/significant findings addressed before implementation~~ (N/A: parent shipped; back-filled by TKT-5S8T)

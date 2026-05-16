---
id: TKT-Z9MR
type: ticket
title: ScriptRunner takes Mutator per-call (delete wsScriptRunner + mcpScriptRunner)
kind: refactor
priority: medium
effort: s
status: done
---

## Summary

Restructure `autocascade.ScriptRunner` and `script.LuaScriptRunner` so the
mutation handle is supplied **per-call** rather than baked into the runner at
construction. This eliminates the cycle that today requires `wsScriptRunner` and
`mcpScriptRunner` — two near-identical adapters whose only job is per-call
resolution of `lua.WriteDeps`.

Precursor for TKT-9JEI (dataentry/rela-server off workspace). Independent and
reviewable on its own.

## Background — the cycle

```
EntityManager:  built from { Store, Meta, ScriptRunner }
ScriptRunner:   built from { lua.WriteDeps }
lua.WriteDeps:  built from { EntityManager }
```

Today both `wsScriptRunner` and `mcpScriptRunner` work around this by closing
over their containing service struct and constructing a fresh
`script.LuaScriptRunner` per `Run` call:

```go
func (r *wsScriptRunner) Run(ctx context.Context, a autocascade.ScriptAction) error {
    return script.NewLuaScriptRunner(r.w.scriptExec, r.w.LuaWriteDeps()).Run(ctx, a)
}
```

A third copy of this dance would land in `appbuild` if we don't refactor first.

## In scope

### 1. New `autocascade.Mutator` interface

5-method consumer-side interface in autocascade. That's the subset Lua bindings
call today (`internal/lua/runtime.go` lines 1295/1352/1398/1418/1440).
`RenameEntity` and `UpdateRelation` are not invoked from scripts.

```go
// internal/autocascade/mutator.go
package autocascade

type Mutator interface {
    CreateEntity(ctx context.Context, e *entity.Entity, opts entitymanager.CreateOptions) (*entitymanager.CreateResult, error)
    UpdateEntity(ctx context.Context, e *entity.Entity) (*entitymanager.UpdateResult, error)
    DeleteEntity(ctx context.Context, id string, cascade bool) (*entitymanager.DeleteResult, error)
    CreateRelation(ctx context.Context, from, relType, to string, props map[string]any) (*entity.Relation, error)
    DeleteRelation(ctx context.Context, from, relType, to string) error
}
```

`*entitymanager.Manager` satisfies it structurally.

**Implementation note — import direction:** Mutator references entitymanager
result types (`CreateResult`, `CreateOptions` etc.). Today `entitymanager`
imports `autocascade` (for Cascade Runner). Reverse import is a cycle. Two ways
to resolve:

- (a) Move the result types into a third package (`internal/entitymanager/result`?) that both autocascade and entitymanager import.
- (b) Declare `Mutator` in `entitymanager` instead; autocascade type-imports it.

Pick (b) — it's simpler and the result types staying in entitymanager is honest.
autocascade's only consumer-side narrowing is the *names* of the methods, which
don't require importing the types if the interface lives in entitymanager.
(Actually they do, because the interface body references the types. So (a) is
needed if we want Mutator in autocascade. Or (b) which puts the interface in
entitymanager — but then it's not at the *consumer's* call site. Tension.)

**Decision deferred to implementation:** pick whichever resolves the cycle with
least ceremony. Document the trade-off in the doc comment.

### 2. `autocascade.ScriptRunner.Run` signature

Grows the mutator arg:

```go
type ScriptRunner interface {
    Run(ctx context.Context, action ScriptAction, mutator Mutator) error
}
```

### 3. `autocascade.Request.Mutator` field

Per-request, not per-Runner. Manager populates it when dispatching. Avoids the
construction cycle: Runner is built before Manager exists.

```go
type Request struct {
    // ... existing fields ...
    Mutator  Mutator
    Scripts  ScriptRunner
    Trigger  *entity.Entity
    // ... etc
}
```

`Runner.executeScriptActions` reads `req.Mutator` and passes it to
`scripts.Run(ctx, action, req.Mutator)`.

### 4. `script.LuaScriptRunner` restructure

```go
// internal/script/luascriptrunner.go
type LuaScriptRunner struct {
    exec     Executor
    readDeps lua.ReadDeps  // static: built once at wiring time
}

func NewLuaScriptRunner(exec Executor, readDeps lua.ReadDeps) *LuaScriptRunner

func (l *LuaScriptRunner) Run(_ context.Context, a autocascade.ScriptAction, m autocascade.Mutator) error {
    deps := lua.WriteDeps{
        ReadDeps:      l.readDeps,
        EntityManager: m,  // Manager satisfies both Mutator and EntityManager
    }
    return l.exec.ExecuteCode(..., deps, ...)
}
```

**`lua.WriteDeps.EntityManager` stays the wide `entitymanager.EntityManager`
type for now.** Narrowing it to `autocascade.Mutator` is a separate hygiene PR.
Manager satisfies both so the value flowing through is correct; the
narrowing-at-the-boundary is discipline-only until the lua sweep.

### 5. Delete `wsScriptRunner` and `mcpScriptRunner`

Both go away. Wiring sites construct LuaScriptRunner directly:

```go
scripts := script.NewLuaScriptRunner(engine, readDeps)
em, _ := entitymanager.New(Deps{..., ScriptRunner: scripts})
```

`Manager.runWriteCascade` sets `req.Mutator = m` (Manager-as-self) when building
Request.

## Out of scope

- Narrowing `lua.WriteDeps.EntityManager` to a lua-side consumer interface. Separate hygiene PR.
- TKT-9JEI (appbuild + entry-point migration) — this ticket is its precursor, but TKT-9JEI is its own work.

## Why

After this lands:

- One ScriptRunner adapter exists (`*script.LuaScriptRunner`), not three.
- The cycle is gone — every wiring site can construct services in straight-line order.
- The script-execution contract becomes engine-agnostic in shape too: any future engine (Python, JS) gets a Mutator alongside the action, no engine-specific deps assembly.

## Risks

- **Engine-agnostic boundary leak.** autocascade.Mutator's method signatures reference entitymanager types (CreateResult etc). That's not Lua-specific but it does couple autocascade to entitymanager. Verify the import direction resolves cleanly.
- **Test stub breakage.** Tests that today stub `autocascade.ScriptRunner` will need to update Run signatures.
- **Result type ownership.** May need to relocate `entitymanager.CreateResult`/etc. if the import cycle can't be broken in-place.

## Acceptance criteria

1. `autocascade.Mutator` defined; `*entitymanager.Manager` satisfies it structurally (compile-time assertion).
2. `autocascade.ScriptRunner.Run` signature: `Run(ctx, action, mutator) error`.
3. `autocascade.Request.Mutator` field exists; `Runner.executeScriptActions` passes it through.
4. `script.LuaScriptRunner` takes `lua.ReadDeps` at construction; assembles WriteDeps inside Run from readDeps + mutator.
5. `internal/workspace/wsscriptrunner.go` deleted.
6. `internal/cli/mcp_wiring.go::mcpScriptRunner` deleted.
7. `Manager.runWriteCascade` sets `req.Mutator = m`.
8. `go test -race ./...` clean. `just ci` green.

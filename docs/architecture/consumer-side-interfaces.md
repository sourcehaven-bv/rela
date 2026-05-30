# Consumer-side interfaces for callbacks and cycles

This is the in-depth version of the "Define interfaces at the call site"
rule in the root `CLAUDE.md`. The worked code examples live as godoc on the
real types (`autocascade.Host`, `mcp.Services`, `scheduler.WorkspaceProvider`,
`store.EntityObserver`) — read those alongside this.

## The rule

When service A needs to call back into something that also holds A, do not
reach for the concrete type or a shared interface package. **Define a small
interface in A's package describing the methods A actually invokes, and have
the wiring site supply an implementation.**

This is how rela avoids constructor cycles, keeps interface surfaces small,
and prevents new collaborators from leaking into unrelated test setups.

## Why it matters

If service A imports type B because A calls B's methods, and B imports A as a
dependency, you have a constructor cycle. The reflexive fixes — late-binding
setters, a shared interface package, passing pointers around — add
indirection without solving the design problem. A consumer-side interface
dissolves the cycle by letting A declare *exactly the contract A needs*
without knowing which concrete type satisfies it.

## Three layers, in order

1. **A defines a small interface** (`Host`, `Mutator`, `Provider`, …) in its
   own package, naming only the methods A invokes — typically two to four.
   No options structs A doesn't pass; no methods A never calls.
2. **A's constructor and methods accept that interface** — as a constructor
   field (if A holds the relationship for its lifetime) or as a per-call
   argument (if per-invocation). Per-call is cleaner when the implementer is
   the *caller* of A's method: the cycle disappears entirely because A holds
   no permanent reference.
3. **The wiring site supplies the implementation** — usually the concrete
   type that *also* depends on A. Straightforward, because both types exist
   by the time the wiring runs.

The canonical worked example is `autocascade.Runner` / `Host` — see the
godoc in `internal/autocascade/host.go`. `Manager` satisfies `autocascade.Host`
*structurally*: no import of the interface, no "implements" declaration. The
cycle disappears because `Runner` borrows a Host for the duration of
`Process` rather than holding one.

## When to use which form

- **Per-call argument** when the implementer is the caller of the consumer's
  method (e.g. `autocascade.Host`, `store.EntityObserver`). Fully dissolves
  cycles.
- **Constructor field** when the consumer holds the relationship across many
  calls (e.g. `mcp.Services`, `scheduler.WorkspaceProvider`). Used when there
  is no cycle, just a desire to keep the consumer's contract narrow.

## What this pattern rules out

- **Concrete type back-references** (`A holds *B; B holds *A`).
- **Late-binding setters** (`a.SetB(b)` after construction) — they turn type
  errors into runtime nil-deref bugs.
- **A shared "interfaces" package** that exists only to break cycles — it
  leaks every participating type into every consumer's test imports.
- **Producer-side interfaces** that publish every method a service exposes.
  Go doesn't unify partial implementations of a wide interface into a narrow
  one; the narrow interface has to live with the consumer.

**Test consequence:** a test for the consumer becomes a stub of the narrow
interface — three methods to mock, not the full producer surface.

## Narrow on returns, not just methods

If a consumer-side interface returns a broad type only so the caller can
invoke one or two methods on it, declare those methods on the interface
directly. Returning the wide type is a soft leak.

```go
// Wrong: leaks the whole metamodel + store surface for two narrow uses.
type Host interface {
    Meta() *metamodel.Metamodel  // only used for meta.ValidateRelation(...)
    Store() store.Store          // only used for store.GetEntity(ctx, id)
}

// Right: declares the actual operations.
type Host interface {
    ValidateRelation(relType, fromType, toType string) error
    GetEntity(ctx context.Context, id string) (*entity.Entity, error)
}
```

The payoff is concrete: `autocascade.Host` collapsed its arch-lint footprint
from `[automation, metamodel, store]` to `[automation]` once `Meta()` and
`Store()` were replaced with the methods Runner actually invoked. The
verification question is "where does the consumer call this?" — if the answer
is "one place, one method," that's the method that belongs on the interface.

## Names declare contracts; docs declare invariants

Method names describe *what's requested*. Behavioral constraints belong in
the doc comment, not the name.

```go
// Wrong: name encodes a "must not" into the contract surface.
CreateEntityNoCascade(...) (*entity.Entity, error)

// Right: name describes the operation; doc carries the constraint.
//
// CreateEntity creates a new entity from the supplied options.
//
// Contract: the implementation must NOT fire follow-up automation
// cascades from within this call. Runner schedules cascade evaluation
// on the returned entity; double-cascading would enforce MaxDepth twice.
CreateEntity(...) (*entity.Entity, error)
```

Behavioral negatives in names ("NoX", "WithoutY", "NonZ") are usually the
author prescribing implementation strategy. The doc form is more honest —
implementers satisfy the constraint however they want.

## Transport-specific types belong at adapter layers

If a consumer's interface references types from a specific runtime
(`lua.WriteDeps`, `http.Request`, `*sql.Rows`), the consumer has absorbed
knowledge it doesn't need: its package now imports the runtime's package, and
every alternative implementation must speak that runtime's vocabulary.

The fix: define an abstract interface in the consumer; build a per-request
adapter at the wiring site that holds the transport-specific state.

```go
// Wrong: consumer's interface is shaped by Lua's API.
type Executor interface {
    ExecuteCode(code string, deps lua.WriteDeps, e, old *entity.Entity) error
    ExecuteFile(path string, deps lua.WriteDeps, e, old *entity.Entity) error
}

// Right: consumer declares only what it needs; transport lives in the
// adapter at the wiring site.
type ScriptRunner interface {
    Run(ctx context.Context, action ScriptAction) error
}
```

Cost: one adapter file per transport. Win: the consumer's package doesn't
import the transport's package, and a second transport plugs in without
touching the consumer. Concrete result: `autocascade` stopped importing
`internal/lua` once the `ScriptRunner` adapter landed; the
`entitymanager.Manager → autocascade → lua → entitymanager` cycle dissolved
at the same time.

## Existing examples to study

- **`mcp.Services`** (`internal/mcp/server.go`) — scoped consumer-side
  interface; Workspace satisfies it. Constructor-field form.
- **`scheduler.WorkspaceProvider`** (`internal/scheduler/scheduler.go`) —
  four-method interface for what the scheduler needs. Constructor-field form.
- **`store.EntityObserver`** (`internal/store/store.go`) — the *inverse*
  shape: store calls *out* to its observers; observers declare what they
  implement. Per-call form.
- **`autocascade.Host`** (`internal/autocascade/host.go`) — the cycle-
  dissolving per-call form, with full godoc.

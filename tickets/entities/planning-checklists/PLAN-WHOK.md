---
id: PLAN-WHOK
type: planning-checklist
title: 'Planning: Wire workspace search backend as fsstore Observer (drop Subscribe goroutine)'
status: done
---

## Understanding

- [x] Problem/requirements clearly understood
- [x] Scope defined (what's in/out documented below)
- [x] Acceptance criteria documented with specific test scenarios

**Problem.** After TKT-LCTG, `internal/workspace` keeps the search backend
(`*bleveindex.Index`) in sync with store writes via:

```
workspace.New
  → store.Subscribe(buf=32) → <-chan store.Event + cancel func
  → go searchSubscriptionLoop(ch)
       → on each event: store.GetEntity + backend.EntityPut/EntityDelete
workspace.Close
  → cancel() → close(ch)
  → searchWG.Wait()  // join the goroutine
  → backend.Close()
```

The canonical pattern, already used by
`internal/store/{memstore,fsstore}/conformance_test.go`, is to pass the backend
(a `store.EntityObserver`) directly to the store at construction. The store
invokes observers **synchronously under its write lock** — no goroutine, no
buffered channel, no drop hazard, no close race.

**Scope (in):**

- `app.FSFactory` grows an `Observers []store.EntityObserver` field. `FSFactory.OpenStore(meta)` passes it through to `fsstore.Config.Observers`.
- `Workspace.New`:
  - Construct `searchBackend := bleveindex.NewMem()` **before** calling `factory.OpenStore(meta)`.
  - If the factory is a `*app.FSFactory`, set `factory.Observers = []store.EntityObserver{searchBackend}` before OpenStore.
  - The store now calls `searchBackend.EntityPut/EntityDelete` synchronously on every write — no subscription needed.
  - Backfill loop still runs (initial state, since the store had no observer when its existing files were read at construction).
- `Workspace.NewForTest`: same shape, using `memstore.WithObserver(searchBackend)`.
- Delete from workspace:
  - `(*Workspace).startSearchSubscription`
  - `(*Workspace).searchSubscriptionLoop`
  - `stopSearch` field
  - `searchWG` field
  - `storeEventBufferSize` constant
- Simplify `Workspace.Close`: drop `stopSearch()` / `searchWG.Wait()`; just close the backend.
- Workspace's `mayDependOn` in `.go-arch-lint.yml` stays unchanged — already includes everything needed.

**Scope (out):**

- Changes to `store.Factory` interface (stays as `OpenStore(meta) (Store, error)`). The Observers wiring is an `*app.FSFactory` concern, not part of the abstract Factory contract.
- Changes to `store.EntityObserver`, `bleveindex.Index`, `search.Backend`, `search.Service`.
- Changes to consumer-side `Searcher()` behavior — observable contract unchanged.
- MCP/CLI/dataentry migrations (TKT-KWAX/TKT-0SP1/TKT-9JEI). They still go through `Workspace.Searcher()` and `Workspace.Store()` and don't care how the index sync is wired.

**Acceptance criteria:**

1. `grep -n 'subscriptionLoop\|stopSearch\|searchWG' internal/workspace/*.go` returns zero matches.
2. `*app.FSFactory` accepts an `Observers` field, validated by a unit test.
3. `bridge_sync_test.go::TestFactoryInitialLoad` still passes (initial entities show up in `Searcher().Search()`).
4. A new test asserts incremental syncs: write entity → `Searcher().Search(ctx, Query{Text: ...})` reflects it without any goroutine join.
5. Net LOC negative for code; ~-60 LOC.
6. `just lint`, `just arch-lint`, `just test -race`, full `just ci` all pass.

## Research

- [x] Searched for existing libraries that solve this problem
- [x] Checked codebase for similar patterns or reusable code
- [x] Looked for reference implementations in other projects
- [x] Reviewed relevant rela concepts for prior art

**Reference implementations in repo:**

- `internal/store/fsstore/conformance_test.go:34` — `cfg.Observers = []store.EntityObserver{idx}`. Canonical wiring for fsstore.
- `internal/store/fsstore/recovery_test.go:261` — same pattern after recovery.
- `internal/store/memstore/conformance_test.go:21` — uses `memstore.WithObserver(idx)`.
- `internal/store/fsstore/fsstore.go:68-72` — `Observers []store.EntityObserver` field with doc: "notified synchronously on entity writes (create, update, delete, rename). They are NOT populated from existing entity files on startup — callers that need that behavior can iterate ListEntities after New returns and feed their observer directly."

That last sentence pins the backfill requirement: observers do NOT get
retroactive notifications for existing entities at construction time. The
backfill loop in `Workspace.New` is still required for the initial state.

**Existing tests already validate the pattern.** This is essentially a "use the
same wiring the conformance suite already uses" change for workspace.

## Approach

- [x] Technical approach chosen and documented
- [x] Approach builds on existing patterns (not reinventing)
- [x] Alternatives considered (document why rejected)
- [x] Dependencies identified (packages, APIs, types)

### Design choice — Observers as a struct field, not factory option

`store.Factory.OpenStore(meta)` is intentionally narrow: "give me a Store for
this metamodel." Widening it to accept observer options would contaminate every
Factory implementation (including future ones — e.g. a Postgres-backed factory)
with a concept that's really about hooking in-process observers. Instead:

```go
// internal/app/factory.go
type FSFactory struct {
    FS        storage.FS
    Paths     *project.Context
    Observers []store.EntityObserver  // NEW — synchronously called on writes
}
```

The narrow `store.Factory` interface is unchanged. Callers that need observer
wiring use the concrete `*FSFactory` type at the wiring site; that's already
what `workspace.WithStoreFactory(f)` accepts (it stores `store.Factory` but
receives `*FSFactory` in production).

### Step 1 — Add Observers field to FSFactory

```go
type FSFactory struct {
    FS        storage.FS
    Paths     *project.Context
    Observers []store.EntityObserver
}

func (f *FSFactory) OpenStore(meta *metamodel.Metamodel) (store.Store, error) {
    // ... existing setup ...
    return fsstore.New(fsstore.Config{
        // ... existing fields ...
        Observers: f.Observers,
    })
}
```

### Step 2 — Rewire workspace.New

Current order:

```go
factory := ... // fall back to &app.FSFactory{FS, Paths}
s, _ := factory.OpenStore(meta)
ws, _ := newWorkspace(... s ...)   // creates searchBackend inside
backfillSearchBackend(...)
startSearchSubscription()
```

New order:

```go
factory := ... // same fallback
searchBackend, _ := bleveindex.NewMem()
if fsFactory, ok := factory.(*app.FSFactory); ok && searchBackend != nil {
    fsFactory.Observers = append(fsFactory.Observers, searchBackend)
}
s, _ := factory.OpenStore(meta)
ws, _ := newWorkspace(... s, searchBackend ...)  // takes pre-built backend
backfillSearchBackend(ctx, searchBackend, s)
// NO subscription; observers handle ongoing sync synchronously.
```

The factory type assertion is the load-bearing detail: only `*FSFactory` knows
about observers. A test-time factory or future remote-store factory might not
support observers — in that case, the workspace silently goes without
incremental sync (consistent with today's behavior when `searchBackend == nil`).

Actually, looking more carefully at where the backend is constructed — today
`newWorkspace` creates it internally. Moving that to `New` (above) means
`newWorkspace` needs a new parameter, OR the backend stays in `newWorkspace` and
the assertion-and-append happens before `newWorkspace` is called.

**Cleanest:**

```go
// in New, before factory.OpenStore:
var searchBackend *bleveindex.Index
if idx, err := bleveindex.NewMem(); err == nil {
    searchBackend = idx
} else {
    slog.Warn("failed to create search index", "error", err)
}
if fsFactory, ok := factory.(*app.FSFactory); ok && searchBackend != nil {
    fsFactory.Observers = append(fsFactory.Observers, searchBackend)
}
s, openErr := factory.OpenStore(meta)
// ...
ws, err := newWorkspace(fs, paths, meta, exec, s, searchBackend, opts...)
```

And `newWorkspace` accepts the backend as a parameter, deletes its own
`bleveindex.NewMem()` call:

```go
func newWorkspace(
    fs storage.FS, paths *project.Context, meta *metamodel.Metamodel,
    scriptExec ScriptExecutor, st store.Store,
    searchBackend *bleveindex.Index,
    opts ...Option,
) (*Workspace, error) {
    // ... no internal NewMem() ...
    ws := &Workspace{... searchBackend: searchBackend ...}
    // ... rest same ...
}
```

### Step 3 — Backfill timing

The observer hook is **wired before** `factory.OpenStore(meta)` builds the
store. But fsstore's `syncIndex` (its initial scan of on-disk files) does NOT
call observers (per the doc comment on `Observers`). So the post-OpenStore
backfill loop is still needed — exactly as today.

### Step 4 — NewForTest changes

```go
func NewForTest(meta *metamodel.Metamodel, opts ...TestOption) *Workspace {
    cfg := &testConfig{script: NopScriptExecutor}
    for _, opt := range opts {
        opt(cfg)
    }

    var searchBackend *bleveindex.Index
    if idx, err := bleveindex.NewMem(); err == nil {
        searchBackend = idx
    }

    st := cfg.store
    if st == nil {
        if searchBackend != nil {
            st = memstore.New(memstore.WithObserver(searchBackend))
        } else {
            st = memstore.New()
        }
    }
    // For WithTestStore-supplied stores, the caller is responsible for
    // observer wiring; we cannot retrofit it after construction. The
    // backfill loop still produces a populated index, but new writes
    // won't reach it. Document this caveat on WithTestStore.

    ws, err := newWorkspace(cfg.fs, cfg.paths, meta, cfg.script, st, searchBackend)
    if err != nil { panic(...) }
    if cfg.store != nil && searchBackend != nil {
        if err := backfillSearchBackend(...); err != nil { panic(...) }
    }
    return ws
}
```

The `WithTestStore` caveat is the one real semantic change: a test that supplies
its own store will see *only the initial backfill* in search results, not
subsequent writes through that store. Today the Subscribe-relay also depends on
the test store implementing `Subscribe(...)` — both `memstore` and `fsstore` do
— so this is mostly a no-op in practice, but worth documenting.

### Step 5 — Drop subscription machinery

- Delete `(*Workspace).startSearchSubscription`
- Delete `(*Workspace).searchSubscriptionLoop`
- Delete `Workspace.stopSearch` and `Workspace.searchWG` fields
- Delete `storeEventBufferSize` constant
- Simplify `Workspace.Close`: no `stopSearch()`, no `searchWG.Wait()`.

### Step 6 — Tests

- New `app/factory_test.go::TestFSFactory_Observers` — passing an observer in `FSFactory.Observers` results in synchronous `EntityPut` calls when the store creates entities.
- New `workspace/bridge_sync_test.go::TestObserverSyncWithoutSubscription` — workspace built with a real fsstore, writes propagate to `Searcher()` results without any goroutine.

**Alternatives considered:**

- **Widen `store.Factory.OpenStore` signature.** Rejected. Contaminates every Factory impl.
- **Move observer setup into `fsstore.Config`-aware factory option.** Rejected. The "option" is just a struct field; that's what we use.
- **Skip the `*FSFactory` type assertion and require observers be set via a separate method.** Rejected. The assertion is one line; a method on the Factory interface would be a wider change.
- **Use `Workspace.WithObserver(o)` as a public option.** Rejected. Workspace constructs its own observer (the search backend); external callers don't pass one in.

**Files to modify:**

- `internal/app/factory.go` — `FSFactory.Observers` field + plumb to `fsstore.Config.Observers`.
- `internal/app/factory_test.go` — add test.
- `internal/workspace/workspace.go` — rewire `New` and `NewForTest`; delete subscription methods; simplify `Close`.
- `internal/workspace/bridge_sync_test.go` — add observer-sync test.

**Files NOT modified:**

- `internal/store/factory.go` — interface stays.
- `internal/store/fsstore/*` — fsstore already supports observers.
- `internal/store/memstore/*` — memstore already supports observers.
- `internal/search/*` — no changes.
- `internal/mcp/`, `internal/cli/`, `internal/dataentry/` — no changes.

## Security Considerations

- [x] Input sources identified (user input, config, external APIs)
- [x] Input validation approach defined (allowlist preferred over blocklist)
- [x] Security-sensitive operations identified (file access, auth, crypto)
- [x] Error handling doesn't leak sensitive information

Pure internal refactor. No new input surfaces, no new file/network access.
Search backend is the same `bleve.NewMemOnly` index as today; only the sync
mechanism changes.

Observer callbacks run under the store's write lock. A slow observer
back-pressures writes (today's Subscribe-relay dropped events instead —
acceptable trade for in-memory Bleve which is fast).

## Test Plan

- [x] Test scenarios documented for each acceptance criterion
- [x] Edge cases identified and documented
- [x] Negative test cases defined (invalid input, error conditions)
- [x] Integration test approach defined (not just unit tests)

**Test scenarios:**

1. **AC1 (no subscription code remains)** — `grep` check during PR.
2. **AC2 (FSFactory.Observers field)** — `TestFSFactory_Observers`: passes a stub observer, creates an entity through the resulting store, asserts the stub received `EntityPut`.
3. **AC3 (initial load)** — existing `TestFactoryInitialLoad` passes unchanged (backfill loop populates index).
4. **AC4 (incremental sync without goroutine)** — `TestObserverSyncWithoutSubscription`: create workspace, write entity via store, search for it, get a hit. No `time.Sleep`, no goroutine join.

**Edge cases:**

- **Test store via WithTestStore.** Caller-supplied store may not have the search observer wired. Document on `WithTestStore` and verify in test that existing tests using this option still pass (search hit-rate may differ for incremental updates).
- **bleveindex.NewMem failure.** Backend nil; FSFactory.Observers stays empty; no incremental sync; today's behavior is "search not available" — preserve.
- **Multiple observers on the same store.** Not used by workspace, but the field is a slice so it works.
- **Observer error propagation.** `EntityPut` returns error; fsstore today logs and continues. Same behavior.

**Negative tests:**

- **Close without backend.** Already exercised by tests that don't seed entities.
- **Re-Close.** Idempotent: subsequent calls find nil fields and no-op.

## Risk Assessment

- [x] Technical risks assessed with mitigations
- [x] Security risks assessed (see Security Considerations)
- [x] Effort estimated (xs/s/m/l/xl)

**Risks:**

1. **Type assertion on factory.** Workspace's `factory.(*app.FSFactory)` works for the standard path. Tests that supply a different factory (rare) skip observer setup and fall back to "backfill only, no incremental sync." Mitigation: document on `WithStoreFactory`.
2. **fsstore notify-during-write semantics.** fsstore calls observers synchronously inside the write critical section. If `EntityPut` ever blocked (it doesn't today — Bleve in-memory is fast), it would back-pressure writes. Mitigation: bleve in-memory is fast; if a slow observer is ever added, switch back to an async relay just for that observer.
3. **TKT-KWAX/TKT-0SP1/TKT-9JEI may want to set their own observers.** Once they migrate, each will construct its own backend + factory. The pattern from this ticket becomes the template they follow. Coordination via PR ordering only — no shared code conflicts expected.
4. **WithTestStore tests may rely on incremental sync working through the supplied store.** Check `tools_test.go`, `cli/test_helpers_test.go`, `dataentry/test_helpers_test.go` to see if any test depends on incremental updates being visible via the search index. Likely no — most search-dependent tests build workspace from disk (which goes through `New` → backfill) rather than reaching for in-place writes that need observer pickup.

**Effort:** S — ~80 LOC code change (mostly delete), one factory field, one type
assertion, two new tests.

## Documentation Planning

- [x] User-facing docs identified (skip if internal refactor)
- [x] Docs-checklist will be created when entering implementation

N/A — internal refactor. No CLI flags, no API endpoints, no user-visible
behavior change.

## Design Review

- [ ] Run `/design-review` before starting implementation
- [ ] All critical/significant findings addressed in plan

**Design Review Findings:** &lt;!-- to be filled in after design review --&gt;

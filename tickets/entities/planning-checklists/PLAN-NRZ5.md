---
id: PLAN-NRZ5
type: planning-checklist
title: 'Planning: Migrate workspace from legacy search.Index to bleveindex+search.Service'
status: done
---

## Understanding

- [x] Problem/requirements clearly understood
- [x] Scope defined (what's in/out documented below)
- [x] Acceptance criteria documented with specific test scenarios

**Problem.** `internal/search` carries two parallel implementations:

- **Clean path:** `search.Backend` interface (extends `store.EntityObserver`) → `*bleveindex.Index`. `search.Service` from `search.New(reader, backend)` satisfies `search.Searcher`. Already exercised by `memstore`/`fsstore` conformance tests and a `recovery_test.go`.
- **Legacy path:** `search.Index` (separate type) → `search.NewIndex()` → `Index.Search(words, phrases, limit)`. Used only by `internal/workspace`, with a workspace-private `wsSearcher` adapter that calls a private `w.search()` over `w.searchIdx`.

The duplication produces two `bleveDoc` definitions, two `Search` signatures,
two field-boost tables. Workspace is the sole consumer of the legacy path.
Migrating workspace to the clean path eliminates the duplication and removes the
load-bearing reason TKT-KWAX (MCP off Workspace) was hard to plan: every
consumer can now build its own Searcher in ~10 LOC instead of reproducing
workspace's hidden lifecycle.

**Scope (in):**

- Workspace replaces `searchIdx *search.Index` with `searchBackend *bleveindex.Index`.
- Workspace's construction wires the backend as a `store.EntityObserver` via `store.Subscribe(...)`, mirroring the current `startSearchReindex` order: subscribe-then-backfill is wrong (loses pre-subscription events); backfill-then-subscribe is what we have today and what the new code keeps.
- Workspace's `Searcher()` returns `search.New(w.store, w.searchBackend)` (cached via `sync.Once`).
- Workspace's `Close()` unsubscribes and closes the backend.
- Delete:
  - `internal/workspace/services.go::wsSearcher`, `streamAll`, `streamText`, `typeSetFromQuery`, `makeHitEmitter` (subsumed by `search.Service`).
  - `internal/workspace/workspace.go::(*Workspace).search(words, phrases, limit)` (private), `indexStoreEntities`, `startSearchReindex`, `reindexLoop`, `entityToSearchDocument`, `storeSearchDocuments`.
  - `internal/search/index.go::Index`, `NewIndex`, `Index.Search`, `IndexBatch`, `Index.Index`, `Index.Remove`, `Index.Close`, `SearchSimple`, the legacy `bleveDoc`, `buildMapping`, `boostedFields`, `boostedWordQuery`, `phraseFields`, `buildBoostedPhraseQuery`, `toBleveDoc` — every type/function only reachable from `Index`.
  - The `searchparser.SplitFreeText` function if no callers remain — verify.

**Scope (out):**

- Changes to `bleveindex.Index` API (no new methods).
- Changes to `search.Backend`, `search.Searcher`, `search.Service`, `search.Query`, `search.Hit` (signatures stay as-is).
- Quoted-phrase query semantics. Today `wsSearcher.streamText` splits Query.Text via `SplitFreeText` and passes phrases separately to the legacy backend. The clean `search.Service.Search` already passes `q.Text` straight to `backend.Search(text, limit)`; phrase-quoted strings are tokenized by Bleve as ordinary words. User chose to accept this drift ("just use bleve").
- Migrating MCP/CLI/dataentry to construct their own Searcher. Those are TKT-KWAX/TKT-0SP1/TKT-9JEI. Workspace's `Searcher()` keeps the same observable contract; underneath, it just uses the clean path.

**Acceptance criteria:**

1. `internal/workspace` no longer imports the legacy `search.Index` type. Verified by grep: `grep -n 'search\\.Index\\b' internal/workspace/*.go` returns zero hits.
2. `internal/search/index.go` is reduced to `Document`, `Hit`, `Searcher`, `Backend`, `Query`, `PropertyFilter`, `FilterOp`, `SortClause`, `SortDirection`, `Service`, `New`, `listAll`, `toSet` (plus `MatchFilters` / `MatchText` filter helpers in `filter.go`). The legacy `Index` type and its API are gone.
3. Existing tests pass without weakening: `internal/workspace/bridge_sync_test.go` (initial load + reindex), `internal/search/search_test.go`, `internal/store/memstore/conformance_test.go`, `internal/store/fsstore/conformance_test.go`, `internal/store/fsstore/recovery_test.go`. Any MCP/CLI tests that exercise search still pass.
4. Net LOC negative. Target: ~250 deletions, ~50 additions.
5. `just arch-lint` passes.

## Research

- [x] Searched for existing libraries that solve this problem
- [x] Checked codebase for similar patterns or reusable code
- [x] Looked for reference implementations in other projects
- [x] Reviewed relevant rela concepts for prior art

**Reference implementations in repo:**

- `internal/store/memstore/conformance_test.go:21` — `search.New(s, idx)` where `idx := bleveindex.NewMem()`. The canonical wiring.
- `internal/store/fsstore/conformance_test.go:37` — same pattern.
- `internal/store/fsstore/recovery_test.go:271` — same pattern after store recovery.
- `internal/search/bleveindex/bleveindex.go::Index` — the Backend implementation. `EntityPut(e)` for inserts/updates, `EntityDelete(id)` for deletes. Implements `store.EntityObserver`.

**Subscription pattern.** `store.Store.Subscribe(buf int) (<-chan store.Event,
func())` is the existing mechanism. Today workspace uses it via
`startSearchReindex` with `storeEventBufferSize = 32`. The new code keeps the
same call but the event handler invokes `backend.EntityPut/EntityDelete` instead
of `searchIdx.Index/Remove`.

**Pattern for store-subscribed observers:** `store.EntityObserver` is the
canonical interface for "thing that wants to react to entity events."
`bleveindex.Index` already implements it. The natural call shape is:

```go
ch, cancel := store.Subscribe(bufferSize)
go func() {
    for ev := range ch {
        switch ev.Op {
        case store.EventEntityCreated, store.EventEntityUpdated:
            if e, err := store.GetEntity(ctx, ev.EntityID); err == nil {
                _ = backend.EntityPut(e)
            }
        case store.EventEntityDeleted:
            _ = backend.EntityDelete(ev.EntityID)
        }
    }
}()
```

This is what `startSearchReindex` + `reindexLoop` do today; the new code is the
same shape minus the legacy `entityToSearchDocument` translation step (because
`bleveindex.Index.EntityPut` accepts `*entity.Entity` directly).

## Approach

- [x] Technical approach chosen and documented
- [x] Approach builds on existing patterns (not reinventing)
- [x] Alternatives considered (document why rejected)
- [x] Dependencies identified (packages, APIs, types)

### Step 1 — Wire the new backend at workspace construction

In `newWorkspace` (after store is set, before the entitymanager is built):

```go
var searchBackend *bleveindex.Index
if idx, err := bleveindex.NewMem(); err == nil {
    searchBackend = idx
} else {
    slog.Warn("failed to create search index", "error", err)
}
ws.searchBackend = searchBackend
```

In `New` (after `newWorkspace` returns), the backfill+subscribe block becomes:

```go
if ws.searchBackend != nil {
    if err := backfillSearchBackend(ctx, ws.searchBackend, s); err != nil {
        slog.Warn("failed to backfill search index", "error", err)
    }
    ws.startSearchSubscription()
}
```

where `backfillSearchBackend` iterates `s.ListEntities(ctx,
store.EntityQuery{})` and calls `EntityPut` for each entity — a one-loop
replacement for `indexStoreEntities + storeSearchDocuments`.
`startSearchSubscription` is the renamed `startSearchReindex` with the body
simplified.

### Step 2 — Replace `Searcher()` accessor

```go
func (w *Workspace) Searcher() search.Searcher {
    w.searcherOnce.Do(func() {
        if w.searchBackend == nil {
            w.searcher = errSearcher{} // returns an error-yielding iterator
            return
        }
        w.searcher = search.New(w.store, w.searchBackend)
    })
    return w.searcher
}
```

`errSearcher` is a no-op implementation that yields `(Hit{}, errors.New("search
not available"))` when called. Today's `wsSearcher` returns the same "search
index not available" error when `w.searchIdx == nil`; preserving that behavior.

### Step 3 — Update Close()

```go
func (w *Workspace) Close() error {
    w.StopWatching()
    if w.stopSearch != nil {
        w.stopSearch()
        w.stopSearch = nil
    }
    if w.searchBackend != nil {
        if err := w.searchBackend.Close(); err != nil {
            // log + zero out
        }
        w.searchBackend = nil
    }
    // ... existing store-lifecycle close ...
    return nil
}
```

`bleveindex.Index.Close()` is already defined (line 207 of bleveindex.go).

### Step 4 — Delete legacy code

After steps 1–3 compile and tests pass:

- Remove `wsSearcher`, `streamAll`, `streamText`, `typeSetFromQuery`, `makeHitEmitter` from `services.go`.
- Remove `(*Workspace).search`, `indexStoreEntities`, `startSearchReindex`, `reindexLoop`, `entityToSearchDocument`, `storeSearchDocuments` from `workspace.go`.
- Remove `Index`, `NewIndex`, `Index.Search/IndexBatch/Index/Remove/Close/SearchSimple`, `bleveDoc`, `buildMapping`, `boostedFields`, `phraseFields`, `boostedWordQuery`, `buildBoostedPhraseQuery`, `toBleveDoc` from `internal/search/index.go`. What remains: `Document`, `Service`, `New`, `Search`, `listAll`, `toSet`, plus the existing `types.go` definitions and filter helpers.
- Check `searchparser.SplitFreeText` for remaining callers; if none, delete it.

**Alternatives considered:**

- **Keep both paths, only migrate workspace.** Rejected. Leaves dead `search.Index` code in the package permanently; future readers see two ways to build a search index and pick wrong.
- **Move workspace's `wsSearcher` to `internal/search` as `search.NewLiveIndex`.** Rejected as documented in the prior plan revision. The clean path (`Backend` + `Service`) already does this; reinventing it would just produce a third implementation.
- **Make `Workspace.Searcher()` return `nil` instead of an error-yielding stub when the backend failed to construct.** Rejected. Today's behavior is "search not available" error; preserving it avoids a behavior change.
- **Skip the legacy-code deletion in this PR.** Rejected. Half-migrations are how parallel implementations accumulate. Single PR keeps the migration honest.

**Files to modify:**

- `internal/workspace/workspace.go` — replace search-index wiring; rename/simplify subscription loop.
- `internal/workspace/services.go` — drop `wsSearcher`, replace `Searcher()` body.
- `internal/search/index.go` — large deletion.
- `internal/search/searchparser/parser.go` — drop `SplitFreeText` if unused.
- `internal/search/index_test.go` — delete tests of legacy `Index` API.
- `internal/search/search_test.go` — verify no legacy references remain.
- `internal/workspace/bridge_sync_test.go` — verify it still asserts observable behavior (entity put → search hit; entity delete → no hit).

**Files NOT modified:**

- `internal/search/bleveindex/bleveindex.go` — no changes.
- `internal/search/types.go` — no changes.
- `internal/search/filter.go` — no changes.
- `internal/mcp/`, `internal/cli/`, `internal/dataentry/` — no changes (they use `Searcher()` accessor which keeps working).

## Security Considerations

- [x] Input sources identified (user input, config, external APIs)
- [x] Input validation approach defined (allowlist preferred over blocklist)
- [x] Security-sensitive operations identified (file access, auth, crypto)
- [x] Error handling doesn't leak sensitive information

**Input sources:** Pure refactor; no new input surfaces. Search queries arrive
from MCP/CLI/Lua as before; their tokenization and Bleve handoff move from
legacy to clean path but the trust boundary is unchanged.

**Security-sensitive operations:** None new. The in-memory Bleve index is
`bleve.NewMemOnly` (same as today). No disk writes, no network.

**Error handling:** Today's "search index not available" error message is
preserved. No new error paths introduce sensitive data.

## Test Plan

- [x] Test scenarios documented for each acceptance criterion
- [x] Edge cases identified and documented
- [x] Negative test cases defined (invalid input, error conditions)
- [x] Integration test approach defined (not just unit tests)

**Test scenarios:**

1. **AC1 (workspace no longer imports legacy)** — `grep -n 'search\\.Index\\b' internal/workspace/*.go` returns zero hits. Manual check on PR.
2. **AC2 (legacy types deleted)** — `grep -n 'func NewIndex\\b\\|type Index struct' internal/search/index.go` returns zero hits.
3. **AC3 (tests pass)**:
   - `bridge_sync_test.go::TestFactoryInitialLoad` — workspace built from store with entities; `Searcher().Search(ctx, Query{Text: ...})` returns expected hits.
   - `bridge_sync_test.go` other tests — incremental updates from store events propagate to searcher.
   - `internal/search/search_test.go` — passes against `bleveindex.NewMem` + `search.New` (already does, no changes).
   - `internal/store/{memstore,fsstore}/conformance_test.go` — pass; they already use the clean path.
4. **AC4 (LOC delta)** — `git diff --stat` on the PR shows net deletions.

**Edge cases:**

- **Empty store at construction.** Backfill loop iterates zero entities; subscription waits for events. No special handling needed.
- **Index construction failure (bleveindex.NewMem returns error).** Today's path warns and sets `searchIdx = nil`; `Searcher()` yields "search not available." New path mirrors this: `searchBackend = nil`, error-stub Searcher.
- **Subscription buffer overflow.** Today uses `storeEventBufferSize = 32`. Preserve the constant; if writes burst past the buffer, today's `store.Subscribe` drops events (or blocks — verify behavior). New code inherits whatever today's behavior is.
- **Concurrent close + event delivery.** `Close()` cancels subscription before closing backend; if an event is mid-processing, it completes against the (still-valid) backend before Close()'s `searchBackend.Close()` lands. Preserve today's `stopSearch` cancel semantics.
- **Entity referenced in event but missing from store.** Today's `reindexLoop` calls `GetEntity` and `continue`s on error; new code does the same. Eventually consistent.
- **`*entity.Entity` Clone in `bleveindex`?** `bleveindex.Index.EntityPut` does NOT clone before indexing — it serializes synchronously. Verify by reading the implementation again. (Already verified above: `entityToDoc(e)` reads fields, returns a `bleveDoc` value; no retention of `*entity.Entity` pointer. Safe.)

**Negative tests:**

- **Search before backfill completes.** Workspace's Searcher is lazy (`sync.Once`), backfill is synchronous in `New()` before `New` returns. By the time any caller has a workspace handle, backfill is done. No race.
- **Search after Close.** Today: panic or stale results. New code: `Searcher()` returns the cached `search.Service` whose `backend` is the now-closed `*bleveindex.Index`. Bleve's behavior on Search-after-Close: error. Acceptable; matches today's behavior (closed `search.Index.Search` also errors).

## Risk Assessment

- [x] Technical risks assessed with mitigations
- [x] Security risks assessed (see Security Considerations)
- [x] Effort estimated (xs/s/m/l/xl)

**Risks:**

1. **Search ranking drift.** Legacy `Index.Search` and `bleveindex.Index.Search` build slightly different Bleve queries (different field sets, slightly different ID-boost handling, no phrase queries). Observable behavior change is likely in ranking order, not in result-set membership. Acceptable per user choice. **Mitigation:** spot-check `bridge_sync_test.go` results; if ranking order is asserted, relax to set-membership assertions.
2. **Subscription order.** Workspace today does backfill-then-subscribe. If reversed, events arriving during backfill could be missed. New code preserves the order verbatim. **Mitigation:** keep step ordering identical to today's `New()` body.
3. **`SplitFreeText` deletion fallout.** If a non-workspace caller exists, deleting breaks it. **Mitigation:** grep before deleting; if a non-workspace caller exists, leave the function in place — that's a 6-line function, cheap to keep.
4. **`bridge_sync_test.go` assumptions.** Test may depend on `w.searchIdx` directly or on `wsSearcher` internals. **Mitigation:** read the test first; if it touches internals, rewrite to assert observable behavior through `w.Searcher()`.

**Effort:** S (~250 LOC delete, ~50 LOC add). Single PR.

## Documentation Planning

- [x] User-facing docs identified (skip if internal refactor)
- [x] Docs-checklist will be created when entering implementation

**Documentation Impact:**

- N/A — Internal refactor. Search behavior is functionally equivalent (with the documented ranking drift); no user-facing docs touch search internals.
- CLAUDE.md does not cite the legacy `search.Index` type; no update needed there.

## Design Review

- [x] ~~Run `/design-review` before starting implementation~~ (N/A: parent shipped; back-filled by TKT-5S8T)
- [x] ~~All critical/significant findings addressed in plan~~ (N/A: parent shipped; back-filled by TKT-5S8T)

**Design Review Findings:** &lt;!-- to be filled in after design review --&gt;

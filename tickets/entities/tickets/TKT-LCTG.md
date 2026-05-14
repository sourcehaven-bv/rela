---
id: TKT-LCTG
type: ticket
title: Migrate workspace from legacy search.Index to bleveindex+search.Service
kind: refactor
priority: high
effort: s
status: done
---

## Summary

`internal/search` has two parallel implementations of full-text search:

1. **Clean path:** `search.Backend` interface (extends `store.EntityObserver`),
implemented by `*bleveindex.Index`. `search.Service` (`search.New(reader,
backend)`) wraps a Backend + EntityReader to satisfy `search.Searcher`. Already
exercised by `memstore`/`fsstore` conformance tests.
2. **Legacy path:** `search.Index` (separate type), `search.NewIndex()`,
`Index.Search(words, phrases, limit)`. Used only by `internal/workspace`, which
builds its own `wsSearcher` adapter that reaches into a workspace-private
`w.searchIdx` field.

Workspace is the sole consumer of the legacy path. Migrating it to the clean
path eliminates ~150 LOC of duplication (legacy `search.Index`, legacy
`search.Service`'s Bleve cousin, workspace's `wsSearcher`, workspace's
`startSearchReindex`/`reindexLoop`/`indexStoreEntities`) and clarifies the
search-lifecycle ownership boundary.

## In scope

- Workspace's `searchIdx *search.Index` field becomes `searchBackend *bleveindex.Index` (or stays as `search.Backend` for interface narrowness).
- Workspace's `Searcher()` returns `search.New(w.store, w.searchBackend)` (cached via `sync.Once` as today), not a hand-rolled `wsSearcher`.
- Initial backfill: iterate the store at workspace construction, call `backend.EntityPut(e)` for each entity (replacing `indexStoreEntities` which used `IndexBatch`).
- Incremental updates: subscribe the backend to store events as a `store.EntityObserver` via `store.Subscribe(...)` (or whatever subscription API the store exposes for `EntityObserver` impls). Replaces `startSearchReindex` + `reindexLoop`.
- Workspace's `Close()` closes the backend (it's `io.Closer`) and cancels the subscription.
- Delete `internal/workspace/services.go::wsSearcher`, `streamAll`, `streamText`, `typeSetFromQuery`, `makeHitEmitter` — all subsumed by `search.Service`.
- Delete `internal/workspace/workspace.go::search(words, phrases, limit)` (private), `indexStoreEntities`, `startSearchReindex`, `reindexLoop`, `storeSearchDocuments`, `entityToSearchDocument`.
- Delete legacy `internal/search/index.go::Index`, `NewIndex`, `Index.Search(words, phrases)`, `Index.IndexBatch`, `Index.Index`, `Index.Remove`, `Index.Close`, `SearchSimple` and the legacy `bleveDoc` and `boostedFields` — once workspace stops using them, no consumer remains.

## Out of scope

- Changes to `search.Backend`, `search.Service`, `search.Searcher`, `search.Query`, `search.Hit` interfaces.
- Changes to `bleveindex.Index` implementation.
- Migrating MCP / CLI / dataentry to construct their own search service — those happen in their respective tickets (TKT-KWAX, TKT-0SP1, TKT-9JEI). Workspace's `Searcher()` accessor still works after this ticket; the implementation underneath just goes through the clean path.

## Depends on

- None.

## Why

TKT-KWAX design review caught that "lift search lifecycle from workspace into
the search package" was the load-bearing prerequisite. Closer inspection shows
the lift is already done — `bleveindex` + `search.Service` exist and are the
canonical path. Workspace is the holdout. Migrating workspace closes the gap and
makes downstream consumer migrations mechanical (each consumer instantiates
`bleveindex.NewMem()` + `search.New( store, backend)` + subscribes; ~10 LOC
each).

Killing the legacy `search.Index` also removes a recurring source of confusion:
the two `bleveDoc` definitions, two `Search` signatures, two boost-field tables.
After this PR there is one search path in the codebase.

## Risks

- **Behavioral regression in search ranking.** Legacy `Index.Search(words, phrases, limit)` and `bleveindex.Index.Search(text, limit)` have different query construction (phrase support, exact-ID matching, field-boost weights). Verify with `internal/workspace/bridge_sync_test.go` and any search-behavior tests that the migration preserves observable behavior. Document any acceptable drift.
- **Phrase queries.** Legacy supports `phrases` (quoted phrase matching); `bleveindex` runs `strings.Fields(text)` over the whole query string. Need to verify that `wsSearcher.streamText`'s caller (Lua search bindings, MCP `search_entities`) doesn't depend on phrase support without quoting through to the new Backend. If they do, `bleveindex.Index.Search` may need a phrase-aware variant, OR the callers wrap their queries appropriately.
- **`indexStoreEntities` initial backfill.** Today uses `IndexBatch` (one Bleve batch operation) for performance. The clean path's analog is per-entity `EntityPut` in a loop. For a 10k-entity project this is N round trips through the Bleve API vs 1 batch. Measure; if regression is meaningful, add `bleveindex.Index.IndexBatch` or equivalent batch path on Backend.
- **`store.Subscribe` semantics.** The legacy path subscribes after construction in a goroutine; the clean path needs the Backend to be subscribed *before* it can receive events. Order matters: backfill then subscribe (current order) is correct; reversing leaks events into an empty index.
- **Test coverage.** `bridge_sync_test.go` validates the initial-load + reindex pipeline today. Verify it still passes; if it depended on legacy internals, rewrite to assert observable behavior on the new path.

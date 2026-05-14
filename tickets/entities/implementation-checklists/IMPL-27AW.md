---
id: IMPL-27AW
type: implementation-checklist
title: 'Implementation: Migrate workspace from legacy search.Index to bleveindex+search.Service'
status: done
---

## Implementation

- [x] Unit tests written for new code
- [x] Integration tests written (test the full flow, not just units)
- [x] All edge cases from planning handled
- [x] Code follows project patterns (consumer-side interfaces, etc.)
- [x] No silent failures

**Summary of changes:**

- `internal/workspace/workspace.go` — replaced `searchIdx *search.Index` with `searchBackend *bleveindex.Index`. New helpers: `backfillSearchBackend`, `startSearchSubscription`, `searchSubscriptionLoop`. Deleted legacy `search`, `indexStoreEntities`, `startSearchReindex`, `reindexLoop`, `entityToSearchDocument`, `storeSearchDocuments`, `flattenProperties`.
- `internal/workspace/services.go` — replaced `wsSearcher` (and its 5 helpers) with a tiny `errSearcher` fallback type. `Searcher()` returns `search.New(store, backend)`.
- `internal/search/index.go` — deleted legacy `Index`, `NewIndex`, `bleveDoc`, `Result`, `Document`, helpers. Kept `Service`, `New`, `listAll`, `toSet`.
- `internal/search/index_test.go` — deleted (tested only the now-removed legacy `Index`).
- `internal/search/searchparser/parser.go` — deleted `SplitFreeText` (only caller was `wsSearcher.streamText`).
- `.go-arch-lint.yml` — workspace gains `bleveindex` dependency, drops `searchparser`.

**Manual verification:**

- `go build ./...` — clean
- `go test -race ./...` — all packages pass
- `just lint` — 0 issues
- `just arch-lint` — OK
- `just ci` — full pipeline passes including frontend, e2e, docs

**Acceptance criteria results:**

1. ✅ `grep search.Index internal/workspace/*.go` — zero hits
2. ✅ Legacy `Index` type, `NewIndex` function — deleted from `internal/search/index.go`
3. ✅ Tests pass (bridge_sync, search, memstore/fsstore conformance, all others)
4. ✅ Net LOC: 69 ins, 882 del = **-813 LOC**
5. ✅ `just arch-lint` passes

**Quality:**

- Consumer-side interfaces respected (workspace consumes `search.Backend` indirectly via `search.New`; the new bleveindex import is the storage backend, not a service locator)
- Constructor pattern preserved (`bleveindex.NewMem()` failure degrades to error-stub Searcher, same observable behavior as today's nil-index path)
- Single-phase init preserved (search backend created in newWorkspace, backfilled in New)
- No silent failures (backfill errors logged; subscription cancel idempotent)

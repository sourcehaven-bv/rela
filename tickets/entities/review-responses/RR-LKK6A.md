---
id: RR-LKK6A
type: review-response
title: Six seam files need postgres twins, not four; MCP wiring + memorybackend audit
finding: 'The plan listed 4 seam files. There are actually 6 functions across files that need postgres handling. appbuild: store_postgres.go (openStore) + search_postgres.go (newSearchObserver/asObserver/buildSearcher). cli: mcp_wiring_store_postgres.go (openMCPStore) + mcp_wiring_search_postgres.go (newMCPSearchObserver/asMCPObserver/buildMCPSearcher). The MCP search seam was under-specified in the plan. Exact signatures must match the !postgres&&!memorybackend variants. Crucially: the FS search files are tagged `!postgres && !memorybackend`, so they''re already excluded under -tags postgres; but there is NO memorybackend search seam (memorybackend reuses... need to confirm) — verify the build graph compiles under each of the 3 tag combinations (default, memorybackend, postgres) with no missing/duplicate symbols.'
severity: significant
resolution: 'Implemented (commit 4117b2ec): the build-tag boundary was lifted to one New recipe per scenario (appbuild_{fs,memory,postgres}.go) over shared prepare()/assemble(), and the same for cli/mcp_wiring_{fs,memory,postgres}.go — superseding the ''six seam functions'' approach with a cleaner top-level split. CI compiles all 3 tag combos (default/memorybackend/postgres); the postgres CI job asserts it.'
status: addressed
---

## Resolution (plan update)

Create exactly these `//go:build postgres` files with signatures matching the FS
twins:
- `internal/appbuild/store_postgres.go` — `openStore(fs, paths, meta, obs) (store.Store, error)`
- `internal/appbuild/search_postgres.go` — `newSearchObserver() *pgSearch`, `asObserver(*pgSearch) store.EntityObserver`, `buildSearcher(ctx, st, *pgSearch) (search.Searcher, io.Closer, error)`
- `internal/cli/mcp_wiring_store_postgres.go` — `openMCPStore(...)` matching FS
- `internal/cli/mcp_wiring_search_postgres.go` — `newMCPSearchObserver()`, `asMCPObserver(...)`, `buildMCPSearcher(...)`

Note the search seam returns a **concrete** type that is threaded into both
`asObserver` and `buildSearcher`; pick one concrete pg type (or reuse the store
handle). The pg search backend likely lives in the same DB as the data, so the
observer can be a no-op/nil (`asObserver(nil) -> nil`, `buildSearcher` skips
backfill) — but it still needs the store handle to run queries; resolve how
`buildSearcher` reaches the pg connection (probably via `st`).

**Verification gate:** CI compiles all three tag combinations: `go build ./...`,
`go build -tags memorybackend ./...`, `go build -tags postgres ./...`.

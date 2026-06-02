---
id: IMPL-EX6AU
type: implementation-checklist
title: 'Implementation: PostgreSQL store + search backend with build-flag variants'
status: in-progress
---

<!-- @managed: claude-workflow v1 -->

## Staged delivery

This XL ticket lands in stages on branch `feat/postgres-store-TKT-M8400`:

- [x] **Stage A — pgstore core + conformance** (commit 296c5f3f). pgstore
implements `store.Store` + `search.Backend` over an injected pgx pool; embedded
migrations; per-schema test isolation. Full `storetest.RunAll` + 6 fuzz
functions pass with `-race` against live PostgreSQL.
- [ ] **Stage B — build seams + DSN wiring.** `//go:build postgres` companions
for the 6 seam functions (appbuild + cli mcp_wiring); `appbuild.WithDatabaseURL`
  + `openStoreParams`; `--database-url` flag / `RELA_DATABASE_URL`; arch-lint
appbuild/cli mayDependOn pgstore; compile all 3 tag combos.
- [ ] **Stage C — CI / release / docs.** goreleaser 2 extra builds (`-postgres`);
CI postgres service-container job + `go list -deps` no-bleve assertion; docs
(deployment, CLAUDE.md, README); justfile targets.

## Development

- [x] Unit tests written for new code — conformance suite (storetest.RunAll)
exercises every method; per-schema isolation in testdb_test.go.
- [x] Integration tests written — full store flow + search via searchFactory;
fuzz incl. FuzzConcurrentOps under `-race`.
- [x] Happy path implemented — all 25 store.Store methods + search.Backend.
- [x] Edge cases from planning handled — empty store, ErrConflict/ErrNotFound/
ErrHasRelations, cascade delete, rename, unicode/special chars (fuzz), nested
JSONB round-trip (int preservation via UseNumber+normalize), watcher
drop-when-full, double-close.
- [x] Error handling in place — pgx errors surfaced; ErrNoRows mapped to
ErrNotFound/ErrConflict; no swallowed errors.

## Test Quality

- [x] Using fixture builders / shared conformance kit (storetest) for test data
- [x] No hardcoded values where object in scope (conformance kit asserts vs seeded objects)
- [x] Only specifying values that matter
- [x] Interpolated values constructed from objects
- [x] Property comparisons use original object

## Manual Verification

- [x] Feature manually tested end-to-end — ran full suite + fuzz against a real
Postgres.app 15 instance (rela_test DB, pg_trgm 1.6).
- [x] Each acceptance criterion verified — see evidence.
- [x] Edge cases manually verified — fuzz + conformance subtests.

**Verification Evidence (Stage A):**

```
$ RELA_TEST_DATABASE_URL=postgres://jeroen@127.0.0.1:5432/rela_test?sslmode=disable \
    go test -race ./internal/store/pgstore/
ok  github.com/Sourcehaven-BV/rela/internal/store/pgstore  6.6s
```

- `TestConformance` (storetest.RunAll, Capabilities{Attachments:true}) — PASS,
~100 subtests incl. entity/relation/query/pagination/watcher/attachment/
validation/search, each with a fresh per-schema empty store.
- All 6 Fuzz* seed corpora PASS under `-race`; 20s active fuzz of
FuzzConcurrentOps (1400+ execs) PASS (after capping test pool size — see below).
- AC4 (default build unaffected): `go list -deps ./cmd/rela-server` →
0 pgx packages; fsstore/memstore conformance still green.
- AC3 (search): RunSearchTests passes — pg substring over search_text matches
MatchText semantics (id + content + string props); Service applies filters.
- Bug found & fixed during verification: JSONB numbers round-tripped as float64
(conformance expects int) → unmarshalProps uses json UseNumber + normalize.
- Harness note: active fuzzing exhausted Postgres connections (SQLSTATE 53300);
fixed by capping test pools (MaxConns=2) + shared-pool/TRUNCATE for the fuzz
factory. Production uses one pool; not a pgstore defect.

## Quality

- [x] Code follows project patterns — mirrors memstore; reuses storeutil
validators/cursors; connection injection per RR-P13ZK; consumer-side DBTX iface.
- [x] Checked for DRY — shared scan helpers, search_text builder, where-builders.
- [x] No security issues — all queries parameterized ($1..); LIKE wildcards
escaped; search uses lowercased column (no raw to_tsquery on user input yet).
- [x] No silent failures — errors returned; observer errors intentionally
ignored matching memstore (`_ = o.EntityPut`).
- [x] No debug code left behind.

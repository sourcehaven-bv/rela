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
- [x] **Stage B — build seams + DSN wiring** (commit 4117b2ec). Lifted the
backend build-tag boundary to ONE `New` recipe per scenario
(`appbuild_{fs,memory,postgres}.go`) over shared `prepare()`/`assemble()`,
replacing the deep 3-seam dance — so the postgres build creates one pgx pool
shared by store+search (the seam the old shape couldn't express). Same for
`cli/mcp_wiring_{fs,memory,postgres}.go`. `appbuild.Config` carries the DSN;
`WithDatabaseURL` + `--database-url` (rela-server + kong global) /
`RELA_DATABASE_URL` resolve it (flag wins). arch-lint: appbuild/cli `mayDependOn
pgstore` + `canUse pgx`.
- [ ] **Stage C — CI / release / docs.** goreleaser 2 extra builds (`-postgres`);
CI postgres service-container job + `go list -deps` no-bleve assertion; docs
(deployment, CLAUDE.md, README); justfile targets.

**Verification Evidence (Stage B):**

- All 3 tag combos build the whole module incl. cmd/: `go build` /
`-tags memorybackend` / `-tags postgres ./...` — clean.
- Dependency isolation (AC1/AC4): `go list -deps -tags postgres ./cmd/rela{,-server}`
→ 0 bleve, 11 pgx pkgs; default `./cmd/rela-server` → 0 pgx, 41 bleve.
- lint clean under all 3 tags; arch-lint OK; full default `go test -race ./...`
→ 60 packages OK, no regressions from the composition-root refactor.
- E2E (AC5 mini): `go build -tags postgres -o rela-pg ./cmd/rela`; `init` +
`migrate`, then `create requirement` ×2 → rows land in Postgres (REQ-001/002,
seq 1/2), `entities/` dir stays empty (DB-backed), `list` reads them back,
schema_version=1 (auto-migrated on first open). Invalid `--database-url` with a
password → error redacts the password (`user=u database=nope`).

## Development

- [x] Unit tests written for new code — conformance suite (storetest.RunAll)
exercises every method; per-schema isolation in testdb_test.go.
- [x] Integration tests written — full store flow + search via searchFactory;
fuzz incl. FuzzConcurrentOps under `-race`; postgres CLI e2e (Stage B).
- [x] Happy path implemented — all 25 store.Store methods + search.Backend;
3 build-tag recipes wired through to cmd/.
- [x] Edge cases from planning handled — empty store, ErrConflict/ErrNotFound/
ErrHasRelations, cascade delete, rename, unicode/special chars (fuzz), nested
JSONB round-trip (int preservation via UseNumber+normalize), watcher
drop-when-full, double-close, missing/invalid DSN (redacted).
- [x] Error handling in place — pgx errors surfaced; ErrNoRows mapped to
ErrNotFound/ErrConflict; no swallowed errors.

## Test Quality

- [x] Using fixture builders / shared conformance kit (storetest) for test data
- [x] No hardcoded values where object in scope (conformance kit asserts vs seeded objects)
- [x] Only specifying values that matter
- [x] Interpolated values constructed from objects
- [x] Property comparisons use original object

## Manual Verification

- [x] Feature manually tested end-to-end — full suite + fuzz + postgres CLI e2e
against a real Postgres.app 15 instance (rela_test DB, pg_trgm 1.6).
- [x] Each acceptance criterion verified — see evidence (Stage A + B).
- [x] Edge cases manually verified — fuzz + conformance subtests + DSN errors.

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
FuzzConcurrentOps (1400+ execs) PASS (after capping test pool size).
- AC3 (search): RunSearchTests passes — pg substring over search_text matches
MatchText semantics (id + content + string props); Service applies filters.
- Bug found & fixed: JSONB numbers round-tripped as float64 (conformance
expects int) → unmarshalProps uses json UseNumber + normalize.
- Harness note: active fuzzing exhausted Postgres connections (SQLSTATE 53300);
fixed by capping test pools + shared-pool/TRUNCATE fuzz factory. Production uses
one pool; not a pgstore defect.

## Quality

- [x] Code follows project patterns — mirrors memstore; reuses storeutil
validators/cursors; connection injection per RR-P13ZK; consumer-side DBTX iface;
build-tag boundary lifted to top-level recipes over shared prepare/assemble.
- [x] Checked for DRY — shared scan helpers, search_text builder, where-builders;
prepare()/assemble() shared across all 3 build recipes (the invariant that keeps
recipes from drifting).
- [x] No security issues — all queries parameterized ($1..); LIKE wildcards
escaped; search uses lowercased column (no raw to_tsquery on user input); DSN
password redacted in errors.
- [x] No silent failures — errors returned; observer errors intentionally
ignored matching memstore (`_ = o.EntityPut`).
- [x] No debug code left behind.

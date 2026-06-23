---
id: PLAN-LUXFP
type: planning-checklist
title: 'Planning: PostgreSQL store + search backend with build-flag variants'
status: done
---

<!-- @managed: claude-workflow v1 -->

## Understanding

- [x] Problem/requirements clearly understood
- [x] Scope defined (what's in/out documented below)
- [x] Acceptance criteria documented with specific test scenarios

**Scope:**

IN scope:
- `internal/store/pgstore`: a `store.Store` implementation backed by PostgreSQL
satisfying all sub-interfaces, passing the full `storetest.RunAll` conformance
suite (incl. fuzz + attachments). **`pgstore.New(db DBTX, opts...)` accepts an
injected pgx pool handle — it does NOT own a DSN/connection** (RR-P13ZK).
- PostgreSQL-backed search `Backend` indexing inside the DB (tsvector + GIN,
plus pg_trgm for fuzzy/wildcard), wired via the existing seam. Backend returns
IDs only; the Service applies filters/types/limit (RR-889RK). `postgres` build
does NOT link bleve.
- `//go:build postgres` companion files for the SIX seam functions across FOUR
files (RR-LKK6A): `appbuild/store_postgres.go`, `appbuild/search_postgres.go`,
`cli/mcp_wiring_store_postgres.go`, `cli/mcp_wiring_search_postgres.go`.
- pgx/v5 driver; embedded SQL migrations (embed.FS + schema_version table),
applied against the injected handle, guarded by a version check.
- DSN threaded via `appbuild.WithDatabaseURL` Option + a shared `openStoreParams`
struct (NOT cmd-flags alone — RR-E7WNC). `--database-url` > `RELA_DATABASE_URL`.
Password redacted from logs. The wiring layer builds the `*pgxpool.Pool` and
passes it to `pgstore.New`; the store never sees the DSN.
- CI: a PostgreSQL service-container job running conformance + integration tests
(race on). goreleaser: FS stays `rela`/`rela-server`; postgres ships as
`rela-postgres`/`rela-server-postgres` (suffixed binary + archive).
- **Forward-compat hook:** every entity/relation row carries `created_at`,
`updated_at`, and a per-schema monotonic `seq` (sequence `rela_seq`), so a
future LISTEN/NOTIFY-with-catchup can reconcile from a watermark WITHOUT a
schema migration.

OUT of scope (deferred):
- Multi-writer / multi-process correctness. Target is a **single rela-server
process owning the database**. Watcher is in-process only (mirrors fsstore/
memstore, intentionally LOSSY — RR-9LYNN). Cross-process notification is a
follow-up; timestamp/seq columns make it additive.
- **Metamodel stays on disk even in the postgres build** (RR-WFF7Z). A filesystem
project dir with `metamodel.yaml` (+ templates/, .rela/) is still required;
PostgreSQL backs entities/relations/attachments/search only.
- `rela-desktop` postgres variant (desktop stays FS).
- FS->PostgreSQL data migration tooling.
- Connection pooling tuning / HA / replication topology.

**Acceptance Criteria:**

1. `go build -tags postgres ./cmd/rela ./cmd/rela-server` works; CI `go list -deps
-tags postgres ./cmd/rela-server` asserts NO bleve. Compile all THREE tag combos
(default, memorybackend, postgres) — RR-LKK6A.
2. `pgstore` passes `storetest.RunAll(...)` + all six Fuzz* with `-race` against a
real PostgreSQL, each of ~100 factory calls yielding a fresh empty store via
per-schema isolation (RR-U9RFH). **Test:** `pgstore/conformance_test.go`
(skipped with a clear message if `RELA_TEST_DATABASE_URL` unset).
3. PostgreSQL search satisfies the 21 RunSearchTests cases (case-insensitive
substring over ID/content/string-props; Backend returns IDs only — RR-889RK).
4. Default (FS) build behaviorally unchanged; `go list -deps` default still has
bleve, not pgx.
5. rela-server (postgres build) starts against a DSN AND a project dir (metamodel
on disk — RR-WFF7Z), migrates idempotently, serves the API. **Test:**
integration test: boot against service-container DB + fixture project dir;
create -> search -> read -> delete.
6. Release publishes 4 binaries. **Test:** goreleaser `--snapshot` dry-run in CI.

## Research

- [x] ~~run `/research`~~ (N/A: contract fully defined by existing seams +
conformance harness; design-review verified it against real code,
RR-U9RFH..RR-P13ZK)
- [x] Searched libraries (pgx/v5; lib/pq rejected)
- [x] Checked codebase patterns (verified file:line in design review)
- [x] Reference implementations (fsstore/memstore mirror)
- [x] Reviewed concepts (store-backends, ci-pipeline)

**Research Doc:** N/A

**Existing Solutions:**

- **Driver:** `github.com/jackc/pgx/v5` (+ `pgxpool`). Native, CGO_ENABLED=0 OK.
- **Migrations:** embedded `.sql` (embed.FS) + `schema_version` table; goose/
golang-migrate rejected.
- **Full-text:** `tsvector`+GIN (ranked) + `pg_trgm` for fuzzy/wildcard. Use
`plainto_tsquery`/`websearch_to_tsquery` (NOT raw `to_tsquery`). Conformance
only exercises substring (MatchText); fuzzy/wildcard is CLI/API parity, document
divergence (RR-889RK).
- **Patterns mirrored (verified file:line):** `fsstore/conformance_test.go:30-42`;
`appbuild/store_fs.go`, `store_memorybackend.go`; `appbuild/search_fs.go`;
`cli/mcp_wiring_{store,search}_fs.go`; `search/types.go` +
`search/index.go:30-70`; `store/store.go:21-26` (errors incl.
`ErrHasRelations`).

## Approach

- [x] Technical approach chosen and documented
- [x] Approach builds on existing patterns (build-tag seams + conformance kit)
- [x] Alternatives considered (LISTEN/NOTIFY now — deferred; lib/pq, goose —
rejected; transaction-rollback test isolation — rejected, RR-U9RFH)
- [x] Dependencies identified (pgx/v5 + pgxpool, pg_trgm extension, Postgres >= 13)

**Technical Approach:**

1. **Connection injection** (RR-P13ZK): define a small `DBTX` interface
(`Exec`/`Query`/`QueryRow`/`Begin`) satisfied by
`*pgxpool.Pool`/`pgx.Conn`/`pgx.Tx`. `pgstore.New(db DBTX, opts...)` validates
`db != nil` and owns no DSN. Prod and tests both inject a **pool** (a bare
`pgx.Tx` would serialize ops and break rename/cascade + FuzzConcurrentOps). The
wiring layer owns/closes the pool; `store.Close()` only tears down the watcher.

2. **Schema** (`migrations/0001_init.sql`):
   - `entities(id TEXT PK, type TEXT, properties JSONB, content TEXT, created_at
TIMESTAMPTZ, updated_at TIMESTAMPTZ, seq BIGINT)`.
   - `relations(from_id, rel_type, to_id, data JSONB, created_at, updated_at, seq)`
PK `(from_id, rel_type, to_id)`; referential integrity enforced in tx (no hard
FKs).
   - `attachments(entity_id, property, file_name, content_type, size, bytes BYTEA,
created_at, updated_at, seq)` PK `(entity_id, property)`.
   - `schema_version(version INT)`; sequence `rela_seq` stamped on every write.
   - GIN tsvector index over `id || title || properties-text || content`; pg_trgm
GIN index for fuzzy/wildcard.

3. **pgstore** mirrors fsstore's method set via pgx. Map DB `updated_at` ->
`entity.Entity.UpdatedAt`/`Relation.UpdatedAt` (RR-VB27Y). `iter.Seq2` listings
stream rows. Pagination keyset/offset -> `Page[T]{Items,NextCursor}`; iterator
variants ignore Cursor/Limit. `HighestID` parses numeric suffix after `PREFIX-`,
max, 0 if none. `PropertyValues` GROUP BY ... ORDER BY count DESC.
`RenameEntity` one tx rewriting PK + relation endpoints ->
`RenameResult{RelationsUpdated}`. `DeleteEntity(false)` with relations ->
`ErrHasRelations`; `(true)` -> `DeleteResult` + per-row cascade events. Dup ->
`ErrConflict`; missing -> `ErrNotFound`. JSONB round-trips nested values
unmutated.

4. **Watcher** (RR-9LYNN): per-subscriber buffered channel, non-blocking
fire-and-forget send, fan-out, idempotent cancel, `Close()` closes all channels.
Emit AFTER commit. NO echo tracker.

5. **Search backend** (RR-889RK): implements `search.Backend`; `Search(text, limit)`
returns IDs only (case-insensitive substring over id/content/string props;
tsvector/trgm for ranking/fuzzy). Lives in the same DB on the injected handle;
`EntityPut`/`EntityDelete` no-ops (or maintain a tsvector column).
`buildSearcher` reaches the connection via the same handle.

6. **DSN threading** (RR-E7WNC): `appbuild.WithDatabaseURL` Option; extend the
openStore seam with an `openStoreParams` struct (shared signature across all 3
builds). In the postgres build, `appbuild` builds `*pgxpool.Pool` from the
resolved DSN, runs migrations, calls `pgstore.New(pool)`. Resolve
`--database-url`
   > `RELA_DATABASE_URL`. CLI flag in `cli/kong.go` globals; `rela-server` adds
   > `-database-url`; MCP + desktop read env. Redact password.

7. **Metamodel on disk** (RR-WFF7Z): postgres build still `project.Discover` +
`metamodel.NewFSLoader(paths.MetamodelPath)`. `--project` stays required.

8. **Test isolation** (RR-U9RFH): one shared pool per test package; per factory
call `CREATE SCHEMA test_<name>_<n>`, a pool view with `BeforeAcquire`/
`AfterConnect` `SET search_path TO <schema>`, migrate into it, `pgstore.New`,
`t.Cleanup(DROP SCHEMA CASCADE)`. searchFactory uses the same scoped handle.
Per-schema gives fresh `rela_seq` + parallel safety; production code path
unchanged. Measure ~100-cycle cost; optimize via template-clone or
TRUNCATE-fallback only if slow.

9. **CI/release:** `pgstore` CI job (`postgres:16` service container, wait-for-ready);
compile all 3 tag combos; `go list -deps` assertions both directions; goreleaser
2 extra builds (`flags: [-tags=postgres]`, `-postgres` names).

**Files to modify / create:**

- NEW `internal/store/pgstore/`: `pgstore.go` (DBTX + New), `entity.go`,
`relation.go`, `attachment.go`, `watcher.go`, `search.go`, `migrate.go`,
`migrations/0001_init.sql`, `conformance_test.go`, `testdb.go` (shared pool +
per-schema helper), integration tests.
- NEW `internal/appbuild/store_postgres.go`, `search_postgres.go`.
- NEW `internal/cli/mcp_wiring_store_postgres.go`, `mcp_wiring_search_postgres.go`.
- EDIT openStore seam -> `openStoreParams` (all 3 variants + callers in
appbuild.New, cli/mcp_wiring.go).
- EDIT `internal/appbuild/appbuild.go` (WithDatabaseURL Option, pool build+close
in postgres seam).
- EDIT `internal/cli/kong.go` (global `--database-url`), `cmd/rela-server/main.go`
(`-database-url`).
- EDIT `.goreleaser.yaml`, `.github/workflows/ci.yml`, `release.yml`.
- EDIT `justfile` (build-cli-postgres, build-server-postgres, test-postgres),
`go.mod` (pgx/v5), `.testcoverage.yml` (pgstore floor), `.go-arch-lint.yml`
(pgstore component + pgx vendor + appbuild/cli mayDependOn).
- EDIT `docs/` + CLAUDE.md + README.

## Security Considerations

- [x] Input sources identified
- [x] Input validation approach defined (parameterized queries; sanitized tsquery)
- [x] Security-sensitive operations identified
- [x] Error handling doesn't leak sensitive information (DSN password redacted)

**Input Sources & Validation:**

- **DSN**: parsed by pgx; on failure surface a sanitized error with password
REDACTED. Env preferred over flag (avoids `ps`/history leak).
- **IDs, property keys, search text** reach SQL -> **always pgx parameterized
($1,$2); never concatenation.** Primary SQLi defense.
- **Search text -> tsquery**: `plainto_tsquery`/`websearch_to_tsquery` or
parameterized trgm `%`; NEVER raw `to_tsquery`. Negative test: `&|!():*` input.
- **JSONB**: driver-serialized.

**Security-Sensitive Operations:**

- TLS via DSN `sslmode`; no silent downgrade; document prod value.
- Migrations: only embedded code-reviewed DDL; no user-supplied path.
- Attachments BYTEA — preserve existing size limits.

## Test Plan

- [x] Test scenarios documented for each acceptance criterion
- [x] Edge cases identified and documented
- [x] Negative test cases defined
- [x] Integration test approach defined

**Test Scenarios (mapped to AC):**

- AC1/AC4: CI `go list -deps -tags postgres` greps OUT bleve; default greps OUT
pgx; compile all 3 tag combos.
- AC2: `pgstore/conformance_test.go` -> `RunAll` + 6 Fuzz, `-race`, real DB,
per-schema fresh store. Skipped (clear msg) if `RELA_TEST_DATABASE_URL` unset.
- AC3: `RunSearchTests` via searchFactory + unit tests (exact/prefix/substring/
fuzzy/wildcard/empty-lists-all/case-insensitivity). Empty-query path is
Service.listAll, not the backend (RR-889RK).
- AC5: boot rela-server against service-container DB + fixture project dir;
create->search->read->delete; migrate twice -> idempotent.
- AC6: `goreleaser release --snapshot --clean` asserts 4 archives.

**Edge Cases:** empty store (lists/search/count empty, LastModified zero,
HighestID 0); concurrent writers (`FuzzConcurrentOps` + `-race`, rely on tx);
unicode/special/null-byte (`FuzzPropertyValuesTypeZoo`,
`FuzzRelationKeyCollision`); relation key collision & rename collapse ->
`ErrConflict`; nested JSONB unmutated (`FuzzCloneNestedValues`); non-cascade
delete with relations -> `ErrHasRelations`; large/missing attachment; watcher
full-buffer drop + Close-closes-channels + cascade events; connection drop ->
surfaced error; migration-at-target -> no-op.

**Negative Tests:** Create dup -> `ErrConflict`; Update/Get/Delete missing ->
`ErrNotFound`; invalid DSN -> clear error, no password leak; tsquery-hostile
input -> no SQL error.

**Integration test approach:** service-container Postgres in CI; locally gated
on `RELA_TEST_DATABASE_URL`. `just test-postgres` runs the postgres-tagged suite
against a dev-provided docker Postgres. Per-test isolation via per-schema +
`search_path` + `DROP SCHEMA CASCADE` (RR-U9RFH).

## Risk Assessment

- [x] Technical risks assessed with mitigations
- [x] Security risks assessed
- [x] Effort estimated

**Risks:**

- R1 bleve in postgres build -> CI `go list -deps` gate.
- R2 silent drift on 2nd writer -> document single-process; seq/updated_at make
catchup additive.
- R3 ~100 factory calls leak state (RR-U9RFH) -> per-schema isolation + DROP
CASCADE; measure, fall back to TRUNCATE if slow.
- R4 fuzzy/wildcard parity (RR-889RK) -> conformance needs only substring; document
divergence.
- R5 CGO/build -> pgx pure-Go; verify CGO_ENABLED=0 in CI.
- R6 go-arch-lint -> add pgstore component, pgx vendor, mayDependOn; run early.
- R7 service-container flakiness -> wait-for-postgres health-check.
- R8 DSN seam signature (RR-E7WNC) -> shared `openStoreParams` struct.
- R9 pool ownership/leak (RR-P13ZK) -> appbuild owns+closes pool; store.Close only
tears down watcher; verify no goroutine/conn leak under `-race`.

**Effort:** XL.

## Documentation Planning

- [x] User-facing docs identified
- [x] Docs-checklist will be created when entering implementation

**Documentation Impact:**

- [x] User guide / reference — deploying rela-server with PostgreSQL (DSN, sslmode,
migrations, postgres build, **still needs a project dir for metamodel**).
- [x] CLI help — `--database-url`.
- [x] CLAUDE.md — `postgres` build tag, third store/search backend, connection-
injection (`pgstore.New(DBTX)`), single-writer constraint + catchup hook,
metamodel-on-disk.
- [x] README — FS vs PostgreSQL builds.
- [x] API docs — N/A.

## Design Review

- [x] Run `/design-review` before starting implementation
- [x] All critical/significant findings addressed in plan

**Design Review Findings (all incorporated above):**

- RR-U9RFH (critical) — ~100 factory calls need fresh empty store -> per-schema
isolation via injected handle + `search_path` + DROP CASCADE.
- RR-P13ZK (significant) — `pgstore.New(db DBTX)` connection injection (pool, not tx);
appbuild owns pool + builds it from DSN.
- RR-889RK (significant) — search Backend returns IDs only; Service does filtering.
- RR-LKK6A (significant) — 6 seam functions / 4 files; compile 3 tag combos.
- RR-E7WNC (significant) — DSN via appbuild Option + shared openStore seam.
- RR-WFF7Z (significant) — metamodel stays on disk; postgres build needs project dir.
- RR-9LYNN (minor) — watcher lossy/unordered; replicate exactly; no echo tracker.
- RR-VB27Y (minor) — exact contract types (ErrHasRelations, structs, UpdatedAt).

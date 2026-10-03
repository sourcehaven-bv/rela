---
paths:
  - "internal/store/**"
  - "internal/appbuild/**"
  - "internal/state/**"
  - "internal/queryplan/**"
  - "internal/cli/db*.go"
  - "internal/cli/mcp_wiring_*.go"
---

# Storage backends

Rules when touching this:

- **The `postgres` build must not link bleve; the default build must not link
  pgx; no build but `sqlite` may link `modernc.org/sqlite`.** CI asserts each of
  these via `go list -deps` (the `postgres` job in `ci.yml`). Keep
  backend-specific imports inside the tagged recipe files.
- **`pgstore.New(db DBTX)` takes an injected pgx pool**, not a DSN. The postgres
  recipe builds one pool, runs `pgstore.Migrate`, and shares it between the
  store and the in-DB search backend. appbuild owns/closes the pool;
  `store.Close()` only tears down the watcher.
- **Build-agnostic wiring lives in `prepare`/`assemble`, never in a recipe.** A
  recipe may choose and order backend steps; if logic would be copy-pasted
  between recipes, it belongs in a shared helper. This is what keeps the three
  recipes from drifting (and where future per-backend audit/ACL variation goes).

  `prepare`'s result is the exported **`appbuild.SharedBase`** (TKT-P938T7): the
  tenant-independent half — validated config, options, parsed `acl.yaml`, loaded
  metamodel — with nothing derived from a store. Build one with `NewSharedBase`
  and call `base.Assemble(store, …)` once per store; `New`/ `Discover` are that
  path with a single store. **The split is NOT along the `Services` field
  list**: `acl.Declarative` is built FROM the store (it needs a store-backed
  `acl.Graph`), so the ACL _policy_ is shared while the _evaluator_ is per-store
  — same for `lua.ReadDeps`. Two invariants keep reuse safe, both pinned by
  tests in `sharedbase_test.go`: assembly must never mutate `meta` or
  `aclPolicy` (they are pointers handed to every assembled `Services`, so a
  write leaks across tenants), and `Services.Close` must tear down only the
  store and search closer it was assembled with — never anything shared, or
  evicting one tenant breaks its siblings.
- **The metamodel is always read from disk**, even in the postgres build —
  `schema.yaml` and `templates/` stay on the filesystem, as does
  operator-authored config generally; PostgreSQL backs
  entities/relations/attachments/search. A postgres deployment still needs a
  `--project` dir.

  The exception is **runtime-written state** (TKT-VC27L3): on the postgres build
  `state.KV` is database-backed (`pgstore.StateKV`, wired via `stateKVFor`), so
  the document render cache, user settings, the operator logo/theme and the
  CalDAV alias table live in the `state_kv` table rather than under `.rela/`.
  That is deliberate — `docs/postgres-backend.md` documents several rela-server
  processes against one database, and node-local state means an uploaded logo is
  served by exactly one of them. Rows sit in the store's schema, so
  schema-per-tenant scopes this state for free. **Key validation is the `state`
  package's job** (`state.ValidatedKV` wraps the backend at the wiring site):
  pgstore must not import `internal/state` (arch-lint forbids a store depending
  on an application package), so it stores whatever key it is handed and the
  wrapper enforces the same rules `storage.RootedFS` gives FSKV. Any new backend
  must pass `internal/state/statetest.RunAll`.

- **Multi-writer change feed** (TKT-WZYWM9). The postgres watcher delivers
  cross-process writes via PostgreSQL `LISTEN/NOTIFY`: each committed write does
  `pg_notify(rela_changed, '<origin>:<schema>:<kind>:<op>:<id>')` inside its
  transaction (so the 5 single-statement writes are wrapped in a tx); a listener
  goroutine (own connection, started in `Open`, stopped in `Close`) turns remote
  notifications into `store.Event`s on the in-process `Subscribe()` fan-out. Two
  payload fields do the routing, both filtered on receipt: a per-store random
  `originID` drops self-echoes (local writes are already emitted in-process),
  and the writing `schema` drops traffic from other schemas sharing the channel.
  NOTIFY is best-effort, so a `seq > watermark` catch-up (overlap window +
  idempotent re-snapshot; runs on connect/reconnect/safety-ticker, NOT per
  notification) recovers anything missed. **The channel is ONE constant
  (`rela_changed`), not one per schema** (TKT-9TOEBH): LISTEN is database-global
  _and_ needs a dedicated session, so a per-schema name would cost one
  permanently-held connection per schema — the term that does not shrink under
  pooling. Isolation lives in the payload instead. What is **not** shared is the
  catch-up: `rela_seq` is per-schema and the catch-up query is unqualified SQL,
  so priming/catch-up stay bound to each store's own pool — do not "simplify"
  them onto a shared connection. If the listener can't connect, the store
  degrades with a warning (local events still work). Exact ordering (xid8 +
  `pg_snapshot_xmin`) is the documented upgrade, not built. The data-entry SSE
  feed consumes this via `App.startStoreEventBridge` (entity events only).
  fsstore/memstore stay in-process single-writer by nature.

- DSN is read from the `RELA_DATABASE_URL` env var **only** — there is no
  `--database-url` flag, so the credential never lands in `ps`/shell history.
  `appbuild.Discover` reads the env into `appbuild.Config.DatabaseURL`; the `db`
  commands read the env directly. Don't add a DSN flag.
- **Derived static-query indexes are all-or-nothing desired state.** The
  PostgreSQL and SQLite reconcilers own only `rela_derived_query__*` /
  `rela_derived_list__*` and derive those indexes from validated static
  dashboard/next-action/list shapes (`appbuild.staticIndexSpecs`, shared by
  both). Never
  reconcile a partial set after a `data-entry.yaml` read/parse/validation
  failure: an absent desired object means DROP, so partial input is destructive.
  Runtime/ad-hoc queries never issue DDL. Pushdown and index inference must use
  the same `internal/queryplan` eligibility decision, and an EXPLAIN test must
  prove each newly supported SQL shape actually uses its generated index, on
  both backends (`EXPLAIN QUERY PLAN` on sqlite). SQLite matches an expression
  index only when the query spells the expression identically, so its DDL is
  built with the query builder's own helpers (`sqlitestore/derivedschema.go`). A
  next-action `condition:` participates on both sides: its store-safe scalar
  equalities (`entity.x == 'lit'`, `entity.x == current_user.id`,
  `is_current_user(entity.x)`) are pushed with the query and derive columns of
  the SAME composite index; `has_current_user(list)` is pushed as a non-scalar
  `PropEqual` (jsonb containment) and deliberately derives no index, since the
  btree over `->>` does not serve it — a GIN shape would need its own EXPLAIN
  test first.
- **Migrations** are embedded SQL (`pgstore/migrations/*.sql`), applied by
  `pgstore.Migrate` in one transaction under a `pg_advisory_xact_lock`
  (concurrent-start safe; forward-only). Auto-applied on first store open; also
  runnable explicitly via the postgres-build `rela db migrate` /
  `rela db status` commands (`pgstore.Status` is the read-only version check).
  `rela db` errors clearly in non-postgres builds.
- A new `store.Store` implementation must pass `internal/store/storetest`
  (`RunAll` + the fuzz functions). pgstore's suite is DB-gated on
  `RELA_TEST_DATABASE_URL` (skips when unset). Run it with `just test-postgres`.

# CLAUDE.md

## Rules for new code

- **Define interfaces at the call site, not next to the implementation.**
  Producer-side interfaces couple consumers to every method the producer
  exposes. Each consumer declares the minimum interface it needs (one to
  three methods). When a callback would create a constructor cycle, the
  consumer defines the narrow interface and the wiring site supplies it —
  see `docs/architecture/consumer-side-interfaces.md` and the godoc on
  `autocascade.Host`, `mcp.Services`, `scheduler.WorkspaceProvider`.
- **Capability bundles, not service locators.** When a subsystem needs
  several collaborators, group them in a purpose-specific struct (see
  `internal/lua/deps.go` with `ReadDeps` / `WriteDeps`), split by read vs.
  write so read-only code can't accidentally mutate state. A scoped
  consumer-side `Services` interface is fine (see `internal/mcp/server.go`);
  a cross-subsystem grab-bag is not.
- **No repository or transaction abstractions.** Depend directly on
  `store.Store`, `tracer.Tracer`, `search.Searcher`,
  `entitymanager.EntityManager`. The old `repo` and `tx` layers are gone
  — do not reintroduce equivalents. The one sanctioned transaction seam
  is `store.Store.Tx` itself (DEC-8UIL0): a contract ON the store with
  per-backend meaning (fs/mem: write mutex, mutual exclusion only;
  postgres: native transaction + advisory lock, rollback, events at
  commit) — not a generic unit-of-work layer stacked above it. Don't
  wrap `Tx` in a new abstraction, and don't do slow external I/O inside
  a `Tx` callback.
- **Capture state once per operation.** Call `ws.Snapshot()` (or the
  equivalent `appState.Load()`) at the top of every handler, command, MCP
  tool, or observer; reuse the returned value for every read in that
  operation. Do not call `ws.Graph()` / `ws.Meta()` repeatedly — multiple
  loads against the underlying `atomic.Pointer` can observe different
  snapshots if a reload lands between them.
- **Don't leak storage or parsing types via return values.** A function
  that returns `*markdown.Document`, `*graph.Graph`, `interface{}`, or any
  type whose package the caller wouldn't otherwise need pulls the
  implementation into every consumer. Return value types or
  domain-package DTOs (`entity.Entity`, `entity.Relation`, `tracer.Result`).
  If you reach for `interface{}` plus a type assertion as a back-channel,
  define a typed dependency instead.
- **Split state-publish from write-serialize.** Use `atomic.Pointer[State]`
  for publishing a new state snapshot (no reader lock, no torn reads) and
  a separate `sync.Mutex` for serializing writers. Do not combine both
  responsibilities into a single `sync.RWMutex` — the lock-upgrade dance
  (`RUnlock → Lock → defer(Unlock → RLock)`) is the symptom, not the fix.
- **Constructors reject nil required fields.** A `New*` function with
  required collaborators returns `error` and validates them up front.
  Never substitute a no-op or sentinel implementation silently — that
  defers the failure to a downstream symptom that is much harder to
  diagnose.
- **Read-out paths go through visibility wrappers, base readers stay
  ungated.** Read-side ACL (entity row-gating + field-level `visible:`
  redaction) is enforced by `internal/visibility` decorators
  (`Reader`, the tracer decorator) injected at the wiring site — never
  by per-consumer redaction calls, and never inside `store`/`tracer`/
  `search` themselves (DEC-ZBI39P; the `search.VisibleSearcher`
  pattern generalized). Hidden = nonexistent (pruned subtrees,
  withheld paths, indistinguishable 404s). A system job that may read
  everything gets an `AllowAllReader` capability at wiring while
  keeping its genuine `system:*` principal for audit — allow-all is
  never inferred from identity. Write-prep reads (entitymanager
  diffing) keep raw store access: a redacted read-modify-write would
  clobber hidden fields.
- **Never redact a read that feeds a write.** Read-out and write-prep are
  different handles on purpose (`lua.ReadDeps.VisibleReader` vs
  `WritePrepStore`). A read-modify-write that loads a *redacted* entity
  drops the caller's hidden properties from the clone and **erases them on
  save** — silent data destruction. If you find yourself "tidying" two
  store handles into one, that is the bug: `luaUpdateEntity` and anything
  like it must keep the raw handle. Pinned by
  `TestScriptReads_UpdatePreservesHiddenProperties`.
- **Boundaries are enforced.** `just arch-lint` checks package import
  rules; run it before PR.

### Don't do this

- **Don't import `internal/graph` or `internal/model`** — both deleted.
  Use `internal/entity`, `internal/store`, `internal/tracer`.
- **Don't add a cross-subsystem service locator** (à la the removed
  `lua.Services`). Use `ReadDeps` / `WriteDeps` or a scoped consumer-side
  interface.
- **Don't call `ai.LoadProvider` directly from a new entry point.** Go
  through `script.NewWriterRuntime`, which calls `lua.LoadContextOptions`.
- **Don't wire AI into the validation path** — per-entity cost blowup
  with no quota. See `internal/ai/` docs for the rationale.
- **Don't reintroduce `internal/workspace`.** It was the legacy
  god-object aggregate; deleted in the workspace-decomposition arc.
  New code wires services individually via `appbuild.Discover` /
  `appbuild.New` or takes focused interfaces at the call site.
- **Don't run user-supplied Lua on the read path.** ACL gates evaluate
  against declarative policy + the graph; Lua participates only at write
  time. This targets unbounded/hot paths (per-row predicates on list reads),
  NOT a bounded single-subject evaluation the caller explicitly requested
  (e.g. performable transitions for one field on one entity). See
  `internal/entitymanager/CLAUDE.md`.

### Subsystem-specific rules (nested CLAUDE.md / godoc)

- **Writes, audit, ACL** → `internal/entitymanager/CLAUDE.md`. All writes
  go through `entitymanager.Manager`; do not write to `store.Store`
  directly from a write path.
- **Data-entry API + `_actions` affordances + write-validation policy** →
  `internal/dataentry/CLAUDE.md`.
- **Vue SPA build/test/architecture** → `frontend/CLAUDE.md`.
- **E2E tests** → `e2e/tests/AGENTS.md`.

## Architecture

rela is a schema-driven entity-graph platform. You define the shape of your
domain in a YAML metamodel (entity types, relation types, properties,
validation rules); rela gives you typed entities, typed relations, and tools
to query / validate / analyze / present the graph. Data is stored as markdown
files with YAML frontmatter.

Traceability (requirements → decisions → components) is one common use case,
not the identity. Other in-tree uses: ISO 27001 ISMS, project management,
DevOps runbooks, issue/ticket tracking (rela dogfoods itself — see `tickets/`),
documentation mirrors (`docs-project/`). Anything with typed entities and
relations fits.

```text
metamodel.yaml → Metamodel (entity types, relations, properties)
                     ↓
entities/*.md  → entity.Entity  ↘
                                 store.Store → tracer.Tracer  (pure reader)
relations/*.md → entity.Relation ↗          → search.Searcher (EntityObserver)
                                            → entitymanager.EntityManager
                                              (writes + automations + validation)
```

The store is the source of truth. `search` maintains a derived index as a
`store.EntityObserver`. `tracer` is a pure reader — no subscription, no
derived state. `entitymanager` is the "human intent" write path that runs
automations and validation on top of the store.

Write-path rules — validation policy (400/422/200-with-warnings), the
audit log, and ACL — live in the nested files
`internal/dataentry/CLAUDE.md` and `internal/entitymanager/CLAUDE.md`.

### Packages

Entry points: `cmd/rela`, `cmd/rela-server`, `cmd/rela-desktop`.

Domain and storage:

| Package                  | Purpose                                                   |
| ------------------------ | --------------------------------------------------------- |
| `internal/entity`        | Domain types: `Entity`, `Relation` (no storage metadata)  |
| `internal/metamodel`     | Schema: entity types, relations, properties, validation   |
| `internal/store`         | Storage abstraction — CRUD + events; `fsstore`/`memstore`/`pgstore` |
| `internal/tracer`        | Pure-reader graph traversal (trace, path, orphans, cycles)|
| `internal/calfeed`       | Pure calendar-feed model + iCalendar/JSON serializers (event-granular; no store/vendor) |
| `internal/search`        | Full-text + structured search (bleve + linear)            |
| `internal/visibility`    | Read-side ACL wrappers: row-gate + field-redact readers, tracer decorator (DEC-ZBI39P) |
| `internal/entitymanager` | Write path: automations, validation, audit, policy        |
| `internal/audit`         | Append-only JSONL audit log of every successful write     |
| `internal/principal`     | Identity attribution (`Principal{User, Tool}`) on ctx     |
| `internal/validator`     | Validation engine invoked by entitymanager                |
| `internal/markdown`      | Parse/write entity and relation markdown                  |
| `internal/project`       | Project discovery, paths (`Context`)                      |
| `internal/appbuild`      | Wiring facade — constructs the focused services bundle    |

Subsystems (see each package's doc comment for details):

| Package               | Purpose                                                        |
| --------------------- | -------------------------------------------------------------- |
| `internal/cli`        | Cobra commands                                                 |
| `internal/mcp`        | MCP server over stdio — tools, resources, prompts, watcher    |
| `internal/dataentry`  | Data entry web app (Go API + Vue 3 SPA in `frontend/`)         |
| `internal/scheduler`  | Sequential Lua script scheduler (`rela scheduler`)             |
| `internal/lua`        | Lua runtime + bindings (`ReadDeps`, `WriteDeps`)               |
| `internal/script`     | Script execution helpers that wrap `lua` with project context  |
| `internal/automation` | Automation engine invoked by `entitymanager`                   |
| `internal/autocascade`| Cascade orchestration (runs automation side-effects)           |
| `internal/ai`         | OpenAI-compatible LLM provider (used from Lua)                 |
| `internal/migration`  | Schema migrations for project YAML files                       |
| `internal/cmdexec`    | Safe external-command core (argv, no shell, `{in}`/`{out}`, timeout, cap) shared by attachment + transform |
| `internal/transform`  | View-export engine: markdown `Renderer` → external-tool format conversion (the `transforms:` registry) |

Other packages under `internal/` are self-descriptive — ls the tree.

### View export & transforms (`internal/transform`)

The `transforms:` map in the metamodel registers named `markdown → format`
external commands (see `docs/transforms.md`). A `transform.Renderer` produces
markdown; the engine runs it through a transform via `internal/cmdexec` (argv
array, no shell, temp-file `{in}`/`{out}`, timeout, output cap — the same
security-reviewed exec pattern `internal/attachment` uses). Rules for new code:

- **Export is downstream of an already-authorized view, never a new capability.**
  Entity/list export in `internal/dataentry` routes through the SAME ACL read
  path as the view (`visibleReader.getVisible` / `scopedSortedEntities`); a
  request may only choose a registered transform *name*, never a command/flag/path.
- **The list-table renderer lives in `internal/dataentry`, not `internal/transform`** —
  it needs the ACL neighbor-visibility gate (`visibleRelationIDs`) so hidden
  neighbor titles never leak into an export. `internal/transform` must NOT import
  `internal/dataentry`; the built-in single-entity renderer lives in `transform`,
  and `dataentry` supplies the list renderer as a `transform.Renderer`.
- **The per-type render override (`views.<type>.export_render`) renders through
  `documentService.RenderMarkdown`** — the same Lua document machinery, reached
  only AFTER the export has resolved the entity through the ACL read gate. Never
  call `script.ExecuteDocument` on a fresh unauthenticated surface, and keep the
  entity id path-validated (`isSafePathSegment`) before it reaches a render.
- **Export downloads are hardened** like attachment downloads (nosniff, sandbox
  CSP, `no-store`, sanitized `Content-Disposition`) — the produced bytes embed
  user content.
- **External commands are CONFINED in `internal/cmdexec`, and it fails closed.**
  Both export and attachment processing run third-party parsers over
  attacker-influenceable bytes, so the shared runner adds: a no-network,
  temp-dir-only sandbox (bubblewrap on Linux, `sandbox-exec` on macOS), rlimits
  (memory/PIDs/file size/CPU, Linux), process-group kill so a converter's helper
  cannot outlive the timeout, and a bounded pool capping concurrent runs. On a
  host with no mechanism, commands REFUSE to run — only command execution is
  blocked, never server startup. Do not add a "can I run?" predicate: call `Run`
  and handle its error; `Describe()` exists solely for the startup log.
- **The transform engine must be built ONCE and shared**, not per request — it
  owns the bounded pool, so a per-request engine gives every request its own pool
  and the concurrency cap bounds nothing.

### Storage backends & build tags

The storage + search backend is chosen at compile time by Go build tags.
The composition root has one `New` recipe per scenario over shared
`prepare()`/`assemble()` helpers — see `internal/appbuild/appbuild_{fs,memory,postgres}.go`
and the matching `internal/cli/mcp_wiring_{fs,memory,postgres}.go`:

| Build tag        | Store      | Search                | Binaries                          |
| ---------------- | ---------- | --------------------- | --------------------------------- |
| *(none, default)*| `fsstore`  | in-memory bleve       | `rela`, `rela-server`             |
| `memorybackend`  | `memstore` | `LinearSearch`        | (tests / experiments; no bleve)   |
| `postgres`       | `pgstore`  | PostgreSQL (`pg_trgm` + tsvector) | `rela-postgres`, `rela-server-postgres` |

Rules when touching this:

- **The `postgres` build must not link bleve; the default build must not
  link pgx.** CI asserts both via `go list -deps` (the `postgres` job in
  `ci.yml`). Keep backend-specific imports inside the tagged recipe files.
- **`pgstore.New(db DBTX)` takes an injected pgx pool**, not a DSN. The
  postgres recipe builds one pool, runs `pgstore.Migrate`, and shares it
  between the store and the in-DB search backend. appbuild owns/closes the
  pool; `store.Close()` only tears down the watcher.
- **Build-agnostic wiring lives in `prepare`/`assemble`, never in a recipe.**
  A recipe may choose and order backend steps; if logic would be copy-pasted
  between recipes, it belongs in a shared helper. This is what keeps the three
  recipes from drifting (and where future per-backend audit/ACL variation goes).
- **The metamodel is always read from disk**, even in the postgres build —
  `metamodel.yaml`, `templates/`, `.rela/` stay on the filesystem; PostgreSQL
  backs entities/relations/attachments/search only. A postgres deployment
  still needs a `--project` dir.
- **Multi-writer change feed** (TKT-WZYWM9). The postgres watcher delivers
  cross-process writes via PostgreSQL `LISTEN/NOTIFY`: each committed write does
  `pg_notify(<schema-scoped channel>, '<origin>:<kind>:<op>:<id>')` inside its
  transaction (so the 5 single-statement writes are wrapped in a tx); a listener
  goroutine (own connection, started in `Open`, stopped in `Close`) turns remote
  notifications into `store.Event`s on the in-process `Subscribe()` fan-out. A
  per-store random `originID` in the payload filters self-echoes (the listener
  skips its own writes — local writes are already emitted in-process). NOTIFY is
  best-effort, so a `seq > watermark` catch-up (overlap window + idempotent
  re-snapshot; runs on connect/reconnect/safety-ticker, NOT per notification)
  recovers anything missed. The channel is schema-scoped (`rela_changed_<schema>`)
  because LISTEN is database-global — all processes of one deployment share a
  schema/channel. If the listener can't connect, the store degrades with a
  warning (local events still work). Exact ordering (xid8 + `pg_snapshot_xmin`)
  is the documented upgrade, not built. The data-entry SSE feed consumes this
  via `App.startStoreEventBridge` (entity events only). fsstore/memstore stay
  in-process single-writer by nature.
- **Content versioning** (TKT-9INY0Y, postgres only). Two tables
  (`entity_versions` = one full snapshot per version; `schema_versions` =
  content-addressed render-schema projection, deduped) plus a dedicated
  `version_seq` sequence. **Use `version_seq`, never `rela_seq`** — `rela_seq`
  feeds the change-feed watermark (`primeWatermark`/`catchUp` scan
  entities/relations/deletions), and burning it on version rows that don't land
  in those tables would erode the overlap budget and drop real events. Capture
  is **hybrid**: rename+delete are captured synchronously at the entitymanager
  boundary (they carry old→new id / pre-delete state the sweep can't
  reconstruct); create/update are captured by a debounced reconciliation
  **sweep** goroutine (`sweep.go`, started/stopped like the listener). The sweep
  runs its **entire tick on ONE acquired pool connection** under
  `pg_try_advisory_lock` — the lock is session-scoped, so issuing the inserts via
  the pool (other sessions) would silently void the single-writer guarantee.
  Attribution comes from ctx only, via exactly two boundary-populated inputs —
  the store never learns the Principal by another route: sync captures carry it
  inside `store.VersionInput`, and create/update writes carry a
  `store.Attribution` on ctx (`store.WithAttribution`, set ONLY at the
  entitymanager boundary and ONLY for a real principal — never translate a
  zero/unknown principal, RR-U964M0) which pgstore stamps into
  `entities/relations.last_edited_by_user/_tool` (TKT-ZIRMGM). The sweep copies
  those columns onto swept versions; NULL columns (legacy rows, unattributed
  writes) fall back to the `version-sweep` system principal — never a guessed
  or literal-"unknown" identity. Author-boundary segmentation (flush-on-author-
  change) is TKT-0IGI4V, not built: two authors in one debounce window merge
  into one version attributed to the last of them. Lineage across a
  rename/id-reuse is fenced by `[lo,hi)` vseq ranges in a recursive-CTE walk (an
  unbounded `entity_id = ANY(...)` read would merge two entities' histories — see
  the version.go doc). `HistoryReader`/`VersionWriter` are optional store
  capabilities (type-asserted like `store.Formatter`), NOT part of `store.Store`.
- **Relation versioning** (TKT-92JL8P, postgres only) extends the above to
  relations, which carry their own props + body. A `relation_versions` table
  reuses `version_seq` + `schema_versions`; identity is a surrogate
  `rel_record_id` **column ON the `relations` row** (`DEFAULT nextval(...)`,
  carried through writes) — NOT reconstructed per sweep-tick, which would race the
  sync path and merge/fork lineages. Delete+recreate of the same `(from,type,to)`
  mints a fresh id (histories don't merge). Capture: create/update via the sweep's
  second `FROM relations` scan (entities-then-relations, same tick/lock);
  **delete synchronously via `DeleteResult.DeletedRelations`** — the single path
  for BOTH explicit `DeleteRelation` and entity **cascade** delete (the store
  bulk-deletes relations below the entitymanager, so cascade edges would otherwise
  lose history). Rename **stitches** (not forks): the entitymanager captures a
  `rename` version per incident relation on the new triple carrying
  `prev_from`/`prev_to`, and `relationLineageIDs` walks those links so history is
  continuous. Since #1127 the store renames **atomically** (bulk in-place
  `UPDATE relations SET from_id=...`), so a relation KEEPS its `rel_record_id`
  across the rename — the lineage is already continuous on one id and the
  `rename` version merely appends a marker (the `prev_from`/`prev_to` stitch walk
  finds no fork; it stays as belt-and-braces for any future non-atomic path).
  Rename capture is **sync-only best-effort**: the atomic re-key does NOT bump
  `relations.updated_at` (TKT-9TQ6I), so the sweep cannot back-fill a rename the
  synchronous hook misses — acceptable because a miss loses only the rename
  marker, never lineage continuity. Read/restore is gated on **both** endpoints
  (FROM ∧ TO) — the FROM
  entity only *owns* the UI placement, it is not the auth boundary (a TO-side
  oracle otherwise). Relations have NO field-level redaction today; relation
  history exposes exactly what a live relation GET does. `RelationHistoryReader`/
  `RelationVersionWriter` are SEPARATE optional capabilities, type-asserted
  independently of the entity ones.
- **Version purge** (TKT-BW6UUL, postgres only) is the audited, irreversible
  exception to append-only history — hard-deletes version rows for compliance
  redaction. `VersionPurger`/`RelationVersionPurger` are SEPARATE optional
  capabilities (`purge.go`), one `PurgeVersions`/`PurgeRelationVersions` method
  each. Load-bearing guardrails (design-review, do not relax): the whole op runs
  under **`sweepAdvisoryLockKey`** (mutually exclusive with a sweep tick — a purge
  racing a capture-insert loses the erasure); it **REFUSES while a live row still
  holds the content** unless `--force-live` (else the sweep re-captures it within
  one interval — a `VersionOpPurge` no-content tombstone whose content_hash = the
  live hash suppresses that re-capture via the sweep's existing dedup); it
  **REFUSES a rename row** (purging one orphans/forks the lineage walk — v1 is
  non-rename-only); `--all` purges the **fenced lineage** (`lineageCTE` /
  `relationLineageIDs`), never `WHERE id=$1` (id-reuse would destroy unrelated
  history). CLI-only (`history-purge`/`relation-history-purge`), dry-run by
  default; trust boundary is operator shell (no ACL check — like `db migrate`),
  audited via the `audit.Audit` sink (`OpPurgeVersion`, `svc.Audit()`), never
  echoing purged content. `schema_versions` is projection-only + FK-shared, so
  purge never deletes it. Purge is necessary-not-sufficient for erasure (live
  row / PITR backups survive) — see the postgres-backend guide.
- DSN is read from the `RELA_DATABASE_URL` env var **only** — there is no
  `--database-url` flag, so the credential never lands in `ps`/shell history.
  `appbuild.Discover` reads the env into `appbuild.Config.DatabaseURL`; the
  `db` commands read the env directly. Don't add a DSN flag.
- **Migrations** are embedded SQL (`pgstore/migrations/*.sql`), applied by
  `pgstore.Migrate` in one transaction under a `pg_advisory_xact_lock`
  (concurrent-start safe; forward-only). Auto-applied on first store open;
  also runnable explicitly via the postgres-build `rela db migrate` / `rela db
  status` commands (`pgstore.Status` is the read-only version check). `rela db`
  errors clearly in non-postgres builds.
- A new `store.Store` implementation must pass `internal/store/storetest`
  (`RunAll` + the fuzz functions). pgstore's suite is DB-gated on
  `RELA_TEST_DATABASE_URL` (skips when unset). Run it with `just test-postgres`.

## Tests

- Prefer table-driven tests with `t.Run(tc.name, ...)` subtests.
- Use `t.Helper()` on assertion helpers.
- `internal/store/storetest` provides the store conformance harness — any
  new `store.Store` implementation must pass it. Likewise any new
  `search.VisibleSearcher` implementation must pass
  `storetest.RunVisibleSearchTests` (the ACL-scoped search contract).
- Race detector is on in CI; don't add `//go:build !race` tags.

## Coverage

Go: `go-test-coverage` enforces **package floor thresholds** (no ratchet);
minimums live in `.testcoverage.yml`. Coverage within the floor is free to
move up or down — floors exist to catch "new untested package added" and
"core package silently lost its tests." The frontend has no coverage
enforcement — unit tests run plain (`npm run test:run`).

- Run locally: `just coverage-check`, `just coverage-html`.
- When a floor fails, add tests — don't lower the threshold without a reason.
- Use `// coverage-ignore: <reason>` sparingly, only for genuinely
  untestable code (main functions, external-tool dependencies,
  OS-specific paths). Reason is required.

## Lint

golangci-lint with project rules. Test files exempt from `dupl`, `funlen`,
magic numbers. Cobra `cmd`/`args` unused parameters allowed. Line length: 120.

**God-object load lines** (`just plimsoll`, CI job "God-object lint"). The
[plimsoll](https://github.com/sourcehaven-bv/plimsoll) linter caps three
independent surfaces — the metric that tracks a type accreting into a
god-object (`App`, `Runtime`, `FSStore` got there because nothing stopped them):

- **`max-methods` (40)** — total methods, exported + unexported. The backstop
  for internal sprawl: a receiver with dozens of private helpers is one struct
  whose fields they can all reach.
- **`max-exported-methods` (20)** — exported methods only. The sharper signal,
  since the public API is the coupling surface consumers bind to. Note these
  often diverge wildly from the total: `App` is 226 methods but only 13
  exported; the genuinely-wide *public* APIs are the store implementations and
  schema value types (`FSStore`, `MemStore`, `Metamodel`).
- **`max-fields` (20)** — exported struct fields.

A new type over any line fails CI. Existing offenders are grandfathered with a
`//plimsoll:max-methods=N` / `max-exported-methods=N` / `max-fields=N` directive
at the declaration site, pinned to the current count so they can't grow; ratchet
those down as you decompose (TKT-N0IKN9). A store-implementation's exported count
is the mandated `store.Store` interface, so its directive is a documented
"required interface" exception rather than a ratchet target. Prefer splitting the
type over raising the number.

## Security

`govulncheck` runs on every PR touching `go.mod` / `go.sum` (the `vulncheck`
job in `ci.yml`) and weekly from `security.yml`. Known-unfixable vulns are
filtered via `scripts/govulncheck-filtered.sh` — keep `IGNORED_OSVS` in sync
with `scripts/govulncheck-fixable.sh`. Run locally: `just govulncheck`.

## Commands

Read the `justfile` for the full set. The non-obvious ones: `just arch-lint`
(package boundary check), `just ci` (full pipeline), `just dev` (data-entry
server locally), `just coverage-check`. `go test -run TestName ./...` for a
single test.

## Project files

```text
metamodel.yaml                  # Entity/relation schema
schedules.yaml                  # Optional: schedules for `rela scheduler`
entities/<type>/                # Markdown entity files by type
relations/                      # Markdown relation files (FROM--type--TO.md)
templates/entities/<type>.md    # Optional: entity templates for defaults
templates/relations/<type>.md   # Optional: relation templates for defaults
.rela/user-defaults.yaml        # Per-user defaults (gitignored)
.rela/scheduler-state.json      # Scheduler last-run timestamps (gitignored)
```

## Working documents

Anything temporary — designs, tickets, QA notes, scratch — goes in
`.ignored/` (gitignored). Do not commit these.

<!-- @managed: claude-workflow start -->

## Rela for Planning & Issue Tracking

This project uses two rela instances via MCP for design and issue tracking:

- **rela-docs**: Documentation entities (concepts, features, guides, tutorials, scenarios)
- **rela-issues-and-design-tickets**: Issue tracking (tickets, features, decisions, concepts, risks, measures, tests)

### Workflow for Creating Tickets/Entities

When creating or updating entities in `rela-issues-and-design-tickets`:

1. **Create the entity** with required properties
2. **Run ALL analyze tools** to check for issues:
   - `analyze_cardinality` - check required relations
   - `analyze_orphans` - find unlinked entities
   - `analyze_properties` - validate property values
   - `analyze_validations` - run custom validation rules
3. **Fix any violations** (create missing relations, add required properties, etc.)
4. **Repeat analysis until ALL checks pass** - do not stop after fixing one issue

### Common Required Relations

| Entity Type | Required Relations |
|-------------|-------------------|
| ticket | `affects` → concept (min 1), `implements` → feature (min 1) |
| feature | `requires` → concept (min 1) |
| test-case/test-suite | `test-covers` → concept (min 1), `verifies` → feature/ticket (min 1) |
| doc-task | `affects` → concept (min 1), `triggered-by` → ticket/feature/decision (min 1), `updates` → guide/tutorial/scenario (min 1) |
| research | `researches` → concept (min 1) |

### Research Documents

For larger features, run `/research <topic>` before planning to survey
approaches and document tradeoffs. This creates a `research` entity (RES-xxxx)
with structured sections: Problem, Context, Options, Recommendation.

**Workflow:**

1. `/research` creates the entity in `in-progress` and links it to concepts
2. The agent surveys the codebase and external approaches
3. Options are documented with pros/cons/effort
4. A recommendation is made and presented for user review
5. The research is linked to the ticket/feature via `has-research`

**When to use:** Enhancements or features where the approach isn't obvious,
multiple viable options exist, or the change touches unfamiliar subsystems.
The planning checklist has a research item that can be skipped with N/A for
smaller work.

### Validation Rules

The metamodel includes validation rules that enforce:

- In-progress bugs should have `why1` and `why2` started
- Done bugs must have 5-whys analysis (`why1`-`why3` required) and `prevention`
- Ready tickets need `effort`, `priority`, and `description`
- Accepted decisions need `date`, `context`, and `consequences`

Always run `analyze_validations` to catch these issues.

### 5-Whys for Bug Analysis

Bug tickets use the 5-whys method for root cause analysis:

| Property | Purpose |
|----------|---------|
| `why1` | What was the immediate cause? |
| `why2` | Why did that happen? |
| `why3` | Why did that happen? |
| `why4` | Why did that happen? |
| `why5` | What is the systemic root cause? |

Done bugs require at least 3 levels (why1-why3). The goal is to reach systemic causes
that can be addressed with process/tooling improvements documented in `prevention`.

### Workflow Checklists

Tickets and bugs use workflow checklists to ensure thorough planning, execution, and review.
Each phase has a dedicated checklist entity with standard items from templates.

**Ticket Workflow:**

```text
backlog → ready → planning → in-progress → review → done
                     │            │           │
                     ▼            ▼           ▼
              planning-      implementation-  review-checklist
              checklist      checklist        (+ docs-checklist
                 │                            for enhancements)
                 ▼
           /design-review
           (before impl)
```

**Bug Workflow:**

```text
backlog → ready → analyzing → in-progress → review → done
                     │            │           │
                     ▼            ▼           ▼
              bug-analysis-  implementation-  review-checklist
              checklist      checklist
```

**Checklist Types:**

| Checklist | Purpose | Required For |
|-----------|---------|--------------|
| `planning-checklist` | Understanding, research, approach, security, risk assessment | Tickets entering `in-progress` |
| `bug-analysis-checklist` | Reproduction, root cause, fix planning | Bugs entering `in-progress` |
| `implementation-checklist` | Development, quality checks | Tickets/bugs entering `review` |
| `review-checklist` | Automated checks, code review, verification | Tickets/bugs entering `done` |
| `docs-checklist` | Code docs, project docs, external docs | Enhancement/docs tickets entering `done` |

**Review Commands:**

| Command | When to Use | Creates |
|---------|-------------|---------|
| `/design-review` | After planning, before implementation | `review-response` entities for design issues |
| `/code-review` | During review phase, after implementation | `review-response` entities for code issues |

**Agent Workflow for Tickets:**

Checklists are **automatically created** when tickets/bugs transition to specific statuses.
The `create_entity` automation with `if_exists: skip` ensures no duplicates.

1. **Start Planning** (status: `planning`)
   - Planning checklist is auto-created and linked via `has-planning`
   - Work through checklist items: understanding, approach, security, test plan
   - Run `/design-review` to catch issues before implementation
   - Address all critical/significant design findings
   - Mark checklist `status=done` when complete

2. **Start Implementation** (status: `in-progress`)
   - Implementation checklist is auto-created and linked via `has-implementation`
   - Work through development and quality items

3. **Start Review** (status: `review`)
   - Review checklist is auto-created and linked via `has-review`
   - Run `/code-review` to perform thorough code review
   - Address all critical/significant code review findings
   - If enhancement or docs ticket, manually create `docs-checklist`
   - Complete all checks before marking done

4. **Create PR** (before `done`)
   - Run `/pr` to create PR and monitor CI until all checks pass
   - Fixes any CI failures (lint, test, coverage) automatically
   - Document PR URL in review-checklist

5. **Complete** (status: `done`)
   - All linked checklists must have `status=done`
   - All checklist items must be checked or skipped with reason
   - PR merged or ready to merge

**Bug Workflow Automations:**

- `analyzing` → auto-creates `bug-analysis-checklist` via `has-bug-analysis`
- `in-progress` → auto-creates `implementation-checklist` via `has-implementation`
- `review` → auto-creates `review-checklist` via `has-review`

**Skipping Checklist Items:**

When an item doesn't apply, use strikethrough with a reason in parentheses:

```markdown
- [x] ~~API docs updated~~ (N/A: no API changes)
- [x] ~~Performance check~~ (N/A: documentation-only change)
```

Items without reasons will fail validation.

### Review Response Protocol

**Triggering Code Review:**

When a ticket/bug enters `review` status, run the `/code-review` command. This invokes the
cranky-code-reviewer agent to perform a thorough code review and automatically creates
`review-response` entities for each finding.

Alternatively, invoke the cranky-code-reviewer agent directly for ad-hoc reviews.

**Creating Review Responses:**

For each finding from code review:

1. Create a `review-response` entity with:
   - `title`: Brief description of the finding
   - `finding`: Full description of the issue
   - `severity`: `critical` | `significant` | `minor` | `nit`
   - `status`: `open`
2. Link to ticket/bug via `has-review-response` relation

**Addressing Review Responses:**

| Severity | Required Action |
|----------|-----------------|
| critical | MUST be fixed before done |
| significant | MUST be fixed before done |
| minor | Should fix, can defer with reason |
| nit | Optional, can wont-fix with reason |

When addressing a finding:

- Fix the issue in code
- Update status to `addressed`
- Document the `resolution` (how it was fixed)

When not addressing:

- Set status to `wont-fix` or `deferred`
- Document the `reason` (justification required)

**Validation Gates:**

Tickets/bugs cannot be marked `done` if they have:

- Open critical review responses
- Open significant review responses

Minor/nit findings may remain open with warnings.

### Automation Actions

Status transitions auto-create checklists (and similar side effects) via
automations declared in the project's `metamodel.yaml`. Action types
(`set`, `create_relation`, `create_entity` with `if_exists`) and
interpolation patterns (`{{new.property}}`, `{{entity.id}}`, `{{today}}`)
are documented in `docs/metamodel.md` and exemplified in the live
`metamodel.yaml`. Read those rather than relying on a copy here — a stale
copy is worse than a pointer.

Common mistake: `{{entity.title}}` is wrong; use `{{new.title}}` for a
property of the triggering entity.
<!-- @managed: claude-workflow end -->

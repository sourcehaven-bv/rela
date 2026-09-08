---
id: PLAN-B7F1R4
type: planning-checklist
title: 'Planning: PostgreSQL read-path performance audit: per-request query counting, seeded demo with ACL + worlds, slow-query driven optimisation'
status: done
---

<!-- @managed: claude-workflow v1 -->

## Understanding

- [x] Problem/requirements clearly understood
- [x] Scope defined (what's in/out documented below)
- [x] Acceptance criteria documented with specific test scenarios

**Problem.** The data-entry SPA drives `pgstore` through store calls shaped for
the filesystem store. Three compounding defects, confirmed by code survey:

1. *Full-row loads.* `entities.content` (the markdown body) sits in the same
row as `properties`. `ListEntities`, `GraphQuery`, `GetEntity`, `ListRelations`,
`SearchVisible` all select it. `ListEntityHeaders` (`pgstore/entity.go:93`,
TKT-1ESTYJ) exists but only `_analyze` uses it, and the ACL pushdown
(`visibility/pushdown.go`) has no header variant. List rows are serialized WITH
`Content` (`entityserializer.go:54`) though no list/kanban/dashboard/search
component reads it (verified by grep).
2. *N+1 per row.* The list endpoint loads the entire type unpaginated
(`api_v1.go:327` `scopedSortedEntities`), sorts and slices in Go (`:658-668`),
then per page row does 2 relation queries (`worldneighbors.go:850`) and one
`GetEntity` per neighbour (`relation_visibility.go:72`). Entity view table
sections do one `ListRelations` + one `GetEntity` per row × per relation column
(`sections.go:307` → `views_handler.go:801-829`). Search does `GetEntityState`
per hit (`queryservice.go:201`). Affordances do `GetEntityState` per declared
face (`affordances.go:1334`). Kanban fetches up to 50 pages sequentially, each
paying the full pipeline (`KanbanView.vue:135`, `api/entities.ts:88`).
`_position` reruns the whole list pipeline per detail page. `internal/tracer`
BFS does `GetEntity` + `ListRelations` per visited node.
3. *No observability.* The only query logging is `debugQueryTracer`
(`pgstore/tracer.go`), Debug-level, all-or-nothing, no per-request aggregation.
No request log middleware exists. No index/EXPLAIN coverage beyond
`graphquery_explain_test.go`. Index gaps: no `(type, face)`, no JSONB index on
`properties` (so `PropertyValues` and property predicates seq-scan), `HighestID`
runs `LIKE 'PFX-%'` without `text_pattern_ops`, and `entities_search_tsv_idx` is
maintained but never queried (verified: `search.go` uses LIKE + `similarity()`
only; folds TKT-ZZL53L).

**Scope:**

IN:
- A. Per-request query stats: count + total DB time on the request context,
surfaced as a `Server-Timing` header and one slog `request` record **only when
Debug logging is enabled** (`rela-server -verbose`, the condition that already
gates the pgx tracer); nothing on the wire by default (RR-64OR7D). Plus a
backend-agnostic store-call counting decorator for tests.
- B. A seeded perf demo project (`prototypes/perf/project/`) with faces +
worlds + `acl.yaml` (roles, membership edges, `visible:` redaction) and a full
`data-entry.yaml` (lists with relation columns/filters, views with table
sections, 2 kanbans, dashboard, gantt, next-actions), plus a deterministic `rela
dev seed` command that fills any store (fs or pg build) at a chosen scale.
- C. Baseline measurement: query count, DB ms, wall ms per SPA endpoint × 3
principals × default/published world, plus the postgres slow-query log with
`auto_explain`. Recorded in the ticket.
- D. Optimisations, in impact order, each pinned by a query-budget test:
  1. Batch loads: `RelationQuery.EntityIDs` (plural of `EntityID` under
identical `Direction`/`FromFace` semantics; fs/mem/pg/sqlite + storetest),
neighbour titles via `ListEntityHeaders{IDs}` filtered by the existing
`visibility.HeaderFilterer`, section relation columns batched per section,
search hits batched, faces batched, tracer BFS batched per frontier.
  2. Content-free listings: a `store.GraphHeaderQueryer` optional capability
(`GraphQueryHeaders`, pg-native projection + generic fallback) and
`visibility.listPushdownHeaders`, so BOTH AllowAll and Query principals read
headers (RR-O5W4A3); list/search/kanban/dashboard rows omit `content` unless
`?include_content=true`.
  3. Push pagination + count into the store for the common list shape (no
relation filter, no free text, sort on id/updated_at or a declared scalar
property): paging fields set only on the per-request copy in `listPushdown`; the
handler receives only the scoped `matched` count (RR-J1VAW0). Go path stays as
fallback for other shapes.
  4. Indexes: migration `0014_read_indexes.sql` adding `entities (type,
face)`, `entities (id text_pattern_ops) WHERE face=''`, GIN `jsonb_path_ops` on
`properties`; `DROP INDEX entities_search_tsv_idx` with a comment naming the
LIKE/trgm strategy; docs paragraph corrected. Extend the derived-index
reconciler eligibility to static `lists.*.where`/`sort` and kanban `where`
shapes.
  5. Cheap fixes: `a.Services()` rebuilt per row, `_position` computed from
`MatchingIDs` instead of a full re-run; reconcile the 87-vs-104 plimsoll
comment.
- E. Docs: `docs/postgres-backend.md` section "Observing query cost", CLI
reference for the seeder, API note on `include_content`, openapi update,
CLAUDE.md rule + raw-store exception list (3 → 4).

OUT (stay as their own tickets):
- Server-side aggregation for count/breakdown cards (TKT-AIEGHU) and
table-card limit/sort pushdown (TKT-8AUD1U) — wire-shape changes to the
dashboard; this ticket only makes their rows content-free.
- Bounding structured search (TKT-T0DK37) — easier after D.3; not done here.
- Pushing field-visibility provenance into SQL (TKT-L59KUM).
- Locale-aware sort semantics for property sort pushdown: pushdown is only
used where the Go comparator and `ORDER BY ... COLLATE "C"` agree (id,
timestamps, enum/string equality); otherwise the Go path stays.
- Any change to write paths, versioning, sweep, or the change feed.
- Frontend redesign of kanban fetching beyond consuming the cheaper API.

**Acceptance Criteria:**

1. **Observability.** With Debug logging enabled, every `/api/v1/*` response
on the postgres build carries `Server-Timing: db;dur=<ms>;desc="<n> queries"`
and one slog record `request` with method, path, status, wall_ms, queries,
db_ms. With Debug disabled neither is emitted. Tests: postgres-tagged handler
test asserts the header and that `n` equals the count observed by a test tracer;
unit test for the context stats type; test that the header is absent at Info
level.
2. **Budget pins (backend-agnostic).** Using the counting store decorator
over memstore: for a list page with relation columns, an entity view with a
table section having 2 relation columns, a search, and a kanban page, the
store-call count at 10 rows equals the count at 50 rows (size independence), and
the measured constant is pinned with a comment giving the pre-change count
(RR-CH8CX9). Targets: list ≤ 6, view ≤ 8, search ≤ 4.
3. **Seeder.** `rela dev seed --profile perf --scale 1` writes a
deterministic project (≈20k entities, ≈60k relations, faces on two types,
membership edges for ACL) into the current backend with
`store.WithAttribution(system:perf-seed)` and an audit record, refuses a
non-empty store without `--force`, validates every id, and completes into local
postgres in under 3 minutes. Same seed → identical ids/properties on both
backends (conformance test at scale 0.01).
4. **Measured improvement.** For each endpoint in the baseline table the
after-column shows: list page, detail view, search, kanban page, dashboard →
query count no longer grows with N (row count) and DB ms drops ≥ 5× at scale 1,
for reader/editor/manager alike (RR-O5W4A3). The table lives in the ticket body.
5. **Indexes proven.** Each new index has an EXPLAIN test in the pattern of
`graphquery_explain_test.go` showing the target query uses it; the derived
reconciler creates an index for a list `where:` shape and drops it when the
config is removed (unit + DB test).
6. **ACL unchanged.** Every batched path yields identical visible sets and
redacted values to the per-row path: (a) a hidden neighbour in a batch of 50
stays hidden and its title never appears in list `included`/section values; (b)
an AllowAll principal under the `published` world doing a 50-row list with
relation columns and a section view sees no draft-face title through any batched
path (RR-7JCLZP); (c) a scoped principal's list `total` equals the visible
count, not the table count (RR-J1VAW0); (d) a `visible:`-redacted field is
redacted in header rows.
7. **Wire compatibility.** List/search rows omit `content` by default;
`?include_content=true` restores it; the SPA passes all e2e tests unchanged
except where it intentionally stops requesting content; verified that
`relationsPatch.ts:107` is fed by the detail endpoint, not a list row.

## Research

- [x] ~~For larger features: run `/research`~~ (N/A: approach is dictated by measured defects and existing patterns; the survey above IS the research)
- [x] Searched for existing libraries that solve this problem
- [x] Checked codebase for similar patterns or reusable code
- [x] Looked for reference implementations in other projects
- [x] Reviewed relevant rela concepts for prior art

**Research Doc:** N/A

**Existing Solutions:**

- `pgx.QueryTracer` is the library-level hook; already used by
`debugQueryTracer` (`pgstore/tracer.go:22`, attached at `open.go:32`). The
tracer slot is singular, so the counting tracer absorbs the debug logging rather
than composing two tracers. No `MultiTracer` in pgx v5.
- `store.HeaderReader`/`ListEntityHeaders` (`pgstore/entity.go:93`) honours
`World`/`FaceIn` via the shared `buildEntitySelectSQL` — verified.
`visibility.RedactHeader` (`policyreader.go:267`) and
`HeaderFilterer.FilterHeaders` (`visibility.go:179`) already exist; what is
missing is `visibleReader.filterVisibleHeaders` in dataentry and a header
variant of the pushdown.
- `EntityQuery.IDs` already batches entity lookups (`store.go:279`);
`RelationQuery` has no batch endpoint field — needs `EntityIDs`.
- `ListEntitiesPage` keyset paging (`store.go:251`) exists, unused by the list
handler. ACL compiles to a `GraphQuery` (`acl/readquery.go:18-37`) copied per
request in `pushdown.go:117-124`; `GraphCount` returns `(matched, total)`.
- Derived-index reconciler (`pgstore/derivedschema.go`, `internal/queryplan`,
TKT-03HCWT) — the sanctioned "automated indexes" mechanism; extend eligibility
rather than adding a second one. EXPLAIN test harness at
`graphquery_explain_test.go:124`.
- Seeding: `scripts/generate-test-data.sh` (bash, own 4-type schema, no
ACL/faces), `graphquery_bench_test.go:63 benchSetup` (Go, 10k rows, test-only),
`prototypes/worlds/project` (richest faces/worlds/ACL config, zero data).
Nothing seeds thousands of rows into postgres.
- Middleware chain `router.go:182-238`; `stampAuditPrincipal` (`:913`) is
the outermost wrapper touching every request. No request-id or access-log
middleware exists.
- Local postgres: Postgres.app 15.17 on 127.0.0.1:5432 (superuser), with
`auto_explain` available; `log_min_duration_statement` currently -1. Migrations:
next free number is 0014 (two `0003_*` files exist, so the runner orders by full
filename).

## Approach

- [x] Technical approach chosen and documented
- [x] Approach builds on existing patterns (not reinventing)
- [x] Alternatives considered (document why rejected)
- [x] Dependencies identified (packages, APIs, types)

**Technical Approach:**

*A. Query stats.*
- `internal/store/querystats.go`: `type QueryStats struct{ queries, nanos atomic.Int64 }`
with `WithQueryStats(ctx) (context.Context, *QueryStats)` and
`QueryStatsFrom(ctx) *QueryStats` (nil when absent). Lives in `store` because
both `pgstore` and `dataentry` may import it under arch-lint and it is part of
the store contract ("a store reports its work").
- `pgstore/tracer.go`: `queryTracer{debug bool}`, always attached in
`Open`. `TraceQueryStart` returns ctx unchanged when neither stats nor debug
apply (no alloc — preserves the documented production guarantee); otherwise
stores start time. `TraceQueryEnd` increments stats and logs at Debug when
enabled. `Tx` views share the pool so they are covered.
- `dataentry/requeststats.go`: `requestStats(next, enabled func() bool)`
wrapped outermost beside `stampAuditPrincipal`; when Debug is enabled it
installs stats, wraps the ResponseWriter to add `Server-Timing` at first
`WriteHeader`, and logs the `request` record after `next.ServeHTTP`. SSE paths
excluded from the header (streams). When disabled it is a pass-through.
- `internal/store/storetest/counting.go`: a `store.Store` decorator counting
method calls by name, for budget tests on memstore. Counts store calls, not SQL
— which is what dataentry's N+1s are made of; the pgstore test pins SQL for one
representative endpoint so the two stay aligned.

*B. Perf demo project + seeder.*
- `prototypes/perf/project/{schema.yaml,acl.yaml,data-entry.yaml,templates/}`.
Domain: `policy` (faces draft/published, `bare_face: draft`), `document` (faces
en/nl), `control`, `risk`, `project`, `task` (status enum, start/due dates,
`parent-of` for gantt), `person` (`visible:`-redacted salary), `team`.
Relations: mitigates, implements, belongs-to, assigned-to, depends-on, member-of
(membership relation), parent-of, references. Worlds: published (otherwise
exclude), editorial (otherwise default), site-nl (select [nl,en]). Roles: reader
(world:published + policy@published, no risk/person), editor, manager;
assignments via `member-of` edges to teams. **No `unique:` properties**
(store-level writes bypass entitymanager's unique scan, RR-7EJIWL). data-entry:
lists with relation columns and a relation filter, views with table sections,
kanbans (task by status, policy pipeline), dashboard (count/breakdown/table),
gantt on project/task, next-actions.
- `internal/perfseed`: pure generator `Generate(Profile, scale, seed)`
emitting `entity.Entity`/`entity.Relation` from word banks with a seeded PRNG;
ids are prefix+counter (unique by construction) and pass `storeutil.ValidateID`;
bodies 0.5–8 KB markdown. `Load(ctx, store, gen)` writes through `store.Store`
inside `Tx` batches under `store.WithAttribution(system:perf-seed)`, faces via
faced ids, and writes one audit record `perf-seed`.
- `internal/cli/dev_seed.go`: `rela dev seed --profile perf --scale N
[--seed S] [--force]`; raw-store write at operator trust (fourth sanctioned
exception after `db migrate`, `history-purge`, data-migration/GC; CLAUDE.md list
updated), refuses when `CountEntities > 0` unless `--force`. On pg the version
sweep captures one version per seeded row; accepted as realistic load and
documented.

*C. Baseline.* Script in `.ignored/perf/` (not committed): creates database
`rela_perf`, `ALTER DATABASE rela_perf SET log_min_duration_statement = 20`,
`SET session_preload_libraries = 'auto_explain'`, `auto_explain.log_min_duration
= 20ms` (reload only, no restart, RR-BPGZ4C); starts `rela-server-postgres
-verbose`; curls the SPA's endpoint set (`_schema`, `_config`, `_dashboard`,
`_sidebar`, list pages 1/2 for each list, `_views` for 5 entities per type,
`_position`, `_search` free text and `type:` queries, kanban pages, `_gantts`,
`_commands`) as reader/editor/manager × default/published/site-nl worlds; parses
`Server-Timing`; collects the postgres log. Results go into the ticket as a
table.

*D. Optimisations* (each its own commit with its budget test; all new dataentry
helpers are free functions taking their read seams, in the `visibleRelationIDs`
shape — never `App` methods, RR-CHIFV2):
1. `store.RelationQuery.EntityIDs []string`: plural of `EntityID`, same
`Direction`/`FromFace` composition; nil = unfiltered, empty = none;
fs/mem/pg/sqlite; storetest asserts `{EntityIDs:[a,b], FromFace:&f}` == union of
the two scalar calls (RR-E0PJ0B). One dataentry helper `pageQueries(req)` builds
every batched `EntityQuery`/`RelationQuery`, stamping
`World`/`FaceIn`/`FromFace` from the request so no call site hand-builds one
(RR-7JCLZP). `servedFacePageEdges` → one `ListRelations{EntityIDs: pageIDs}`;
`worldNeighborsForPage` same. `visibleRelationIDs` → `ListEntityHeaders{IDs}`
then new `visibleReader.filterVisibleHeaders` delegating to the existing
`HeaderFilterer`. `resolveRelationColumnValues` → per-section batch: one
`ListRelations` for all row ids and types, one header batch for targets.
`runVisibleFreeTextSearch` → `ListEntities{IDs}` (world-aware). `computeFaces` →
one `ListEntities{IDs:[id], AllStates:true}`. `tracer` BFS → per-frontier
`ListRelations{EntityIDs}`.
2. `store.GraphHeaderQueryer` optional capability (`GraphQueryHeaders(ctx,
GraphQuery) iter.Seq2[EntityHeader, error]`), pg-native, generic fallback
dropping content; `visibility.listPushdownHeaders` mirrors `listPushdown` for
all three branches; `visibleReader.listHeaders`. Serializer gets a header
variant that omits `content`. `handleV1ListEntities`, `handleV1Search`, kanban
and dashboard card queries use it; `?include_content=true` keeps the entity
path.
3. List pushdown: when there is no relation filter, no free text, and the
sort key is id/updated_at or a declared scalar property (metamodel allowlist),
call `GraphCount` through a helper returning only `matched`, and a paged
`GraphQuery` with `OrderBy`/`Limit`/`Offset` set on the per-request copy inside
`listPushdown` (test mirrors the world-bleed test). pg implements in SQL
(`properties->>$n` parameterised, `NULLS LAST` matching the Go comparator),
fs/mem via `graphquerynaive`.
4. Migration `0014_read_indexes.sql` (above) + `queryplan` eligibility for
list/kanban `where:` and list `sort:` (index `(type, (properties->>sortkey))`),
same all-or-nothing reconcile rule.
5. Hoist `a.Services()`; `_position` via `MatchingIDs` over the sorted id
list; reconcile the plimsoll comment.

**Alternatives considered:**
- A `DBTX` counting decorator instead of the pgx tracer: misses the listener
connection and needs `Begin`-returned `Tx` wrapping; tracer covers all.
- A separate `internal/querystats` package: adds an arch-lint component for
one 40-line type; `store` already is the shared contract.
- Always-on `Server-Timing`: rejected — the query count is row-dependent on
fallback paths and would be a machine-readable existence channel (RR-64OR7D); it
is an operator diagnostic.
- Seeding via `rela import`/`rela sync push` from generated markdown: import
has no face support and sync's relation path cannot address a tail
(BUG-FACEVER); direct store writes avoid both and are 10× faster.
- Seeder as a shell script (extending `generate-test-data.sh`): heredoc per
file is too slow at 20k and cannot write postgres.
- `OmitContent bool` on `EntityQuery`: rejected for the same reason
TKT-1ESTYJ rejected it — a content-less `entity.Entity` lies to the 337
`.Content` readers; keep the typed header.
- A second redaction implementation for headers: rejected; `RedactHeader`
exists and `Redact` is documented non-composable — single point stays.
- Fixing kanban by a dedicated endpoint: not needed once list pages are
cheap and content-free; revisit if measurement says otherwise.

**Files to modify:**
- `internal/store/store.go` (RelationQuery.EntityIDs, GraphQuery paging,
GraphHeaderQueryer), `internal/store/querystats.go` (new),
`internal/store/storetest/*` (conformance for the new fields, counting
decorator)
- `internal/store/pgstore/{tracer.go,open.go,relation.go,entity.go,graphquery.go,derivedschema.go,migrations/0014_read_indexes.sql}`
  + `graphquery_explain_test.go`, new `tracer_stats_test.go`
- `internal/store/{fsstore,memstore,sqlitestore,graphquerynaive}` (EntityIDs, paging, header fallback)
- `internal/queryplan` (list/kanban shapes)
- `internal/visibility/pushdown.go` (header pushdown, paging on the copy)
- `internal/dataentry/{router.go,requeststats.go(new),api_v1.go,worldneighbors.go,relation_visibility.go,views_handler.go,sections.go,queryservice.go,affordances.go,entityserializer.go,visiblereader.go,services.go,app.go}` + tests
- `internal/tracer/tracer.go`
- `internal/perfseed/*` (new), `internal/cli/dev_seed.go` (new), `.go-arch-lint.yml`
- `prototypes/perf/project/*` (new)
- `frontend/src/api/entities.ts` (no content on list fetches; kanban page size)
- `internal/openapi`, `docs/postgres-backend.md`, `docs/cli-reference.md`, `docs/data-entry.md`, `docs/acl-security.md`, `CLAUDE.md`

## Security Considerations

- [x] Input sources identified (user input, config, external APIs)
- [x] Input validation approach defined (allowlist preferred over blocklist)
- [x] Security-sensitive operations identified (file access, auth, crypto)
- [x] Error handling doesn't leak sensitive information

**Input Sources & Validation:**
- `?include_content=true|false`: boolean parse, anything else → false.
- `?page`/`per_page` already validated; pushdown reuses the parsed values
and caps `per_page` as today.
- Sort key pushdown: allowlist = declared scalar properties of the type from
the metamodel; anything else falls back to the Go comparator. Never interpolated
— properties go through `properties->>$n` parameters.
- Seeder flags: `--scale` float in (0, 10], `--seed` int, `--profile` from a
fixed set; operator shell trust, same as `db migrate`.

**Security-Sensitive Operations:**
- *Read-side ACL.* Every batched read replaces a per-row gated read; the
batch runs through the same `filterVisible`/`FilterHeaders` +
`Redact`/`RedactHeader` on raw store rows exactly once. Row-gating stays a
pushed `ReadQuery` allowlist — no runtime deny introduced. Batched queries are
built by one helper that stamps `World`/`FaceIn`/`FromFace`, closing the
AllowAll + FaceIn fail-open shape (AC6b).
- *Count oracle.* Only the ACL-scoped `matched` count reaches the handler
(AC6c), mirroring today's `len(entities)` (RR-SSPCCI).
- *Server-Timing header / request log.* Emitted only under Debug logging;
documented in `docs/acl-security.md` beside the timing-exposure note
(TKT-VR5U3Q) as an operator diagnostic that must not be enabled on a
multi-principal deployment without understanding it exposes per-response query
counts.
- *Seeder* writes raw store (no entitymanager): operator-only, audited,
attributed, refuses non-empty store; never wired into the server.
- *Indexes* change no semantics; the derived reconciler keeps its
all-or-nothing rule (invalid `data-entry.yaml` → no reconcile).

## Test Plan

- [x] Test scenarios documented for each acceptance criterion
- [x] Edge cases identified and documented
- [x] Negative test cases defined (invalid input, error conditions)
- [x] Integration test approach defined (not just unit tests)

**Test Scenarios:**
- AC1: `store/querystats_test.go` (concurrent increments, nil when absent);
`pgstore/tracer_stats_test.go` (DB-gated: N queries → stats N, Tx included, no
ctx alloc when absent and debug off); `dataentry/requeststats_test.go` (header
present on API under Debug, absent under Info, absent on SSE, record fields).
- AC2: `dataentry/querybudget_test.go` over memstore + counting decorator:
list page 10 vs 50 rows with 2 relation columns; view with table section; search
100 hits; kanban page; equality across sizes plus pinned constant.
- AC3: `perfseed` determinism test (same seed twice → identical ids and
property hashes), fs vs mem parity at scale 0.01, refuse-non-empty test, id
validation test, attribution/audit record test.
- AC4: manual, recorded table (before/after), postgres slow log excerpt.
- AC5: EXPLAIN tests per index; reconciler unit test for list `where:` shape
→ spec; DB test create/drop.
- AC6: visibility tests a–d for list, section and search, including the
AllowAll-under-published-world case and the world-bleed mirror test for paging
fields on the copy.
- AC7: e2e suite green; openapi test updated for `include_content`;
`relationsPatch.ts` data source verified.

**Edge Cases:**
- Page with zero relations; neighbour ids duplicated across rows; neighbour
id that no longer exists (dangling relation) — batch returns fewer headers than
ids, map lookup yields "unknown" exactly as today.
- Entity with faces where only a non-default face exists in the world.
- `EntityIDs` empty slice vs nil: nil = unfiltered, empty = matches nothing
(pin in storetest; pg uses `= ANY($1::text[])` with an explicit cast).
- `per_page` > cap, `page` beyond last page under pushdown → empty data,
correct total.
- Sort on a property absent on some rows: pushed `ORDER BY` NULLS LAST must
match the Go comparator's placement; test both.
- Stats when a query errors: still counted, duration recorded.
- Seeder at scale 0.01 (≈200 entities) and `--force` on a non-empty store.

**Negative Tests:**
- `?include_content=maybe` → treated as false.
- Sort key not a declared property → Go path, no SQL error.
- Seeder on non-empty store without `--force` → exit 1, nothing written.
- Invalid `data-entry.yaml` → reconciler leaves existing derived indexes.
- Header path handed to a `.Content` consumer → does not compile (type).

## Risk Assessment

- [x] Technical risks assessed with mitigations
- [x] Security risks assessed (see Security Considerations)
- [x] Effort estimated (xs/s/m/l/xl)

**Risks:**
- *Scope size.* Five optimisation slices plus seeder plus observability is
`xl` territory. Mitigation: ship as sequential PRs on one ticket in the order A
→ B → C → D.1 → D.2 → D.4 → D.3 → D.5, each independently mergeable; because A
emits nothing by default, stop points after C and after D.2 are safe and would
spin the rest into child tickets.
- *Sort-pushdown semantic drift* (`localeCompare` vs `COLLATE "C"`).
Mitigation: pushdown only for keys where the comparators provably agree;
differential test comparing Go path and pushed path on the seeded set.
- *Header pushdown duplicates GraphQuery SQL.* Mitigation: pg builds both
from one `buildGraphQuerySQL` with a projection switch, as
`buildEntityHeaderListSQL` already does for `ListEntities`.
- *Wire change (content omitted).* Some external client may read `content`
from list rows. Mitigation: opt-in `include_content`, documented, openapi
updated; MCP and CLI do not use the v1 list endpoint.
- *Migration on large tables.* GIN on `properties` and `text_pattern_ops`
index builds lock briefly; migrations already run under an advisory lock at
startup. Documented in the postgres guide.
- *Local measurement not representative.* Localhost postgres hides network
latency, which makes N+1 worse in production, not better; the query-count metric
is latency-independent, which is why AC4 is stated in counts first.

**Effort:** l for A–C + D.1–D.2; xl if all of D ships in this ticket.

## Documentation Planning

- [x] User-facing docs identified
- [x] Docs-checklist will be created when entering implementation

**Documentation Impact:**
- [x] docs/postgres-backend.md — "Observing query cost" (Server-Timing under
`-verbose`, request log, slow-query log recipe), new indexes, extended derived
index eligibility, corrected search paragraph (LIKE/trgm, no tsvector)
- [x] docs/cli-reference.md — `rela dev seed`
- [x] docs/data-entry.md — `include_content`, list rows are content-free
- [x] docs/acl-security.md — Server-Timing note beside timing exposure
- [x] CLAUDE.md — rule: "listings read headers, never full entities; batch
neighbour loads by page, never per row"; raw-store exception list 3 → 4
- [x] ~~docs/metamodel.md~~ (N/A: no schema syntax changes)
- [x] ~~README.md~~ (N/A: operator docs live in docs/)

## Design Review

- [x] Run `/design-review` before starting implementation
- [x] All critical/significant findings addressed in plan

**Design Review Findings:** RR-64OR7D, RR-7EJIWL, RR-CH8CX9, RR-BPGZ4C
(self-review); RR-O5W4A3, RR-J1VAW0, RR-7JCLZP, RR-E0PJ0B, RR-WQ3DQS, RR-CHIFV2
(go-architect review). All addressed in the plan above; confirmed claims
(ListEntityHeaders honours World/FaceIn; frontend reads no list-row content; tsv
index unused; next migration 0014) folded into Research.

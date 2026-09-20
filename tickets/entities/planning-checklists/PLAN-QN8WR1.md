---
id: PLAN-QN8WR1
type: planning-checklist
title: 'Planning: PostgreSQL read-path follow-ups: keyset position, title-ranked free text, header-backed view collections, bounded gantt drill-down'
status: done
---

<!-- @managed: claude-workflow v1 -->

## Understanding

- [x] Problem/requirements clearly understood
- [x] Scope defined (what's in/out documented below)
- [x] Acceptance criteria documented with specific test scenarios

**Priority (user, 2026-09-20):** statement counts and database time on the
PostgreSQL backend come first, SQLite second. memstore may be as inefficient as
needed. fsstore gets a speedup only where it is easy.

**Scope:**

IN:
1. `_position` answered by the store. New optional capability
`store.PositionQueryer` with a generic fallback; pgstore answers with one
window-function statement (measured 16 ms against 50 ms plus a Go sort of 11,000
rows). Used only for the request shapes `listpushdown.go` already pushes; every
other shape keeps the Go path.
2. Free-text ranking on the display-title property. Matching stays the LIKE
over `search_text`; ranking becomes `similarity(<title of the row's type>, q)`
(measured 196 ms against 1,345 ms). The type-to-title-property map comes from
the metamodel at wiring. With no map the current prefix ranking stays.
3. View collections read headers. `loadViewEntities` loads bodies only for a
collection that a `display: content` or `cards` section renders.
4. Gantt drill-down reads headers and loads hierarchy edges for the subtree
ids only, instead of whole entities and every edge of the relation type.
5. SQLite: native `ListEntityHeaders`, and `GraphQueryHeaders`/`GraphQuery`/
`CountMatched` in SQL for the simple shape (type, property equality, world,
order, limit, offset; no relation predicate, no `Any`). Other shapes keep
`graphquerynaive`.

OUT:
- The full gantt forest. The roll-up fold needs every descendant.
- fs seeding speed. The profile shows bleve segment merges from one index
update per entity; batching observers inside `Tx` is not an easy change.
- SQL evaluation of relation predicates on SQLite (recursive CTEs). Own ticket.
- memstore efficiency.

**Acceptance Criteria:**
1. `_position` on a pushable scope issues no whole-type read on pgstore: a
DB-gated test counts statements through `store.QueryStats` and asserts the count
is independent of row count; a differential test asserts the store answer equals
the Go path for asc, desc, absent and JSON-null sort keys, first, last,
single-row and not-in-scope ids.
2. `GraphPosition` conformance (`storetest.RunGraphPositionTests`) passes on
all four backends, including a scoped (ACL-shaped) query and a world.
3. Search: `TestSearch_RanksTitleMatchAboveBodyMention` and
`RunVisibleSearchTests` pass with the title map configured and without it; gated
and ungated builders stay in lockstep (existing contract test).
4. Views: a `storetest.Counting` test shows a table-only view issues no
full-entity list read; a view with a `content` section still returns bodies;
redaction and the inaccessible marker behave as before.
5. Gantt drill-down: response is byte-identical to the full-build answer for
the same root (existing parity tests), and the relation read carries
`EntityIDs`.
6. SQLite: storetest `RunAll` passes; a differential test runs every simple
shape through both the SQL path and `graphquerynaive` and compares rows and
order.
7. Re-measured table in the ticket body shows the drop for each path.

## Research

- [x] ~~For larger features: run `/research`~~ (N/A: each change is dictated by a measurement recorded in the ticket)
- [x] Searched for existing libraries that solve this problem
- [x] Checked codebase for similar patterns or reusable code
- [x] Looked for reference implementations in other projects
- [x] Reviewed relevant rela concepts for prior art

**Research Doc:** N/A

**Existing Solutions:**
- `store.MatchedCounter`/`GraphHeaderQueryer` are the pattern for an optional,
type-asserted capability with a generic fallback; `PositionQueryer` copies it.
- `buildGraphQuerySQLSelect` already produces the ordered, world-wrapped SELECT;
the position statement wraps the same ORDER BY in window functions, so list
order and position order cannot drift.
- `planListPushdown` is the single eligibility decision; `_position` reuses it
with page arguments ignored.
- `metamodel.EntityDef.GetPrimaryProperty` defines the title property.
- sqlitestore already has `buildEntitySelectSQL(q, alias, columns)` and
`jsonprops.go`; pgstore's `buildEntityHeaderListSQL` is the model.
- Keyset comparison was measured faster (8 ms) and rejected: absent-as-largest
ordering with mixed directions makes the row comparison a per-key disjunction,
and the window form is correct by construction.

## Approach

- [x] Technical approach chosen and documented
- [x] Approach builds on existing patterns (not reinventing)
- [x] Alternatives considered (document why rejected)
- [x] Dependencies identified (packages, APIs, types)

**Technical Approach:**

*1. Position.* `store.Position{Index, Total int; Prev, Next *EntityRef}` with
`EntityRef{ID, Type}`. `PositionQueryer.GraphPosition(ctx, q, id) (Position,
bool, error)`; `store.GraphPosition` falls back to streaming `GraphQueryHeaders`
with paging cleared. pgstore: `SELECT id, type, rn, total, prev_id, prev_type,
next_id, next_type FROM (SELECT ..., row_number() OVER w, count(*) OVER (),
lag/lead OVER w FROM (<graph select without paging>) WINDOW w AS (<same ORDER
BY>)) WHERE id = $n`. `handleV1EntityPosition`: for `source=list`, build the
plan; eligible → store; else existing path.

*2. Search rank.* `pgstore.WithSearchTitleProperties(map[type]property)` on the
search backend and store (both builders read one shared value). `rankExpr`
becomes `similarity(CASE WHEN type = ANY($a) THEN properties->>$p ... ELSE id
END, $q)`, all values bound. `pgstore.Open` gains variadic options; the postgres
recipe passes the map built from the metamodel. Template display properties have
no single property and fall to `id`.

*3. Views.* `viewContentSources(view)` returns the collection names rendered by
`content`/`cards` sections. `applyViewTraverse` passes `wantContent` for the
rule's target collection to `loadViewEntities`, which reads
`store.ListEntityHeaders{IDs, World}` and `headerEntity` otherwise. The
synthetic view in `sections.go` is covered by the same function.

*4. Gantt.* `collectGanttRound` uses `store.GraphQueryHeaders` +
`visibility.RedactHeader`. `ganttEdgesForType` takes the node ids in subtree
mode and queries `RelationQuery{Type, EntityIDs}` (both directions), which still
sees an outside parent claiming an in-set node.

*5. SQLite.* `ListEntityHeaders` via the header column list.
`simpleGraphShape(q)` gate; SQL built from `buildEntitySelectSQL` plus
`json_extract`-based predicates that follow `propmatch` emptiness, `ORDER BY ...
COLLATE BINARY` with absent/null last ascending and first descending,
`LIMIT/OFFSET`.

**Files to modify:** `internal/store/{graphquery.go,position.go}`,
`storetest/{graphposition.go,storetest.go,counting.go}`,
`pgstore/{graphquery.go,position.go,search.go,visiblesearch.go,open.go,pgstore.go}`,
`sqlitestore/{entity.go,graphquery.go,graphsimple.go}`,
`appbuild/appbuild_postgres.go`, `cli/mcp_wiring_postgres.go`,
`dataentry/{scope.go,listpushdown.go,viewworld.go,views.go,gantt_handler.go}`
and tests; `docs-project` guides for postgres and sqlite.

## Security Considerations

- [x] Input sources identified (user input, config, external APIs)
- [x] Input validation approach defined (allowlist preferred over blocklist)
- [x] Security-sensitive operations identified (file access, auth, crypto)
- [x] Error handling doesn't leak sensitive information

**Input Sources & Validation:** the `_position` id and scope are already
validated; the id reaches SQL only as a bound parameter. Sort and filter keys
pass the existing pushdown allowlist. Title property names come from the
metamodel and are bound, not interpolated.

**Security-Sensitive Operations:**
- Position runs the principal's compiled read query, as the list pushdown
does, so `Total`, `Prev` and `Next` cover visible rows only. An id outside the
visible set yields the same 404 as today.
- Header-backed view collections go through the same gate and redaction; the
header carries `Inaccessible`.
- Gantt keeps gate, redact once, fold, cap in that order. Edges with an
endpoint outside the gated node set are still dropped.
- Ranking by title does not change which rows match, only their order, and
the gated builder orders by the same expression. A redacted title property could
influence order for a principal who cannot see it; the current prefix ranking
has the same property today (the prefix holds all string properties), so this is
not a new channel. Recorded for the design review.

## Test Plan

- [x] Test scenarios documented for each acceptance criterion
- [x] Edge cases identified and documented
- [x] Negative test cases defined (invalid input, error conditions)
- [x] Integration test approach defined (not just unit tests)

**Test Scenarios:** as listed per acceptance criterion; pg tests are DB-gated
and also run through a schema-pinned DSN where the statement is new.

**Edge Cases:** id first/last/only row; id absent; duplicate sort values (id
tiebreak); absent and JSON-null keys; world with `otherwise: exclude`; empty
title map; type with a template display property; view rule whose target feeds
both a table and a content section; gantt root with an outside parent and with a
cycle through the root; SQLite property value containing quotes.

**Negative Tests:** store error in position surfaces as the list pipeline error
class; ineligible scope silently takes the Go path; SQLite non-simple shape
takes the naive path.

## Risk Assessment

- [x] Technical risks assessed with mitigations
- [x] Security risks assessed (see Security Considerations)
- [x] Effort estimated (xs/s/m/l/xl)

**Risks:**
- Ranking change is user visible: body mentions no longer affect order.
Mitigation: documented; title matches first is the intended behaviour.
- SQLite SQL path drifting from naive semantics. Mitigation: narrow gate plus
the differential test.
- `pgstore.Open` signature change touches two wiring sites. Variadic, so
existing calls compile.

**Effort:** m

## Documentation Planning

- [x] User-facing docs identified (skip if internal refactor)
- [x] Docs-checklist will be created when entering implementation

**Documentation Impact:**
- [x] docs/postgres-backend.md (search ranking paragraph)
- [x] docs/sqlite-backend.md (which query shapes run in SQL)
- [x] ~~docs/metamodel.md~~ (N/A: no schema syntax change)
- [x] ~~docs/cli-reference.md~~ (N/A: no command change)
- [x] ~~README.md~~ (N/A)

## Design Review

- [x] Run `/design-review` before starting implementation
- [x] All critical/significant findings addressed in plan

**Design Review Findings:** RR-R8D16P, RR-MR9811, RR-RBGNO0 (critical); RR-KJDYTU, RR-HXW679, RR-86E1L8, RR-CGGMVM, RR-VFHYZS, RR-EL3500 (significant); RR-MW946F, RR-PVH4VZ, RR-FVD7H2, RR-O2QK4Y, RR-T7WMMF (minor).

**Plan amendments from the review (these override the text above):**
- Rank expression is `similarity(lower(CASE ...), q)`. The title map holds only a declared `display_property` or a conventional `title`/`name`/`label`; other types rank by id. It reaches pgstore as `map[string]string` from `appbuild_postgres.go`.
- `_position` uses the store only for `source=list` and only through ONE eligibility function shared with `listPage` (view condition, query scope, world, then `planListPushdown`).
- Views: traversal always reads headers; after the fixpoint, ONE batched read loads bodies for ids in collections that a section renders with anything other than `table`, `properties` or `list`. The command runner asks for all bodies explicitly.
- sqlitestore chunks `EntityIDs` relation reads under the bind-variable limit.
- SQLite graph pushdown gate is a zero-value allowlist: a query with any field it does not translate declines to `graphquerynaive`.
- Position window runs over the finished graph select as a subquery, with tests for a world and an empty `OrderBy`.
- New dataentry logic is package functions. Store plimsoll directives rise by the optional-capability methods, with the reason written at the directive.

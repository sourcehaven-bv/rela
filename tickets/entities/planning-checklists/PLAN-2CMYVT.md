---
id: PLAN-2CMYVT
type: planning-checklist
title: 'Planning: Index markdown id mentions as backlinks'
started: "2026-10-07"
completed: "2026-10-07"
status: done
---

<!-- @managed: claude-workflow v1 -->

## Understanding

- [x] Problem/requirements clearly understood
- [x] Scope defined (what's in/out documented below)
- [x] Acceptance criteria documented with specific test scenarios

A code span in an entity body whose whole content is an entity id (`B-1`)
renders as a link to B. Only the forward direction exists. B cannot list the
entities that mention it. User decisions (2026-10-07): a derived index, not real
relations; surfaces are the data-entry entity page, REST API, CLI, MCP and Lua;
entity bodies are the only source.

**Scope:**

In scope:
- Extract mentions from every entity body on write, on all four backends (fs, memory, postgres, sqlite).
- One read capability: "which live entity states mention id X".
- ACL row-gating of the sources through the existing gated graph query.
- Surfaces: `GET /api/v1/{plural}/{id}/mentioned-by`, a "Mentioned by" section on the SPA entity page, `rela show` (table and JSON), MCP `show_entity`, Lua `rela.get_mentioned_by(id)`.
- Backfill of existing data on each backend.

Out of scope:
- Relation bodies and comments as sources.
- Traversal of mentions in tracer, `GraphQuery`, `related(...)` conditions, list columns or filters.
- Rewriting mention text in other bodies when B is renamed.
- Mentions as relations in audit, versioning, automations or cardinality.

**Acceptance Criteria:**
1. A's body contains `` `B-1` ``; B's mentioned-by lists A on every backend.
2. Editing A to remove the span removes A from B's list. Editing it back restores it.
3. Deleting A removes it. Soft-deleting A hides it; undo brings it back.
4. Renaming A to A2 lists A2, not A. Renaming B to B2 leaves B2 with no backlink from A (the text in A still says `B-1 `).
5. A mention written before B exists appears once B is created.
6. A mention of a faced entity renders as a link to the face the reader's world serves, and the faced entity lists its backlinks.
7. Fenced/indented code blocks, link text, and spans with extra text (`` `see B-1` ``) do not count.
8. A source the principal cannot read is absent from every surface. A hidden source and no source are indistinguishable.
9. Postgres: a write through process 1 is visible to a read through process 2 without restart.
10. fs: an external edit to A's file (git pull, editor) is reflected, both while watching and at the next process start.
11. Existing projects are backfilled: upgrading a pg/sqlite database or opening an fs project lists backlinks for content written before the upgrade.
12. Each surface (REST, SPA, CLI table + JSON, MCP, Lua) shows the list; SPA hides the section when empty.

## Research

- [x] For larger features: run `/research ` to create a structured research doc
- [x] Searched for existing libraries that solve this problem
- [x] Checked codebase for similar patterns or reusable code
- [x] Looked for reference implementations in other projects
- [x] Reviewed relevant rela concepts for prior art

**Research Doc:** N/A. Options were surveyed in planning and the user chose the
approach directly (recorded under Approach).

**Existing Solutions:**
- Detection: `internal/dataentry/mentions.go ` `scanCodeSpanCandidates ` (pure goldmark scan of `ast.CodeSpan `). No id shape check; existence decides. Lua `entity_refs `/`resolve_refs ` (`internal/lua/markdown.go `) apply the same rule.
- Derived-index precedent: pgstore `search_text ` computed in Go and written in the same statement (`pgstore/entity.go:1322 `, `:345 `, `:463 `), re-keyed on rename in `rekeyStateFamily `. sqlite FTS5 maintained inside the write transaction. fs bleve is an in-process observer with an mtime watermark.
- Soft delete on pg moves rows to `marked_entities `, so any read joined to live `entities ` already excludes marked sources.
- Inbound gating precedent: `visibility.Resolver.EndpointsReadableErr ` and `FilterHeaders `.
- Obsidian-style backlinks are the reference UX: a flat "linked mentions" list on the target page.

## Approach

- [x] Technical approach chosen and documented
- [x] Approach builds on existing patterns (not reinventing)
- [x] Alternatives considered (document why rejected)
- [x] Dependencies identified (packages, APIs, types)

**Technical Approach:**

**As built (supersedes items 2 and 7 below, and the rejected "separate store
capability" alternative):**

- The `GraphQuery.MentionsTarget ` predicate was dropped. Backlinks use an optional store capability, `store.MentionIndex.MentionSources(ctx, target, after, limit) `, keyset-paged in `(id, face) ` order and returning only live rows. The two original objections are answered:
  - Bypass: an archguard test (`internal/archguard/mentionindex_test.go `) allows the capability only under `internal/store/ ` and `internal/visibility/ `.
  - Paging: the capability is keyset-paged. `visibility.Resolver.MentionedBy ` gates, ranks and redacts each page, with a scan budget of 1000 raw rows per call.
- Plimsoll: each store's directive went up by one (the one new method). Helpers became free functions.
- fsstore: a lazy index built under the write lock (no generation counter), persisted to `mentions-index.json ` with per-file `(mtime, size, targets) `. Entries are filtered by liveness on read, so deletes need no hook.
- pgstore: `mention_targets TEXT[] ` with a GIN index plus `mentions_hash `. A trigger clears the targets when an older binary changes a body, and a background reindexer refills NULL rows at open and every 5 minutes.
- sqlitestore: a `mention_targets ` JSON column fanned out by triggers into `entity_mention_targets `. NULL rows are indexed on read.
- Lua `entity_refs ` was left unchanged: it resolves ids and does not scan bodies for the index.
- Cursor encoding lives in `visibility ` (`MentionPage.Cursor `, `ParseMentionCursor `), shared by REST and Lua.

1. Extraction (pure). New leaf package `internal/mentions ` (imports goldmark and `internal/entity ` only): `Targets(body string) []string ` keeps code spans with plain text only that `entity.ValidateID ` accepts, sorted, deduplicated, capped at 1000 per body (logged when truncated). `dataentry.collectMentions ` and Lua `entity_refs ` call it. Arch-lint edges added for the four stores and dataentry, each with a justifying comment.
2. Query. `store.GraphQuery ` gains `MentionsTarget string `: match rows whose own body mentions the id. It is a caller predicate ANDed with everything else, so the ACL `ReadQuery `, faces, paging, ordering and `CountMatched ` apply unchanged. Backlinks are `GraphQueryHeaders ` through the visibility-gated reader every surface already uses. No new store method or capability. `graphquerynaive ` evaluates it through its existing source interface; `storetest.RunGraphDifferential ` covers it on both database backends.
3. Targets are stored for every accepted shape, existing or not, so a mention written before B exists works once B is created. Renaming B does not rewrite targets. Rows are per face: each face's body is indexed on its own row.
4. postgres: migration `0020_mention_targets.sql ` adds `entities.mention_targets text[] `, `entities.mentions_hash text `, and a GIN index on `mention_targets `. Both are written in the same statement as `search_text `. Rename, delete, DeleteFace, mark, unmark and PurgeMarked carry or drop the row, so they need no extra code. Predicate: `mention_targets @> ARRAY[$n] `. An open-time pass, paged and outside the migration transaction, reindexes rows where `mentions_hash ` is NULL or differs from the content hash. That covers the backfill and writes from older binaries during a rolling deploy (healed at the next process start; documented).
5. sqlite: `entities.mention_targets ` JSON column computed in Go, a fan-out table `entity_mention_targets(rowid, target) ` kept by AFTER INSERT/UPDATE/DELETE triggers through `json_each ` (mirroring `entity_search `), indexed on `target `. Predicate: `EXISTS ` on the fan-out table. Same open-time reindex pass for NULL columns. Single-process, so no mixed-version case.
6. memstore: reverse map `target → set(stateKey) ` plus forward map, updated under the store lock on create, update, delete, DeleteFace, rename, mark and unmark, and visible to Tx views.
7. fsstore: in-memory forward/reverse maps built lazily on the first query that uses the predicate. The build runs under a generation counter and merges only entries untouched since it started. Persisted as `mentions-index.json ` under the cache dir with per-file `(mtime, size, targets) `, written atomically through `RootedFS `; later builds stat every file and reparse only changed ones. Marked entities are excluded; inaccessible (git-crypt) files have no mentions. Write paths and the watcher's `reconcileEntityPath `/`handleEntityRemoval ` keep it current once built.
8. Faced target types: forward mentions resolve through the world-aware batch resolver relation neighbours use (`worldNeighbors.resolveHeads `, `worldneighbors.go:201 `) instead of `loadDefaultFaceHeaders `, so `` `POL-1` `` links to the face the reader's world serves. Ids that resolve today give the same result. Backlinks key on the bare id, so a faced B lists its sources on any of its faces.
9. Titles: computed at each surface through a helper extracted from `buildMention ` (redacted header in, title out). `internal/visibility ` only gates.
10. Surfaces:
- REST `GET /api/v1/{plural}/{id}/mentioned-by?limit=&cursor= ` returns `{items: [{id, type, face?, title}], next_cursor?} `. B is resolved through `visibleReader.getVisible ` first, so a hidden B gives the same 404 as an absent one. No total count.
- SPA: "Mentioned by" card on `EntityDetail.vue ` beside `RelationCards `, first page plus "Show more", hidden when empty.
- CLI `rela show `: "Mentioned by:" block with `← A-1 (type) title ` lines, capped at 50 with a "… more" line; JSON field `mentioned_by ` plus `mentioned_by_truncated `. Runs with the CLI's existing operator-trust reader.
- MCP `show_entity `: `mentioned_by ` (cap 50) plus `mentioned_by_truncated `.
- Lua: `rela.get_mentioned_by(id, {limit=, cursor=}) ` through the script's gated reader. Not added to `admin.* `.

**Alternatives rejected:**
- Real relations of a built-in type: writes relation files, pollutes audit, versioning, automations and cardinality, and needs a built-in type in user schemas.
- In-process observer for all backends: misses writes from other postgres processes.
- Search-then-verify: loads bodies per request and cost grows with how often the id appears as text.
- Side table on postgres: needs bookkeeping on every row-moving path (create, update, both deletes, rename, mark, unmark, purge). A column on the row is carried by all of them for free.
- Separate store capability (`MentionSources `): unpaged, bypassable by type assertion, and adds store methods over the plimsoll caps. The predicate reuses the gated, paged query path.

**Files to modify:**
- `internal/mentions/ ` (new), `.go-arch-lint.yml `
- `internal/dataentry/mentions.go `, `internal/lua/markdown.go `
- `internal/store/graphquery.go `, `internal/store/graphquerynaive/naive.go `, `internal/store/storetest/ ` (new `mentions.go `, differential generator)
- `internal/store/pgstore/migrations/0020_mention_targets.sql `, `pgstore/entity.go `, `graphquery.go `, new `mentions.go ` (reindex pass), `open.go `
- `internal/sqlitedb/migrate.go `, `sqlitedb.go `, `internal/store/sqlitestore/entity.go `, `graphsql.go `, new `mentions.go `
- `internal/store/memstore/memstore.go `, `graphquery.go `, `tx.go `, new `mentions.go `
- `internal/store/fsstore/ ` new `mentions.go `, `entity.go `, `watcher.go `, `softdelete.go `, `graphquery.go `
- `internal/visibility/policyreader.go ` (TODO text only)
- `internal/dataentry/api_v1.go ` + route, `internal/apiwire/v1/responses.go `
- `frontend/src/ ` API client, `EntityDetail.vue `, new `MentionedByCard.vue `
- `internal/cli/show.go `, `internal/cli/output.go `, `internal/mcp/convert.go ` (+ show_entity tool), `internal/lua/runtime.go `
- Docs listed below.

## Security Considerations

- [x] Input sources identified (user input, config, external APIs)
- [x] Input validation approach defined (allowlist preferred over blocklist)
- [x] Security-sensitive operations identified (file access, auth, crypto)
- [x] Error handling doesn't leak sensitive information

**Input Sources & Validation:**
- Entity bodies (untrusted user markdown). Only code-span text accepted by `entity.ValidateID ` is stored, at most 1000 per body. Stored values are bound SQL parameters only.
- Path id on the REST route: validated as today (`isSafePathSegment `), B must pass the row gate first.
- REST `limit ` (1–200, default 50) and opaque `cursor `; invalid values return 400.
- Lua id argument: passed to the gated reader; empty id raises.

**Security-Sensitive Operations:**
- Existence disclosure. A backlink reveals that A exists and mentions B. Sources are row-gated per face through `visibility `; hidden sources are dropped silently, so the list length is post-filter. Titles redact like `collectMentions ` (BUG-R9EHKV).
- Body redaction does not exist today (`policyreader.go ` TODO). If it is added, the mention index becomes a body-content oracle. The TODO text names the mention predicate.
- Pagination and a per-body target cap bound the work an author can impose on readers of B.
- The derived list is per-principal and is never cached across principals.
- No new raw-store write path; index rows are written inside the existing store writes.

## Test Plan

- [x] Test scenarios documented for each acceptance criterion
- [x] Edge cases identified and documented
- [x] Negative test cases defined (invalid input, error conditions)
- [x] Integration test approach defined (not just unit tests)

**Test Scenarios:**
- AC1–6: `storetest.RunMentionTests `, run by all four backends (pg via `just test-postgres `). Covers create, update, delete, soft delete + unmark, rename source, rename target, target created later, faced sources, tx rollback leaves no rows (pg/sqlite).
- AC6: table tests for `markdown.MentionTargets `.
- AC7: `visibility ` unit test with a policy hiding one source; dataentry handler test asserting the hidden source is absent and that a hidden B returns the same 404 as a missing id.
- AC8: pgstore two-store test on one schema-pinned DSN.
- AC9: fsstore test editing a file on MemFS then (a) delivering a watcher event and (b) reopening the store with a stale persisted index.
- AC10: pg/sqlite migration test seeding rows before the migration; fs test with no persisted index.
- AC11: handler test, CLI golden test, MCP tool test, Lua binding test, Vitest for the card, one e2e test on the entity page.
- Budget test (`storetest.Counting `): mentioned-by cost is the same at 10 and 50 sources.
- Conformance also covers PurgeMarked, DeleteFace of one face, a write during the fs lazy build, a pg row written without the column (stale hash healed at reopen), and the target cap.
- Paging: 120 sources, limit 50, three pages, no duplicates; hidden sources interleaved do not shorten a page.
- Differential: `RunGraphDifferential ` generator includes `MentionsTarget `.
- Faced targets: a mention of a faced entity renders as a link under each world (dataentry and Lua); a faced B lists its backlinks; a mention whose served face is hidden from the reader stays plain code.

**Edge Cases:**
- Empty body, body with only code blocks, nested emphasis inside a span, unicode id shapes rejected by `ValidateID `.
- Self-mention: A mentioning A lists A on its own page (consistent with forward rendering).
- Same target mentioned twice in one body: one row.
- Two faces of A both mention B: both faces listed when readable; a principal who can read only one face sees one.
- Very large body or more than 1000 distinct spans: first 1000 sorted targets kept, warning logged.

**Negative Tests:**
- `` `not an id` `` and `` `B-1@face` `` are not indexed.
- Unknown target id returns an empty list, not an error.
- A store without the capability: surfaces return an empty list and log once (only reachable in test fakes).

## Risk Assessment

- [x] Technical risks assessed with mitigations
- [x] Security risks assessed (see Security Considerations)
- [x] Effort estimated (xs/s/m/l/xl)

**Risks:**
- fs cold build reads every body once. Mitigation: lazy build, persisted per-file stamps; later runs stat only.
- pg backfill on large tenants. Mitigation: paged scan, one-time, under the migration lock, marker row makes it resumable.
- Mixed-version pg writers: index stale for rows they write until the next process start. Documented.
- Write-path cost: one goldmark parse per body write. Measured with the existing perf project; the forward renderer already parses the same body.
- Drift between backends. Mitigation: one conformance suite.
- Write paths that bypass the normal entity write (bulk migrator, data migration, perf seed, markdown import) could skip indexing. Mitigation: index inside the lowest store write function each backend shares; conformance test covers import on sqlite.

Effort: xl.

## Documentation Planning

- [x] User-facing docs identified (skip if internal refactor)
- [x] Docs-checklist will be created when entering implementation

**Documentation Impact:**
- [x] docs/data-entry.md: "Mentioned by" section and mention rules
- [x] docs/data-entry/api-reference.md: `mentioned-by ` endpoint
- [x] docs/cli-reference.md: `rela show ` output
- [x] docs/mcp-server.md: `show_entity ` field
- [x] docs/lua-scripting.md: `rela.get_mentioned_by `
- [x] docs/acl-security.md: backlinks are row-gated; body-redaction caveat

## Design Review

- [x] Run `/design-review ` before starting implementation
- [x] All critical/significant findings addressed in plan

**Design Review Findings:** 18 review-responses linked to TKT-Q6AKUN (7
significant), all addressed in this plan.

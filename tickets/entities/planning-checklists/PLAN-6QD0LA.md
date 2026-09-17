---
id: PLAN-6QD0LA
type: planning-checklist
title: 'Planning: Database-backed comment stores: pgcomments and sqlitecomments over an injected pool'
status: in-progress
---

<!-- @managed: claude-workflow v1 -->

## Understanding

- [x] Problem/requirements clearly understood
- [x] Scope defined (what's in/out documented below)
- [x] Acceptance criteria documented with specific test scenarios

**Scope:**

Recorded in full on TKT-OGTVJW (§ Scope, § Out of scope). In short: two new
`comments.Store` backends over an injected database handle, `pgstore.Open`
losing its DSN form so the composition root owns the pool, and per-build backend
selection in `internal/appbuild/comments.go`. No interface change, no
HTTP/ACL/metamodel change, no migration of existing `.rela/comments/` YAML.

**Acceptance Criteria:**

The eight criteria on TKT-OGTVJW. Each maps to a named test under § Test Plan.

## Research

- [x] ~~For larger features: run `/research`~~ (N/A: the approach was fixed by
existing precedent, not open — see below)
- [x] ~~Searched for existing libraries~~ (N/A: this is two SQL tables against
pools the project already owns)
- [x] Checked codebase for similar patterns or reusable code
- [x] Looked for reference implementations in other projects
- [x] Reviewed relevant rela concepts for prior art

**Research Doc:** N/A — the design question was settled by prior art in-tree.

**Existing Solutions:**

- **TKT-VC27L3 (`state.KV` on postgres)** is the same defect class and supplied
the fix shape: node-local runtime state under a multi-process topology, moved
into the tenant's schema so schema-per-tenant scopes it for free.
- **TKT-4NU9ZD (sqlite content versioning)** vs **TKT-L1A3PH (`state.KV` stays
on the filesystem for sqlite)** are the two competing precedents for the sqlite
tier. Versioning wins here: commentary is content ABOUT content, so it must
travel with `rela.db`, whereas a render cache buys a single process nothing.
- **`pgstore.New(db DBTX)` + `NewSearchBackend(pool)`** is the injection
pattern already in the tree; the two new backends are a third and fourth
consumer of the same pool, not a new mechanism.
- **`commentstest.RunAll`** already existed from TKT-FIO205, which is what made
a second and third backend cheap.

## Approach

- [x] Technical approach chosen and documented
- [x] Approach builds on existing patterns (not reinventing)
- [x] Alternatives considered (document why rejected)
- [x] Dependencies identified (packages, APIs, types)

**Technical Approach:**

Each backend declares its OWN narrow handle interface (`DBTX`) and depends only
on `internal/comments` and `internal/entity` — never `internal/store`, which
arch-lint enforces. One row per comment, keyed `(target_key, id)`, with the
anchor as a JSON/JSONB discriminated union so adding an anchor kind never
migrates stored rows.

`pgstore.Open` splits into `NewPool(ctx, dsn) (DBTX, io.Closer, error)` and
`Open(ctx, db DBTX, dsn)`. `NewPool` returns `DBTX` rather than `*pgxpool.Pool`
specifically so appbuild never imports pgx (CI asserts the default build does
not link it). `Migrate` becomes the caller's job. The DSN survives only where
the listener needs its own dedicated connection.

The recipe seam is the existing `backendOverrides` struct, which already carried
`projectConfig` and `stateKV`; `commentStore` joins them and a nil value selects
`filecomments`.

**Alternatives rejected:**

- *Keep a DSN-taking `Open` as a convenience wrapper* — rejected on the user's
call: two entry points leave the next backend free to pick the one that hides
pool ownership again. Test call sites moved to a shared helper instead.
- *Have `Open` also build the comments backend* — the other way to resolve
"`Open` is doing too much or too little". Rejected: it would put a third
unrelated consumer inside the store package, which is the coupling the split
exists to remove.

**Files to modify:**

- NEW `internal/comments/pgcomments/`, `internal/comments/sqlitecomments/`
- NEW `internal/store/pgstore/migrations/0015_comments.sql`
- `internal/sqlitedb/sqlitedb.go`, `internal/sqlitedb/migrate.go` (schema 4→5)
- `internal/store/pgstore/open.go`
- `internal/appbuild/{appbuild,comments,appbuild_postgres,configloader_sqlite}.go`
- `.go-arch-lint.yml`, `docs-project/entities/guides/GUIDE-comments.md`

## Security Considerations

- [x] Input sources identified (user input, config, external APIs)
- [x] Input validation approach defined (allowlist preferred over blocklist)
- [x] Security-sensitive operations identified (file access, auth, crypto)
- [x] Error handling doesn't leak sensitive information

**Input Sources & Validation:**

- **Comment body and anchor ref** — user-supplied over HTTP. Unchanged from
stage 1: validated by `comments.Service` above the backend, stored as opaque
text/JSON and parameterised in every statement. A backend that validated again
would just be a second place to get it wrong.
- **Entity id, in `Rename` and `DeleteAllFaces`** — reaches SQL as a LIKE
PATTERN, which is the one genuinely new input surface here. `ValidateID` admits
`[A-Za-z0-9_-]`, and `_` is LIKE's single-character wildcard, so the id is
escaped (`\`, `_`, `%`) with an explicit `ESCAPE '\'`. Unescaped, renaming
`TKT_1` would also re-key `TKT-1` and silently merge two unrelated threads.
Pinned by `TestRenameDoesNotMatchLikeWildcards` on both backends.

**Security-Sensitive Operations:**

- **Tenant isolation** — rows sit in the store's schema and are reached through
the schema-pinned pool, so schema-per-tenant scopes comments exactly as it
scopes entities. `TestSchemaIsolation` proves two schemas on one database do not
see each other's comments.
- **Authorization is unchanged and stays above the backend** — `comments.Service`
and `internal/comments/authz.go` gate every call; a store is a dumb row sink by
design, and giving it an opinion would create a second policy point.
- **No credential handling is added** — both backends receive a live handle and
never see a DSN, so there is no new place for one to be logged or leaked.
- **Error messages** name the target key and comment id (both non-secret
identifiers the caller supplied) and wrap the driver error; no row content is
echoed.

## Test Plan

- [x] Test scenarios documented for each acceptance criterion
- [x] Edge cases identified and documented
- [x] Negative test cases defined (invalid input, error conditions)
- [x] Integration test approach defined (not just unit tests)

**Test Scenarios:**

| AC | Test |
|----|------|
| 1 | `TestConformance` in both packages (`commentstest.RunAll`) |
| 2 | `commentstest`' concurrency case, run against both backends |
| 3 | `pgcomments.TestCrossProcessVisibility` — two independent pools, one schema |
| 4 | `pgcomments.TestSchemaIsolation` |
| 5 | Compiles: `Open` no longer accepts a DSN. `appbuild` postgres recipe tests |
| 6 | `sqlitecomments.TestCommentsSurviveReopen`; `state.KV` wiring untouched |
| 7 | `TestBuildComments_NilBackendUsesFilesystem`, `TestBuildComments_BackendOverrideIsUsed`, `TestBuildComments_DisabledYieldsNilService` |
| 8 | `just arch-lint` |

Integration is genuine, not mocked: the pg suite runs against a real PostgreSQL
behind the `RELA_TEST_DATABASE_URL` gate (skip by default, hard FAIL when
`RELA_TEST_DATABASE_REQUIRED` is set, mirroring pgstore), and the sqlite suite
opens real `rela.db` files.

**Edge Cases:**

- **Sub-second timestamp ordering on sqlite.** `List` orders in SQL as a STRING
compare, and RFC3339Nano is variable-width: Go strips trailing zeros and omits
the fraction entirely at a whole second, so `12:00:00Z` sorts AFTER
`12:00:00.000000005Z`. A thread would silently reorder between reads. Fixed with
a nine-digit fixed-width format; `TestOrderingWithSubSecondTimestamps` fails
under RFC3339Nano.
- **Ties on `created_at`** — broken by the server-minted id, so a coarse clock
cannot reorder a thread.
- **`_` in an entity id** — see § Security.
- **Rename into an OCCUPIED destination** — merges rather than discarding, since
rela permits id reuse and dropping the occupant's comments would destroy data
nobody asked to remove.
- **Rename must move EVERY face**, not just the bare id, or a draft thread is
stranded at an id that no longer exists.
- **Never-edited comments** — `updated_at` is SQL NULL, not the zero time, which
would round-trip as year 1 and read as a real edit.
- **Empty thread** — returns a non-nil empty slice, so JSON is `[]` not `null`.

**Negative Tests:**

- `TestNewRejectsNilHandle` on both backends: a nil handle fails at
construction, not at the first comment anyone posts.
- `Update`/`Delete` of an absent comment return `comments.ErrNotFound` rather
than succeeding silently (zero rows affected is not success).
- `TestBuildComments_EnabledWithoutPathsFails`: a configured feature with
nowhere to store data is a wiring error, not a silent downgrade.

## Risk Assessment

- [x] Technical risks assessed with mitigations
- [x] Security risks assessed (see Security Considerations)
- [x] Effort estimated (xs/s/m/l/xl)

**Risks:**

- **Silent fallback to `filecomments`.** The highest-consequence risk: a wiring
regression produces a fully working service that passes every other test, while
a postgres deployment quietly reverts to node-local comments — the exact defect
this ticket fixes, with no error to notice. Mitigated by
`TestBuildComments_BackendOverrideIsUsed`, which asserts the comment lands in
the supplied backend AND that no `.rela/comments/` directory is created;
verified by mutation (breaking the wiring fails the test).
- **`pgstore.Open`'s signature change is wide.** Mitigated by deleting the DSN
form outright, so no call site can be silently left behind — a missed one does
not compile.
- **appbuild accidentally linking pgx**, breaking the default build's dependency
guarantee. Mitigated by `NewPool` returning `DBTX` + `io.Closer`; arch-lint and
the CI `go list -deps` assertion both cover it.
- **Pool leak on a partial failure in the postgres recipe.** Mitigated by
closing the pool on every early return.

**Effort:** l (as recorded on the ticket).

## Documentation Planning

- [x] User-facing docs identified
- [x] Docs-checklist will be created when entering implementation

**Documentation Impact:**

- [x] `docs-project/entities/guides/GUIDE-comments.md` → regenerated
`docs/comments.md` — "Where comments are stored" was actively WRONG for the
postgres build after this change; now a per-backend table with the multi-process
motivation.
- [ ] `docs/postgres-backend.md` — mentions which tables the database holds.
- [ ] `CLAUDE.md` — the storage-backend section and the `internal/comments`
row in the package table.
- [ ] `docs/sqlite-backend.md` (if it enumerates `rela.db`'s tables).

## Design Review

- [x] ~~Run `/design-review` before starting implementation~~ (N/A: the design
was settled interactively with the user before any code was written — the `Open`
split, the delete-the-DSN-form choice and the sqlite-in-`rela.db` choice were
each put to them and decided; see § Approach for the rejected alternatives)
- [x] All critical/significant findings addressed in plan

**Design Review Findings:** N/A — see above.

---
id: PLAN-ZYYWJ7
type: planning-checklist
title: 'Planning: store: paged variant of GraphQueryer (GraphQueryPage) — pgstore SQL pushdown + naive early-stop + conformance'
status: done
---

<!-- @managed: claude-workflow v1 -->

## Understanding

- [x] Problem/requirements clearly understood
- [x] Scope defined (what's in/out documented below)
- [x] Acceptance criteria documented with specific test scenarios

**Problem:** `store.GraphQueryer.GraphQuery` returns an unbounded
`iter.Seq2[*entity.Entity, error]`. TKT-YWDGZD wants to fix `rela.list_entities`
unbounded materialization by pushing the ACL predicate into the store
(`acl.Request.readQuery` already reduces to a candidate-independent
`store.GraphQuery`). Pushing the ACL down without paging just relocates the
unbounded materialization from the Lua binding into the store call. Bound the
surface first.

**Scope — IS in scope:**

- `store.GraphQuery` gains `Limit int` / `Cursor string` fields.
- `store.GraphQueryer` gains `GraphQueryPage(ctx, q) (GraphQueryPage, error)`.
- pgstore: SQL-native `LIMIT` + keyset predicate on the existing builder.
- graphquerynaive: shared paged implementation with early stop at the limit.
- fsstore / memstore: one-line delegation each.
- `acl.NullGraphQueryer`: implement the new method (empty page).
- `search.Visible` / any other `store.GraphQueryer` consumer: compile fix only.
- storetest: paged conformance subtests inside `RunGraphQueryTests`.
- Fix the stale comment at `internal/store/graphquery.go:17`.

**IS NOT in scope:**

- ACL vocabulary in the DSL — the generic shape is right (explicit non-goal).
- Changing `rela.list_entities` (that is TKT-YWDGZD, the downstream consumer).
- Changing `GraphCount` / `MatchingIDs` signatures.
- Making any existing caller (`dataentry`, `search`, sidebar counts) *use*
paging. This ticket adds the capability; adoption is separate.

**Acceptance Criteria:**

1. `GraphQueryPage` with `Limit == 0` returns every match in one page and an
empty `NextCursor`. *Test:* seed 3 matching tickets, `Limit: 0` → 3 items,
`NextCursor == ""`.
2. `GraphQueryPage` with `Limit > 0` returns at most `Limit` items and a
non-empty `NextCursor` **iff** more matches exist. *Test:* seed 3 matches,
`Limit: 2` → 2 items + non-empty cursor; resume → 1 item + empty cursor.
3. `NextCursor` is empty when the page exactly exhausts the result set.
*Test:* seed 3 matches, `Limit: 3` → 3 items, `NextCursor == ""` (the `limit+1`
probe must not set a cursor on an exact fit).
4. Walking with the cursor yields every match exactly once, in stable
ascending-id order, and the walk is identical across repeated runs. *Test:* 50
entities inserted in reverse order, walk with `Limit: 7` twice, assert equality
and total count.
5. Cursors are opaque and round-trip through `storeutil.EncodeCursor` /
`DecodeCursor`; a malformed cursor is an error, not a silent restart. *Test:*
`Cursor: "not-base64!!!"` → error.
6. A cursor past the last match returns an empty page with an empty cursor.
*Test:* `encodeTestCursor("zzz-sentinel")` → 0 items, `NextCursor == ""`.
7. Paging composes with every predicate shape the unpaged suite covers
(HasInbound, HasOutbound, InheritThrough, EntityInheritThrough, OfTypes).
*Test:* one paged walk over an `InheritThrough` query returns the same set as
the unpaged `GraphQuery`.
8. `total` stays reachable from the paged path — `GraphCount` still returns
`(matched, total)` and the paged result carries `Total`. *Test:* seed 3 tickets
of which 2 match; page with `Limit: 1` and assert `Total == 3` on the page (and
unchanged `GraphCount` behavior).
9. All three backends (fsstore, memstore, pgstore) satisfy 1–8 via the shared
conformance suite.

## Research

- [x] For larger features: run `/research` to create a structured research doc
- [x] Searched for existing libraries that solve this problem
- [x] Checked codebase for similar patterns or reusable code
- [x] Looked for reference implementations in other projects
- [x] Reviewed relevant rela concepts for prior art

**Research Doc:** N/A — the contract to mirror is already specified in-tree
(`store.ListEntitiesPage`), so there is no open design question worth a
`/research` survey. The only real decision (page struct vs `store.Page[T]`) is
documented under Alternatives below.

**Existing Solutions:**

No external library applies — this is an internal store contract. The in-tree
prior art is complete and directly reusable:

- `internal/store/store.go:242` — the `ListEntitiesPage` contract text
(limit-0 semantics, opaque cursor, `NextCursor` non-empty iff more). This is the
specification being mirrored.
- `internal/store/store.go:276` — `store.Page[T]` generic result type.
- `internal/store/storeutil/storeutil.go:95` — `EncodeCursor` /
`DecodeCursor` (base64 RawURL over the sort key). Reused verbatim.
- `internal/store/storeutil/storeutil.go:134` — `PaginateSortedKeys`, the
sorted-slice keyset walker used by fsstore + memstore. **Not** reusable here: it
takes a pre-sorted key slice and a cheap `match(key) bool`, while
graphquerynaive's match is an expensive per-entity graph walk over a
`ListEntities` stream. The naive paged path implements its own early-stop loop
instead.
- `internal/store/pgstore/entity.go:66` — `ListEntitiesPage`: the exact
`limit+1` fetch, `items[:q.Limit]`, `EncodeCursor(last.ID)` shape to copy.
- `internal/store/pgstore/entity.go:558` — `entityWhere` keyset predicate
(`id > $n`) to copy into the graphquery builder.
- `internal/store/storetest/pagination.go` — the conformance test shapes
(cursor walk, exact-fit last page, stale cursor, invalid cursor, stable order
across calls). The new graphquery subtests mirror these.

**Collation check (load-bearing):** `entities.id` is declared `TEXT COLLATE "C"`
in `migrations/0001_init.sql:31`, so PostgreSQL's `id > $n` and `ORDER BY e.id`
are byte-wise — identical to Go's string comparison in graphquerynaive. Keyset
paging is therefore consistent across backends with no extra `COLLATE` clause
needed on the new predicate. `pgstore/ordering_test.go` already pins this
property.

## Approach

- [x] Technical approach chosen and documented
- [x] Approach builds on existing patterns (not reinventing)
- [x] Alternatives considered (document why rejected)
- [x] Dependencies identified (packages, APIs, types)

**Technical Approach:**

*1. DSL + interface (`internal/store/graphquery.go`)*

Add to `GraphQuery`, mirroring `EntityQuery`'s field comments:

```go
Cursor string // pagination cursor from a previous page (empty = start); ignored by GraphQuery
Limit  int    // max entities per page (0 = no limit); ignored by GraphQuery
```

`GraphQuery` (the iterator method) explicitly **ignores** both, exactly as
`ListEntities` ignores `EntityQuery.Cursor/Limit`. That keeps every existing
caller behaviorally unchanged.

Add to `GraphQueryer`:

```go
GraphQueryPage(ctx context.Context, q GraphQuery) (GraphQueryPage, error)
```

with a dedicated result type rather than `store.Page[*entity.Entity]`, so
`total` survives (reporter note 2):

```go
// GraphQueryPage is one page of GraphQuery results. Total is the
// count of entities of q.EntityType ignoring the predicates — the
// same "total" GraphCount returns — so callers get truncation
// reporting (count / total / truncated) without a second round trip.
type GraphQueryPage struct {
    Items      []*entity.Entity
    NextCursor string
    Total      int
}
```

Also fix the stale `graphquery.go:17` comment: pgstore is SQL-native today.

*2. graphquerynaive (`naive.go`)*

New `RunPage(ctx, r, q) (store.GraphQueryPage, error)`:

- `storeutil.DecodeCursor(q.Cursor)` up front; propagate the error (never
silently restart at page 1).
- Stream `r.ListEntities(ctx, store.EntityQuery{Type: q.EntityType})` — it is
already ascending-by-id on all backends per the `EntityReader` contract.
- Skip entities with `e.ID <= cursorKey` **before** calling `matches` — the
expensive graph walk must not run for already-emitted rows.
- Count `total` (all entities of the type) while streaming, so `Total` costs
no extra pass.
- Append matches; when `q.Limit > 0` and we have collected `Limit+1` matches,
set `NextCursor = EncodeCursor(items[Limit-1].ID)`, truncate to `Limit`.

**Early stop caveat:** the scan cannot stop at the *limit*-th match, because
`Total` requires visiting every entity of the type. It *can* stop calling
`matches` (the expensive part) once `Limit+1` matches are known — the remainder
of the stream only increments `total`. That is the honest version of "stop
early": O(type) row visits stay (unchanged, per the ticket), but the per-entity
graph walk is bounded by the page. This is worth stating explicitly in the godoc
so nobody reads `Total` as free.

*3. fsstore / memstore*

One method each in their `graphquery.go`, delegating to
`graphquerynaive.RunPage(ctx, s, q)` — same shape as the existing three.

*4. pgstore (`graphquery.go`)*

`buildGraphQuerySQL` currently takes `(q, countOnly bool)`. Two booleans would
be a smell, so switch to a small unexported mode enum or add a `keysetAfter
string` parameter alongside `countOnly` (the latter matches
`buildEntityListSQL(q, keysetAfter)` next door — prefer it for symmetry):

- `buildGraphQuerySQL(q store.GraphQuery, countOnly bool, keysetAfter string)`.
- When `keysetAfter != ""`, append `AND e.id > $N` (via `b.arg`, never
interpolated) to the outer WHERE. `countOnly` callers pass `""`.
- `GraphQueryPage` decodes the cursor, computes `fetch := q.Limit; if fetch > 0
{ fetch++ }`, appends `LIMIT %d` (integer, not user data), scans rows, truncates
and encodes the cursor exactly like `ListEntitiesPage`.
- `Total` comes from the same `SELECT count(*) FROM entities WHERE type = $1`
that `GraphCount` already runs — factor it into a tiny helper so the two paths
cannot drift.

Note the existing `ORDER BY e.id` in the row path is already present and is what
makes the keyset valid; the count path keeps no ORDER BY.

*5. Other `store.GraphQueryer` implementers*

`acl.NullGraphQueryer` (`internal/acl/graph.go:80`) needs `GraphQueryPage`
returning an empty page. Grep confirms it is the only non-backend implementer;
`search.Visible` *consumes* the interface and needs no change.

**Alternatives considered:**

- *`Limit`/`Cursor` on the existing `GraphQuery` iterator instead of a new
method.* Rejected: it would silently change behavior for every current caller
and breaks the "iterator ignores paging fields" symmetry `ListEntities` /
`ListEntitiesPage` already establishes.
- *Reuse `store.Page[*entity.Entity]`.* Rejected: it has no room for `Total`,
and reporter note 2 explicitly asks that the paged path keep `total` reachable
so the #1241 list-render override gets truncation reporting free. A dedicated
`GraphQueryPage` type costs one struct and keeps `Page[T]` honest as "items +
cursor".
- *Offset paging.* Rejected: unstable under concurrent writes and inconsistent
with the keyset contract the ticket asks to mirror.
- *Make the naive path stop the scan entirely at the limit (skip `Total`).*
Rejected: drops `total`, which note 2 asks to preserve. Documented the real cost
instead.

**Files to modify:**

- `internal/store/graphquery.go` — `Limit`/`Cursor` fields, `GraphQueryPage`
type + method on `GraphQueryer`, stale-comment fix.
- `internal/store/graphquerynaive/naive.go` — `RunPage`.
- `internal/store/fsstore/graphquery.go` — delegate.
- `internal/store/memstore/graphquery.go` — delegate.
- `internal/store/pgstore/graphquery.go` — keyset + LIMIT in the builder,
`GraphQueryPage` method, shared total helper.
- `internal/acl/graph.go` — `NullGraphQueryer.GraphQueryPage`.
- `internal/store/storetest/graphquery.go` — paged conformance subtests.

## Security Considerations

- [x] Input sources identified (user input, config, external APIs)
- [x] Input validation approach defined (allowlist preferred over blocklist)
- [x] Security-sensitive operations identified (file access, auth, crypto)
- [x] Error handling doesn't leak sensitive information

**Input Sources & Validation:**

- `q.Cursor` — opaque base64, ultimately caller-supplied (a future HTTP/Lua
consumer may pass one through). Validated by `storeutil.DecodeCursor`, which
errors on malformed base64. The decoded value is used **only** as a bound value
through `sqlBuilder.arg` (`$N` placeholder) in pgstore and as a Go string
comparison in the naive path — never interpolated into SQL text. A decoded
cursor that is not a real id is harmless: it is just a keyset bound, yielding an
empty or partial page (conformance-tested).
- `q.Limit` — an `int` from the caller. It is formatted into the SQL as
`LIMIT %d` (an integer, so no injection surface), matching `ListEntitiesPage`.
Negative or zero is treated as "no limit" per contract. Note: this ticket
deliberately does **not** impose a maximum page size — the store honors what it
is asked for; a default/max belongs to the consumer (TKT-YWDGZD), which is where
a script-facing default must be decided.
- All other `GraphQuery` fields are unchanged and already flow through
`sqlBuilder.arg` — the injection-safety godoc block at
`pgstore/graphquery.go:145` must be extended to name the cursor as another
parameterised value.

**Security-Sensitive Operations:**

- **This is ACL infrastructure.** `GraphQuery` is what the read gate reduces
to, so a paging bug is a potential *visibility* bug: a page that silently skips
a matching row makes a visible entity look nonexistent, and one that repeats a
row is a correctness (not confidentiality) defect. Mitigated by the "yields
every match exactly once" conformance test across all backends, and by the
byte-wise `COLLATE "C"` alignment noted in Research.
- Paging must not become a *disclosure* channel: `Total` is a count of
entities of the type **ignoring predicates** — exactly what `GraphCount` already
returns to the same callers today, so this exposes nothing new. It is a pre-ACL
count and callers that surface it to end users must keep treating it as the
"filtered by" denominator, not as visible-row truth. No change in what is
exposed; flagged so it is not later mistaken for a post-ACL count.
- Errors returned are `pgstore: graph query page: %w` style — wrapping driver
errors, no entity content, no cursor contents echoed.

## Test Plan

- [x] Test scenarios documented for each acceptance criterion
- [x] Edge cases identified and documented
- [x] Negative test cases defined (invalid input, error conditions)
- [x] Integration test approach defined (not just unit tests)

**Test Scenarios:** Each acceptance criterion 1–8 maps to a named subtest added
to `RunGraphQueryTests` in `internal/store/storetest/graphquery.go` (criterion →
subtest listed inline in Understanding above). Criterion 9 is satisfied
structurally: `RunGraphQueryTests` is already invoked from `RunAll`, which
fsstore, memstore, and pgstore conformance tests all run — so all three backends
are covered by construction, with pgstore's run DB-gated on
`RELA_TEST_DATABASE_URL` (`just test-postgres`).

This *is* the integration-test approach: the conformance suite runs against real
backends (real filesystem for fsstore, real PostgreSQL for pgstore), not mocks.
Additionally:

- `pgstore/graphquery_explain_test.go` already pins SQL shape without a live
DB — add a case asserting the keyset predicate and `LIMIT` appear only when
requested, and that the cursor value is a `$N` placeholder (injection-safety
pin).
- A naive-level unit test that the expensive `matches` walk is not invoked for
rows before the cursor (via a counting `Reader` stub), pinning the early-stop
behavior the ticket asks for.

**Edge Cases:**

- `Limit == 0` → full result set, empty cursor (criterion 1).
- `Limit < 0` → treated as 0/no-limit; assert it does not panic or emit a
cursor.
- `Limit` larger than the dataset → all items, empty cursor.
- `Limit` exactly equal to the match count → no cursor (the `limit+1` probe
must find nothing).
- Empty store / no matching entities → empty items, empty cursor, `Total == 0`.
- Query with **no** predicates (both `HasInbound` and `HasOutbound` nil, the
degenerate "everything of this type" case the existing suite covers) → paging
still applies.
- Cursor past the last id → empty page.
- Cursor pointing at an id that exists but does *not* match the predicate →
must resume correctly from the next *matching* id, not skip a page.
- Entities whose ids sort non-obviously (uppercase/lowercase/digits mixed) →
covered by the byte-wise ordering assertion; the 50-entity stable-order test
uses zero-padded ids like the existing pagination suite.
- Cycle / self-loop graphs (already in the suite) combined with paging.
- Unicode ids — `EncodeCursor`/`DecodeCursor` round-trip is already tested in
`storeutil_test.go`; no new coverage needed.

**Negative Tests:**

- `Cursor: "not-base64!!!"` → returns an error from `GraphQueryPage`, and
crucially **not** a silent page-1 restart (this is the mistake that would make a
paginated ACL walk loop forever).
- Store/driver error mid-page → returned as an error, not a short page. A
short page with an empty `NextCursor` is indistinguishable from "end of
results", so swallowing an error here would silently truncate an ACL-filtered
list. Assert the error propagates.

## Risk Assessment

- [x] Technical risks assessed with mitigations
- [x] Security risks assessed (see Security Considerations)
- [x] Effort estimated (xs/s/m/l/xl)

**Risks:**

| Risk | Mitigation |
|------|-----------|
| Adding a method to `store.GraphQueryer` breaks every implementer. | Only 4 implementers (3 backends + `acl.NullGraphQueryer`); all updated in this ticket. The compiler finds any miss. |
| Keyset ordering diverges between Go byte-compare and PostgreSQL collation, causing skipped or repeated rows in an ACL-filtered walk. | `entities.id` is already `COLLATE "C"` (verified, `0001_init.sql:31`); `pgstore/ordering_test.go` pins it; the shared conformance suite runs the identical walk against all backends. |
| `Total` on the naive path tempts a reader into thinking the scan stopped early when it did not. | Explicit godoc on `RunPage` stating O(type) row visits remain and only the per-entity graph walk is page-bounded. |
| `buildGraphQuerySQL` gains a third positional parameter and becomes hard to read. | Mirror the neighbouring `buildEntityListSQL(q, keysetAfter)` signature; if it still reads badly, introduce a small options struct. Called from 3 sites only. |
| Scope creep into TKT-YWDGZD (defaults, max page size, Lua surface). | Explicit non-goal; no consumer changes in this ticket. |
| `plimsoll` method-count lines on the store types. | Adding 1 method to `FSStore`/`MemStore`/`Store`; these carry documented "required interface" directives — check `just plimsoll` and bump the pinned count with the required-interface rationale if it trips. |

**Effort:** m — matches the ticket's `effort` property. Mechanical against a
well-specified in-tree contract; the pgstore builder change and the conformance
suite are the bulk.

## Documentation Planning

- [x] User-facing docs identified (skip if internal refactor)
- [x] Docs-checklist will be created when entering implementation

**Documentation Impact:**

- [x] N/A for user-facing docs — this is an internal store interface with no
CLI, HTTP, metamodel, or Lua surface change in this ticket. Nothing in `docs/`
describes `GraphQueryer` today.
- [x] Godoc **is** the deliverable here and is treated as such: the
`GraphQueryer` / `GraphQueryPage` contract text, the `RunPage` early-stop
caveat, and the extended SQL-injection-safety block in `pgstore/graphquery.go`.
- [x] `internal/store/graphquery.go:17` stale-comment fix (reporter note 1).
- [x] ~~`CLAUDE.md` update~~ (N/A: no new architectural rule; this follows the
existing store-contract pattern already described there).

## Design Review

- [x] ~~Run `/design-review` before starting implementation~~ (N/A: the plan was
presented to the user in full and approved directly, including a design question
the user overruled — see below. The design was then reviewed adversarially by
`/code-review`, which produced RR-245QB2 against the type shape.)
- [x] All critical/significant findings addressed in plan

**Design Review Findings:**

Design feedback arrived through two channels instead of `/design-review`:

1. **User, at plan approval.** Rejected the proposed `Total` field on the paged
result: a sometimes-meaningful total is worse than none, and `GraphCount`
already returns `(matched, total)`. Dropping it also let `RunPage` genuinely
stop early rather than scanning the full type to count — removing the caveat
this plan had flagged as unsatisfying.
2. **RR-245QB2, from `/code-review`.** The `Cursor`/`Limit`-on-shared-struct
pattern this plan proposed (mirroring `EntityQuery`) was a silent ACL footgun,
since three of four `GraphQueryer` methods cannot honor a limit. Replaced with a
separate `store.GraphPageQuery` embedding `GraphQuery`, so the compiler enforces
that paging fields exist only where they are obeyed.

Both changed the delivered design relative to this plan; the ticket body and
IMPL-9IF39M record the final shape.

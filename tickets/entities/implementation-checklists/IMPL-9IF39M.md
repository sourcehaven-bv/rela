---
id: IMPL-9IF39M
type: implementation-checklist
title: 'Implementation: store: paged variant of GraphQueryer (GraphQueryPage) — pgstore SQL pushdown + naive early-stop + conformance'
status: done
---

<!-- @managed: claude-workflow v1 -->

## Development

- [x] Unit tests written for new code
- [x] Integration tests written (test full flow, not just units)
- [x] Happy path implemented
- [x] Edge cases from planning handled
- [x] Error handling in place (errors surfaced, not swallowed)

**Scope change from planning (user-approved):** `Total` was dropped from the
paged result. Planning proposed a dedicated `GraphQueryPage` struct carrying
`Total`; the user pushed back that a sometimes-meaningful total is worse than
none. `GraphCount` already returns `(matched, total)` — strictly more than a
page could report — so `total` stays available (ticket note 2) without living on
the page. Consequences:

- The result type is now plain `store.Page[*entity.Entity]`, reusing the one
paging vocabulary the store already has.
- The naive path can now **genuinely** stop early. With `Total` it would have
had to visit every entity of the type to count; without it the scan breaks at
`Limit+1` matches. This removes the caveat planning flagged as unsatisfying.

**Files changed:**

| File | Change |
|------|--------|
| `internal/store/graphquery.go` | `Cursor`/`Limit` fields; `GraphQueryPage` on `GraphQueryer`; stale-comment fix |
| `internal/store/graphquerynaive/naive.go` | `RunPage` with cursor-skip-before-match + early break |
| `internal/store/fsstore/graphquery.go` | delegate |
| `internal/store/memstore/graphquery.go` | delegate |
| `internal/store/pgstore/graphquery.go` | keyset param on builder, `GraphQueryPage`, injection-safety godoc |
| `internal/acl/graph.go` | `NullGraphQueryer.GraphQueryPage` |
| `internal/store/storetest/graphquery.go` | 14 paged conformance subtests |
| `internal/store/graphquerynaive/naive_test.go` | new — work-done assertions |
| `internal/store/pgstore/graphquery_paging_sql_test.go` | new — SQL shape, no DB needed |
| `.go-arch-lint.yml` | `graphquerynaive` → `storeutil` (shared cursor codec) |
| `fsstore.go` / `memstore.go` / `pgstore.go` | plimsoll +1, documented as interface-tracking |

## Test Quality

- [x] Using fixture builders or factories for test data
- [x] No hardcoded values in assertions when object is in scope
- [x] Only specifying values that matter for the test
- [x] Interpolated values constructed from objects, not hardcoded
- [x] Property comparisons use original object, not hardcoded strings

Conformance tests use the existing `seedGraphQueryEntities` / `mustRel` helpers
plus a local `aliceOwnsTickets` fixture and a shared `ownedByAlice` query value,
copied per-subtest so no test mutates another's input. Two new helpers
(`pageIDs`, `walkGraphQueryPages`) keep the walk logic in one place;
`walkGraphQueryPages` bounds itself at 100 pages so a cursor-contract violation
fails the test instead of hanging the suite.

**Mutation-tested the two assertions that matter** — a test that cannot fail is
worth nothing, so both were verified by breaking the implementation:

- Removing the early break: `entitiesYielded` went 6 → 500, test **failed** as
intended ("500 is not less than or equal to 10").
- Moving the cursor skip to *after* `matches()`: first attempt **passed**, so
the assertion was too loose (`firstPageScans+5`). Tightened to require constant
per-page cost; re-ran the mutation and it now **fails** (11 vs 6).

## Manual Verification

- [x] Feature manually tested end-to-end
- [x] Each acceptance criterion verified with test scenario from planning
- [x] Edge cases manually verified

**Verification Evidence:**

*Live PostgreSQL 15.17 (Postgres.app).* `RELA_TEST_DATABASE_URL` was unset, so
the pgstore conformance suite — the ticket's "real win" — would have silently
skipped. Created a scratch DB and ran it for real (dropped afterwards):

- Full pgstore suite under `-race`: `ok ... 27.600s`.
- All 14 paged subtests confirmed **PASS, not SKIP**, by name in `-v` output.

*End-to-end walk (ad-hoc test, run then removed):* 250 tickets, only evens owned
by alice, so the predicate does real work rather than degenerating:

```
walked 125 matches in 13 pages of 10
GraphCount: matched=125 total=250 (truncation reporting intact)
```

- Paged walk `require.Equal`s the unpaged `GraphQuery` result set, **in order**.
- Every page asserted `len(items) <= 10` — the LIMIT genuinely reaches the DB.
- `GraphCount` unchanged, confirming note 2's truncation reporting survives.

*Acceptance criteria 1–9:* all pass, each as its named conformance subtest
(`Page_full_when_limit_zero`, `Page_walk_yields_every_match_once`,
`Page_no_cursor_when_limit_exactly_exhausts`, `Page_stable_order_across_calls`,
`Page_invalid_cursor_returns_error`, `Page_cursor_past_end_returns_empty`,
`Page_composes_with_InheritThrough`, …). Criterion 8 changed with the scope
change: `Total` is no longer on the page, so it is verified via the unchanged
`GraphCount_matched_and_total` subtest plus the manual run above. Criterion 9
holds structurally — the suite runs from `RunAll` on all three backends,
verified executing on fsstore, memstore, **and** pgstore.

*Edge cases from planning, all covered as subtests:* negative limit, limit >
dataset, limit == exact match count, empty store, no matches, no-predicate
degenerate query, cursor past end, and **cursor sitting on a non-matching id**
(resumes at the next matching id rather than skipping a page).

*Checks:* `go build` clean on all three build tags (default / `postgres` /
`memorybackend`); `go test ./...` clean; `-race` clean; `golangci-lint` 0
issues; `just arch-lint` OK; `just plimsoll` OK; `just coverage-check` PASS
(total 76.6%).

## Quality

- [x] Code follows project patterns (check similar code)
- [x] Checked for DRY opportunities — repeated literals, expressions, or
patterns extracted to a helper / constant / type where it sharpens the contract
- [x] No security issues introduced
- [x] No silent failures (errors logged AND returned)
- [x] No debug code left behind

`GraphQueryPage` mirrors `ListEntitiesPage` line-for-line in both backends
(`limit+1` fetch → truncate → `EncodeCursor(last.ID)`), and the pgstore builder
took a `keysetAfter` parameter matching `buildEntityListSQL` next door, so the
two paging paths read the same. The cursor codec is `storeutil`'s, shared with
every other paged read — a private codec here would produce cursors that are not
interchangeable across the one contract.

**Security.** The decoded cursor reaches SQL only via `sqlBuilder.arg` (`$N`
placeholder), never interpolated — pinned by a test asserting the literal value
is absent from the SQL text and present in `args`. `LIMIT` is an `int`. The
injection-safety godoc block was extended to name the cursor. A malformed cursor
**errors** rather than restarting at page 1 (silent restart = infinite paginated
walk); a read error propagates rather than returning a short page, because a
short page with an empty cursor is indistinguishable from end-of-results and
would silently truncate an ACL-filtered list.

**Keyset correctness.** `entities.id` is already `COLLATE "C"`, so PostgreSQL's
`e.id > $N` / `ORDER BY e.id` is byte-wise and agrees with Go's string compare
in the naive path — the ordering risk flagged in planning needed no new code,
and the reasoning is recorded in the builder godoc so a future collation change
does not quietly break mid-walk paging.

Scratch DB `rela_test_tktju3s5n` dropped; ad-hoc end-to-end test removed.

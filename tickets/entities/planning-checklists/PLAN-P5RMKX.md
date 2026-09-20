---
id: PLAN-P5RMKX
type: planning-checklist
title: 'Planning: Section sort: plus one declared order per enum, on every sort path'
status: done
---

<!-- @managed: claude-workflow v1 -->

## Understanding

- [x] Problem/requirements clearly understood
- [x] Scope defined (what's in/out documented below)
- [x] Acceptance criteria documented with specific test scenarios

**The decision this plan is built on** (user, 2026-09-20): **sorting happens in
SQL**, because otherwise paging does not work properly and the overhead of
loading a whole type is unacceptable. Sort semantics change to whatever the
sqlite and postgres backends can support, and the Go comparator conforms to SQL
rather than the reverse.

That single principle resolved every open design-review question, including four
the reviewer raised that this plan had not anticipated. It also inverts the
plan's earlier shape: the job is no longer "make the Go path type-aware", it is
"make one SQL-expressible ordering and hold every path to it".

**Scope:**

IN scope:

1. One ordering rule for the query-sort path, defined as what SQL can express:
   byte order on the stored string form, enum rank where declared values exist,
   nulls last ascending / first descending, id ascending as the final tiebreak
   in both directions.
2. `applyV1Sorting` and `filter.SortMulti` both implement that rule, so the REST
   list path, search and dashboards agree.
3. The enum `CASE` rank pushed into SQL, with declared values in the derived
   index name and in `StaticIndexSpecs`' dedup key.
4. `parent_sort:` / `child_sort:` on `ViewSection`, applied per level before any
   cap.

OUT of scope:

- Per-use-site `values:` ordering. Rejected on measurement; see the ticket.
- Changing `store.GraphQuery.OrderBy`'s byte-wise contract. It already says what
  this plan now implements everywhere, so it needs no change — it was right and
  the Go comparator was the outlier.
- `filter`'s type-aware comparators (`compareDates`, `compareIntegers`,
  `compareBooleans`) for NON-query callers. They keep their current behaviour;
  only the query-sort path stops routing through them.
- The ~28 display-only `natsort` call sites (analyze output, MCP tools, schema
  and template listings, export). They sort their own slices and never reach
  `SortMulti`.

**Accepted behaviour changes** (all release-note material):

| Change | Before | After |
| --- | --- | --- |
| String sorts | natsort: case-insensitive, numeric-aware | byte order: `Zebra` before `apple`, `item10` before `item9` |
| `sort=id` | natsort in search/CLI, byte on v1 | byte order everywhere |
| Enum sorts on the v1 list path | alphabetical | declared order |

The first two are regressions in readability, accepted deliberately to keep
paging correct. Measured on this repo's own 4,308 ticket titles: **99% of
positions move**.

**Acceptance Criteria:**

The old AC1 ("the two sorters agree") is deleted, not repaired — after
unification both sides it compared are the same function, so it asserted a
tautology (RR-Z7V8PI). Every criterion below names a mutation that must make it
fail.

1. **Pushed and Go paths agree, page for page.** Extend
   `TestListPushdown_MatchesGoPathPageForPage` with an enum sort key (asc and
   desc), a mixed-case string key, a key with embedded numbers, and a sparse
   property, across page boundaries.
   *Mutation*: emit byte-wise `ORDER BY` while the Go path ranks — must fail on
   page 1.
   *Note*: the current fixture's titles are same-case and digit-free, where byte
   and natural order coincide. That is why it passes today over a live
   divergence, so widening the fixture is part of the criterion.

2. **Declared enum order, asserted on BOTH paths.** An enum declared
   `todo, doing, blocked, done` sorts in that order, not alphabetically, both
   when pushdown is eligible and when the request is forced onto the Go path.
   *Mutation*: let the SQL side silently decline — must fail on the pushed
   assertion rather than passing on memstore.

3. **Descending is a valid ordering.** Equal keys keep input order under a
   descending sort; a secondary ascending key stays ascending under a
   descending primary; ties break by id ascending in both directions.
   *Mutation*: restore `return !less` — must fail. (Currently 13 equal keys
   return reversed, and a secondary key inverts entirely.)

4. **A schema edit rebuilds the index.** Two metamodels differing only in enum
   value order, run through the REAL derivation
   (`queryplan.StaticIndexSpecs` → `listIndexName`), produce different index
   names.
   *Mutation*: omit values from the hash — must fail.
   *Why through the derivation*: hand-built `DerivedObjectSpec`s would pass with
   the `queryplan` half unwired (RR-Z7V8PI).

5. **Two value orders do not collide in the dedup map.** Two lists sorting the
   same property under different declared orders both survive
   `StaticIndexSpecs`.
   *Mutation*: leave values out of the `byKey` key — one spec is silently
   dropped and the test must catch it.

6. **The index survives a generic plan.** The EXPLAIN test forces
   `plan_cache_mode = force_generic_plan` (or executes ≥6 times) and asserts an
   Index Scan.
   *Mutation*: use bound parameters in the `CASE` — must fail with a Seq Scan.
   Measured: 4 → 1,915 buffers.

7. **Section sort orders rows before the per-parent cap.** Seed MULTIPLE parents
   whose combined children exceed `nestedNodeBudget`, so the shared budget is
   exercised, not just one parent's preview cap.
   *Mutation*: sort after the cap — must fail on a late parent.

8. **Edge semantics match SQL.** Nulls last ascending and first descending;
   unknown properties sort byte-wise; list-valued properties sort by their
   string form; non-ISO date formats sort lexically.
   *Mutation*: restore the `.(string)` type assertion — list and non-string
   values stop sorting and the test must fail.

## Research

- [x] For larger features: run `/research` to create a structured research doc
- [x] Searched for existing libraries that solve this problem
- [x] Checked codebase for similar patterns or reusable code
- [x] Looked for reference implementations in other projects
- [x] Reviewed relevant rela concepts for prior art

**Research Doc:** N/A — recorded in the ticket body (Postgres measurements,
rejected alternatives) as a decision record for this design rather than a survey.

**Existing Solutions:**

*The enum rank already exists and is reusable.* `buildEnumIndex`
(`filter/sort.go:65`) maps value → declared position, reading `propDef.Values`
for an inline enum or `meta.Types[...].Values` for a custom type; `compareEnums`
(`:88`) compares by that index and puts unknown values last. Verified by probe.
That logic carries over unchanged — it is the one semantic that IS
SQL-expressible, via a `CASE` rank.

*What does NOT carry over*, and this corrects the earlier claim that this ticket
writes no new comparison code: the surrounding type dispatch. Under the SQL-wins
decision the query-sort path needs ONE comparator (format to string, compare
bytes, apply the enum rank when declared values are present), not six type-aware
ones.

*`graphquerynaive` is the reference implementation.* `sqlitestore/graphquery.go:12-20`
states it outright: delegating rather than writing SQL pushdown is "a deliberate
first step… the naive implementation is the behavioral reference every backend
is verified against". sqlite, fsstore and memstore all route `GraphQuery`
through it, so only pgstore emits real SQL. Making SQL-expressible semantics the
authority matches how the codebase already reasons about backend agreement, and
`graphquerynaive.Order` (`naive.go:95-126`) already implements exactly the target
rule — byte-wise, nulls-as-largest, id tiebreak.

*The stale-index pattern already exists.* `uniqueIndexShape`
(`derivedschema.go:90`) versions an index definition so a change renames it and
the reconciler drops what it no longer computes (BUG-HC6I2T). AC4 applies that
idea to enum values.

*The differential harness already exists.*
`TestListPushdown_MatchesGoPathPageForPage` (`listpushdown_test.go:64`) runs
every request shape at two page sizes across three pages against both paths.
AC1 widens its fixture rather than building a harness.

*External research.* Four Postgres idioms measured on a 200k-row replica of
rela's index shape; results and sources in the ticket. `array_position` and a
rank lookup table both force a sequential scan and are the two most-recommended
answers online. Native pg enums are fastest but cannot be reordered without
recreating the type. An indexed `CASE` rank matches byte-wise performance and
serves both directions from one index via a backward scan.

## Approach

- [x] Technical approach chosen and documented
- [x] Approach builds on existing patterns (not reinventing)
- [x] Alternatives considered (document why rejected)
- [x] Dependencies identified (packages, APIs, types)

**Technical Approach:**

*Step 1 — define the rule once.* Write the query-sort comparator in `filter`:
three-way `int` return, format the value to its string form, compare bytes,
apply an enum rank when declared values are present. Invert ONLY the key
comparison for descending; apply the id tiebreak outside the inversion, always
ascending. This is `graphquerynaive.Order`'s rule, which is `GraphQuery.OrderBy`'s
documented contract, which is what pgstore emits — so one rule, already written
down in three places, finally implemented in all of them.

The existing `compareDates` / `compareIntegers` / `compareBooleans` /
`comparePropValues` stay for non-query callers and are simply not on this path.

*Step 2 — both Go entry points use it.* `applyV1Sorting` and
`filter.SortMulti`'s query-sort route both call the new comparator.
`SortByID` drops `natsort.Less` for byte order, which collapses the explicit
`sort=id` key and the implicit tiebreak into the same rule.

*Step 3 — push the enum rank into SQL.* `store.OrderSpec` grows an optional
ordered value list; `queryplan` populates it and adds it to both the index spec
and the `byKey` dedup key; pgstore emits a `CASE` with **literal** arms (via
`quoteLiteral`) in `ORDER BY` and in `createListIndexDDL`, and `listIndexName`
hashes the values. `graphquerynaive.Order` applies the same rank in Go so
sqlite/fs/mem agree.

*Step 4 — section sort.* `parent_sort:` / `child_sort:` on `ViewSection`
(nested) and `sort:` (flat), each sorting its level's collection before the cap.

**Why literals, not bound parameters, in `ORDER BY`** (RR-TXFI2O): a
bound-parameter `CASE` keeps the index under a custom plan but falls to a
Parallel Seq Scan once the plan cache goes generic after five executions —
measured 4 → 1,915 buffers, 32.2ms. Literal arms hold the index under a forced
generic plan. The values are operator-authored config, the same trust level the
existing derived-index DDL already interpolates. **Say this in the code comment**,
or a later reader will "fix" it back to parameters and reintroduce a silent 480×
regression.

**Alternatives considered:**

- *Narrow `StringShaped` to decline string-sort pushdown* (keeping natsort in
  Go). Rejected by the user: it loses paging pushdown on `title`, the
  second-most-sorted property, costing a whole-type scan — measured 2,061
  buffers / 18.7ms at 200k rows versus 5 buffers pushed.
- *Teach SQL natural ordering.* Not indexable; rules itself out.
- *Per-site `values:`* and *native pg enums*. Rejected on measurement; recorded
  in the ticket.

**Files to modify:**

| File | Change |
| --- | --- |
| `internal/filter/sort.go` | New three-way query-sort comparator; `SortByID` to byte order; direction-aware null placement; id tiebreak |
| `internal/dataentry/api_v1.go` | `applyV1Sorting` uses the shared comparator |
| `internal/store/graphquery.go` | Optional ordered-values list on `OrderSpec` |
| `internal/store/graphquerynaive/naive.go` | Apply the enum rank (keeps sqlite/fs/mem in step) |
| `internal/queryplan/queryplan.go` | Populate values in the order/index spec; add them to the `byKey` dedup key |
| `internal/store/pgstore/graphquery.go` | `CASE` rank with literal arms in `ORDER BY` |
| `internal/store/pgstore/derivedschema.go` | `CASE` in index DDL; hash values in `listIndexName` |
| `internal/dataentryconfig/config.go` | `parent_sort:` / `child_sort:` / `sort:` on `ViewSection` |
| `internal/dataentryconfig/validate.go` | Validate sort properties; refuse the wrong key for the display |
| `internal/dataentry/sections_nested.go` | Sort each level before the cap |

## Security Considerations

- [x] Input sources identified (user input, config, external APIs)
- [x] Input validation approach defined (allowlist preferred over blocklist)
- [x] Security-sensitive operations identified (file access, auth, crypto)
- [x] Error handling doesn't leak sensitive information

**Input Sources & Validation:**

| Input | Source | Validation |
| --- | --- | --- |
| `sort=` query param | HTTP, untrusted | Already parsed to `(property, direction)` pairs; property must resolve in the metamodel or pushdown declines. Never interpolated into SQL. |
| Enum `values:` | `schema.yaml`, operator-authored | **Becomes SQL text in a `CASE` and in index DDL.** See below. |
| `sort:` in `data-entry.yaml` | operator-authored | Validated against the source type at config load. |

**Security-Sensitive Operations:**

*Enum values reaching SQL is the one real concern in this ticket.* The `CASE`
arms are built from `values:`, which is operator-authored config, not end-user
input. Two different rules apply depending on where it lands:

- In the derived index DDL, parameters are not available — DDL cannot be
  parameterised. The existing code solves this with `quoteLiteral` /
  `quoteIdent` / `safeDDLName` (`derivedschema.go:285-290`). Reuse those; do not
  invent new escaping.
- **In `ORDER BY`, values must ALSO be literals via `quoteLiteral`** — not bound
  parameters. This reverses what this plan originally said, on measurement
  (RR-TXFI2O): a bound-parameter `CASE` stops matching the literal-valued
  expression index once Postgres switches to a generic plan, costing 4 → 1,915
  buffers. So `quoteLiteral` is load-bearing on the query path too, and the code
  comment must say why, or the next reader will revert it.

Threat model note: an attacker who can edit `schema.yaml` already has the
operator's shell and does not need a SQL injection. So this is defence in depth
and a correctness requirement (a value containing a quote must not break DDL
generation), not a privilege boundary. A value with an embedded quote is a
realistic accident, and `quoteLiteral` is what makes it a non-event.

No ACL surface changes. Sorting reorders rows the principal can already see; the
pushed query remains the principal's compiled read query, and the scoped count
is unchanged. Row-gating happens before ordering on both paths.

No error message gains config or data content.

## Test Plan

- [x] Test scenarios documented for each acceptance criterion
- [x] Edge cases identified and documented
- [x] Negative test cases defined (invalid input, error conditions)
- [x] Integration test approach defined (not just unit tests)

**Test Scenarios:**

| AC | Test | Level |
| --- | --- | --- |
| 1 two sorters agree | Same fixture through `/api/v1/entities?sort=` and the search path; identical id sequences | Integration, real handler |
| 2 pushed == Go | Enum rows added to `TestListPushdown_MatchesGoPathPageForPage` | Integration, both paths |
| 3 declared order | Explicit sequence assertion, declared order ≠ alphabetical | Integration |
| 4 index identity | `listIndexName` differs for two value orders; EXPLAIN shows index use | Unit + postgres-gated |
| 5 sort before cap | Nested section with more children than the preview cap | Integration, real view pipeline |
| 6 nulls + tiebreak | Sparse property asc/desc; equal keys with unordered ids | Unit in `filter`, then integration |

**Edge Cases:**

- Property missing on some rows; JSON null vs absent key (both are "no value").
- Value present in data but absent from `values:` (legacy/removed) — sorts last,
  alphabetically among themselves. Already true; pin it.
- Empty string as an enum value.
- Enum with one value; enum with zero values (`buildEnumIndex` returns nil → byte
  order fallback).
- A value containing a single quote, reaching both `ORDER BY` and index DDL.
- Multi-key sort mixing an enum and a date.
- A date property declaring a non-ISO `format:` (must decline pushdown).
- `sort=id` and `sort=modified` (virtual properties; must not change order).
- Sort property declared on one entity type but not another in a mixed result set
  (`comparePropValues` has a `typeRank` path for this).
- Equal sort keys spanning a page boundary — the paging-corruption case.
- A section whose collection is empty, or shorter than the cap.

**Negative Tests:**

- `sort=` naming an unknown property: pushdown declines, Go path leaves order
  untouched. Current behaviour; must not change.
- Section `sort:` naming a property absent from the source type: config load
  error, matching how `columns:` fails. Where `determineTargetType` returns `""`
  (multi-`to:` relation), skip validation rather than guessing.
- Integer property sort stays on the Go path (existing
  `listpushdown_test.go:137` case must keep passing).

**Integration approach:** every AC except 4 and 6 drives the real HTTP handler
through the real ACL gate, matching how TKT-M0WMEE's budget tests are written.
AC4's EXPLAIN half is postgres-gated on `RELA_TEST_DATABASE_URL` and skips
without it, like the rest of the pgstore suite.

## Risk Assessment

- [x] Technical risks assessed with mitigations
- [x] Security risks assessed (see Security Considerations)
- [x] Effort estimated (xs/s/m/l/xl)

**Risks:**

| Risk | Severity | Mitigation |
| --- | --- | --- |
| Descending comparator corrupts secondary keys (reproduced) | **High** | AC3; three-way comparator, both directions asserted |
| Bound-parameter `CASE` silently loses the index in production | **High** | AC6 forces a generic plan; literal arms, with the reason in the code comment |
| Stale index after a schema edit (measured 650× slowdown) | **High** | AC4; hash values into the index name, reusing `uniqueIndexShape` |
| Two value orders colliding in the dedup map (silent from birth) | **High** | AC5; values in `byKey` |
| Paging corruption from the missing id tiebreak | **High** | AC3; tiebreak outside the inversion, always ascending |
| **String and `sort=id` ordering changes for every user** | **High** | Accepted by decision. Release note is mandatory, not optional — 99% of real titles move |
| Backends drifting out of step | Medium | `graphquerynaive` carries the same rank; AC1 covers both paths |
| Enum values reaching SQL or DDL unescaped | Medium | `quoteLiteral` on both; embedded-quote edge case tested |
| Section sort missing the per-parent budget interaction | Medium | AC7 seeds multiple parents past `nestedNodeBudget` |
| Scope creep into a per-site override | Low | Explicitly out of scope, with measurements recorded |

**Effort:** m. Step 1 is one comparator replacing a type dispatch, so it deletes
more than it adds; step 2 is the bulk (three packages plus an EXPLAIN test);
step 3 is mechanical. The interim estimate above `m` assumed type-aware
comparison on both sides, which the SQL-wins decision removed.

One risk worth naming plainly: the EXPLAIN test and the postgres half of AC4
cannot be verified on this machine without a database. Postgres.app is available
locally and was used for the planning measurements, so this is runnable, but CI
is the authority.

## Documentation Planning

For enhancements: identify what documentation needs updating.

- [x] User-facing docs identified (skip if internal refactor)
- [x] Docs-checklist will be created when entering implementation

**Documentation Impact:**

- [x] `docs/data-entry.md` — document `sort:` on a view section beside the
      existing `sort:` docs (`:1463`, `:2049`), and state that an enum sorts in
      declared order with unknown values last.
- [x] `docs/metamodel.md` — note that the ORDER of `values:` is now
      load-bearing, not just the set. This is the change most likely to surprise
      an operator: reordering `values:` reorders every list sorted on it.
- [x] `docs/postgres-backend.md` — **required**, checked during planning rather
      than deferred. The "Derived schema" section (`:109-126`) states
      "Equivalent queries share an index even when their literal values or
      filter order differ", which becomes FALSE for enum sort keys once declared
      values enter the index name: two lists sorting the same enum under
      different declared orders must get different indexes (AC4/AC5). The same
      passage lists what gets no index and will need the enum `CASE` shape
      described alongside it.
- [x] Release note — **three** ordering changes on upgrade: enum-sorted list
      views move to declared order; string sorts become byte order
      (case-sensitive, non-numeric); `sort=id` becomes byte order in search and
      CLI. The last two affect every list, not only enum-sorted ones.

## Decisions taken

Design review found four critical and four significant defects. All eight are
now `addressed`; the two that needed a product call were answered by the user on
2026-09-20 with one principle: **sorting stays in SQL, and sort semantics become
whatever sqlite/postgres can support.**

| ID | Severity | Resolution |
| --- | --- | --- |
| RR-C4QYTO | critical | Strings compare byte-wise; natsort leaves the query-sort path. Pushdown on `title` is kept. |
| RR-6F2UF2 | critical | Three-way comparator; id tiebreak outside the inversion. Scope shrinks to ONE comparator, not six. |
| RR-TXFI2O | critical | `ORDER BY` `CASE` uses literal arms; EXPLAIN test forces a generic plan. |
| RR-Z7V8PI | critical | AC1 deleted as a tautology; AC2/AC4 assert per-path and derive from real metamodels. |
| RR-D1QOQ7 | significant | Dates compare byte-wise. No behaviour change; the divergence never opens. `StringShaped` needs no narrowing. |
| RR-PGEEZX | significant | `sort=id` byte order everywhere; `sort=modified` stays declined (not SQL-expressible). |
| RR-QUQ0OE | significant | Lists and undeclared properties sort by string form, matching `->>`. |
| RR-I3QG9P | significant | `parent_sort:` / `child_sort:`, mirroring `ParentColumns`/`ChildColumns`. |

**What the decision changed about this plan.** Three prerequisites the review
added were removed again, because the principle dissolves rather than solves
them:

- No `StringShaped` narrowing for strings or non-ISO dates. Both stay
  pushdown-eligible, since the Go path now matches what SQL does.
- No six-comparator restructuring. The query-sort path needs one comparator.
- No decision about mixed date/datetime instants — that branch simply is not on
  this path.

**Effort: `m` stands.** The earlier re-estimate upward assumed type-aware
comparison on both sides. Conforming to SQL is less code than that, and deletes
more than it adds.

## Design Review

- [x] Run `/design-review` before starting implementation
- [x] All critical/significant findings addressed in plan — all 8 `addressed`

**Design Review Findings:**

| ID | Severity | Summary |
| --- | --- | --- |
| RR-6F2UF2 | critical | Descending path is an invalid comparator; reverses secondary keys |
| RR-C4QYTO | critical | Strings sort natsort in Go vs byte-wise in SQL (99% of real titles differ) |
| RR-TXFI2O | critical | Bound-parameter CASE loses the index on a generic plan (4 -> 1,915 buffers) |
| RR-Z7V8PI | critical | AC1/AC3/AC4 pass with the defect live |
| RR-D1QOQ7 | significant | Non-ISO date formats and mixed date/datetime diverge |
| RR-PGEEZX | significant | sort=id and sort=modified change v1 API behaviour |
| RR-QUQ0OE | significant | List-valued and undeclared properties silently stop sorting |
| RR-I3QG9P | significant | ViewSection.Sort level is ambiguous; nested cap is per-parent |

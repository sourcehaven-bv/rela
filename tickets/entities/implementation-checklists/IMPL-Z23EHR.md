---
id: IMPL-Z23EHR
type: implementation-checklist
title: 'Implementation: Section sort: plus one declared order per enum, on every sort path'
status: in-progress
---

<!-- @managed: claude-workflow v1 -->

## Development

- [x] Unit tests written for new code — 12 in `internal/filter/querysort_test.go`,
      6 in `internal/store/pgstore/ordersql_test.go`, 8 config-validation cases.
- [x] Integration tests written (test full flow, not just units) — the
      differential harness drives the real HTTP handler on both paths; the
      section tests run the real view pipeline via `buildSections`; the EXPLAIN
      test runs against a real PostgreSQL.
- [x] Happy path implemented
- [x] Edge cases from planning handled — nulls (absent and JSON null) in both
      directions, values dropped from an enum, list-valued and undeclared
      properties, non-string scalars, conflicting declared orders across types,
      an embedded quote in a value, `sort=id`, empty and single-row slices.
- [x] ~~Error handling in place~~ (N/A: no new error paths. Config mistakes are
      load errors, which is the existing mechanism.)

**What was built**

| File | Change |
| --- | --- |
| `internal/filter/querysort.go` | NEW. `QuerySort` — the one ordering rule, matching what the store can express. |
| `internal/dataentry/api_v1.go` | `applyV1Sorting` uses it. |
| `internal/dataentry/queryservice.go` | Search/dashboard path uses it. |
| `internal/store/graphquery.go` | `OrderSpec.Values` carries the declared order. |
| `internal/store/graphquerynaive/naive.go` | Ranks in Go for sqlite/fs/mem. |
| `internal/store/pgstore/graphquery.go` | `CASE` rank in `ORDER BY`, literal arms. |
| `internal/store/pgstore/derivedschema.go` | Matching `CASE` in DDL; values hashed into the index name. |
| `internal/store/derivedschema.go` | `DerivedObjectSpec.OrderValues`. |
| `internal/queryplan/queryplan.go` | `DeclaredValues`; values in the spec and the dedup key. |
| `internal/dataentry/listpushdown.go` | Sets `Values` on the pushed query. |
| `internal/dataentryconfig/config.go` | `sort:` / `parent_sort:` / `child_sort:` on `ViewSection`. |
| `internal/dataentryconfig/validate.go` | `validateSectionSort`; wrong-key-for-display refused. |
| `internal/dataentry/sections_nested.go` | `entitySorter`; sorting before both caps. |

**On the plan's central claim.** It said this ticket "writes no new comparison
code — it deletes a duplicate". That was wrong, and design review disproved it
before implementation started. The two sorters differed on strings, dates, ids,
lists, undeclared properties and the meaning of descending. What the user's
SQL-wins decision then did was shrink the fix rather than grow it: one
comparator instead of six type-aware ones, and two planned `StringShaped`
narrowings became unnecessary.

## Test Quality

- [x] Using fixture builders or factories for test data — `qsDefs`/`qsSort`/
      `rowsWithStatus` in filter; `nestedFixtureWithProps` extends the existing
      `nestedFixture` rather than adding a parallel set.
- [x] No hardcoded values in assertions when object is in scope — AC7 derives
      its fixture size from `nestedNodeBudget` and `nestedChildPreview`.
- [x] Only specifying values that matter for the test
- [x] Interpolated values constructed from objects, not hardcoded
- [x] Property comparisons use original object, not hardcoded strings

**Fixtures chosen so the test cannot pass by luck.** Every enum fixture
declares an order that is the REVERSE of its alphabetical order, so a path that
ranks and a path that compares text cannot agree by accident. The differential
harness's titles were same-case and digit-free, where byte order and natural
order coincide — it was passing over a live divergence, and now uses mixed case
with unpadded numbers.

## Manual Verification

- [x] Feature manually tested end-to-end
- [x] Each acceptance criterion verified with test scenario from planning
- [x] Edge cases manually verified

**Verification Evidence**

**AC1 — pushed and Go paths agree, page for page.** The differential harness
now covers `sort=status`, `-status`, `status,title`, `-status,due`,
`title,status`, `id`, `-id` at two page sizes across three pages, plus the
pre-existing date and title shapes.
Mutation: drop `Values` from the pushed query (the half-landed state) → fails
on `sort=status` page 1.

**AC2 — declared order on both paths.** `status_type` declares `open, closed`,
reversing alphabetical order.
Mutation: remove the rank lookup → 3 filter tests fail.

**AC3 — descending is a valid ordering.** Reproduced the original defect first:
13 equal keys sorted descending returned `MLKJIHGFEDCBA` (reversed, not
stable), and a descending primary inverted its ascending secondary key
entirely.
Mutation: restore the multi-pass `return !less` architecture → 4 tests fail.
Note an earlier mutation of mine passed and was **inert** — the single-pass
comparator short-circuits on `c != 0`, so inverting a non-equal comparison is
still correct. The defect needs both `!less` and the per-key stable passes.

**AC4/AC5 — index identity.** `listIndexName` differs across no-values,
declared, reordered and inserted-value specs; `OrderValues` is left nil when
nothing is ranked so existing indexes do not churn.
Mutation: omit values from the hash → name-collision test fails.

**AC6 — the index survives a generic plan.** Ran against a real PostgreSQL 18:
both directions use `rela_derived_list__…`, descending via a backward scan.

Two defects found here, both mine:

1. **The property NAME had to be a literal too.** I interpolated the values but
   left `properties ->> $2` bound. Measured via `PREPARE`/`EXECUTE` under
   `force_generic_plan`: bound → Sort + Seq Scan; literal → Index Scan.
2. **The EXPLAIN test could not prove what it claimed.** It passed when I
   mutated the literal back to a bind parameter. `plan_cache_mode` governs
   CACHED plans, and a one-shot `EXPLAIN` over the extended protocol has no
   cache entry, so PostgreSQL substitutes parameters at plan time and finds the
   index either way. Rather than leave a test asserting something it cannot
   check, the bind-vs-literal guard moved to the generated SQL text
   (deterministic, and the layer where the mistake is made) and the EXPLAIN
   test documents its own limit.

**AC7 — section sort before the per-parent cap.** The fixture seeds enough
parents to exhaust `nestedNodeBudget` partway through, so late parents are cut
by the shared budget rather than the preview cap; only the last three children
of each parent are `open`, so an unsorted prefix would contain none of them.
The test fails if it emits no parent with 3+ children, so it cannot pass
vacuously.
Mutation: sort after the cap → fails.

**AC8 — edge semantics match SQL.** Nulls last ascending / first descending;
unknown properties sort byte-wise; lists sort by string form.
Mutation: restore the `.(string)` type assertion → list test fails.

## Quality

- [x] Code follows project patterns — `parent_sort`/`child_sort` mirror
      `ParentColumns`/`ChildColumns`; the index-name hash reuses the
      `uniqueIndexShape` mechanism from BUG-HC6I2T; `validateSectionSort`
      follows `validateLevelColumns`.
- [x] Checked for DRY opportunities — `queryplan.DeclaredValues` is the single
      resolver used by both the pushdown planner and the index spec, so the
      query and its index cannot disagree about what a property's order is.
      `filter`'s existing per-type comparators are left alone for non-query
      callers rather than deleted.
- [x] No security issues introduced — enum values and property names are
      interpolated into SQL, which is new on the query path. Both come from
      operator-authored `schema.yaml` (the trust level the derived-index DDL
      already interpolates) and both go through `quoteLiteral`. The code
      comment says why it must not be "fixed" back to bind parameters, and a
      test covers an embedded quote.
- [x] No silent failures — the previous comparator returned `false` for every
      pair of non-string values, which silently did not sort at all. That is
      gone: values render to the text form `->>` yields.
- [x] No debug code left behind — three temporary probe files and two scratch
      databases removed; `grep` for `zz_` returns nothing.

**Verification run:** all four build tags compile; `arch-lint` clean (the new
`dataentry → filter` edge is allowed); `golangci-lint` 0 issues; `gofmt` clean;
the full `pgstore` conformance suite passes against real PostgreSQL (67s).

**Behaviour changes shipped**, all three in the release note and docs:
enum-sorted lists move to declared order; string sorts become byte order
(case-sensitive, non-numeric); `sort=id` becomes byte order in search and CLI.
`sort=modified` stops being honored on the query path — it has no stored
column, so no backend can order by it.

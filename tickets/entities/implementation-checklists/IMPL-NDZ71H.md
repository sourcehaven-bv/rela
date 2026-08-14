---
id: IMPL-NDZ71H
type: implementation-checklist
title: 'Implementation: Extend store.GraphQuery with property predicates and relation negation'
status: done
---

<!-- @managed: claude-workflow v1 -->

## Development

- [x] Unit tests written for new code
- [x] Integration tests written (test full flow, not just units)
- [x] Happy path implemented
- [x] Edge cases from planning handled
- [x] Error handling in place (errors surfaced, not swallowed)

Changes:

- `internal/propmatch/` (new, pure stdlib leaf) — the single authoritative
definition of "empty property" plus untyped equality. Extracted rather than
duplicated because `internal/filter` (metamodel-aware) and the store backends
sit on opposite sides of an arch-lint boundary: `graphquerynaive` may depend
only on `entity` + `store`, and `filter` is a *branch* (pulls `metamodel`,
`pattern`, `natsort`), so depending on it would drag that subtree into the store
layer.
- `internal/filter/match.go` — delegates its emptiness decision (scalars and the
empty-list case in `matchList`) to `propmatch`. Ordered/pattern operators still
fall through to metamodel-aware comparison.
- `internal/store/graphquery.go` — `Props []PropPredicate` (equality only;
ordered comparison is deliberately out of scope, it needs the declared type) and
`RelationPredicate.Negate`. Negate is a separate flag, not an overload of a nil
predicate: nil already means "do not constrain this direction".
- `internal/store/graphquerynaive/naive.go` — property check runs first (pure
in-memory, so a non-match skips the per-candidate relation I/O); negation
inverts the relation match; empty `Endpoints` now means "any endpoint".
- `internal/store/pgstore/graphquery.go` — `buildPredicateParts` factored out so
the two query shapes (`GraphQuery`/`GraphCount` and `MatchingIDs`) cannot drift;
negation as `NOT EXISTS`; property predicates as jsonb comparisons.

Edge cases handled: absent key vs JSON null vs empty string vs empty list (all
one "empty" state); multi-select equality (any element matches); non-string
scalars comparing by text form; exclusion filters not widening to include unset
rows; `?` returning NULL for a missing key (COALESCE, so a surrounding NOT
behaves).

## Test Quality

- [x] Using fixture builders or factories for test data
- [x] No hardcoded values in assertions when object is in scope
- [x] Only specifying values that matter for the test
- [x] Interpolated values constructed from objects, not hardcoded
- [x] Property comparisons use original object, not hardcoded strings

Conformance tests live in `storetest` (the shared harness), so all three
backends are held to one contract rather than each asserting its own behaviour.
`seedEntityWithProps` was added alongside the existing `seedGraphQueryEntities`
following the same helper style. Table-driven with `t.Run(tc.name, ...)` per
project convention.

## Manual Verification

- [x] Feature manually tested end-to-end
- [x] Each acceptance criterion verified with test scenario from planning
- [x] Edge cases manually verified

**Verification Evidence:**

Stood up a live PostgreSQL (`rela_test`) and ran the pgstore conformance suite
against it — the *only* way the parity bugs surfaced, since the suite skips
silently without `RELA_TEST_DATABASE_URL` (filed as RR-0EWZQW).

Two real defects were caught this way and would otherwise have shipped:

1. **42P18 on every any-endpoint query.** Making empty `Endpoints` mean "any"
left the endpoints parameter registered but unreferenced; PostgreSQL rejects
that with "could not determine data type of parameter $2". Every non-DB test
passed. Fixed by registering the arg lazily.
2. **List-shape divergence** (RR-RGAXHK, critical). Confirmed directly in psql:
`properties ->> 'p'` renders `[]` as the two-character string `"[]"` and
`["a","b"]` as JSON text, so is-empty and equality both disagreed with the naive
backend. Fixed by branching on `jsonb_typeof` and using the containment operator
for arrays.

Final state: `Props_value_shapes` (6 op/target combinations × 9 seeded entities
covering scalars, empty list, populated lists, int, bool, absent, blank) passes
identically on memstore (naive path) and pgstore (SQL path, live DB).

The ACL fail-closed guard (RR-6947C1) was mutation-tested: removing the guard
makes `TestReadQuery_FailsClosedOnEmptyMemberSet` fail with a clear message,
confirming the test exercises what it claims. An earlier version of that test
passed with the guard removed — it went through the public API, where
`ForPrincipal` rejects the scenario — so it was replaced with an in-package test
that constructs the `Request` directly.

## Quality

- [x] Code follows project patterns (check similar code)
- [x] Checked for DRY opportunities — `buildPredicateParts` extracted because
`buildMatchingIDsSQL` and `buildGraphQuerySQL` were duplicating the predicate
assembly; leaving them separate would have meant applying every future predicate
twice, which is exactly how the two shapes drift.
- [x] No security issues introduced — every user-supplied value routes through
`sqlBuilder.arg` (positional placeholder); format strings interpolate only `$N`
placeholders and compile-time literals. Independently confirmed in review.
- [x] No silent failures (errors logged AND returned)
- [x] No debug code left behind

`just lint` 0 issues; `just arch-lint` OK; `just coverage-check` PASS (package
floor and total, 76.9% overall; new `propmatch` at 97.0%); full `./internal/...`
green; pgstore green with `-race` against a live database.

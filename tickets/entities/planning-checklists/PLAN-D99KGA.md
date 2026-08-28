---
id: PLAN-D99KGA
type: planning-checklist
title: 'Planning: Derive PostgreSQL indexes from static pushed-down query predicates'
status: done
---

<!-- @managed: claude-workflow v1 -->

## Understanding

- [x] Problem/requirements clearly understood
- [x] Scope defined (what's in/out documented below)
- [x] Acceptance criteria documented with specific test scenarios

**Scope:**

In scope: statically declared query strings in `data-entry.yaml` that execute
through `dataentry.executeQuery`: dashboard cards, global next-action sources,
and `pick_one` option queries. Extract their already push-eligible, non-empty
scalar string equality predicates, group them by entity type and complete query
shape, and reconcile matching PostgreSQL expression indexes at boot and through
`rela db status|reconcile`. Keep fsstore/memstore behavior unchanged.

Also in scope is making the pushed store predicate explicitly scalar. The
current generic equality SQL contains a scalar/list `CASE`; a normal expression
index cannot reliably serve that shape. The scalar contract is safe here because
the pushdown eligibility check already requires a non-list string declaration on
every queried type, while the authoritative Go filter remains in place.

Out of scope: runtime/ad-hoc query observation, indexes for request URL filters,
automatic computed/hidden properties, relation predicates, full text, sorting,
ordered/not-equal/glob/empty predicates, cost-based decisions, hot-reload DDL,
and indexes for predicate `condition:` expressions.

**Acceptance Criteria:**

1. A static single-type query with one non-empty, declared scalar string
equality produces one deterministic `DerivedQueryIndex` specification; a unit
test covers dashboard, next-action, and `pick_one` sources.
2. Equivalent query shapes deduplicate regardless of source, map order, filter
order, or literal value; distinct property sets/types remain distinct.
3. Only filters the existing pushdown can safely execute are indexed. Tests
reject missing/multiple types, undeclared/list/typed properties, glob,
not-equal, empty-value and free-text queries.
4. pgstore reconciliation creates, preserves, dry-run reports, and drops only
its `rela_derived_query__*` indexes, without touching unique-rule or
operator-owned indexes.
5. The scalar GraphQuery predicate remains backend-parity correct for scalar,
absent, empty, JSON-null and invalid list-shaped stored values.
6. A live PostgreSQL EXPLAIN test with realistic cardinality names the generated
index for a representative `type + property equality` GraphQuery.
7. Store-open and `rela db status|reconcile` derive the same desired set from
`schema.yaml` plus valid `data-entry.yaml`; missing config yields only unique
specs, while invalid config is reported rather than silently dropping owned
indexes.
8. `just arch-lint`, targeted tests, `just test-postgres`, and `just ci` pass.

## Research

- [x] ~~For larger features: run `/research` to create a structured research doc~~ (N/A: direct extension of two existing in-tree mechanisms)
- [x] Searched for existing libraries that solve this problem
- [x] Checked codebase for similar patterns or reusable code
- [x] Looked for reference implementations in other projects
- [x] Reviewed relevant rela concepts for prior art

**Research Doc:** N/A; this is a bounded composition of TKT-F4TIS6's GraphQuery
property pushdown and TKT-3Q0GP1's derived-schema reconciler.

**Existing Solutions:**

- `internal/dataentry/helpers.go` owns the conservative pushdown eligibility
rules and keeps `filter.MatchAll` authoritative.
- `internal/store/pgstore/graphquery.go` currently emits a `CASE` supporting
both scalar and list shapes. That general expression is not the same shape as a
scalar `properties->>'key'` B-tree expression index.
- `internal/store/derivedschema.go` and
`internal/store/pgstore/derivedschema.go` already provide stateless,
advisory-locked, prefix-owned desired-vs-actual reconciliation. Extend this
mechanism instead of introducing migrations or an index registry table.
- PostgreSQL expression and partial indexes are the native solution; no new Go
library is needed. PostgreSQL's leftmost B-tree matching and predicate
implication will be verified by EXPLAIN rather than assumed.
- TKT-F4TIS6 and FEAT-79DTF9 establish backend parity and the static
next-action query use case. TKT-3Q0GP1 establishes ownership, dry-run and
boot-only reconciliation conventions.

## Approach

- [x] Technical approach chosen and documented
- [x] Approach builds on existing patterns (not reinventing)
- [x] Alternatives considered (document why rejected)
- [x] Dependencies identified (packages, APIs, types)

**Technical Approach:**

1. Extract the safe filter-to-store-predicate compilation from `dataentry` into
a small shared query-planning package. It returns GraphQuery predicates plus
static index shapes from the same decision, preventing eligibility drift.
2. Add an explicit scalar-string equality representation to
`store.PropPredicate`. pgstore lowers it to a direct `->>` comparison with a
scalar-shape guard; graphquerynaive mirrors that contract. The existing generic
equality operator stays unchanged for other callers.
3. Collect validated static queries from dashboard cards, global next-action
sources, and `pick_one`. Canonicalize each desired index as entity type plus
sorted unique property names; literal values are deliberately excluded so
queries differing only in values share an index. Create one composite index per
complete shape, not one index per individual clause.
4. Add `DerivedQueryIndex` and a property-list field to
`store.DerivedObjectSpec`. Generate a deterministic SHA-256-derived name in a
disjoint `rela_derived_query__` namespace.
5. Extend pgstore's existing reconciler with a per-kind plan: catalog only the
query prefix, drop only orphaned query indexes, and create partial expression
indexes scoped by literal validated entity type. DDL identifiers/literals go
through the existing defensive quoting/validation.
6. Centralize loading of all derived specs (unique plus static query specs) so
appbuild boot and `rela db status|reconcile` cannot disagree. Missing
`data-entry.yaml` is valid. A present invalid file must surface an error and
must not run reconciliation with an incomplete desired set, which would
otherwise drop valid indexes.
7. Keep reconciliation boot/manual-only. Document restart or `rela db
reconcile` after static query configuration changes.

Alternatives rejected: a GIN index over all `properties` is broad, larger and
does not directly serve the current `CASE`; one index per property loses the
compound query shape and multiplies indexes; runtime `auto_explain`/statistics
is explicitly out of scope; silently reconciling only parsable queries can
destructively misclassify desired indexes as orphans.

**Files to modify:**

- `internal/store/graphquery.go`, `internal/store/graphquerynaive/naive.go`
- `internal/store/derivedschema.go`
- `internal/store/pgstore/graphquery.go`, `derivedschema.go` and tests
- `internal/dataentry/helpers.go` and pushdown tests
- new shared static-query planning/derived-spec collector package and tests
- `internal/appbuild/derivedschema_postgres.go` and assertions
- `internal/cli/db_postgres.go` and tests
- `docs/postgres-backend.md`; root `CLAUDE.md` only if a durable invariant is
introduced

## Security Considerations

- [x] Input sources identified (user input, config, external APIs)
- [x] Input validation approach defined (allowlist preferred over blocklist)
- [x] Security-sensitive operations identified (file access, auth, crypto)
- [x] Error handling doesn't leak sensitive information

**Input Sources & Validation:**

Inputs are operator-authored `schema.yaml` and `data-entry.yaml`. The normal
metamodel/config validators run first. Only allowlisted query shapes already
accepted for scalar pushdown produce specs. Entity/property names receive the
existing DDL safety validation as defense in depth. Query literal values never
enter index definitions.

**Security-Sensitive Operations:**

DDL remains PostgreSQL-build-only, serialized by the existing schema-scoped
advisory lock, uses deterministic Rela-owned name prefixes, and never drops
outside the query-index sub-prefix. A parse/validation failure aborts
desired-set calculation before DDL, preventing accidental drop-on-partial-input.
Index contents are entity data, but status/log output reports only
config-declared type/property names, never values.

## Test Plan

- [x] Test scenarios documented for each acceptance criterion
- [x] Edge cases identified and documented
- [x] Negative test cases defined (invalid input, error conditions)
- [x] Integration test approach defined (not just unit tests)

**Test Scenarios:**

AC1-3: table-driven collector/compiler unit tests using parsed real config and
metamodel fixtures. AC4: pgstore reconciler unit SQL-shape tests plus DB-gated
create/idempotence/dry-run/drop/prefix-isolation tests. AC5: extend storetest
GraphQuery conformance and run mem/fs/pg implementations. AC6: DB-gated seeded
EXPLAIN test after ANALYZE, asserting the deterministic index name. AC7:
appbuild/CLI tests feed missing, valid and invalid configs and compare specs and
failure behavior. AC8: project commands.

**Edge Cases:**

No static queries means no query indexes. Duplicate sources and reordered
filters deduplicate. Empty equality stays unindexed because its semantics span
missing/null/blank/empty-list. Invalid list-shaped storage under a declared
scalar field is excluded by the scalar prefilter and therefore cannot widen the
authoritative result. Unsafe/control-character schema names are reported
unenforced. Concurrent reconcilers retain the existing non-blocking advisory
lock behavior. A maximum of one index per unique static query shape prevents
literal-value cardinality from multiplying indexes.

**Negative Tests:**

Malformed/invalid config returns a load error and performs no query-index DDL.
Unsupported/free-text/dynamic query shapes produce no spec, not an error.
Infrastructure/catalog failures return the existing reconcile error. An unsafe
spec is `DerivedUnenforced`, never interpolated. A hand-created/operator index
and derived unique index survive query-index removal.

## Risk Assessment

- [x] Technical risks assessed with mitigations
- [x] Security risks assessed (see Security Considerations)
- [x] Effort estimated (xs/s/m/l/xl)

**Risks:**

- **Unused indexes:** exact SQL/index compatibility is proven with EXPLAIN;
unsupported shapes are skipped.
- **Semantic narrowing/widening:** one compiler produces both the pushed
predicate and index shape; Go remains authoritative; conformance covers
malformed stored shapes.
- **Destructive reconciliation from bad config:** fail before reconciliation,
use a disjoint owned prefix, and test prefix isolation.
- **Index explosion/write amplification:** deduplicate by complete canonical
shape and ignore literal values; runtime queries are excluded.
- **Boot latency/locking:** retain boot-only, non-blocking advisory-lock
reconciliation. Effort remains `m`.

## Documentation Planning

For enhancements: identify what documentation needs updating.

- [x] User-facing docs identified (skip if internal refactor)
- [x] Docs-checklist will be created when entering implementation

**Documentation Impact:**

- [x] `docs/postgres-backend.md` — which static queries receive indexes,
reconciliation lifecycle, and limitations.
- [x] `docs/cli-reference.md` only if `db status/reconcile` output wording
changes materially.
- [x] Root `CLAUDE.md` only if needed to pin the derived query-index ownership
and no-partial-desired-set invariant.
- [x] ~~`docs/data-entry.md`~~ (N/A: query syntax and UI do not change).

## Design Review

- [x] Run `/design-review` before starting implementation
- [x] All critical/significant findings addressed in plan

**Design Review Findings:** [[RR-CJ9PG6]], [[RR-YEKG49]], [[RR-PXW1PF]]

- Significant: generic scalar/list `CASE` SQL would make the proposed B-tree
index unusable. Addressed by explicit scalar predicate semantics and an EXPLAIN
acceptance gate.
- Significant: parsing invalid config as an empty/partial desired set could
drop still-desired indexes. Addressed by all-or-nothing desired-set loading.
- Minor: one index per query clause would multiply write cost and ignore
compound selectivity. Addressed by one canonical composite index per complete
static query shape.

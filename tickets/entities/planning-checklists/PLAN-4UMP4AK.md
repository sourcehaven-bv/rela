---
id: PLAN-4UMP4AK
type: planning-checklist
title: 'Planning: Store capability for range fingerprints'
started: "2026-11-07"
completed: "2026-11-07"
status: done
---

<!-- @managed: claude-workflow v1 -->

## Understanding

- [x] Problem/requirements clearly understood
- [x] Scope defined (what's in/out documented below)
- [x] Acceptance criteria documented with specific test scenarios

Step 3 of FEAT-CMVZQFI. Set reconciliation compares the additive fingerprint of
an address range of elements between two instances. This ticket adds the store
read that answers "count and fingerprint sum of the elements in [lo, hi)" and
"list the elements in [lo, hi)", over the rows one reader may read, with the ACL
row gate and the field-visibility decision inside the backend query. It also
resolves RR-5EXMHVP (the security constraints deferred from step 2).

**Scope:**

In scope:
- Store-level request and result types in `internal/store` (`elementrange.go`), following the `store.RollupQuery` precedent (gate and `RowCondition` in the store's own vocabulary, so the store does not import `search` or `reconcile`): `ElementScope` (per entity type: `AllowAll` / `Query` row gate, face allowlist, `Fields`; per relation type: `Fields`), `ElementRange` (kind, `Lo` inclusive, `Hi` exclusive, `Hi == ""` unbounded), `RangeSum` (count, pending, four limb sums), `RangeEntry` (address, hex hash or pending, row key), `ValidateElementRange`.
- Two methods on every backend: `RangeFingerprint(ctx, scope, r) (RangeSum, error)` and `ListRange(ctx, scope, r, limit) ([]RangeEntry, error)`. No producer-side interface: the consumer (step 5) declares the two-method interface at its call site.
- pgstore and sqlitestore answer each call in one statement over the stored element columns, using the existing graph-query predicate builders for the row gate and `rowConditionsSQL` for field-grant conditions. sqlitestore falls back to the reference for a name its builder cannot render (as its graph queries do).
- New package `internal/store/elementnaive`: the reference implementation over ordinary store reads (`ListEntities`, `ListRelations`, `MatchingFaces`, `store.HoldingRows`), computing elements in memory with `reconcile`. memstore and fsstore delegate to it, as they delegate graph queries to `graphquerynaive`.
- `storetest`: a conformance suite (`RunElementRangeTests`, run from `RunAll` for every backend) and a randomized differential test (`RunElementRangeDifferential`) of each backend against `elementnaive` over an identically seeded memstore, plus budget tests.
- Resolve RR-5EXMHVP: the rules are enforced by construction and pinned by tests.

Out of scope:
- Computing view hashes for pending rows (redacted rows and rows whose stored element is NULL): step 4 (TKT-N1KNNZU), which builds them from visibility-redacted rows.
- Compiling an ACL request into an `ElementScope` and the HTTP endpoint: step 5.
- Per-row relation meta visibility: a relation type's rows are folded only when the scope says the reader sees every meta field of that type; otherwise all its rows are pending.
- Caching anything across calls (forbidden across principals anyway).

**Acceptance Criteria:**
1. Every backend (memstore, fsstore, sqlitestore, pgstore) passes one conformance suite: counts, limb sums and listed entries for entity and relation ranges equal the expected values computed from `reconcile` elements of the visible rows.
2. A randomized differential test (fixed seeds) compares each SQL backend with `elementnaive` over an identically seeded memstore on generated scopes (AllowAll, relation-predicate gates, face allowlists, hidden fields with grant conditions) and generated ranges: `RangeSum` and `ListRange` agree exactly.
3. Row gate before fold: a row the gate rejects changes neither count nor sum; a relation counts only when its head has a readable face and its tail row (its `FromFace`, or any face for an identity tail) is readable, both under the scope.
4. A stored hash is folded or listed only for rows where the reader has no hidden field: a row of a type with `Fields == nil`, a row with a gated property present whose visibility condition does not hold, a relation of a type with `Fields == nil`, and any row with a NULL element are pending (counted, not summed, listed without hash).
5. Budget: one range query is one SQL statement on pgstore at 10 and at 50 rows (`store.QueryStats`); the reference costs the same number of store reads at 10 and 50 rows (`storetest.Counting`). EXPLAIN tests show the entity and relation range scans use `entities_element_idx` / `relations_element_idx` on pg and sqlite.

## Research

- [x] For larger features: run `/research` to create a structured research doc
- [x] Searched for existing libraries that solve this problem
- [x] Checked codebase for similar patterns or reusable code
- [x] Looked for reference implementations in other projects
- [x] Reviewed relevant rela concepts for prior art

**Research Doc:** RES-G8CYB1R (recommends range-based reconciliation with an
additive SQL fingerprint and the stored hash only where the reader has no
redacted field).

**Existing Solutions:**
- `store.RollupQuery` / `store.RollupGeneric` / `store.HoldingRows` (internal/store/rollup.go): an aggregate under a caller-supplied gate and field-visibility `RowCondition`s, native in pg/sqlite (`rowConditionsSQL`) with a Go reference. This is the model for the request shape and for the hidden-field conditions.
- `search.TypeScope` / `search.FieldScope` and pgstore `buildVisibilityDisjunction` / `fieldGrantCond`: how search applies the row gate and field grants inside the query. `visibility.visibleWhen` already turns a `FieldScope` into one `RowCondition` per gated property; step 5 reuses it to build `ElementScope.Fields`.
- `search.Visible` admission (internal/search/visible.go): the gate probe (`Faces = AllFaces`, face allowlist) and `MatchingFaces` per type; the reference follows it.
- `visibility.EndpointsReadable`: head entity-level, tail at `FromFace`. Relation rule copied from it.
- `graphquerynaive` + `storetest.RunGraphDifferential`: reference-plus-differential pattern.
- Negentropy (range-based set reconciliation): the fingerprint/list pair per range.

## Approach

- [x] Technical approach chosen and documented
- [x] Approach builds on existing patterns (not reinventing)
- [x] Alternatives considered (document why rejected)
- [x] Dependencies identified (packages, APIs, types)

**Technical Approach:**

Types (`internal/store/elementrange.go`):
- `ElementScope{Entities map[string]ElementTypeScope; Relations map[string]RelationElementScope}`. A type absent from a map has no rows in scope.
- `ElementTypeScope{AllowAll bool; Query *GraphQuery; Faces []entity.Face; Fields *FieldVisibility}`. As `search.TypeScope`: AllowAll admits every row, Query admits the rows `MatchingFaces` returns for the probe (`EntityType` = key, `Faces = AllFaces`, ordering and paging cleared), neither denies. `Faces` narrows further (never widens Query.FaceIn).
- `FieldVisibility{Hidden []HiddenField}` and `HiddenField{Property string; Visible RowCondition}`. `Fields == nil` means "visibility not known": every row is pending. `&FieldVisibility{}` means the reader sees every field. A row is foldable when, for each hidden field, the property key is absent from the row or `Visible` holds on the row. A present key with a JSON null counts as present (conservative).
- `RelationElementScope{Fields *FieldVisibility}`: nil means every row pending; non-nil must have no Hidden entries in this step (refused).
- `ElementRange{Relations bool; Lo, Hi string}`; `Hi == ""` is unbounded (the empty string is the least address, so it is free to mean "to the end").
- `RangeSum{Count, Pending uint64; LimbSums [4]uint64}`; fold with `reconcile.FingerprintFromLimbSums`. Count includes pending rows.
- `RangeEntry{Address, Hash string; Pending bool; Type string; Ref entity.Ref; Relation entity.RelationKey}`. Hash is "" exactly when Pending. Entries come in bytewise address order.
- `ValidateElementRange(scope, r)`: Lo <= Hi when bounded; no `*` key; Query.EntityType empty or equal to the key; no empty non-nil Faces; relation Fields without Hidden; HiddenField.Property non-empty. `ListRange` also refuses limit <= 0.

pgstore (`elementrange.go`), one statement per call:
- Gate: one disjunction over sorted scope types, each `(e.type = $t AND e.face = ANY(faces) AND <graphWorldScope + buildPredicatePartsPrefixed of the probe>)`, CTEs prefixed per type. Rendered once and reused.
- Foldable: `e.element_hash IS NOT NULL AND NOT (OR over types with nil Fields or Hidden: e.type = $t AND NOT clean_t)`, `clean_t` = AND over hidden fields of `(NOT (e.properties ? $p) OR <rowConditionsSQL>)`.
- Entities: `FROM entities e WHERE entityAddressSQL >= $lo [AND < $hi] AND (gate)`; aggregate `count(*)`, `count(*) FILTER (WHERE NOT ok)`, `COALESCE(sum(fp_i) FILTER (WHERE ok), 0)::bigint`. List: same source, `ORDER BY address LIMIT $n`.
- Relations: `FROM relations rel WHERE relationAddressSQL range AND rel.rel_type = ANY($types) AND EXISTS(head: entities e WHERE e.id = rel.to_id AND gate) AND EXISTS(tail: e.id = rel.from_id AND (rel.from_face = '' OR e.face = rel.from_face) AND gate)`; foldable = hash not NULL and the type has non-nil Fields.
- An empty scope answers zero / nothing without a query.

sqlitestore: the same statements through its own builder (`?N`, `json_each`
lists, key presence via `json_type(e.properties, path) IS NOT NULL`, sums via
CASE). An unsafe name sends the whole call to `elementnaive` over the store, as
`MatchingFaces` does.

elementnaive: per entity type, `ListEntities(Type, AllFaces)`, filter to the
range, gate via `MatchingFaces` (one per gated type) or the face allowlist,
hidden fields via `store.HoldingRows`, element via `reconcile.EntityElement`.
Relations: `ListRelations` per relation type, filter to range, endpoint rows via
one `ListEntities(IDs)` and the same gate per type, element via
`reconcile.RelationElement`. Results sorted in Go string order.

Shared logic lives once: validation and the result shape in `store`, the
in-memory gate and fold in `elementnaive`. pg and sqlite keep only dialect
rendering (they already mirror each other function by function).

Alternatives rejected:
- Take `map[string]search.TypeScope` directly: memstore/fsstore may not import `search`, and the field grants need a per-property form anyway. The store-vocabulary types mirror the Rollup precedent; step 5 converts with `visibleWhen`-style code.
- Producer-side `store.ElementRanger` interface with a dispatch helper (the Rollup shape): the ticket asks for the consumer-side interface; delegation in memstore/fsstore makes every backend carry the methods, so no fallback dispatch is needed.
- Fill NULL elements inside the read: a read path that writes needs the write lock; pending rows go to step 4's Go path instead.
- Hide-by-type only (all rows of a type with any `visible:` grant pending): correct but leaves most rows of common types on the slow path; the per-row SQL predicate mirrors search and is pinned by the differential test.
- Materialize the visible entity set for relation ranges: costs the whole scope even for a small leaf range; correlated EXISTS per relation is bounded by the range.

**Files to modify:**
- `internal/store/elementrange.go` (new), `internal/store/elementrange_test.go` (new)
- `internal/store/elementnaive/naive.go` (new) and tests
- `internal/store/memstore/`, `internal/store/fsstore/`: delegation (new small files)
- `internal/store/pgstore/elementrange.go` (new), tests (budget, EXPLAIN, NULL element)
- `internal/store/sqlitestore/elementrange.go` (new), tests
- `internal/store/storetest/elementrange.go`, `elementrange_diff.go` (new), `storetest.go` (RunAll wiring), `counting.go` (forward the two methods, counted)
- `internal/reconcile/reconcile.go` (package doc: where the rules are enforced)
- `.go-arch-lint.yml` (elementnaive component; memstore/fsstore/sqlitestore may depend on it)
- `.claude/rules/storage.md` (one bullet), `.archplan/baseline.json` if metrics move

## Security Considerations

- [x] Input sources identified (user input, config, external APIs)
- [x] Input validation approach defined (allowlist preferred over blocklist)
- [x] Security-sensitive operations identified (file access, auth, crypto)
- [x] Error handling doesn't leak sensitive information

**Input Sources & Validation:**
- `ElementScope` is server-derived from the ACL (step 5), never wire-supplied. Validation refuses ambiguous shapes; missing entries deny.
- `ElementRange` bounds will come from a peer in step 5: any string is accepted as a bound and only ever bound as a parameter. Lo > Hi is `ErrInvalidQuery`.
- All values are bind parameters (pg) or bound values and `safeLiteral`-checked JSON paths (sqlite, with the reference as fallback).

**Security-Sensitive Operations:**
- RR-5EXMHVP rules: (1) the gate is in the WHERE clause under the aggregate, so a hidden row changes neither count nor sum; (2) relations need both endpoint EXISTS under the scope; (3) the stored hash/limbs are summed or listed only when the per-row foldable predicate holds, and `Fields == nil` is the fail-closed default; (4) no cache, the result is per call.
- Pending is server-internal: whether a row is pending reflects a grant condition. Step 5 must not put it on the wire (it completes pending rows with view hashes from step 4). Documented on the type.
- Errors wrap the backend error; they name no row content.

## Test Plan

- [x] Test scenarios documented for each acceptance criterion
- [x] Edge cases identified and documented
- [x] Negative test cases defined (invalid input, error conditions)
- [x] Integration test approach defined (not just unit tests)

**Test Scenarios:**
- AC1: `storetest.RunElementRangeTests` from RunAll (all four backends): fixed seed with two entity types, faces, a relation-gated type, relations with identity and faced tails, a hidden property set on some rows with a grant condition. Cases: full range sums equal reconcile of the expected visible set; sub-ranges partition (sum of halves equals whole); bounds (Lo inclusive, Hi exclusive, unbounded, empty, Lo == Hi); list order and limit; type-crossing ranges ("a/..." vs "a-b/...").
- AC2: `storetest.RunElementRangeDifferential` run by sqlite and pg conformance tests (memstore/fsstore delegate to the reference): a few hundred generated (scope, range) pairs.
- AC3: cases with a denied type, a gate that admits some rows, a face allowlist, a relation with hidden head, hidden tail face, identity tail with one readable face, endpoint type outside the scope.
- AC4: cases for nil Fields (all pending), empty Fields (all folded), hidden property absent (folded), present with condition false (pending), present with condition true (folded), relation with nil Fields; pg and sqlite backend tests with a raw NULL element (pending, still counted).
- AC5: pg QueryStats budget test, elementnaive Counting budget test, EXPLAIN tests on pg and sqlite.
- Negative: validation table test in `store`.

**Edge Cases:**
- Addresses with non-ASCII ids and faces; type names that are prefixes of others.
- Empty scope, empty range, Hi == "".
- Limb sums for many rows: covered by `reconcile` (2^31 rows bound).

**Negative Tests:**
- Lo > Hi, `*` key, Query type mismatch, empty Faces, relation Hidden, empty Property, limit <= 0: all `ErrInvalidQuery`.

## Risk Assessment

- [x] Technical risks assessed with mitigations
- [x] Security risks assessed (see Security Considerations)
- [x] Effort estimated (xs/s/m/l/xl)

**Risks:**
- Gate drift between the SQL and the reference: the differential test with a memstore reference catches it.
- A gate predicate that fails open on an unusual name (sqlite): unsafe names fall back to the reference.
- Relation range cost: two correlated EXISTS per relation in range; bounded by the range, no N+1 statements. EXPLAIN only; tuning deferred.
- fs/mem reference costs O(rows of the scope types) per call plus one canonical hash per in-range row. Acceptable for the local backends; noted for step 5.

Effort: l.

## Documentation Planning

For enhancements: identify what documentation needs updating.

- [x] User-facing docs identified (skip if internal refactor)
- [x] Docs-checklist will be created when entering implementation

**Documentation Impact:**
- [x] N/A - Internal change, no user-facing docs needed (store capability; `.claude/rules/storage.md` gets the developer note, the godoc carries the contract)

## Design Review

- [x] Run `/design-review` before starting implementation
- [x] All critical/significant findings addressed in plan

**Design Review Findings:** 21 review responses linked to TKT-IQU106C
(RR-V3I92NN, RR-8B0032A, RR-3RVT441, RR-45G02BY, RR-X6CH9X0, RR-JSYG4MH,
RR-70FL0KV, RR-VW6OBC5, RR-8T5T0UK, RR-ISVXQH8, RR-P95GU6V, RR-39PJXUS,
RR-8IX47M9, RR-KS8ZG4Z, RR-HVDTUKI, RR-NNRW98K, RR-T2DQV3D, RR-EESC60X,
RR-36EG7RU, RR-LNQKZDP, RR-ZLEZR4F). Every significant finding is addressed
except RR-3RVT441 (gate closure cost), deferred with a reason.

**Revised approach after design review** (supersedes the sections above where
they differ):
- The reader is a separate value, not store methods: `pgstore.NewElementRanges`,
`sqlitestore.NewElementRanges`, `elementnaive.New(st)` for fsstore and memstore.
No delegation files, no Counting forwarding, no decorator problem.
- `ElementTypeScope.Visibility` (nil means every row pending), `FieldVisibility`
covers properties only; a roll-up name without a condition makes rows pending.
- `ElementRange{Relations, Lo, Hi, Unbounded}`: the zero range is empty;
bounds up to `MaxRangeBound` bytes; `RangeList{Limit, PendingOnly}` up to
`MaxRangeList`.
- `store.PlanElementRange` validates and classifies each type (FoldPending,
FoldStored, FoldPerRow) and clips each type to its address window, so the
backends only render SQL.
- The ok expression is COALESCEd to false, so a NULL condition is pending.
- The archguard test `TestNoUnlistedElementRangeReads` pins callers outside
`internal/store` to an (empty) allowlist.
- The differential runs from the SQL backends against elementnaive over the
same store.

---
id: PLAN-4JKZZJ
type: planning-checklist
title: 'Planning: Push down query scopes built from related() and equalities'
status: done
---

<!-- @managed: claude-workflow v1 -->

## Understanding

- [x] Problem/requirements clearly understood
- [x] Scope defined (what's in/out documented below)
- [x] Acceptance criteria documented with specific test scenarios

**Scope:**

In scope: a data-entry list whose query scope is an AND of `related()` terms
(subject `entity`, any direction, chains allowed) and constant property
equalities (including `current_user.id` once the identity is bound) is served
through `planListPushdown`: store-side paging, ordering and `CountMatched`.
To make that possible for principals whose read gate occupies
`GraphQuery.HasInbound`, `store.GraphQuery` gains `Related`: a conjunctive,
caller-supplied list of directed relation predicates, ANDed with everything.

Out of scope (fall back to today's Go path, same results):
`or`, `not`, `not related(...)`, list membership (`has_current_user`),
non-equality comparisons, two leaves constraining one attribute, leaves the
metamodel gate rejects, a traversal the gate refuses (Go path returns 422),
the nested-slot half of TKT-44PVX2 (a chained incoming hop into a role-gated
endpoint: 422 with candidates, 200 without, on both paths), a view
`condition:`, `?q=` or relation filters (these already disable pushdown).

A traversal the gate denies is answered without a scan: if no other term is
refused, the list is an empty page with total 0, as the Go path returns.

**Acceptance Criteria:**

1. A scoped list with pushable shape issues no `MatchingIDs` and no
   `ListEntityHeaders`; exactly one count and one page query
   (budget test, 10 vs 50 rows).
2. Pushed and Go paths return identical rows, order and total for outgoing,
   incoming, chained, equality-AND-related and identity scopes
   (differential test in `listpushdown_test.go`).
3. A principal whose read gate sets `HasInbound` still gets pushdown for an
   incoming-first-hop scope, and still sees only gated rows.
4. Unpushable scopes keep today's behaviour, including 422
   `query_scope_unsupported` parity.
5. On postgres the pushed page and count queries use a derived index
   (EXPLAIN test).
6. `GraphQuery.Related` behaves identically on every backend (storetest).

## Research

- [x] ~~For larger features: run `/research` to create a structured research doc~~ (N/A: approach follows the existing pushdown and traversal-lowering code; one design choice settled with Jeroen)
- [x] ~~Searched for existing libraries that solve this problem~~ (N/A: internal query-planning change)
- [x] Checked codebase for similar patterns or reusable code
- [x] ~~Looked for reference implementations in other projects~~ (N/A: internal query-planning change)
- [x] Reviewed relevant rela concepts for prior art

**Research Doc:** N/A

**Existing Solutions:**

- `appbuild/queryscopetraversal.go` `traversalHop` + `acl.Request.GateTraversal`
  (`acl/traversal.go:148`) + `acl.TraversalQuery` already lower one
  `related()` term into a gated `RelationPredicate`. Reused unchanged.
- `dataentry/listpushdown.go` `planListPushdown` / `run`: the pushdown path
  to extend.
- `store.GraphQuery.Narrowing` (`store/graphquery.go:77`): precedent for a
  caller-only field kept separate from ACL `Any`, so a caller predicate can
  never become an alternative route to authorization.
- `queryplan.conditionEqualities` (`queryplan.go:367`): shared eligibility core
  for index derivation.
- `querybudget_test.go:408`, `queryscopeindex_explain_test.go:38`: test
  templates.

## Approach

- [x] Technical approach chosen and documented
- [x] Approach builds on existing patterns (not reinventing)
- [x] Alternatives considered (document why rejected)
- [x] Dependencies identified (packages, APIs, types)

**Technical Approach:**

1. Store: `GraphQuery.Related []DirectedRelation` where
   `DirectedRelation{Incoming bool; Pred RelationPredicate}` (a bool, so no
   both-directions zero value); ANDed with every other field. Caller-only;
   the ACL never writes it. pgstore renders each in `buildPredicateParts`
   (shared by page, count and `MatchingIDs`) as `EXISTS`/`NOT EXISTS` via
   `buildPredicateSQL` with a named `rel%d` CTE prefix;
   `buildVisibilityDisjunction` errors on a non-empty `Related`.
   `graphquerynaive` checks each with `matchesPredicate`;
   `CheckEndpointShape` walks them. storetest pins AND semantics,
   coexistence with `HasInbound`, direction, and paging + count. Type doc
   and the pgstore prefix comment updated.
2. predicate: an exact conjunction walker returning every leaf of the AND
   spine, `exact=false` on `or`, `not` or an unknown node.
3. queryplan: `LowerScope(prog, meta, entityType, identity)` accounts leaf by
   leaf: exact only if every leaf yields exactly one pushed predicate (a
   string equality passing the metamodel gate, non-empty, one per attribute;
   or `related()` on `entity`). It decides pushdown eligibility only; index
   columns stay on `ConditionIndexProperties`, and a test asserts
   `LowerScope` props are a subset of `listScopeIndexProperties`.
4. appbuild: `Lower(ctx, entityType, gate)` is a func field on
   `resolvedQueryScope` next to `Filter` and `Bind` (the consumer interface
   stays at three methods). It returns a fragment (Props + Related), or
   `ok=false`, or `empty=true` when a traversal is denied and none is
   refused. Each traversal is gated per request through `traversalHop` +
   `GateTraversal`. Identity comes from `predicatefns.QueryIdentityFrom(ctx)`,
   never cached on the resolver.
5. dataentry: `api_v1.go:399` runs `planListPushdown`'s cheap eligibility
   checks first, then `Lower`. `planListPushdown` copies the ACL query and
   appends the fragment's Props and Related into fresh slices (the
   `ReadQueryResult` is cached; no aliasing). `AdaptQueryScopes` and
   `adaptedQueryScopes` grow with the resolver.

Alternative rejected: use only free `HasInbound`/`HasOutbound` slots. The
ACL occupies `HasInbound` for every principal reading via a role relation,
so incoming-first-hop scopes would never be pushed for them.

Alternative rejected: extend `NarrowBranch`. It is a disjunction of property
arms; a relation predicate there would be OR-ed, and the type deliberately
carries no relation predicate.

**Files to modify:**

- `internal/store/graphquery.go`, `internal/store/pgstore/graphquery.go`,
  `internal/store/graphquerynaive/naive.go`, `internal/store/storetest/`
- `internal/predicate/prefilter.go`
- `internal/queryplan/queryplan.go`
- `internal/appbuild/queryscopes.go`, `internal/appbuild/queryscopetraversal.go`
- `internal/store/pgstore/visiblesearch.go`
- `internal/dataentry/queryscope.go`, `api_v1.go`, `listpushdown.go`,
  `scopedread.go` (doc comments: `listPage` pushdown comment, the
  `scopedHeaders` duplicate note, `scopeRequest.Scope`, listpushdown header)
- `CLAUDE.md` "Collection reads" bullet

## Security Considerations

- [x] Input sources identified (user input, config, external APIs)
- [x] Input validation approach defined (allowlist preferred over blocklist)
- [x] Security-sensitive operations identified (file access, auth, crypto)
- [x] Error handling doesn't leak sensitive information

**Input Sources & Validation:**

- Scope expressions come from operator config (`data-entry.yaml`), compiled
  at load. The request only selects a scope name. Lowering is an allowlist:
  only constant equalities and `related()` on `entity`; anything else falls
  back.
- `current_user` comes from the authenticated principal on ctx.

**Security-Sensitive Operations:**

- The ACL row gate (`Any`, `HasInbound`, faces) must stay intact.
  `Related` is a separate, conjunctive field, so a scope can only narrow.
  Test: principal with role-relation read + incoming scope sees no
  ungated rows.
- Every traversal is gated per request with the caller's `acl.Request`, as
  on the Go path; no cross-principal caching of lowered fragments.
- The count comes from the same composed query, so it is post-gate (no
  existence oracle).

## Test Plan

- [x] Test scenarios documented for each acceptance criterion
- [x] Edge cases identified and documented
- [x] Negative test cases defined (invalid input, error conditions)
- [x] Integration test approach defined (not just unit tests)

**Test Scenarios:**

- AC1: new budget test asserting `counting.Calls()`; the existing traversal
  budget test is repointed at an unpushable scope.
- AC2/AC3: differential test in `listpushdown_test.go` against the Go path,
  including a role-relation principal.
- AC4: eligibility table test (or, not, not related, has_current_user,
  two leaves on one attribute, `entity.id`, non-string property, empty
  literal, `current_user.tool`, refused -> 422, denied -> empty page).
- AC5: `TestExplainPushedTraversalScopeListUsesDerivedIndexes` (pg): a
  sorted list config; page and count plans have no Seq Scan on entities.
- AC6: storetest `Related` cases, run by every backend.

**Edge Cases:**

- Two `related()` terms in the same direction; chained hops; incoming first
  hop with ACL `HasInbound` set.
- Identity scope with no identity bound: fall back (Go path).
- Empty result: count 0, empty page.
- Sorting on a property plus paging past the end.

**Negative Tests:**

- `not related`, `or`, comparison operators: not lowered, same results.
- Traversal filtering a `visible:`-hidden field: 422 on both paths.

## Risk Assessment

- [x] Technical risks assessed with mitigations
- [x] Security risks assessed (see Security Considerations)
- [x] Effort estimated (xs/s/m/l/xl)

**Risks:**

- Go and SQL equality semantics differ for some value: differential test.
- Pushed query built with different world/faces than the Go path: reuse the
  ACL query copy that already carries both.
- Store contract change across backends: storetest runs on all four.

Effort: xl (raised from l by the store contract change).

## Documentation Planning

- [x] User-facing docs identified (skip if internal refactor)
- [x] Docs-checklist will be created when entering implementation

**Documentation Impact:**

- docs-project source for the query-scopes guide: note that simple scopes are
  paged in the store, and which shapes fall back.
- `store.GraphQuery.Related` godoc.

## Design Review

- [x] Run `/design-review` before starting implementation
- [x] All critical/significant findings addressed in plan

**Design Review Findings:** RR-YOK2SJ, RR-5YZKF8, RR-3MMGN9 (significant);
RR-3WEG3X, RR-JLRLPW, RR-8D8IGY, RR-7TDPYA, RR-03QVNC (minor); RR-UU1GPR
(nit). All addressed in this plan.

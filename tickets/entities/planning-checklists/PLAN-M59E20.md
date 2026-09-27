---
id: PLAN-M59E20
type: planning-checklist
title: 'Planning: related(): traverse incoming edges (filter B on properties of A where A -> B)'
status: done
---

<!-- @managed: claude-workflow v1 -->

## Understanding

- [x] Problem/requirements clearly understood
- [x] Scope defined (what's in/out documented below)
- [x] Acceptance criteria documented with specific test scenarios

Goal: given `A -[r]-> B`, a data-entry view on B can use a `query_scopes:` entry
that filters on A's properties, e.g. on `feature`: `related(entity,
'implementedBy', { status = 'in-progress' })`.

TKT-RELTRV built the pieces but wired none of them (no production caller of
`ValidateTraversals`, `GateTraversal` or `Bindings.SetTraversal`), so this
ticket also wires `related()` end to end for ONE surface.

**Scope:**

In scope:
- Incoming hops, written as the relation's declared inverse ID
(`InverseDef.ID`). Hops of both directions can be chained.
- Validation at load for `query_scopes:` (unknown relation, wrong start type,
union without `type =`, ineligible property) as a scope compile error.
- ACL gating per request through `acl.Request.GateTraversal`, direction-aware.
- Evaluation in data-entry list/kanban/export reads that apply a query scope.
- Derived index for the traversed type of a query-scope traversal (postgres).

Out of scope (follow-up tickets):
- Other surfaces: view `condition:`, next-actions, CLI `--filter`, automations,
validation. They keep failing at evaluation as today.
- Relations without a declared inverse cannot be walked backwards.
- Symmetric relations: refused with a load error (stored once in an arbitrary
direction, so a one-direction walk would miss edges). Also for outgoing.
- Ordered comparison and list properties (unchanged from TKT-RELTRV).
- Pushing the traversal into the paged list query. A scope already disables
the pushdown fast path; that stays.

**Acceptance Criteria:**
1. A `feature` query scope `related(entity, 'implementedBy', { status = 'in-progress' })`
lists exactly the features with at least one in-progress implementing ticket
(data-entry integration test on fs and memory, storetest on all backends).
2. A chain mixing directions resolves: `{ 'implementedBy', 'affects' }` with
`type = 'concept'` (predicatefns + storetest).
3. A bad traversal in `query_scopes:` fails at load, naming the relation:
unknown name, inverse used from the wrong type, union without `type =`,
symmetric relation, undeclared or list property.
4. ACL: a ticket the principal cannot read does not make its feature match;
a conditionally visible property on the far type is refused; a face-restricted
far type fails closed.
5. Cost: query count for a scoped list is the same at 10 and 50 rows
(`storetest.Counting` budget), i.e. one traversal query per traversal per
request, not per row.
6. Postgres: EXPLAIN shows the derived index on the far (FROM) type is used.

## Research

- [x] For larger features: run `/research` to create a structured research doc
- [x] Searched for existing libraries that solve this problem
- [x] Checked codebase for similar patterns or reusable code
- [x] Looked for reference implementations in other projects
- [x] Reviewed relevant rela concepts for prior art

**Research Doc:** RES-RELTRV (parent design; this extends it, no new research
doc)

**Existing Solutions:**
- Store already models direction: `GraphQuery.HasInbound`,
`EndpointPredicate.HasInbound`; pgstore `buildPredicateSQL` and
`graphquerynaive.hasMatchingRelation` take the direction as a parameter.
- Inverse IDs are validated unique and non-colliding at load
(`metamodel/loader.go:610` `validateRelationInverses`) and resolved by
`Metamodel.InverseOwner`; data-entry resolves names the same way
(`dataentry/relations_direction.go:41` `resolveDirection`).
- `store.MatchingIDs(ctx, q, ids)` answers "which of these candidates match"
in one query: the per-request batch primitive needed here.
- ~~External libraries / reference implementations~~ (N/A: internal query-language extension)

## Approach

- [x] Technical approach chosen and documented
- [x] Approach builds on existing patterns (not reinventing)
- [x] Alternatives considered (document why rejected)
- [x] Dependencies identified (packages, APIs, types)

**Technical Approach:** (revised after design review)

1. Resolution (`predicatefns/traversal.go`, DONE). `ResolveTraversal` returns
`[]ResolvedHop{Relation, Incoming, Target}`. Canonical name first, else
`InverseOwner`. Union and `type =` rules apply to the walked side. Symmetric
relations are refused whichever name matched; an incoming hop over a relation
with empty `From` is refused. `queryplan` shares it.
2. Subject (`predicate/traversal.go`). `TraversalSpec` records the subject
identifier; `ValidateTraversals` refuses anything but `entity`.
`TraversalSpec.Key()` gives a canonical key (path, type, sorted typed props) for
answer lookup.
3. Store (DONE). Endpoint-match hops read the default state only (default
endpoint face, default-tailed edge) on every backend.
4. ACL (`acl/traversal.go`).
   - `TraversalHop` gains `Incoming`. One exported builder lowers a hop
chain to an ungated `RelationPredicate` (direction and chaining in one place);
`GateTraversal` uses it and adds the gates.
   - `GateTraversal` drops the `FieldVisibility` parameter and reads the
policy from its `Declarative`, so the field gate cannot be skipped.
   - `DenyAll` on any hop -> `ErrTraversalDenied` (answer: no match).
Faces, `Any`, inheritance in the endpoint's inbound slot (now checked regardless
of `Next`), and a chained incoming hop colliding with the ACL inbound predicate
-> `ErrTraversalUnsupported`, surfaced as an error, never as "no match" (so `not
related(...)` cannot widen).
5. Load (`scopes.Compile`, `appbuild/queryscopevisibility.go`).
`ValidateTraversals` per scope joins `CompileError`. A traversal filtering a
`ConditionallyVisible` property is a boot error (policy-wide,
principal-independent). A chained incoming hop into a type whose read is granted
through a relation gets a load warning.
6. Seam (`dataentry/queryscope.go`, `scopedread.go`, `appbuild/queryscopes.go`).
The per-row `Evaluate` becomes a batch `Filter(ctx, scope, entityType, headers
[]store.EntityHeader, gate, match)` with unnamed func types (appbuild cannot
name dataentry types): `gate func(ctx, acl.TraversalHop)
(*store.RelationPredicate, error)` and `match func(ctx, store.GraphQuery,
[]string) (map[string]bool, error)`. `Filter` gates and answers each distinct
traversal once with a FRESH `GraphQuery{EntityType, HasInbound|HasOutbound}`,
then evaluates rows with the answers passed explicitly (`MatchesAs` variant
taking a `TraversalFunc`; the func checks the subject id equals the row id).
Nothing is stored on ctx or on the resolver. `applyScope` supplies
`readGateFromContext(ctx).GateTraversal` and a `match` closure over
`svc.Store.MatchingIDs` that stamps World and FaceIn via `stampScope`.
7. No-ACL deployments. `readGate` gains `GateTraversal`; `aclReadGate`
delegates to `acl.Request`; `nopReadGate` returns the ungated builder's result,
consistent with its `ReadQuery` answering AllowAll.
8. Index (`queryplan`). Derive `TraversalIndexSpecs` from every scope
declared in the metamodel (any scope is selectable by `?query_scope=`).
9. Unwired surfaces. View `condition:` and next-action `condition:` refuse
`related()` at load (conditionlint), so operators get a load error instead of a
10. The next-action count-zero hint applies default scopes through
`applyScope`, so it is covered by step 6 and tested.

**Alternatives rejected:**
- Explicit `'<-rel'` syntax: user chose inverse IDs (the vocabulary the UI shows).
- Go-side per-row lookup via `ListRelations`: the N+1 TKT-RELTRV forbids.
- Pushing the traversal into the main list query: collides with the ACL
query's `HasInbound` slot and the scope path is already Go-filtered.

**Files to modify:**
- internal/predicate/traversal.go (subject, Key)
- internal/predicatefns/traversal.go, evaluator.go (+ tests)
- internal/store/graphquery.go, pgstore/graphquery.go, graphquerynaive/naive.go, storetest/endpointmatch.go (done)
- internal/acl/traversal.go (+ tests)
- internal/scopes/scopes.go (+ tests)
- internal/appbuild/queryscopes.go, queryscopevisibility.go
- internal/dataentry/queryscope.go, scopedread.go, readgate.go (+ integration tests)
- internal/conditionlint (refuse related() on view/next-action conditions)
- internal/queryplan/traversal.go, queryplan.go (+ tests)
- internal/store/pgstore/graphquery_explain_test.go
- docs/metamodel.md

## Security Considerations

- [x] Input sources identified (user input, config, external APIs)
- [x] Input validation approach defined (allowlist preferred over blocklist)
- [x] Security-sensitive operations identified (file access, auth, crypto)
- [x] Error handling doesn't leak sensitive information

**Input Sources & Validation:**
- `query_scopes:` in schema.yaml (operator config, not secret). Validated at
load against the metamodel; invalid means a load error.
- `?query_scope=` selects a declared name only; unchanged.

**Security-Sensitive Operations:**
- Default state only: a named face or face-tailed edge never satisfies a
traversal (would disclose another world's content).
- Field gate cannot be skipped: the policy comes from the Request itself.
- Answers are per call; never on ctx, the resolver, or a cache.
- The traversal is an inference channel over the far entity's existence and
field values. Every hop is gated with `GateTraversal` (row gate, field gate,
client ceiling, face refusal, no inheritance expansion) per request, per
principal. Answers are never cached across requests or principals.
- Incoming hops add a slot-collision case in the gate; it fails closed.
- Errors name relations and properties only (config, not data).

## Test Plan

- [x] Test scenarios documented for each acceptance criterion
- [x] Edge cases identified and documented
- [x] Negative test cases defined (invalid input, error conditions)
- [x] Integration test approach defined (not just unit tests)

**Test Scenarios:**
- AC1: storetest inbound EndpointMatch cases (all backends); dataentry
integration test listing features through the scope.
- AC2: predicatefns resolution table + storetest chained inbound->outbound.
- AC3: predicatefns and scopes tests per error; real-schema test on
tickets/schema.yaml (`feature` -> `implementedBy`).
- AC4: acl tests for inbound hop, chained inbound hop, slot collision;
dataentry test with a hidden ticket.
- AC5: `storetest.Counting` budget test at 10 and 50 rows. Meaningful for
the SQL backends only: the naive `MatchingIDs` does per-candidate lookups below
the Counting wrapper.
- AC6: pgstore EXPLAIN on the inbound shape (done) and on the exact
`MatchingIDs` query with candidate ids.
- Two principals interleaved on one resolver get different answers.
- No-ACL deployment: traversal scope works ungated.
- Next-action count-zero hint with a `related()` default scope.
- Face: draft face / draft-tailed edge never matches (done, all backends).

**Edge Cases:**
- Entity with no incoming edges: no match.
- Dangling edge (FROM entity deleted): no match.
- Empty candidate set: no traversal query issued.
- Traversal under `not` / `or`: evaluated from the answer table, correct.
- Same traversal twice in one scope: answered once.
- Hot reload changes the scope: scopes are compiled per request call
(`viewQueryScope` calls the resolver func each time), so no stale program.

**Negative Tests:**
- Canonical name used from the TO side; inverse name from the FROM side.
- Symmetric relation; relation with no inverse used backwards.
- Program evaluated without a bound answer table: error, not false.

## Risk Assessment

- [x] Technical risks assessed with mitigations
- [x] Security risks assessed (see Security Considerations)
- [x] Effort estimated (xs/s/m/l/xl)

**Risks:**
- Resolution and index derivation drift: mitigated by one shared resolver
and the existing `_AgreesWithValidation` test extended to inverse hops.
- ACL slot collision on chained incoming hops: fail closed with a test.
- Large candidate sets in `MatchingIDs`: bounded by the scoped read that
produced them; measured with the Counting budget.

Effort: xl.

## Documentation Planning

- [x] User-facing docs identified (skip if internal refactor)
- [x] Docs-checklist will be created when entering implementation

**Documentation Impact:**
- [x] docs/metamodel.md: `related()` in `query_scopes:`, inverse IDs, limits.

## Design Review

- [x] Run `/design-review` before starting implementation
- [x] All critical/significant findings addressed in plan

**Design Review Findings:** 16 review-responses linked to TKT-CXQEV0 (1
critical, 6 significant, 7 minor, 2 nit). All are folded into the approach
above; the face finding is already fixed in code.

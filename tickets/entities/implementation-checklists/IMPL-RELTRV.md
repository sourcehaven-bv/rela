---
id: IMPL-RELTRV
type: implementation-checklist
title: 'Implementation: relation traversal in conditions'
status: done
---

## Development

- [x] Unit tests written for new code
- [x] Integration tests written (test full flow, not just units)
- [x] Happy path implemented
- [x] Edge cases from planning handled
- [x] Error handling in place (errors surfaced, not swallowed)

## Test Quality

- [x] Using fixture builders or factories for test data
- [x] No hardcoded values in assertions when object is in scope
- [x] Only specifying values that matter for the test
- [x] ~~Interpolated values constructed from objects, not hardcoded~~ (N/A:
      no string interpolation in these tests)
- [x] Property comparisons use original object, not hardcoded strings

## Manual Verification

- [x] Feature manually tested end-to-end
- [x] Each acceptance criterion verified with test scenario from planning
- [x] Edge cases manually verified

**Verification Evidence:**

*Backend parity.* `storetest.RunEndpointMatchTests` (7 scenarios) passes on
fsstore, memstore, sqlitestore and pgstore — the Go reference and the SQL
lowering agree on every case, including dangling edges and negation.

*Index behaviour, measured on PG 18.6.* A derived index is partial on
`jsonb_typeof(...) = 'string'`; PostgreSQL matches a partial index only when
the query implies its predicate, so an unguarded endpoint comparison is
correct but silently unindexed (bitmap scan, 4990 rows filtered, 1.725 ms vs
0.541 ms indexed). The lowering routes through `propCondOn` so the guard is
always emitted.

*Negative controls — the tests were verified to FAIL when the thing they
guard is removed:*

- ACL row gate deleted → `TestGateTraversal_DeniedWhenTargetTypeUnreadable`
  and `..._GatesEveryHopOfAChain` fail.
- Endpoint comparison hand-rolled without the scalar guard →
  `TestEndpointMatchExplainUsesDerivedIndex` fails.

Both were restored and re-verified green. A security test that cannot fail
proves nothing, so this was checked rather than assumed.

*End-to-end index loop.* The pgstore EXPLAIN test reconciles the spec
`queryplan.TraversalIndexSpecs` DERIVES (not a hand-written one) and asserts
the traversal query uses that index, so derivation and lowering cannot drift.

*Against the real schema.* `tickets/schema.yaml` loaded directly:
`related(entity, 'caused-by', { status = 'done' })` is refused with
"relation \"caused-by\" has 4 target types (concept, decision, ticket, bug);
add type='<one of them>'", and the ascribed form is accepted.

## Quality

- [x] Code follows project patterns (check similar code)
- [x] Checked for DRY opportunities — `propCond` was refactored to
      `propCondOn(alias)` so the endpoint filter reuses the ONE scalar
      spelling rather than duplicating it; that reuse is what keeps the
      partial index reachable, so it sharpens the contract rather than
      abstracting for its own sake.
- [x] No security issues introduced — the inference channel this feature
      opens is gated by `Request.GateTraversal` (row gate per hop, field
      gate on conditional `visible:`, three fail-closed refusals for
      shapes `EndpointPredicate` cannot express).
- [x] No silent failures (errors logged AND returned)
- [x] No debug code left behind

**Known gaps, deliberately deferred:** nothing calls `GateTraversal` /
`ValidateTraversals` yet — the layers are tested in isolation and wiring them
to a live surface (view `where:`, `--filter`) is the follow-up. Ordered
comparison, "all" semantics, and traversal on affordance surfaces are v2.

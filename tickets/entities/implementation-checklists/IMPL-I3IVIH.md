---
id: IMPL-I3IVIH
type: implementation-checklist
title: 'Implementation: Validation relation gates: a consumer-side graph seam, with direction and target-type filters'
status: done
---

<!-- @managed: claude-workflow v1 -->

## Development

- [x] Unit tests written for new code
- [x] Integration tests written (test full flow, not just units)
- [x] Happy path implemented
- [x] Edge cases from planning handled
- [x] Error handling in place (errors surfaced, not swallowed)

Three commits, in the order the plan specified so a regression in the move is
visible before any feature sits on top of it:

1. `7a82e318` — the seam. `validation.Graph` + `Related` declared at the
consumer; store-backed adapter in the new leaf package
`internal/validationgraph`; both entry points (`validator.New`,
`analysis.newValidationService`) wire it from the reader they already hold. No
behaviour change.
2. `6f2eb1e5` — `direction:` and `target_type:` on `RelationConstraint`, their
load-time validation, and docs.
3. `06db812d` — delete `lua.ReadDeps.OutgoingRelations`, now unreferenced.

## Test Quality

- [x] Using fixture builders or factories for test data
- [x] No hardcoded values in assertions when object is in scope
- [x] Only specifying values that matter for the test
- [x] Interpolated values constructed from objects, not hardcoded
- [x] Property comparisons use original object, not hardcoded strings

`atlasWorkspace` / `relationWorkspace` / `keysMeta` build the graph and
metamodel per case; tests name only the properties the case turns on.

The `validation` internal tests use a local `testGraph` rather than the real
adapter, deliberately: `validationgraph` imports `validation`, so the production
adapter is unreachable from an internal test there — and keeping them
independent means an adapter bug cannot hide by being both the code under test
and its own fixture. `relation_direction_test.go` is an EXTERNAL test (`package
validation_test`) and does use the real adapter, so the query shape is exercised
end to end.

## Manual Verification

- [x] Feature manually tested end-to-end
- [x] Each acceptance criterion verified with test scenario from planning
- [x] Edge cases manually verified

**Verification Evidence:**

*The seam changed no verdicts.* `rela analyze validations` over `tickets/`
(which exercises all 14 shipped gates) diffed byte-identical before and after,
with ticket data held constant so only the code differed — run again after
commits 2 and 3, still identical.

*The C4 guard actually guards.* Mutation-tested: swapping the adapter's query
from `EntityID` back to `From` (the mistake the review found — `Direction` is
honoured for `EntityID`/`EntityIDs` only, so `{From: id, Direction: Incoming}`
silently returns OUTGOING edges) fails exactly the two incoming subtests of
`TestRelatedEntities_DirectionSelectsTheEdgeSet`. Restored, green. A test that
only asserted "which endpoint did we read" would have passed against that bug,
which is why this one asserts the edge SET from both ends.

*The atlas case works.* `TestRelationConstraint_IncomingWithTargetType` models
`gaat_over` (from `taak` and `terugkerend`, to `procedure`) and covers: no
edges; an open taak (satisfied); a completed taak (violation); a `terugkerend`
alone (violation — the case a bare count cannot express); one of each (the taak
is what counts).

*Direction selects edges, not endpoints.*
`TestRelationConstraint_DirectionSelectsDifferentEdges` runs one graph three
ways — incoming sees the edge, outgoing sees nothing, omitted behaves as
outgoing.

*Fail-closed survived, including its new failure mode.* The shipped
`TestRelationConstraint_UnevaluableTargetFailsClosed` passes unmodified, and
`TestRelationConstraint_UnresolvedEdgeCountsUnderMaxWithTargetType` covers the
one the type filter introduces: an unresolved edge has no type to compare, so
skipping it "because it does not match" would undercount — which a `max:` bound
reads as success. It falls through to the bound instead.

*Load-time checks.* `TestValidateValidationRelations_NewKeys` covers bad
direction, direction-on-symmetric, undeclared target_type, unreachable
target_type (plus the same value being legal on the other side, proving
direction picks the side), a `where` property the target_type lacks, one that
only SOME reachable types declare (legal), and one that NONE declares (refused).
`TestValidateValidationRelations_TargetTypeAlias` covers M2.

*Gates.* `just arch-lint` clean (`internal/validation` still does not import
`internal/store`); `just lint` exit 0; `just comment-lint` gate clean (no
unresolvable doc links across 15172 comments); `just plimsoll` clean; `go test
./internal/...` no failures.

## Quality

- [x] Code follows project patterns (check similar code)
- [x] Checked for DRY opportunities — repeated literals, expressions, or
patterns extracted to a helper / constant / type where it sharpens the contract
(don't extract for its own sake; CLAUDE.md "three similar lines is better than a
premature abstraction" still holds)
- [x] No security issues introduced
- [x] No silent failures (errors logged AND returned)
- [x] No debug code left behind

Patterns followed: `validation.Graph` mirrors `acl.Graph` (consumer-declared,
store-backed at the wiring site, `NullGraph` for tests); the loader checks sit
inside the existing `validateValidationRelations` and use its message style;
`WithGraph` matches the existing `WithCache` option shape, which is what let all
four `validator.New` call sites stay untouched.

Extracted where it sharpened a contract: `farEndOf` and `directionOf` in the
adapter (picking the wrong end is invisible in the result, so the choice lives
in one place), `sameEntityType` (alias resolution needed on both sides),
`wherePropertyName` (a local scan rather than importing `internal/filter`, which
`metamodel` may not depend on). `RelationDirectionOutgoing` /
`RelationDirectionIncoming` are named constants rather than repeated literals.

Security: the gate still reads through whatever reader the wiring site passes,
so visibility is unchanged — a raw store handle would have silently turned a
visibility-scoped gate into a global one. `validationgraph.New` rejects a nil
reader, because a graph that counts nothing satisfies every `max:` gate.

No silent failures: a lookup error propagates and the constraint is reported as
a `LoadError` rather than counted as zero; an unreadable far entity comes back
`Resolved=false` rather than being dropped.

Comment correction also made here: the `failClosed` block read as a general
guarantee, but ACL-hidden edges are pruned upstream by `PolicyReader` and never
reach it. Added a cross-reference so the next reader does not infer a stronger
property than the read path delivers (design-review RR-YTRVNW).

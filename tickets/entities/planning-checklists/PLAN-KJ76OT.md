---
id: PLAN-KJ76OT
type: planning-checklist
title: 'Planning: Consolidate cardinality analyzers; stop swallowing CountRelations errors'
status: done
---

<!-- @managed: claude-workflow v1 -->

## Understanding

- [x] Problem/requirements clearly understood
- [x] Scope defined (what's in/out documented below)
- [x] Acceptance criteria documented with specific test scenarios

**Scope:**

IN scope:

1. Collapse `checkMinOutgoing`/`checkMaxOutgoing`/`checkMinIncoming`/
`checkMaxIncoming` (`internal/analysis/analysis.go:345-451` — the ticket's
`analysis.go:344-451` cites the pre-move tracer path; code now lives in
`internal/analysis`) into one parameterised check driven by a per-direction spec
(subject types, direction, min/max bounds, constraint labels, violation relation
label). One entity scan and one count per (relation, direction) instead of two
of each.
2. Fold `countOutgoingByType`/`countIncomingByType` (analysis.go:453-469)
into one `countRelations` helper that PROPAGATES the `store.CountRelations`
error instead of `n, _ :=` swallowing it.
3. Error policy (the documented decision): a store error fails the
cardinality run loudly — `CheckCardinality` gains an error return and aborts on
the first count failure with entity/relation context; it never reports a
violation computed from a failed count. Callers updated: `cli/analyze.go`
AnalyzeCardinalityCmd (return the error), `cli/validate.go` runCardinalityCheck
(propagate to the command error), `AnalyzeAll` (gains an error return; its one
CLI caller propagates).
4. Pin current behaviour with tests BEFORE refactoring (coverage is thin:
only min_outgoing + scope today).

OUT of scope:

- Any world/pointer concept (TKT-9KZGJO). The parameterisation is SHAPED
so a future world parameter changes subject population / counting scope /
violation identity in one place, but no world field/hook is added now.
- `collectEntities`'s log-and-return-partial behaviour on ListEntities
iteration errors (documented pre-existing under-count semantics shared by all
analyses; an entity missing from the scan cannot FABRICATE a violation —
different failure class from the count bug, and changing it touches every
analysis).
- The MCP server's separate cardinality implementation
(`internal/mcp/tools_analysis.go:68-146`) — a FIFTH copy with the same swallowed
`CountRelations` error. Deliberately kept against Store+Meta because mcp has no
analysis.Service dependency; unifying it is an architectural decision flagged to
the architect, not taken here.

**Acceptance Criteria:**

1. Identical violations for existing metamodels: same set, same ordering
(per relation: all min-then-max outgoing over From types in order, then
min-then-max incoming over To types), same constraint strings, same
incoming-label behaviour (inverse relation id when declared). Pinned by new
table tests written against the OLD code first.
2. `max_outgoing: 0` / `max_incoming: 0` remain meaningful (max nil-check
only), `min_*: 0` remains a skip — pinned by tests.
3. A failing `CountRelations` aborts the run with a wrapped error naming
the entity and relation; no violations are returned alongside it. Pinned by a
test using a store wrapper whose CountRelations fails.
4. `rela analyze cardinality`, `rela validate --check cardinality`, and
`rela analyze all` surface the error as a command failure (non-zero exit), not
as fabricated violations.

## Research

- [x] ~~For larger features: run `/research` to create a structured research doc~~ (N/A: refactor with fixed approach from design doc §12.5+§12.6; the parameterisation shape is the only free variable and is dictated by the four existing functions)
- [x] ~~Searched for existing libraries that solve this problem~~ (N/A: internal consolidation)
- [x] Checked codebase for similar patterns or reusable code
- [x] ~~Looked for reference implementations in other projects~~ (N/A)
- [x] Reviewed relevant rela concepts for prior art

**Research Doc:** N/A (feature-level context in RES-GFWP85/RES-NH3P12 for
FEAT-9CD2MX)

**Existing Solutions:**

- The four functions themselves (analysis.go:345-451) define the contract;
the MCP handler (`checkCardinalityBound`, tools_analysis.go:103) already
demonstrates a min+max-in-one-scan shape (though with different output and its
own swallowed errors).
- Error-propagation precedent: the package's documented under-count
logging (`FindOrphansWithScope` godoc) explains why other analyses swallow; the
ticket's rationale (fabricated violations, not just under-counts) is what
justifies cardinality diverging to fail-loud.

## Approach

- [x] Technical approach chosen and documented
- [x] Approach builds on existing patterns (not reinventing)
- [x] Alternatives considered (document why rejected)
- [x] Dependencies identified (packages, APIs, types)

**Technical Approach:**

Introduce a small direction spec derived per relation:

```go
type cardinalitySpec struct {
    direction     store.Direction // count direction
    subjectTypes  []string        // relDef.From (outgoing) / relDef.To (incoming)
    minBound, maxBound *int
    minConstraint, maxConstraint string // "min_outgoing"... labels
    relationLabel string          // relName; incoming uses inverse id if declared
}
```

`CheckCardinality` iterates relations, derives the outgoing and incoming specs,
and calls one `checkCardinality(ctx, relName, spec, scope)
([]CardinalityViolation, error)` that: skips when min is nil/0 AND max is nil;
scans each subject type once; counts each subject once via `countRelations`
(error → wrapped return); then emits min violations and max violations as two
passes over the cached counts — preserving the old grouped ordering exactly
while halving scans and counts.

The spec is the future world seam: subject population (subjectTypes → query),
counting scope (countRelations), and violation identity (the two emit sites)
each live in exactly one place. No world field is added.

Alternatives rejected:
- Interleaved min/max emission per entity (single pass): simpler but
changes violation ordering — behaviour must be identical.
- Accumulating all count errors before returning: more machinery for no
caller benefit; fail-fast matches "fail the run loudly".
- Keeping void signatures + logging the error: exactly the fabricated-
violation bug class the ticket exists to remove (a logged error still yields
count 0 downstream unless the subject is skipped, and a silently skipped subject
is an invisible under-count).

**Files to modify:**

- `internal/analysis/analysis.go` (consolidation + error returns)
- `internal/analysis/analysis_test.go` (behaviour-pinning table tests
first; then error-propagation tests with a failing-store wrapper)
- `internal/cli/analyze.go` (AnalyzeCardinalityCmd, AnalyzeAllCmd)
- `internal/cli/validate.go` (runCardinalityCheck)

## Security Considerations

- [x] Input sources identified (user input, config, external APIs)
- [x] Input validation approach defined (allowlist preferred over blocklist)
- [x] Security-sensitive operations identified (file access, auth, crypto)
- [x] Error handling doesn't leak sensitive information

**Input Sources & Validation:**

- Inputs are the operator's metamodel (already parsed/validated) and the
store. No new parsing, no user-supplied strings interpolated beyond
entity/relation ids already present in analysis output.

**Security-Sensitive Operations:**

- None new. The error message wraps the store error with entity id +
relation name — both already surfaced by the analyzers' normal output; no
credentials/paths beyond what the store error itself carries.

## Test Plan

- [x] Test scenarios documented for each acceptance criterion
- [x] Edge cases identified and documented
- [x] Negative test cases defined (invalid input, error conditions)
- [x] Integration test approach defined (not just unit tests)

**Test Scenarios:**

- AC1: table test over a metamodel exercising all four constraints at
once (min+max outgoing on one relation, min+max incoming with a declared inverse
on another), asserting the full ordered violation slice (ids, constraints,
labels, required/actual). Written and passing against the OLD code before the
refactor, unchanged after.
- AC2: cases for max_outgoing=0 (entity with one edge violates),
min_outgoing=0 (never violates), both-nil (no scan, no violations).
- AC3: failing-store wrapper (embeds store.Store, CountRelations returns
an injected error) → CheckCardinality returns nil violations + wrapped error
mentioning the entity id and relation; AnalyzeAll propagates.
- AC4: CLI behaviour covered by the signature change (compiler enforces
callers handle the error) + existing CLI tests still green.

**Edge Cases:**

- Relation with multiple From/To types (ordering across types).
- Entity type in From with zero entities.
- min set on a relation whose From type has entities but zero relations
(the classic violation).
- Inverse label only on incoming violations; outgoing keeps relName.
- Scope filtering unchanged (subjects filtered before counting — old code
counted only in-scope entities; preserve to keep store-call pattern).

**Negative Tests:**

- CountRelations error (AC3). ListEntities iteration error stays
log-and-partial (out of scope, documented).

## Risk Assessment

- [x] Technical risks assessed with mitigations
- [x] Security risks assessed (see Security Considerations)
- [x] Effort estimated (xs/s/m/l/xl)

**Risks:**

- Ordering regression → mitigated by pinning the full ordered slice
before refactoring (map iteration across relations is already nondeterministic;
ordering is only defined within one relation, which is exactly what the test
pins per-relation).
- Signature ripple (error returns) → bounded: three CLI call sites, all
already return error; compiler surfaces any missed caller.
- Behaviour drift in the min-0/max-0 asymmetry → dedicated test cases.

Effort: s (matches ticket property).

## Documentation Planning

For refactors: identify what documentation needs updating.

- [x] User-facing docs identified (skip if internal refactor)
- [x] Docs-checklist will be created when entering implementation

**Documentation Impact:**

- [x] ~~docs/metamodel.md~~ (N/A: constraint semantics unchanged)
- [x] ~~docs/cli-reference.md~~ (N/A: commands unchanged; a store outage now errors instead of printing false violations — behaviour users were never promised)
- [x] ~~docs/data-entry.md~~ (N/A)
- [x] ~~CLAUDE.md~~ (N/A: no new pattern)
- [x] ~~README.md~~ (N/A)
- Godoc on `CheckCardinality`/`cardinalitySpec` documents the error
policy and the one-place world seam (in-code docs, part of item 1).

## Design Review

- [x] Run `/design-review` before starting implementation
- [x] All critical/significant findings addressed in plan (none found)

**Design Review Findings:**

No critical/significant findings; no review-response entities created. Two notes
folded into the plan:
- (minor) `validate --format json` must not half-emit a cardinality
result before the error path returns — error handling goes before any output
write in `runCardinalityCheck`.
- (nit) The ticket's cited location (`analysis.go:344-451`, tracer)
predates the package move; verified 2026-08-19 the code lives in
`internal/analysis/analysis.go:345-469`. Noted on the ticket body. Architectural
flag (out of scope, reported to architect): the MCP server carries a FIFTH
cardinality implementation with the same swallowed CountRelations error
(`internal/mcp/tools_analysis.go:68-146`).

---
id: PLAN-L4581J
type: planning-checklist
title: 'Planning: MCP analyze_cardinality: delete the fifth copy, call the consolidated analysis service'
status: done
---

<!-- @managed: claude-workflow v1 -->

## Understanding

- [x] Problem/requirements clearly understood
- [x] Scope defined (what's in/out documented below)
- [x] Acceptance criteria documented with specific test scenarios

**Scope:**

IN: delete `checkCardinalityBound` / `checkCardinalityForRelation` from
`internal/mcp/tools_analysis.go`; make one cardinality implementation serve
both the MCP tool and the three CLI surfaces; MCP tests for the two bugs and
the wording change.

OUT: world-awareness itself (TKT-9KZGJO, Step 5) — this ticket only ensures
that work finds ONE implementation. Also out: narrowing `analysis.Deps.Store`,
and the other four MCP-local analyses (`analyze_unique`, `analyze_properties`)
which have the same shape but are not this ticket.

**Acceptance Criteria:**

1. `internal/mcp` has no hand-rolled cardinality scan — verified by the
   deleted functions being absent and the handler calling the shared one.
2. A failing `CountRelations` fails the MCP tool call and reports NO
   violations — `TestHandleAnalyzeCardinality_CountErrorFailsTheToolCall`.
3. A truncated `ListEntities` scan leaves a diagnosable trace instead of
   passing silently — `TestHandleAnalyzeCardinality_TruncatedScanIsLogged`.
4. MCP and CLI report the same violations and the same wording —
   `TestHandleAnalyzeCardinality_IncomingUsesInverseLabel`.
5. No arch-lint change required.

## Research

- [x] ~~Run `/research`~~ (N/A: small refactor, approach settled by reading)
- [x] ~~Searched for existing libraries~~ (N/A: internal refactor)
- [x] Checked codebase for similar patterns or reusable code
- [x] Looked for reference implementations in other projects
- [x] Reviewed relevant rela concepts for prior art

**Research Doc:** N/A — effort `s`, no open design question once the
arch-lint constraint was established.

**Existing Solutions:**

The decisive prior art is in `internal/schema` itself. `RelationLister`
(`validate_properties.go:63`) is the same shape this needed: a narrow
consumer-side reader declared at the call site, with a godoc saying it exists
so an ACL-scoped reader can be passed. `schema.NewStoreCounter` and
`schema.ValidateRelationProperties` are already called from
`internal/mcp/tools_analysis.go` with the gated `Deps.Store`, so the pattern
was established on this exact file.

`validationgraph.Reader` (2 methods) is the same idea in another package.

## Approach

- [x] Technical approach chosen and documented
- [x] Approach builds on existing patterns (not reinventing)
- [x] Alternatives considered (document why rejected)
- [x] Dependencies identified (packages, APIs, types)

**Technical Approach:**

Move the consolidated checker into `internal/schema` as a free function over
a narrow reader, and make `analysis.CheckCardinality` a one-line wrapper.

    schema.CheckCardinality(ctx, r CardinalityReader,
        meta *metamodel.Metamodel, scope map[string]bool)

`CardinalityReader` is `ListEntities` + `CountRelations` — exactly what the
check calls, and a subset of both `store.Store` and MCP's existing
`GraphReader`, so both callers pass what they already hold.
`analysis.CardinalityViolation` becomes a type alias so CLI surfaces need no
schema import and keep compiling unchanged.

**Alternatives considered:**

*Add `analysis` to `mcp.mayDependOn` and narrow `analysis.Deps.Store`*
— rejected. It is closer to the ticket's literal wording but worse on two
counts. It widens MCP's transitive dependency surface to `lua`, `validation`
and `validationgraph`, none of which the MCP tool surface has any business
holding, and it is defeated by a detail: `store.ListEntityHeaders` takes the
full 7-method `store.EntityReader`, so narrowing `analysis.Deps.Store` would
also force a signature change on shared `store` API used well outside this
ticket. Moving the checker to a package both consumers already depend on
avoids all of it and needs no arch-lint edit.

*Keep MCP's existing message wording* — rejected. It re-introduces the
formatting fork this ticket exists to close.

**Files to modify:**

- `internal/schema/cardinality.go` (new) — the implementation.
- `internal/analysis/analysis.go` — wrapper + type alias; ~180 lines deleted.
- `internal/mcp/tools_analysis.go` — handler calls the shared check;
  `checkCardinalityBound` / `checkCardinalityForRelation` deleted.
- `internal/mcp/tools_cardinality_test.go` (new) — the three tests.

## Security Considerations

- [x] Input sources identified (user input, config, external APIs)
- [x] Input validation approach defined (allowlist preferred over blocklist)
- [x] Security-sensitive operations identified (file access, auth, crypto)
- [x] Error handling doesn't leak sensitive information

**Input Sources & Validation:**

The tool takes no arguments, so there is no caller-supplied input to validate.
The metamodel is operator-authored config, which per CLAUDE.md is not secret.

**Security-Sensitive Operations:**

The one that matters is the READ GATE, and the approach preserves it. MCP
reads through `Deps.Store`, a visibility-wrapped `GraphReader`, not a raw
store. Declaring `CardinalityReader` at the call site is what keeps that
true: the check sees exactly the rows the caller's reader yields, so a gated
wiring gets a gated scan. Injecting an `analysis.Service` would have meant
handing MCP a service built over the RAW store, silently widening the read —
which is the substantive reason the rejected alternative was rejected, beyond
the dependency argument.

Counts stay ungated, unchanged from before and consistent with the line
`GraphCounter` already draws (a count is structural, it names no row).

Error text carries store errors to the caller. The MCP transport is local
stdio where the filesystem is the trust boundary, and the same errors already
reach `rela analyze cardinality` on the terminal.

## Test Plan

- [x] Test scenarios documented for each acceptance criterion
- [x] Edge cases identified and documented
- [x] Negative test cases defined (invalid input, error conditions)
- [x] Integration test approach defined (not just unit tests)

**Test Scenarios:**

| Criterion | Test |
|---|---|
| 2 (fabricated violations) | `..._CountErrorFailsTheToolCall` |
| 3 (silent truncation) | `..._TruncatedScanIsLogged` |
| 4 (CLI/MCP parity) | `..._IncomingUsesInverseLabel` |
| 1, 5 | the deletion itself; `just arch-lint` |

The seven existing `TestCheckCardinality_*` tests in `internal/analysis`
(ordering, labels, bound edge cases, multiple source types, min/max grouping,
count-error, per-face counting) run unchanged against the moved code and are
the regression net for the move.

Each new test was verified to FAIL against the pre-change handler — otherwise
it pins nothing. All three do.

**Edge Cases:**

- Relation with neither bound set — skipped without a scan (early return).
- `max: 0` — forbids any edge; distinguished from nil by the pointer.
- Faced entity with a content-scoped edge — counted per face, so a draft's
  edge does not satisfy the published face's bound.
- Incoming bound on a relation with no declared inverse — falls back to the
  forward name.
- Empty graph / no relations declared — returns a non-nil empty slice so JSON
  callers serialize `[]`, not `null`.

**Negative Tests:**

A failing `CountRelations` must produce an error result AND no violations —
the test asserts both, since asserting only the error would still pass an
implementation that reported fabricated violations alongside it.

## Risk Assessment

- [x] Technical risks assessed with mitigations
- [x] Security risks assessed (see Security Considerations)
- [x] Effort estimated (xs/s/m/l/xl)

**Risks:**

*MCP output changes shape for incoming violations.* Accepted and documented
in the handler godoc. It is a fix — the inverse id is the name the subject
entity's operator uses — and it makes MCP agree with the CLI. Low blast
radius: the tool returns prose for an agent to read, not a stable API.

*Faced entities newly appear as subjects* (the scan is now `AllStates`). This
can surface violations the MCP tool previously missed. That is the TKT-4Y6CMV
correctness fix, already live on the CLI; MCP was the outlier.

*The moved code loses its `internal/analysis` test coverage.* Mitigated: the
analysis tests call through the wrapper, so they still exercise every line —
verified green after the move.

Effort: s (as estimated).

## Documentation Planning

- [x] User-facing docs identified (skip if internal refactor)
- [x] Docs-checklist will be created when entering implementation

**Documentation Impact:**

- [x] N/A - Internal refactor. The two behaviour changes are MCP tool output
  wording and which subjects are scanned; neither is documented in
  `docs/` (the MCP tool list does not reproduce violation message formats).
  Rationale is captured in the godoc on `schema.CardinalityReader` and
  `handleAnalyzeCardinality`, which is where the next person looks.

## Design Review

- [x] ~~Run `/design-review` before starting implementation~~ (N/A: effort `s` refactor; the two load-bearing decisions were confirmed with the user directly before any code was written)
- [x] All critical/significant findings addressed in plan

**Design Review Findings:** the two load-bearing decisions (where the
implementation lives; adopting the shared wording) were put to the user
before implementation and both were confirmed. No separate `/design-review`
run for a refactor of this size.

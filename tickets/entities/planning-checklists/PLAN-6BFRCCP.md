---
id: PLAN-6BFRCCP
type: planning-checklist
title: 'Planning: Docs: query scopes with not/or lower into the store'
started: "2026-11-24"
completed: "2026-11-24"
status: done
---

<!-- @managed: claude-workflow v1 -->

## Understanding

- [x] Problem/requirements clearly understood
- [x] Scope defined (what's in/out documented below)
- [x] Acceptance criteria documented with specific test scenarios

**Scope:** In: correct the "A list page answers the whole scope in the store
query" text in GUIDE-metamodel and regenerate docs/metamodel.md. Out: code.
Lowering `not related(...)` into the store already shipped:
`queryplan.LowerCondition` (TKT-I39Y7N4, TKT-8HY05WB) lowers `not` over
traversals to `RelationPredicate.Negate` / `CondNot`, pgstore and sqlitestore
render `NOT EXISTS` (`existsCond`), and `storetest.RunGraphDifferential` already
draws `Negate` relations and `NotCond(RelatedCond)` trees against
graphquerynaive on both backends. `LowerScope` (visible:) is untouched.

**Acceptance Criteria:**
1. The guide no longer says `not`/`or` scopes run after the store read; verified by reading the rendered docs/metamodel.md.
2. `just docs-check` is clean after commit.

## Research

- [x] ~~For larger features: run `/research`~~ (N/A: docs correction)
- [x] ~~Searched for existing libraries~~ (N/A: docs correction)
- [x] Checked codebase for similar patterns or reusable code
- [x] ~~Looked for reference implementations in other projects~~ (N/A: docs correction)
- [x] Reviewed relevant rela concepts for prior art

**Research Doc:** N/A

**Existing Solutions:** internal/queryplan/conditionlower.go (LowerCondition),
internal/appbuild/conditionlower.go (lowerQueryCondition gates negated
traversals), internal/store/pgstore/graphquery.go existsCond,
internal/store/storetest/graphdiff.go genRelation/genCond,
internal/dataentry/listcondition_pushdown_test.go and listpushdown_scope_test.go
pin pushdown of `not related(...)`.

## Approach

- [x] Technical approach chosen and documented
- [x] Approach builds on existing patterns (not reinventing)
- [x] Alternatives considered (document why rejected)
- [x] Dependencies identified (packages, APIs, types)

**Technical Approach:** Rewrite the paragraph to list what LowerCondition
admits, add an Inbox `condition: not related(...)` example, state NOT EXISTS and
the shared row gating, then `just docs`. Alternative of reimplementing the
lowering rejected: it exists and is tested.

**Files to modify:** docs-project/entities/guides/GUIDE-metamodel.md,
docs/metamodel.md (generated).

## Security Considerations

- [x] Input sources identified (user input, config, external APIs)
- [x] ~~Input validation approach defined~~ (N/A: no code change)
- [x] ~~Security-sensitive operations identified~~ (N/A: no code change)
- [x] ~~Error handling doesn't leak sensitive information~~ (N/A: no code change)

**Input Sources & Validation:** Threat model: none; the text only describes
existing behavior. It must not overstate gating: it states a hidden entity does
not stop a row matching `not related(...)`, which matches the existing
"Filtering on related entities" section.

**Security-Sensitive Operations:** None.

## Test Plan

- [x] Test scenarios documented for each acceptance criterion
- [x] ~~Edge cases identified and documented~~ (N/A: docs correction; behavior pinned by existing tests)
- [x] ~~Negative test cases defined~~ (N/A: docs correction)
- [x] ~~Integration test approach defined~~ (N/A: docs correction)

**Test Scenarios:** just docs; just docs-check; CI docs job.

## Risk Assessment

- [x] Technical risks assessed with mitigations
- [x] Security risks assessed (see Security Considerations)
- [x] Effort estimated (xs/s/m/l/xl)

**Risks:** Text drifting from LowerCondition's admitted set; mitigated by citing
only shapes its godoc lists. Effort xs.

## Documentation Planning

- [x] User-facing docs identified (skip if internal refactor)
- [x] ~~Docs-checklist will be created when entering implementation~~ (N/A: the ticket is the docs change)

**Documentation Impact:** docs/metamodel.md.

## Design Review

- [x] Run `/design-review` before starting implementation
- [x] All critical/significant findings addressed in plan

**Design Review Findings:** None critical or significant; one nit (illustrative
relation name) accepted.

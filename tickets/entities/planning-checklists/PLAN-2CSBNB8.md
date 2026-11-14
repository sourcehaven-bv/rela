---
id: PLAN-2CSBNB8
type: planning-checklist
title: 'Planning: Push list conditions and non-lowering scopes into the store'
started: "2026-11-14"
completed: "2026-11-14"
status: done
---

<!-- @managed: claude-workflow v1 -->

## Understanding

- [x] Problem/requirements clearly understood
- [x] Scope defined (what's in/out documented below)
- [x] Acceptance criteria documented with specific test scenarios

**Scope:**

In scope: list `condition:` pushdown on the list API (and so nav items and
dashboard cards that page through `listPage`), nav status rule counts in the
store, and list query scopes lowered by a new `queryplan.LowerCondition` (`~=`,
memberships, `not related(...)`).

Out of scope: ordered comparisons (`<`, `>=`, ...), `or`, `not` over anything
but a traversal, kanban and export conditions, `visible:` lowering
(`queryplan.LowerScope` is unchanged).

**Acceptance Criteria:**

1. A list whose condition lowers and reads only fields visible on every row is
counted with `CountMatched` and paged in the store; rows equal the Go path.
2. A condition reading a field hidden from some row stays on the Go path.
3. Nav status rules that lower are one scoped count each.
4. The four calls-only budget tests become rows plus calls and fail on the old
path.
5. `LowerScope` refuses everything it refused before.

## Research

- [x] ~~For larger features: run `/research`~~ (N/A: the approach follows the existing scope pushdown of TKT-XKCNCL)
- [x] ~~Searched for existing libraries~~ (N/A: internal query lowering)
- [x] Checked codebase for similar patterns or reusable code
- [x] ~~Looked for reference implementations in other projects~~ (N/A: internal query lowering)
- [x] Reviewed relevant rela concepts for prior art

**Research Doc:** N/A

**Existing Solutions:**

- `queryplan.LowerScope` and `QueryScopeResolver.Lower` (TKT-XKCNCL): the
model for gating each traversal per request.
- `predicate.Program.Conjunction` and `ConditionPrefilters`: the conjunct walk
the new lowering widens.
- `gatedListQuery` / `FieldVisibleOnEveryRow`: the rule for when a field can be
judged in the store for a principal.

## Approach

- [x] Technical approach chosen and documented
- [x] Approach builds on existing patterns (not reinventing)
- [x] Alternatives considered (document why rejected)
- [x] Dependencies identified (packages, APIs, types)

**Technical Approach:**

- `predicate.Program.ConditionConjunction` walks the AND tree and also admits
`~=` and `not related(...)`.
- `queryplan.LowerCondition` turns that into `store.PropPredicate`s and
traversal terms, refusing type mismatches (date properties, list vs scalar).
- `appbuild.lowerQueryCondition` gates each traversal per request: a denied
positive term empties the page, a denied negated term drops out, a refused term
falls back to the Go path.
- `listNarrowing.pushdownPlan` lowers the scope, then the condition, through
`lowerViewCondition`, which checks every attribute with `FieldVisibleOnEveryRow`
first.
- `navStatusInStore` counts each rule on the narrowed plan.

Alternative rejected: widening `LowerScope`. `visible:` relies on it refusing
`not` and `~=`.

**Files to modify:** internal/predicate/prefilter.go,
internal/queryplan/conditionlower.go, internal/appbuild/conditionlower.go,
internal/appbuild/queryscopes.go, internal/appbuild/viewconditions.go,
internal/dataentry/listpushdown.go, internal/dataentry/nav_status.go, tests.

## Security Considerations

- [x] Input sources identified (user input, config, external APIs)
- [x] Input validation approach defined (allowlist preferred over blocklist)
- [x] Security-sensitive operations identified (file access, auth, crypto)
- [x] Error handling doesn't leak sensitive information

**Input Sources & Validation:**

Conditions come from operator config, compiled at load. Only an allowlist of
node shapes lowers; anything else keeps the Go path.

**Security-Sensitive Operations:**

Row gating stays the principal's compiled read query; the condition only ANDs
onto it. Traversals are gated per request with the request's traversal gate. A
field hidden from some row is never judged in the store, so the store cannot
reveal a hidden value through which rows match. Totals are the scoped count
after the gate.

## Test Plan

- [x] Test scenarios documented for each acceptance criterion
- [x] Edge cases identified and documented
- [x] Negative test cases defined (invalid input, error conditions)
- [x] Integration test approach defined (not just unit tests)

**Test Scenarios:**

1. `TestListPushdown_ConditionMatchesGoPath` (memstore, sqlite, postgres):
rows, totals and the path taken, per principal and list.
2. Same test, principal frank with status hidden.
3. `TestQueryBudget_NavStatusInStoreIsSizeIndependent`.
4. The converted budget tests, run once with the lowering disabled.
5. `TestLowerCondition_DoesNotWidenLowerScope`.

**Edge Cases:** empty-string assignee, unset watchers, empty watcher list,
denied negated traversal, refused traversal, gate returning a negated predicate,
duplicate traversal terms, no identity for `current_user`.

**Negative Tests:** `or` stays on the Go path; ordered comparison and date
equality refuse to lower; refused traversal falls back.

## Risk Assessment

- [x] Technical risks assessed with mitigations
- [x] Security risks assessed (see Security Considerations)
- [x] Effort estimated (xs/s/m/l/xl)

**Risks:**

- A store/Go semantic mismatch (nil, list vs scalar, case): mitigated by the
differential test on all three backends and by refusing ambiguous types.
- Effort: m.

## Documentation Planning

- [x] User-facing docs identified (skip if internal refactor)
- [x] ~~Docs-checklist will be created when entering implementation~~ (N/A: no user-facing behaviour change; rows are unchanged)

**Documentation Impact:** `.claude/rules/collection-reads.md` only.

## Design Review

- [x] ~~Run `/design-review` before starting implementation~~ (N/A: the approach mirrors the reviewed TKT-XKCNCL scope pushdown; cranky-code-reviewer and rela-security-reviewer review the diff at review)
- [x] ~~All critical/significant findings addressed in plan~~ (N/A: no design review run)

**Design Review Findings:** N/A

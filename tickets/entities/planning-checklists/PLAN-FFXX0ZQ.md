---
id: PLAN-FFXX0ZQ
type: planning-checklist
title: 'Planning: Push desktop notification conditions into the store'
started: "2026-11-14"
completed: "2026-11-14"
status: done
---

<!-- @managed: claude-workflow v1 -->

## Understanding

- [x] Problem/requirements clearly understood
- [x] Scope defined (what's in/out documented below)
- [x] Acceptance criteria documented with specific test scenarios

**Scope:** In: `desktopnotify.Config.Evaluate` answers a condition that
`queryplan.LowerScope` lowers exactly in the store (capped `GraphQueryHeaders`
per rule, `store.CountMatched` for the badge); other conditions push
`queryplan.ConditionPrefilters`. Rules report at most `MaxMatchesPerRule`
matches; the tracker keeps ids past the cap. Out: lowering `~=` and ordered
comparisons against `today()` (TKT-I3MGCRT, overlaps TKT-I39Y7N4). Store
changes.

**Acceptance Criteria:**
1. A lowered rule and badge cost the same calls and rows at 100 and 1,000 padding rows (`TestEvaluate_LoweredConditionIsScaleFree`).
2. A non-lowering condition reads only rows passing its pushed equalities (`TestEvaluate_PrefilteredConditionSkipsExcludedRows`).
3. Matches and badge are unchanged for existing configs (`TestEvaluate_MatchesAndBadge`, `TestTracker`).
4. A rule over the cap reports the lowest 100 ids and is listed in `Truncated`; the tracker does not re-notify ids past the cap (`TestEvaluate_CapsMatchesPerRule`, `TestTracker_TruncatedRuleKeepsIDsPastCap`).
5. The `Config.Evaluate` allowlist entry is gone; only `evaluation.scan` remains as debt.

## Research

- [x] ~~For larger features: run `/research` to create a structured research doc~~ (N/A: small change on existing pushdown patterns)
- [x] ~~Searched for existing libraries that solve this problem~~ (N/A: internal store pushdown)
- [x] Checked codebase for similar patterns or reusable code
- [x] ~~Looked for reference implementations in other projects~~ (N/A: internal pattern)
- [x] Reviewed relevant rela concepts for prior art

**Research Doc:** N/A

**Existing Solutions:** `queryplan.LowerScope` (exact lowering, used by
`appbuild/queryscopes.go` and `affordances/searchscope.go`),
`queryplan.ConditionPrefilters` (superset equalities), `store.GraphQueryHeaders`
and `store.CountMatched`, `storetest.AssertScaleFree`.

## Approach

- [x] Technical approach chosen and documented
- [x] Approach builds on existing patterns (not reinventing)
- [x] Alternatives considered (document why rejected)
- [x] Dependencies identified (packages, APIs, types)

**Technical Approach:** Lower each condition once at load. Exact: rule reads
`GraphQueryHeaders` with `Limit: MaxMatchesPerRule+1` in the world; badge uses
`CountMatched`. Not exact: one `GraphQueryHeaders` read per (type, prefilters)
with the prefilters pushed, Go evaluation over the result. Evaluate takes
`store.GraphQueryer`. Alternatives rejected: widening `LowerScope` for ordered
comparisons (it also gates `visible:` policies, where ordered comparisons are
refused); paging the Go scan with Limit/Offset (cost unchanged, only hides it
from archguard).

**Files to modify:** internal/desktopnotify/{config,evaluate,tracker,doc}.go,
cmd/rela-desktop/notify.go, internal/archguard/unpagedread_allowlist_test.go,
.go-arch-lint.yml, docs-project/entities/guides/GUIDE-desktop.md.

## Security Considerations

- [x] Input sources identified (user input, config, external APIs)
- [x] Input validation approach defined (allowlist preferred over blocklist)
- [x] Security-sensitive operations identified (file access, auth, crypto)
- [x] Error handling doesn't leak sensitive information

**Input Sources & Validation:** desktop.yaml (operator config), already compiled
and validated by `Load`. Lowered predicates come from the compiled program
through the metamodel gate in `LowerScope`.

**Security-Sensitive Operations:** None new. The desktop is single-user with no
read ACL; reads stay world-scoped as before.

## Test Plan

- [x] Test scenarios documented for each acceptance criterion
- [x] Edge cases identified and documented
- [x] Negative test cases defined (invalid input, error conditions)
- [x] Integration test approach defined (not just unit tests)

**Test Scenarios:** See acceptance criteria. `TestLower` pins which conditions
lower.

**Edge Cases:** Exactly at the cap (not truncated), over the cap, insertion
order differing from id order, `or`/`~=`/`true` conditions (not exact, no
prefilter).

**Negative Tests:** Existing `TestLoad_Errors` and nil-argument tests still
apply.

## Risk Assessment

- [x] Technical risks assessed with mitigations
- [x] Security risks assessed (see Security Considerations)
- [x] Effort estimated (xs/s/m/l/xl)

**Risks:** Lowering must be exact or matches change: mitigated by using
`LowerScope`, whose exactness contract is pinned by its own tests. The cap
changes behaviour for rules with over 100 matches: documented, and the tracker
keeps ids past the cap to avoid repeat notifications. Effort: s.

## Documentation Planning

- [x] User-facing docs identified (skip if internal refactor)
- [x] Docs-checklist will be created when entering implementation

**Documentation Impact:** GUIDE-desktop: the 100-match cap and which conditions
the database answers.

## Design Review

- [x] ~~Run `/design-review` before starting implementation~~ (N/A: small change; cranky-code-reviewer and rela-security-reviewer review the diff instead)
- [x] All critical/significant findings addressed in plan

**Design Review Findings:** None (see code review responses).

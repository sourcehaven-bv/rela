---
id: PLAN-BN5O42C
type: planning-checklist
title: 'Planning: Lower ordered and negated desktop notification conditions'
started: "2026-11-22"
status: done
---

<!-- @managed: claude-workflow v1 -->

## Understanding

- [x] Problem/requirements clearly understood
- [x] Scope defined (what's in/out documented below)
- [x] Acceptance criteria documented with specific test scenarios

**Scope:**

In: desktop.yaml conditions lower through `queryplan.LowerCondition` (the
store answers `or`, `not`, `~=`, ordered string and integer comparisons). The
Go scan stays only for conditions that still do not lower, mainly date
comparisons against `today()`; its read is narrowed by every top-level
conjunct that does lower. The evaluator switches to `LoweringSemantics` so the
Go path and the store agree.

Out: lowering dates against `today()` (TKT-4Z2BIRG). Making an unlowerable
condition a load error. Removing the scan (follow-up ticket after
TKT-4Z2BIRG).

**Acceptance Criteria:**
1. `or`, `not`, `~=` and ordered string/integer conditions are exact: the rule
   reads at most MaxMatchesPerRule+1 rows and the badge is one count
   (budget test with `storetest.AssertScaleFree`).
2. For those conditions the store returns the same rows as the Go evaluator,
   on memstore, fs and sqlite (postgres in CI if no local server).
3. A date condition with a lowerable conjunct (`entity.status ~= 'done' and
   entity.vervaldatum <= today()`) reads only rows passing `~= 'done'`.
4. Docs describe which conditions the database answers.

## Research

- [x] ~~For larger features: run `/research` to create a structured research doc~~ (N/A: the lowering exists; this wires it in)
- [x] ~~Searched for existing libraries that solve this problem~~ (N/A: in-repo lowering)
- [x] Checked codebase for similar patterns or reusable code
- [x] ~~Looked for reference implementations in other projects~~ (N/A: in-repo pattern)
- [x] Reviewed relevant rela concepts for prior art

**Research Doc:** N/A

**Existing Solutions:**
- `queryplan.LowerCondition` (TKT-8HY05WB) and its differential test
  `internal/queryplan/conditiondiff_test.go`.
- `appbuild.lowerQueryCondition`: the data-entry caller, with
  `LoweringSemantics` evaluators in `appbuild/queryscopes.go`.

## Approach

- [x] Technical approach chosen and documented
- [x] Approach builds on existing patterns (not reinventing)
- [x] Alternatives considered (document why rejected)
- [x] Dependencies identified (packages, APIs, types)

**Technical Approach:**
- `Config.lower` calls `queryplan.LowerCondition` with an empty identity. A
  result with no traversals is exact; its `Where` converts to a
  `store.Cond` (no relation leaves can occur, since related() is refused at
  compile). The rule query and the badge count carry Props and Where.
- Otherwise the scan is narrowed by a new
  `queryplan.LowerConditionConjuncts`: the top-level AND conjuncts that lower
  exactly, whose AND the condition implies. It replaces
  `ConditionPrefilters` here (a superset of what that pushed). Needs a
  `predicate.Program.ConditionConjuncts` that reads each top-level conjunct
  as a `CondTree` and skips the ones that do not read.
- The evaluator becomes `NewEvaluatorWithClock(...).LoweringSemantics()`:
  an ordered comparison on an unset value is false, not an error, so a
  lowered `not (entity.n < 3)` matches the same rows in Go and in the store.
- `scanKey` includes the Where tree.

Alternatives: widening `LowerScope` (rejected: `visible:` depends on it
refusing these shapes); keeping `ConditionPrefilters` for the scan (rejected:
it pushes only equalities, so the documented example reads every row).

**Files to modify:**
internal/desktopnotify/{config,evaluate}.go and tests;
internal/queryplan/conditionlower.go; internal/predicate/condtree.go;
internal/archguard/unpagedread_allowlist_test.go;
docs-project/entities/guides/GUIDE-desktop.md.

## Security Considerations

- [x] Input sources identified (user input, config, external APIs)
- [x] Input validation approach defined (allowlist preferred over blocklist)
- [x] Security-sensitive operations identified (file access, auth, crypto)
- [x] Error handling doesn't leak sensitive information

**Input Sources & Validation:**
desktop.yaml, operator-authored. Conditions compile through the predicate
engine; lowering admits an allowlist of shapes and declines everything else.

**Security-Sensitive Operations:**
None new. The desktop is single-user with no read ACL; it reads the raw store
(`cmd/rela-desktop/notify.go`), so the `visible:` rewrite of PR #2518 does
not apply.

## Test Plan

- [x] Test scenarios documented for each acceptance criterion
- [x] Edge cases identified and documented
- [x] Negative test cases defined (invalid input, error conditions)
- [x] Integration test approach defined (not just unit tests)

**Test Scenarios:**
- Budget: `AssertScaleFree` for an `or`/`~=`/ordered condition, cost one
  capped query plus one count.
- Differential: conditions over status (string) and an integer property,
  evaluated through Evaluate on memstore, fsstore and sqlite, against the
  Go evaluator run on every row.
- Narrowed scan: budget test for `~= 'done' and date <= today()`.
- Superset differential in queryplan (RR-CK9WBAK) and Cond-limit test (RR-YECEECW).
- TestLower table updated.

**Edge Cases:**
Unset and empty values; `nil ~= v`; ordered comparison on unset; integer
stored as string; `not` over an ordered leaf.

**Negative Tests:**
Date conditions stay on the Go path (exact=false).

## Risk Assessment

- [x] Technical risks assessed with mitigations
- [x] Security risks assessed (see Security Considerations)
- [x] Effort estimated (xs/s/m/l/xl)

**Risks:**
- Go and store disagree on a lowered shape: mitigated by LoweringSemantics
  and the cross-backend differential.
- LoweringSemantics changes the Go path for unlowered conditions: an
  ordered comparison of an unset string or integer is false, not an error,
  so `entity.n < 3 or <date>` can now match on its date part. Unset dates
  against today() still error. Pinned by
  TestEvaluate_UnsetOrderedComparisonIsFalse and stated in GUIDE-desktop.

Effort: m.

## Documentation Planning

- [x] User-facing docs identified (skip if internal refactor)
- [x] Docs-checklist will be created when entering implementation

**Documentation Impact:**
GUIDE-desktop "Conditions" section.

## Design Review

- [x] Run `/design-review` before starting implementation
- [x] All critical/significant findings addressed in plan

**Design Review Findings:** RR-YECEECW, RR-CK9WBAK (significant, addressed); RR-8D68YA8, RR-BNOW76W, RR-F8GQHND (minor, addressed); RR-EG4DG3G, RR-NN5FC68 (nit, addressed); RR-G2HVBJ3 and the De Morgan nit (wont-fix).

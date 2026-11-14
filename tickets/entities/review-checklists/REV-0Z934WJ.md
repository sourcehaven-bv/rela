---
id: REV-0Z934WJ
type: review-checklist
title: 'Review: Push list conditions and non-lowering scopes into the store'
started: "2026-11-14"
completed: "2026-11-14"
status: done
---

<!-- @managed: claude-workflow v1 -->

## Automated Checks

- [x] All tests pass (touched packages only, with -race: predicate, queryplan, appbuild, dataentry, graphquerynaive, memstore, storetest, sqlitestore -tags sqlite, pgstore and dataentry postgres tests on PG16; TestGoldenParity on default and sqlite tags)
- [x] Lint clean (golangci-lint serially on the changed packages)
- [x] Comment lint gate clean (`just comment-lint`)
- [x] ~~Coverage maintained (`just coverage-check`)~~ (N/A: needs a repo-wide test run, which this machine is not allowed; every changed package gained tests and CI enforces the floors)

**Comment findings.** `just comment-lint` is clean; `just arch-lint` and
plimsoll on the touched packages pass.

## Code Review

- [x] Run `/code-review` command (cranky-code-reviewer and rela-security-reviewer)
- [x] All critical review-responses addressed (none raised)
- [x] All significant review-responses addressed (RR-DLNBH0E, RR-WKIE108)
- [x] Self-reviewed the diff for unrelated changes

**Review Responses:** RR-P3B1V15, RR-RG4O28T, RR-WKIE108, RR-SQI8GTF (wont-fix),
RR-2O56DAS, RR-S7M4O5A, RR-DLNBH0E, RR-LAMI7RP, RR-PAJ4TI1, RR-65XJ4M1

## Acceptance Verification

- [x] Each acceptance criterion tested (reference planning checklist PLAN-2CSBNB8)
- [x] Test evidence documented in implementation checklist (IMPL-73G6E26)

**Acceptance Status:**

- List conditions and the scopes LowerScope declines are pushed into the store: PASS (TestListPushdown_ConditionMatchesGoPath asserts one CountMatched and no MatchingFaces on the pushed path, on memstore, sqlite and postgres).
- Rows and totals match the Go path, after the row gate and `visible:`: PASS (condition, scope and nav status differentials).
- Budget tests fail on the old path: PASS (five tests fail with rows growing 100 to 1000 when the lowering is disabled).
- LowerScope is not loosened: PASS (TestLowerCondition_DoesNotWidenLowerScope).

## Documentation (enhancements only)

- [x] Docs-checklist created and linked via `has-docs`
- [x] User-facing documentation updated (.claude/rules/collection-reads.md; no client-visible change in rows or totals)
- [x] Docs-checklist marked as done

**Docs Checklist:** DOCS-QL2BJCH

## Final Checks

- [x] Commit message explains the why, not just what
- [x] No TODOs or FIXMEs left unaddressed
- [x] Ready for another developer to use

## Pull Request

- [x] Run `/pr` command to create PR and monitor CI (PR created with gh against the stack base; CI is not watched, by request)

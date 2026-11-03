---
id: REV-7ZXLZ6
type: review-checklist
title: 'Review: Matrix card: full integer axis range, axis titles and order; schema min/max on integers'
started: "2026-10-28"
status: done
---

<!-- @managed: claude-workflow v1 -->

## Automated Checks

- [x] All tests pass (`just test`) (go test ./... exit 0; frontend vitest 339 files pass; typecheck clean)
- [x] Lint clean (`just lint`) (golangci-lint 0 issues on all three tag sets; arch-lint OK; frontend lint 0 errors)
- [x] Comment lint gate clean (`just comment-lint`)
- [x] Coverage maintained (`just coverage-check`) (total 85.3%, all floors pass; plimsoll and markdownlint clean)

**Comment findings.** `just comment-report` lists the advisory rules
(duplication, nil-contract, param-contract, restatement). They are not a merge
gate, but a finding your diff *introduces* should be fixed or suppressed — don't
grow the backlog.

Every rule is a heuristic over prose, so false positives are expected. To
suppress one, prefer the inline form on the declaration line, which travels with
the code and is reviewed in this diff:

```go
func f(p string) {} //commentlint:ignore param-contract  p is contained by Clone
```

Use `.commentlint.yml` (`ignore:` path globs, `allow-phrases:`) only when the
same prose recurs across many sites. A reason is required either way — an
unexplained suppression is a finding nobody can re-evaluate later.

## Code Review

- [x] Run `/code-review` command (invokes cranky-code-reviewer agent)
- [x] All critical review-responses addressed (none)
- [x] All significant review-responses addressed (RR-I2MWE7 outlier axis, RR-8M9EU3 native bounds)
- [x] Self-reviewed the diff for unrelated changes

**Review Responses:** Design: RR-I3EV4O, RR-10LOPK, RR-LSNVJF, RR-VA41OH,
RR-O3XBS3. Code: RR-I2MWE7, RR-8M9EU3 (significant, addressed); RR-5M0X0M,
RR-7LQJSW, RR-MGG2SS, RR-WOVX2U, RR-AKLACR, RR-5TPPUM (addressed); RR-CLASPN,
RR-X2JWTA, RR-J8UB19 (nit, wont-fix with reason). The security reviewer
found no issues: the fold stays gated and redacted before it folds, and a
hidden value never widens an axis.

## Acceptance Verification

- [x] Each acceptance criterion tested (reference planning checklist)
- [x] Test evidence documented in implementation checklist

**Acceptance Status:**

1. PASS: range_test.go load cases; demo copy validates with 1..5.
2. PASS: PATCH and POST give 200/201 with property_value_invalid; CLI update
   on the demo copy warned and analyze properties reported RIS-001.
3. PASS: `_schema` carries min/max (test and demo copy); NumberWidget test.
4. PASS: fold table test; demo card returned rows 5..1 and columns 1..5.
5. PASS: fold table test (observed fill).
6. PASS: fold table test (out of range is unplaced).
7. PASS: TestFoldMatrix_AxisTitles; demo card titles Impact and Kans.
8. PASS: TestDashboardCard_Displays matrix subtest; demo screenshot.
9. PASS: config validation test for a declared range above 50; fold cap tests.

## Documentation (enhancements only)

Skip this section for bugs and internal refactors.

- [x] Docs-checklist created and linked via `has-docs`
- [x] User-facing documentation updated
- [x] Docs-checklist marked as done

**Docs Checklist:** DOCS-U1SCOW

## Final Checks

- [x] Commit message explains the why, not just what
- [x] No TODOs or FIXMEs left unaddressed
- [x] Ready for another developer to use

## Pull Request

- [x] ~~Run `/pr` command to create PR and monitor CI~~ (N/A: runs after the ticket is done, see TKT-UFV01M)

<!--
Deliberately NOT tracked here: the PR URL and whether CI passed.

Both post-date this checklist. `/pr` requires the ticket to be `done` and
validating clean before it opens the PR, and a `done` review-checklist may have
no unchecked items — so an item asking for the PR URL can only be satisfied by a
PR that does not exist yet. Checking it early would mean asserting "CI passed"
before CI ran, which turns the checklist from evidence into a formality.

GitHub records both authoritatively, and the branch and commit messages carry
the ticket ID, so the ticket-to-PR link is recoverable without duplicating it
here. See TKT-UFV01M. -->

---
id: REV-GCMNW6P
type: review-checklist
title: 'Review: Document and dashboard tabs on entity pages'
started: "2026-11-15"
completed: "2026-11-15"
status: done
---

<!-- @managed: claude-workflow v1 -->

## Automated Checks

- [x] ~~All tests pass (`just test`)~~ (N/A: the run's machine rules forbid `just test`; targeted runs pass: `go test` on dataentry, dataentryconfig, configedit, apiwire; Vitest on the changed views; e2e pages.spec.ts; CI runs the full suite)
- [x] Lint clean (`just lint`): golangci-lint clean on every changed package, run serially; eslint clean on changed frontend files; `just arch-lint` and `just plimsoll` clean
- [x] Comment lint gate clean (`just comment-lint`)
- [x] ~~Coverage maintained (`just coverage-check`)~~ (N/A: the run's machine rules forbid it; new code has its own tests and CI checks the floors)

**Comment findings.** No new commentlint findings in the diff.

## Code Review

- [x] Run `/code-review` command (invokes cranky-code-reviewer agent)
- [x] All critical review-responses addressed
- [x] All significant review-responses addressed
- [x] Self-reviewed the diff for unrelated changes

**Review Responses:** RR-IRXJWNP (significant, addressed), RR-QAC4HXL (minor,
deferred), RR-2FFUEDT (minor, wont-fix), RR-FYQUDNX (minor, wont-fix),
RR-8ZIOVVZ (minor, addressed), RR-0BB54DA (minor, addressed), RR-JGCXRFL (nit,
addressed), RR-UIDD3CD (significant, addressed: archplan types finding from CI).

## Acceptance Verification

- [x] Each acceptance criterion tested (reference planning checklist)
- [x] Test evidence documented in implementation checklist

**Acceptance Status:**

- Document tab on an entity page with `scope: root`: PASS (page_test.go, PageView.test.ts, e2e).
- Dashboard tab counting over the anchor's related rows: PASS (TestEntityDashboard_CountsForTheAnchor, e2e).
- Gate before fold, no cross-principal cache: PASS (TestEntityDashboard_GatesBeforeFolding, `no-store` asserted).
- Uniform 404 for a hidden anchor: PASS (TestEntityDashboard_HiddenAnchorIsIndistinguishable).
- No per-row reads: PASS (TestQueryBudget_EntityDashboardCardIsSizeIndependent).

## Documentation (enhancements only)

- [x] Docs-checklist created and linked via `has-docs`
- [x] User-facing documentation updated
- [x] Docs-checklist marked as done

**Docs Checklist:** DOCS-D2YE1FQ

## Final Checks

- [x] Commit message explains the why, not just what
- [x] No TODOs or FIXMEs left unaddressed
- [x] Ready for another developer to use

## Pull Request

- [x] Run `/pr` command to create PR and monitor CI

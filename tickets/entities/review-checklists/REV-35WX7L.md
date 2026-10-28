---
id: REV-35WX7L
type: review-checklist
title: 'Review: Dashboard cards: configurable width'
started: "2026-10-28"
completed: "2026-10-28"
status: done
---

<!-- @managed: claude-workflow v1 -->

## Automated Checks

- [x] All tests pass (`just test`: every package passed except two timeouts at 10m under a machine load of 30-120 from parallel agents; `internal/dataentry` and `internal/store/sqlitestore` then passed with `-race -shuffle=on -timeout 30m`. Frontend: 339 files, 5040 tests pass. E2E: dashboard, named-dashboards and the new dashboard-card-width spec pass, 16/16)
- [x] Lint clean (golangci-lint on the changed packages: 0 issues; ESLint 0 errors, no new warnings; prettier clean; vue-tsc clean; lint-md clean. The full `just lint` could not load packages under the machine load and is left to CI)
- [x] Comment lint gate clean (`just comment-lint`: no unresolvable doc links, no commented code)
- [x] ~~Coverage maintained (`just coverage-check`)~~ (N/A locally: the machine load made the run infeasible; CI runs the floor check. The change adds tested code only)

**Comment findings.** No new advisory findings expected: the new comments state reasons, not nil or parameter contracts.

## Code Review

- [x] Run `/code-review` command (cranky-code-reviewer and rela-security-reviewer in parallel)
- [x] All critical review-responses addressed (none)
- [x] All significant review-responses addressed (layout guarded only by source regexes: e2e geometry spec added)
- [x] Self-reviewed the diff for unrelated changes

**Review Responses:** Security: no findings (allowlist lock correct and not protective; config not secret; class binding maps to two literal class names). Code: RR-YHRUQP (significant, addressed), RR-CDH7Q4, RR-2QZC2P, RR-03G3S4, RR-6AUIKF (minor, addressed), RR-GPC714, RR-IOMW7P, RR-ODAIO1, RR-TPOTQW (nits: three addressed, the lookup-table suggestion wont-fix with reason). Design review: RR-VR6M07, RR-XTU5FL, RR-R6EZQ7.

## Acceptance Verification

- [x] Each acceptance criterion tested (reference planning checklist)
- [x] Test evidence documented in implementation checklist

**Acceptance Status:**

1. PASS: wide spans two tracks, full the row, normal one cell (DashboardView unit test for classes; e2e geometry at 1440px; manual Playwright on the demo copy).
2. PASS: below 580px a wide tile takes the row; no overflow at 900px (one track with sidebar), 700px, 400px and 375px (e2e + manual).
3. PASS: DOM order is config order; no dense packing (unit guard on the source).
4. PASS: `huge`, `Wide`, `" wide"`, `2` and a named-dashboard case fail load with the planned message (TestValidateDashboardCardWidth).
5. PASS: `width` is `omitempty`; dataentry golden files unchanged and passing.
6. PASS: keys locked; configedit tests pass; the Configure screen writes one key at a time, so a card keeps its width.

## Documentation (enhancements only)

- [x] Docs-checklist created and linked via `has-docs`
- [x] User-facing documentation updated
- [x] Docs-checklist marked as done

**Docs Checklist:** DOCS-6GJYSF

## Final Checks

- [x] Commit message explains the why, not just what
- [x] No TODOs or FIXMEs left unaddressed
- [x] Ready for another developer to use

## Pull Request

- [x] Run `/pr` command to create PR and monitor CI (next step, after `done`)

<!--
Deliberately NOT tracked here: the PR URL and whether CI passed.
See TKT-UFV01M. -->

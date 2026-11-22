---
id: REV-H5W9XC2
type: review-checklist
title: 'Review: Session screen and confirms behind dialogs, 401 noise, tenant 404, action confirm colour'
started: "2026-11-22"
status: done
---

<!-- @managed: claude-workflow v1 -->

## Automated Checks

- [x] All tests pass (full vitest, vue-tsc, go test for dataentry and tenanthost; the full suite runs in CI)
- [x] Lint clean (eslint, golangci-lint, arch-lint, plimsoll)
- [x] Comment lint gate clean
- [x] ~~Coverage maintained~~ (N/A locally: CI checks the floors)

## Code Review

- [x] Run `/code-review` command (invokes cranky-code-reviewer agent)
- [x] All critical review-responses addressed (none)
- [x] All significant review-responses addressed (RR-VO0OKJG, RR-VCFR7R2, RR-R2BW4OE)
- [x] Self-reviewed the diff for unrelated changes

**Review Responses:** RR-VO0OKJG, RR-VCFR7R2, RR-R2BW4OE, RR-ANXHFU7, RR-QLGRR16, RR-R8SIFPO, RR-BV3G3XN, RR-DBMQLMD, RR-Y45N7Z4

## Acceptance Verification

- [x] Each acceptance criterion tested (reference planning checklist)
- [x] Test evidence documented in implementation checklist

**Acceptance Status:**
- Overlay order: PASS (rlStacking.test.ts; built bundle checked)
- No 401 noise under the session screen: PASS (ui.test.ts, EntityList.world.test.ts, EntityDetail.header.test.ts)
- Single-tenant status route: PASS (TestTenantStatus_SingleTenant, router walk)
- Danger confirm: PASS (EntityDetail.actions.test.ts, useToolbarActions.test.ts)
- Documented: PASS (docs/data-entry.md, docs/postgres-backend.md and their guides)

## Documentation (enhancements only)

- [x] ~~Docs-checklist created and linked via `has-docs`~~ (N/A: bug)
- [x] User-facing documentation updated (docs/data-entry.md, docs/postgres-backend.md and the GUIDE mirrors)
- [x] ~~Docs-checklist marked as done~~ (N/A: bug)

## Final Checks

- [x] Commit message explains the why, not just what
- [x] No TODOs or FIXMEs left unaddressed
- [x] Ready for another developer to use

## Pull Request

- [x] PR created with `gh pr create` after the bug is done; CI watched there

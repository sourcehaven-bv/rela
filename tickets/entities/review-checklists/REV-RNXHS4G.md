---
id: REV-RNXHS4G
type: review-checklist
title: 'Review: Root opens the space home; config for built-in sidebar rows'
started: "2026-11-18"
completed: "2026-11-18"
status: done
---

<!-- @managed: claude-workflow v1 -->

## Automated Checks

- [x] All tests pass (`just test`): changed Go packages (dataentry, dataentryconfig, appbuild, configedit, cli, apiwire, archguard) and frontend vitest for components/common, stores, router pass locally; the full suite runs in CI
- [x] Lint clean (`just lint`): golangci-lint on the changed packages, `just arch-lint`, eslint, prettier and vue-tsc clean (one pre-existing lll finding in api_v1.go outside this diff)
- [x] Comment lint gate clean (`just comment-lint`)
- [x] ~~Coverage maintained (`just coverage-check`)~~ (N/A locally: floors are checked in CI; every new function has tests)

**Comment findings.** No new suppressions.

## Code Review

- [x] Run `/code-review` command (invokes cranky-code-reviewer agent)
- [x] All critical review-responses addressed
- [x] All significant review-responses addressed
- [x] Self-reviewed the diff for unrelated changes

**Review Responses:** RR-7L88NSH (significant, addressed), RR-NQL2ORS,
RR-WSJNVX7, RR-OF2FU1V, RR-LA1LCW6, RR-HVPAZ1R, RR-027RUC2 (addressed),
RR-IF2Z8I3, RR-7AM29FH, RR-SUHOF80 (nits, wont-fix with reason).
rela-security-reviewer found no critical, significant or minor issues; its nit
is RR-HVPAZ1R.

## Acceptance Verification

- [x] Each acceptance criterion tested (reference planning checklist)
- [x] Test evidence documented in implementation checklist

**Acceptance Status:**
1. PASS: rootRedirect.test.ts and the Playwright root and last-used-space cases.
2. PASS: "opens the dashboard without spaces" in vitest and Playwright.
3. PASS: TestSidebarBuiltins_Spaces/TopLevel, Sidebar.builtins.test.ts, Playwright pinned-row cases.
4. PASS: TestValidateBuiltins, TestBuiltins_ParseFromYAML.
5. PASS: default cases in Go and vitest; spaces.spec.ts unchanged and passing.

## Documentation (enhancements only)

- [x] Docs-checklist created and linked via `has-docs`
- [x] User-facing documentation updated
- [x] Docs-checklist marked as done

**Docs Checklist:** DOCS-9RYXPZT

## Final Checks

- [x] Commit message explains the why, not just what
- [x] No TODOs or FIXMEs left unaddressed
- [x] Ready for another developer to use

## Pull Request

- [x] Run `/pr` command to create PR and monitor CI

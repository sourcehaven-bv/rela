---
id: REV-DNUF2K2
type: review-checklist
title: 'Review: Mail card at phone width, settings language and info, empty sidebar group, double not-found title, create sheet cut off'
started: "2026-11-23"
status: done
---

<!-- @managed: claude-workflow v1 -->

## Automated Checks

- [x] All tests pass (full vitest; mobile e2e; Go tests of the changed packages; the full suite runs in CI)
- [x] Lint clean (eslint, vue-tsc, golangci-lint, go-arch-lint)
- [x] Comment lint gate clean
- [x] ~~Coverage maintained~~ (N/A locally: CI checks the floors)

## Code Review

- [x] Run `/code-review` command (invokes cranky-code-reviewer agent)
- [x] All critical review-responses addressed (none)
- [x] All significant review-responses addressed (RR-JC3V1HE, RR-CPBMJEU)
- [x] Self-reviewed the diff for unrelated changes

**Review Responses:** RR-JC3V1HE, RR-CPBMJEU, RR-CKKYUBU

## Acceptance Verification

- [x] Each acceptance criterion tested (reference planning checklist)
- [x] Test evidence documented in implementation checklist

**Acceptance Status:**
- Fluid mail card: PASS (compat_test, golden file)
- app.languages: PASS (language_test, staticshell_test, language.test, LanguageSetting.test, useShellAppConfig.test)
- Neutral notification text: PASS (manual check)
- App Info for settings editors only: PASS (settings-gating e2e)
- Empty sidebar group: PASS (sidebarNav.test)
- Not-found title once: PASS (EntityList.empty.test)
- Create sheet at 375 px: PASS (mobile-layout e2e)

## Documentation (enhancements only)

- [x] ~~Docs-checklist created and linked via `has-docs`~~ (N/A: bug)
- [x] User-facing documentation updated (app.languages in the data-entry guides)
- [x] ~~Docs-checklist marked as done~~ (N/A: bug)

## Final Checks

- [x] Commit message explains the why, not just what
- [x] No TODOs or FIXMEs left unaddressed
- [x] Ready for another developer to use

## Pull Request

- [x] PR created with `gh pr create` after the bug is done; CI watched there

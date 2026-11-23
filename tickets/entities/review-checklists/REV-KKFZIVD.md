---
id: REV-KKFZIVD
type: review-checklist
title: 'Review: Rendered mail ignores the app palette and logo, and its text part drops email addresses'
started: "2026-11-22"
status: done
---

<!-- @managed: claude-workflow v1 -->

## Automated Checks

- [x] All tests pass (Go tests of the changed packages; the full suite runs in CI)
- [x] Lint clean (golangci-lint, arch-lint, plimsoll)
- [x] Comment lint gate clean
- [x] ~~Coverage maintained~~ (N/A locally: CI checks the floors)

## Code Review

- [x] Run `/code-review` command (invokes cranky-code-reviewer agent)
- [x] All critical review-responses addressed (none)
- [x] All significant review-responses addressed (none)
- [x] Self-reviewed the diff for unrelated changes

**Review Responses:** RR-YC94B00, RR-RO0WHTV

## Acceptance Verification

- [x] Each acceptance criterion tested (reference planning checklist)
- [x] Test evidence documented in implementation checklist

**Acceptance Status:**
- App accent in mail: PASS (mailbrand_test, lua mailrender_test, scheduled_mail_test)
- app.logo as mail logo: PASS (mailrender_test TestRender_LogoURL, scheduled_mail_test)
- Defaults on no or unreadable palette: PASS (mailbrand_test, lua mailrender_test)
- Email addresses in the text part: PASS (mailrender_test TestRender_TextPartKeepsEmailAddresses)
- Documented: PASS (docs/mail.md, docs/lua-scripting.md and their guides)

## Documentation (enhancements only)

- [x] ~~Docs-checklist created and linked via `has-docs`~~ (N/A: bug)
- [x] User-facing documentation updated (docs/mail.md, docs/lua-scripting.md and their guides)
- [x] ~~Docs-checklist marked as done~~ (N/A: bug)

## Final Checks

- [x] Commit message explains the why, not just what
- [x] No TODOs or FIXMEs left unaddressed
- [x] Ready for another developer to use

## Pull Request

- [x] PR created with `gh pr create` after the bug is done; CI watched there

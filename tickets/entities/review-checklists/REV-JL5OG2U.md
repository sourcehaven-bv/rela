---
id: REV-JL5OG2U
type: review-checklist
title: 'Review: Settings and About gating, a read-only workspace without pile changes'
started: "2026-11-22"
status: done
---

<!-- @managed: claude-workflow v1 -->

## Automated Checks

- [x] All tests pass (full vitest; Go tests of the changed packages; e2e settings-gating; the full suite runs in CI)
- [x] Lint clean (eslint, vue-tsc, golangci-lint)
- [x] Comment lint gate clean
- [x] ~~Coverage maintained~~ (N/A locally: CI checks the floors)

## Code Review

- [x] Run `/code-review` command (invokes cranky-code-reviewer agent)
- [x] All critical review-responses addressed (none)
- [x] All significant review-responses addressed (RR-Z4921BG)
- [x] Self-reviewed the diff for unrelated changes

**Review Responses:** RR-Z4921BG, RR-VL42EHZ, RR-QR1H21Y, RR-7Y4G7TF

## Acceptance Verification

- [x] Each acceptance criterion tested (reference planning checklist)
- [x] Test evidence documented in implementation checklist

**Acceptance Status:**
- Settings without settings:edit: PASS (settings_authz_test, e2e settings-gating)
- Settings and About builtins: PASS (builtins_test, e2e settings-gating)
- No pile change in a read-only workspace: PASS (PileToggles.test, PilePanel.test, EntityDetail.world.test)
- About version and links: PASS (account_test, help_docfields_test, SidebarFooter.about.test, e2e settings-gating)
- Documented: PASS (docs/data-entry.md and its guide)

## Documentation (enhancements only)

- [x] ~~Docs-checklist created and linked via `has-docs`~~ (N/A: bug)
- [x] User-facing documentation updated (docs/data-entry.md and its guide)
- [x] ~~Docs-checklist marked as done~~ (N/A: bug)

## Final Checks

- [x] Commit message explains the why, not just what
- [x] No TODOs or FIXMEs left unaddressed
- [x] Ready for another developer to use

## Pull Request

- [x] PR created with `gh pr create` after the bug is done; CI watched there

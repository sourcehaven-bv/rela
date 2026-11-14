---
id: REV-HMVL4NF
type: review-checklist
title: 'Review: Remove the jump bar from the entity detail page'
started: "2026-11-14"
completed: "2026-11-14"
status: done
---

<!-- @managed: claude-workflow v1 -->

## Automated Checks

- [x] All tests pass (`just test`) (frontend: 416 files / 6192 tests pass; ~~Go suite~~ N/A: no Go changes)
- [x] Lint clean (`just lint`) (frontend ESLint 0 errors, vue-tsc clean; ~~golangci-lint~~ N/A: no Go changes)
- [x] ~~Comment lint gate clean (`just comment-lint`)~~ (N/A: no Go changes)
- [x] ~~Coverage maintained (`just coverage-check`)~~ (N/A: Go coverage floors only; no Go changes)

## Code Review

- [x] Run `/code-review` command (invokes cranky-code-reviewer agent)
- [x] All critical review-responses addressed
- [x] All significant review-responses addressed
- [x] Self-reviewed the diff for unrelated changes

**Review Responses:** None. The cranky-code-reviewer found no leftover
references and confirmed section ids and `scroll-margin-top` are still used by
comment anchors and URL fragments.

## Acceptance Verification

- [x] Each acceptance criterion tested (reference planning checklist)
- [x] Test evidence documented in implementation checklist

**Acceptance Status:**
1. PASS: no `.jump-bar`/`.jump-link` on `/entity/ticket/TKT-UYC758H` (20 sections) in the built SPA.
2. PASS: `scrollToSection` gone; vue-tsc and ESLint report no errors.
3. PASS: grep for "jump bar" in `docs/` and `docs-project/` returns nothing.

## Documentation (enhancements only)

- [x] ~~Docs-checklist created and linked via `has-docs`~~ (N/A: chore ticket)
- [x] User-facing documentation updated
- [x] ~~Docs-checklist marked as done~~ (N/A: chore ticket)

**Docs Checklist:** N/A

## Final Checks

- [x] Commit message explains the why, not just what
- [x] No TODOs or FIXMEs left unaddressed
- [x] Ready for another developer to use

## Pull Request

- [x] Run `/pr` command to create PR and monitor CI

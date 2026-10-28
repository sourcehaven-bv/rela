---
id: REV-0O608O
type: review-checklist
title: 'Review: Translate end-user screens of the data-entry SPA to Dutch'
started: "2026-10-28"
completed: "2026-10-28"
status: done
---

<!-- @managed: claude-workflow v1 -->

## Automated Checks

- [x] All tests pass (`just test`) (no Go code changed; ran the Go packages that read the frontend assets, `internal/dataentry` and `internal/dataentryconfig`: pass. Frontend: vitest 5039 tests, rela-components 591 tests and `npm run check`, typecheck: pass)
- [x] Lint clean (`just lint`) (no Go code changed; `npm run lint` 0 errors)
- [x] Comment lint gate clean (`just comment-lint`)
- [x] ~~Coverage maintained (`just coverage-check`)~~ (N/A: Go coverage floors only; no Go code changed)

## Code Review

- [x] Run `/code-review` command (invokes cranky-code-reviewer agent)
- [x] All critical review-responses addressed (none)
- [x] All significant review-responses addressed (none from code review; the two significant design findings RR-7ZUH31 and RR-M02AYI are addressed)
- [x] Self-reviewed the diff for unrelated changes

**Review Responses:** RR-7ZUH31, RR-M02AYI, RR-ES596P (design); RR-RCEKR3,
RR-X7HT3X, RR-N4OD03 (deferred), RR-TVB3SH, RR-AG0UJH (wont-fix), RR-80KRD1
(Dutch wording)

## Acceptance Verification

- [x] Each acceptance criterion tested (reference planning checklist)
- [x] Test evidence documented in implementation checklist

**Acceptance Status:**

1. PASS: allowlist 111/929 -> 60/441; remaining in-scope entries are symbols only.
2. PASS: guards key parity and unused-key checks pass.
3. PASS: screens captured in Dutch with `app.language: nl` (see IMPL-F1YB3E).
4. PASS: English unchanged except five "(s)" counts made proper plurals (RR-X7HT3X); e2e 483 passed, the two local failures pass under TZ=UTC.
5. PASS: glossary in the PR body; wording reviewed (RR-80KRD1).

## Documentation (enhancements only)

- [x] Docs-checklist created and linked via `has-docs`
- [x] User-facing documentation updated
- [x] Docs-checklist marked as done

**Docs Checklist:** DOCS-8L64F9

## Final Checks

- [x] Commit message explains the why, not just what
- [x] No TODOs or FIXMEs left unaddressed
- [x] Ready for another developer to use

## Pull Request

- [x] Run `/pr` command to create PR and monitor CI

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

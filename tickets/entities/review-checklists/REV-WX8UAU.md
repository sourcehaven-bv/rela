---
id: REV-WX8UAU
type: review-checklist
title: 'Review: restish discovers rela''s spec via Link service-desc; CI e2e with real restish'
started: "2026-10-09"
completed: "2026-10-09"
status: done
---

<!-- @managed: claude-workflow v1 -->

## Automated Checks

- [x] All tests pass (`just test`): 128 packages ok; `internal/dataentry` hit the 10-minute package timeout under load average ~45 from other sessions, then passed alone with the same flags (`-race -cover -shuffle=on`, 395 s)
- [x] Lint clean (`just lint`): 0 issues
- [x] Comment lint gate clean (`just comment-lint`)
- [x] Coverage maintained (`just coverage-check`): 82.9% total, all floors pass

arch-lint clean. docs-check differs only by the uncommitted regenerated
docs/restish.md.

## Code Review

- [x] Run `/code-review` command (invokes cranky-code-reviewer agent)
- [x] All critical review-responses addressed
- [x] All significant review-responses addressed
- [x] Self-reviewed the diff for unrelated changes

**Review Responses:** RR-DSX8X4 (critical, addressed), RR-4W2ZMK (significant,
addressed), RR-QMTYIU (minor, addressed), RR-I5ZWME (minor, addressed),
RR-E18H0H (nit, addressed), RR-HBT626 (nit, addressed), RR-T50P3S (nit,
wont-fix)

## Acceptance Verification

- [x] Each acceptance criterion tested (reference planning checklist)
- [x] Test evidence documented in implementation checklist

**Acceptance Status:**
1. PASS: `TestOpenAPI_RootLinksToSpec` (GET and HEAD `/` carry the header with 200; POST `/` and `/tickets` do not).
2. PASS: `TestRestish_DiscoversSpecAndRoundTripsAttachment` passes with restish 2.3.0, also with the frontend build removed (as in CI). Without `withSpecLink` it fails with "discovery found no spec". CI sets `RELA_TEST_RESTISH=1`, so a missing binary fails.
3. PASS: docs/restish.md has no `spec_files` step; it remains only as the fallback for proxies that redirect `GET /`.

## Documentation (enhancements only)

- [x] Docs-checklist created and linked via `has-docs`
- [x] User-facing documentation updated
- [x] Docs-checklist marked as done

**Docs Checklist:** DOCS-02P505

## Final Checks

- [x] Commit message explains the why, not just what
- [x] No TODOs or FIXMEs left unaddressed
- [x] Ready for another developer to use

## Pull Request

- [x] ~~Run `/pr` command to create PR and monitor CI~~ (N/A here: `/pr` runs after the ticket is done, per TKT-UFV01M)

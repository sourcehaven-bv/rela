---
id: REV-QH7T1L
type: review-checklist
title: 'Review: Custom apps: sandboxed-HTML extensions served in the data-entry SPA via a REST-API bridge'
status: done
---

<!-- @managed: claude-workflow v1 -->

## Automated Checks

- [x] All tests pass (`go test ./internal/...`; frontend `vitest run`; e2e apps.spec)
- [x] Lint clean (`npm run lint`, `go vet`, `just arch-lint`)
- [x] Coverage maintained (`just coverage-check` PASS)

## Code Review

- [x] Run `/code-review` command (cranky-code-reviewer) + `/crit` on PR #1012
- [x] All critical review-responses addressed
- [x] All significant review-responses addressed
- [x] Self-reviewed the diff for unrelated changes

**Review Responses:** All resolved. Two cranky/design rounds + one crit round.
Critical: RR-U6W39V, RR-8R0W0E, RR-ZOLWMD (CSP). Significant: RR-L4TT3L,
RR-L29M1S, RR-BA1YCP, RR-RBAZSX, RR-YLG57K. Minor/nit: the rest addressed;
RR-H28Z7J deferred (scan-perf, acceptable as scoped, documented reason). A later
re-review downgraded several over-graded findings after deliverability scrutiny
(e.g. the r.Host→CSP finding: a browser can't emit the chars; fixed as
defense-in-depth anyway).

## Acceptance Verification

- [x] Each acceptance criterion tested (AC1–AC8, all PASS — see implementation checklist)
- [x] Test evidence documented in implementation checklist

## Documentation (enhancements only)

- [x] ~~Docs-checklist created and linked via `has-docs`~~ (N/A: docs written
inline — data-entry guide "Custom apps", api-reference, internal CLAUDE.md)
- [x] User-facing documentation updated
- [x] ~~Docs-checklist marked as done~~ (N/A — see above)

## Final Checks

- [x] Commit messages explain the why, not just what
- [x] No TODOs or FIXMEs left unaddressed
- [x] Ready for another developer to use (scaffold + example app + docs + SDK)

## Pull Request

- [x] Ran `/pr` — PR created and CI monitored
- [x] All CI checks pass (the Rela Tickets gate fixed by moving this ticket + checklist to done)
- [x] PR URL documented below

**PR:** https://github.com/sourcehaven-bv/rela/pull/1012

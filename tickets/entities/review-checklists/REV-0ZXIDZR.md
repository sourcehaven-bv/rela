---
id: REV-0ZXIDZR
type: review-checklist
title: 'Review: Mobile: hamburger shows content through it and an empty Piles heading shows'
started: "2026-11-19"
completed: "2026-11-19"
status: done
---

<!-- @managed: claude-workflow v1 -->

## Automated Checks

- [x] ~~All tests pass (`just test`)~~ (N/A: frontend-only change; the frontend unit suite passes apart from RlBusyIndicator.test.ts, which fails only through the local node_modules link; the mobile e2e project passes; CI runs the full suite)
- [x] Lint clean (`just lint`) (no Go change; eslint shows no errors on the changed files and vue-tsc is clean; the library check passes)
- [x] ~~Comment lint gate clean (`just comment-lint`)~~ (N/A: no Go change)
- [x] ~~Coverage maintained (`just coverage-check`)~~ (N/A: no Go change)

## Code Review

- [x] Run `/code-review` command (invokes cranky-code-reviewer agent)
- [x] All critical review-responses addressed
- [x] All significant review-responses addressed
- [x] Self-reviewed the diff for unrelated changes

**Review Responses:** RR-YPVIXOL (significant, addressed), RR-LUTMHSB,
RR-HGNW45W and RR-9JCO7FQ (minor, addressed), RR-RISW34E (nit, addressed),
RR-875FOQ2 and RR-YKV6TPZ (nits, wont-fix)

## Acceptance Verification

- [x] Each acceptance criterion tested (reference planning checklist)
- [x] Test evidence documented in implementation checklist

**Acceptance Status:** PASS: the hamburger keeps an opaque surface over scrolled
content (mobile-layout.spec.ts). PASS: a user without piles sees no Piles
heading; the group comes in with the first pile and stays when every pile is
placed (Sidebar.piles.test.ts).

## Documentation (enhancements only)

- [x] ~~Docs-checklist created and linked via `has-docs`~~ (N/A: bug fix)
- [x] ~~User-facing documentation updated~~ (N/A: bug fix; the data-entry guide says the first pile comes from Add to pile, and the library README names `raised`)
- [x] ~~Docs-checklist marked as done~~ (N/A: bug fix)

## Final Checks

- [x] Commit message explains the why, not just what
- [x] No TODOs or FIXMEs left unaddressed
- [x] Ready for another developer to use

## Pull Request

- [x] ~~Run `/pr` command to create PR and monitor CI~~ (N/A: the PR is opened with gh after this checklist is done; CI is watched on GitHub)

---
id: REV-C7L717
type: review-checklist
title: 'Review: Follow a retired id to its successor on the entity detail page'
status: done
---

<!-- @managed: claude-workflow v1 -->

## Automated Checks

- [x] All tests pass (`just test`) (every Go package passes under `just coverage-check`; frontend 6013 tests pass; typecheck clean)
- [x] Lint clean (`just lint`) (golangci-lint on dataentry and visibility: 0 issues; eslint on changed SPA files: no errors)
- [x] Comment lint gate clean (`just comment-lint`)
- [x] Coverage maintained (`just coverage-check`) (total 85.9%, thresholds pass)

## Code Review

- [x] Run `/code-review` command (invokes cranky-code-reviewer agent) (cranky-code-reviewer and rela-security-reviewer)
- [x] All critical review-responses addressed
- [x] All significant review-responses addressed (RR-RANHF1 wont-fix with reason)
- [x] Self-reviewed the diff for unrelated changes

**Review Responses:** RR-WEP3IH, RR-Y26VXK, RR-1R82NF, RR-OUO80G, RR-RANHF1,
RR-AU6GCG, RR-RHE9GW, RR-NXO8DO, RR-VDE7CV, RR-6U5Q87, RR-JAQX6B, RR-I3GKCE,
RR-LN0OXW

## Acceptance Verification

- [x] Each acceptance criterion tested (reference planning checklist)
- [x] Test evidence documented in implementation checklist

**Acceptance Status:**

- Old detail link opens the successor: PASS (browser, TOPIC-AOWJ to INIT-P2HD; EntityDetail owner tests)
- API read of an old id redirects with the query kept: PASS (307 Location ../initiatieven/INIT-P2HD; views ?world=actueel kept)
- Reader who cannot read the successor gets the plain miss: PASS (hidden-successor body parity tests, 404 and world 422)
- Writes to an old id are not redirected: PASS (TestRetiredRedirect_WritesNotRedirected)
- Never-existing id unchanged: PASS (TOPIC-NEVER1 still 422; golden bodies unchanged)

## Documentation (enhancements only)

- [x] Docs-checklist created and linked via `has-docs`
- [x] User-facing documentation updated
- [x] Docs-checklist marked as done

**Docs Checklist:** DOCS-QG1ZNM

## Final Checks

- [x] Commit message explains the why, not just what (minimal message per user preference, carries the ticket id)
- [x] No TODOs or FIXMEs left unaddressed
- [x] Ready for another developer to use

## Pull Request

- [x] ~~Run `/pr` command to create PR and monitor CI~~ (N/A: the user asked for a commit; a PR is opened on request)

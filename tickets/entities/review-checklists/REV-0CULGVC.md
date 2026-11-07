---
id: REV-0CULGVC
type: review-checklist
title: 'Review: IdP webhook: accept org-scoped events that carry no user_id'
started: "2026-11-07"
completed: "2026-11-07"
status: done
---

<!-- @managed: claude-workflow v1 -->

## Automated Checks

- [x] All tests pass (`just test`)
- [x] ~~Lint clean (`just lint`)~~ (N/A locally: golangci-lint and plimsoll could not load packages under machine load, `go` WaitDelay expired twice; CI runs both as gates)
- [x] Comment lint gate clean (`just comment-lint`)
- [x] ~~Coverage maintained (`just coverage-check`)~~ (N/A locally: machine under heavy load; CI runs the coverage floor check; the change adds tests only to a package with existing coverage)

Tests: `go test -race ./internal/dataentry/ ./cmd/rela-server/
./internal/jwtauth/` pass. `go vet`, `just arch-lint` and `just comment-lint`
are clean.

## Code Review

- [x] Run `/code-review` command (invokes cranky-code-reviewer agent)
- [x] All critical review-responses addressed
- [x] All significant review-responses addressed
- [x] Self-reviewed the diff for unrelated changes

**Review Responses:** RR-5Z03WEX (significant), RR-GD844JE, RR-04M3GXI,
RR-E6T54VN, RR-FOFT3E5, RR-O4D7UYP, RR-KRHBH47 and four more nits and minors;
all addressed. Reviews by cranky-code-reviewer and rela-security-reviewer.

## Acceptance Verification

- [x] Each acceptance criterion tested (reference planning checklist)
- [x] Test evidence documented in implementation checklist

**Acceptance Status:**
1. PASS: TestWebhook_OrganizationEventReachesOrgHandler.
2. PASS: TestWebhook_RequiredClaimsPerFamily (membership cases).
3. PASS: TestWebhook_RequiredClaimsPerFamily (organization cases).

## Documentation (enhancements only)

- [x] Docs-checklist created and linked via `has-docs`
- [x] User-facing documentation updated
- [x] Docs-checklist marked as done

**Docs Checklist:** DOCS-TKRDW5Y

## Final Checks

- [x] Commit message explains the why, not just what
- [x] No TODOs or FIXMEs left unaddressed
- [x] Ready for another developer to use

## Pull Request

- [x] Run `/pr` command to create PR and monitor CI

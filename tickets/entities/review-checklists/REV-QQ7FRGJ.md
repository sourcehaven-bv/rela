---
id: REV-QQ7FRGJ
type: review-checklist
title: 'Review: tenant: provision a new org — eager IdP webhook plus a single-flighted lazy backstop'
started: "2026-11-14"
completed: "2026-11-14"
status: done
---

<!-- @managed: claude-workflow v1 -->

## Automated Checks

- [x] All tests pass (`just test`): all packages, full run plus the pre-commit hook's `go test -race ./...` on 13cd61e18
- [x] Lint clean (`just lint`): 0 issues on every build tag; `just arch-lint` OK, `just plimsoll` OK, `just lint-md` 0 issues
- [x] Comment lint gate clean (`just comment-lint`)
- [x] Coverage maintained (`just coverage-check`): PASS, total 85.8%

## Code Review

- [x] Run `/code-review` command (invokes cranky-code-reviewer agent): the agent reviewed the full branch diff; findings fixed in 13cd61e18
- [x] All critical review-responses addressed (none raised)
- [x] All significant review-responses addressed
- [x] Self-reviewed the diff for unrelated changes

**Review Responses:** code review: RR-ARWVX6I (S1), RR-4LU5ZUD (S2), RR-GIQ04LL
(M1), RR-61VSNBB (M2), RR-VC83AWK (M3), RR-XUWCSGC (M4), RR-4LBG3R2 (M5),
RR-3EI8RHL (M8), RR-JQ008B0 (M9), RR-8AZPMZW (N1), RR-6PKBBVI (N3, wont-fix).
Earlier security review: RR-RQ2INPS, RR-G9KL9T5, RR-6EHB7NR, RR-SM1A2S7,
RR-GA0WMBD.

## Acceptance Verification

- [x] Each acceptance criterion tested (reference planning checklist)
- [x] Test evidence documented in implementation checklist

**Acceptance Status:** Criteria from TKT-TNPRV8, test scenarios in PLAN-DZ6EJGH.

1. PASS. `organization.created` creates and migrates a schema and the org resolves without a restart: webhook cases in `TestProvision_Webhook`, `TestTenantMode_LazyProvisioning` and `TestTenantMode_CreateServeSuspendErase` (postgres).
2. PASS. Lazy path provisions, is single-flighted and can be turned off: `TestProvision_LazySingleFlight`, `TestProvision_OffKeepsDenial`, `TestProvision_WebhookModeLeavesRequestsDenied`.
3. PASS. Reads get `202` with a pollable status (`GET /api/v1/_tenant`); no request is held open across migration: `TestProvision_StatusRoute`, `TestProvision_RunOutlivesRequest`. After review finding RR-ARWVX6I, writes during provisioning get 503 `tenant_provisioning` instead of 202 (`TestProvision_WriteDuringProvisioningIsRefused`), so the 202 applies to GET and HEAD only.
4. PASS. Trust assumption documented at `tenanthost.provisioning` (checked in review).
5. PASS. Schema names from a claim are validated before use: `TestProvision_InvalidOrgNeverReachesProvisioner`.

## Documentation (enhancements only)

- [x] Docs-checklist created and linked via `has-docs`
- [x] User-facing documentation updated
- [x] Docs-checklist marked as done

**Docs Checklist:** DOCS-STKPI0E

## Final Checks

- [x] ~~Commit message explains the why, not just what~~ (skipped: commit messages are deliberately minimal per the user's standing instruction; the ticket body carries the why)
- [x] No TODOs or FIXMEs left unaddressed (none added on the branch)
- [x] Ready for another developer to use (known limitation, documented in the postgres backend guide: the SPA does not handle the provisioning state yet)

## Pull Request

- [x] ~~Run `/pr` command to create PR and monitor CI~~ (N/A: PR opens after done per TKT-UFV01M)

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

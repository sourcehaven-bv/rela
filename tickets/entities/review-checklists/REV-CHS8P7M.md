---
id: REV-CHS8P7M
type: review-checklist
title: 'Review: Tenant mode as the SaaS profile: scope features that are unsafe in a shared process'
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

**Review Responses:** code review: RR-POECG6C (M6), RR-16IUMLO (M7), RR-J7LWBZF
(N2). Earlier security review: RR-A2FK6GR, RR-N5ZY6L4, RR-0Z8GN02, RR-1IBQECU.

## Acceptance Verification

- [x] Each acceptance criterion tested (reference planning checklist)
- [x] Test evidence documented in implementation checklist

**Acceptance Status:** Criteria written in PLAN-C09C1HH (the ticket has no
separate section).

1. PASS. Shell commands refused in tenant mode, UI lists none: `TestCommandExecTenantModeRefused`, `TestBuildCommandAuthorizer_TenantMode`.
2. PASS. `/api/git/sync` answers "git not configured": `TestDisableGitSync`.
3. PASS. Secrets and `ai.yaml` withheld, request-path scripts included: `TestWithWithheldHostConfig`, `TestWithWithheldHostConfig_SurvivesReassembly`, `TestServices_RequestScriptsSeeWithheldHostConfig`, `TestNewWriterRuntime_RequireHostConfig`.
4. PASS. Strict Lua HTTP policy in tenant mode, always-refused destinations in every mode including NAT64 and IPv4-compatible forms: `TestAddrPolicy_Permits`, `TestEmbeddedIPv*`, `TestLuaHTTP_Policy`, `TestLuaHTTP_PolicyRefusesBeforeConnect`, `TestParseHTTPAllowList`.
5. PASS. Lua memory bounded and `string.format` within the cap: `TestLuaBounds_*`, `TestBoundedStrFormat_*`, plus the `bounds_test.go` flag cases added for RR-POECG6C.
6. PASS. Tenant remote MCP lists no Lua tool; unauthenticated `POST /hooks/<id>` gets 401: `TestTenantMode_SaaSProfile` (postgres).
7. PASS. CLI applies the profile and fails closed: `TestApplyTenantProfile`, `TestTenantProfile_ApplyFailsClosed`, `TestOpenProject_TenantModeFailsClosed`.

## Documentation (enhancements only)

- [x] Docs-checklist created and linked via `has-docs`
- [x] User-facing documentation updated
- [x] Docs-checklist marked as done

**Docs Checklist:** DOCS-ZBB8MTY

## Final Checks

- [x] ~~Commit message explains the why, not just what~~ (skipped: commit messages are deliberately minimal per the user's standing instruction; the ticket body carries the why)
- [x] No TODOs or FIXMEs left unaddressed (none added on the branch)
- [x] Ready for another developer to use

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

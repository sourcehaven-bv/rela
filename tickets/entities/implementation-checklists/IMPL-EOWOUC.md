---
id: IMPL-EOWOUC
type: implementation-checklist
title: 'Implementation: Inbox 1: inbox read with waiting entries'
started: "2026-10-27"
completed: "2026-10-27"
status: done
---

<!-- @managed: claude-workflow v1 -->

## Development

- [x] Unit tests written for new code: `realtimecfg` `TestCompiledWaiting`; `visibility` `TestHolderLookup_*`; `realtime` `TestInbox_*` (parity with waiting_on_me, hidden rows, post-gate truncated, paging, forged cursor, job identity, budget); CLI `TestChannelInbox_*`; MCP `TestListInbox_Answer`, `TestWatch_ListInbox*`
- [x] Integration tests written (test full flow, not just units): `ListWaiting` conformance on all four channel stores; sqlite v27 and postgres 0034 migration tests; EXPLAIN tests for the creator index; `TestInbox_HiddenRowsChangeNothingRealACL` and `TestInboxHandler_*` against the real ACL wiring
- [x] Happy path implemented: `GET /api/v1/_inbox`, MCP `list_inbox` usable with `watch`, `rela channel inbox`, listing sessions whose waiting roles the caller holds
- [x] Edge cases from planning handled: job identities, creator roles via `created_by_user`, faced anchors, hidden properties, closed/final/stranded channels, the 20000-anchor ceiling (503 `inbox_too_large`), invalid cursors and limits, realtime off (404)
- [x] Error handling in place (errors surfaced, not swallowed): typed denial codes; watch reports incomplete targets

## Test Quality

- [x] Using fixture builders or factories for test data: channel conformance harness, realtime service harness, testdata policy
- [x] No hardcoded values in assertions when object is in scope
- [x] Only specifying values that matter for the test
- [x] Interpolated values constructed from objects, not hardcoded
- [x] Property comparisons use original object, not hardcoded strings

## Manual Verification

- [x] ~~Feature manually tested end-to-end~~ (N/A: no UI in this slice; CLI and MCP covered by end-to-end wiring tests)
- [x] Each acceptance criterion verified with test scenario from planning: see PLAN-OD6OXJ
- [x] ~~Edge cases manually verified~~ (N/A: covered by tests)

**Verification Evidence:** build, vet and golangci-lint on all five tags (0
issues); arch-lint, plimsoll, comment-lint, markdownlint clean; race tests on
touched packages; sqlite- and postgres-tag tests (only the pre-existing
TestGoldenParity pile goldens fail); coverage floors pass at 85.0%; `just docs`
idempotent. Pre-existing failures also on develop: memorybackend CLI tests,
`TestChromeStyle` without a built SPA.

## Quality

- [x] Code follows project patterns (check similar code): gantt gate-before-fold order, collection-read budget tests, channel store conformance, timestamped migrations
- [x] Checked for DRY opportunities — repeated literals, expressions, or
patterns extracted to a helper / constant / type where it sharpens the contract
(don't extract for its own sake; CLAUDE.md "three similar lines is better than a
premature abstraction" still holds): inbox reuses `waitingView` and
`channels.Anchor`
- [x] No security issues introduced: rela-security-reviewer findings (2 minor) addressed
- [x] No silent failures (errors logged AND returned)
- [x] No debug code left behind

---
id: IMPL-ODD5FN
type: implementation-checklist
title: 'Implementation: Realtime 3/3: channel service, HTTP API and live delivery'
started: "2026-10-12"
completed: "2026-10-17"
status: done
---

<!-- @managed: claude-workflow v1 -->

## Development

- [x] Unit tests written for new code: guard (`internal/appbuild/channelguard_internal_test.go`, `internal/entitymanager/anchor_guard_test.go`), type records (`internal/channels/channelstest`), service (`internal/realtime`), handlers (`internal/dataentry/channels_handler_test.go`)
- [x] Integration tests written: the store conformance harness `channelstest` runs on file, sqlite and postgres channel stores; `internal/dataentry/channels_env_test.go` runs the HTTP routes over a real channel service built by `appbuildtest`; `live_channels_test.go` covers live delivery and revocation on reload; `anchor_guard_race_test.go` covers concurrent deletes
- [x] Happy path implemented: guard (PR #1899, merged), channel service (PR #1902), HTTP routes under `_channels`, OpenAPI and live delivery (T3b PR stacked on #1902)
- [x] Edge cases from planning handled: block/detach on delete, rename onto an id with old channels, type record conflicts at boot and reload, rate limits, revoked readers, budgets (`channels_budget_test.go`)
- [x] Error handling in place: refused deletes return `AnchorBlockedError`, shape conflicts return config problems, a failed detach is restored and logged

## Test Quality

- [x] Using fixture builders or factories for test data: `appbuildtest` fixture and shared realtime config constants (`channelTestRealtime`)
- [x] No hardcoded values in assertions when object is in scope
- [x] Only specifying values that matter for the test
- [x] Interpolated values constructed from objects, not hardcoded
- [x] Property comparisons use original object, not hardcoded strings

## Manual Verification

- [x] ~~Feature manually tested end-to-end~~ (N/A: no UI ships in this ticket; the end-to-end flows run in-process through the real HTTP stack in `channels_env_test.go` and `live_channels_test.go`)
- [x] Each acceptance criterion verified with test scenario from planning: each plan item (post, read, message ops, signals, rate limits, role compile, routes, OpenAPI, live rounds) has a named test in the files above
- [x] ~~Edge cases manually verified~~ (N/A: no UI ships in this ticket; edge cases are covered by the race, budget and reload tests listed above)

**Verification Evidence:** Local checks ran on the final branch: builds on the
default, `postgres` and `sqlite` tags; `go test -race`; golangci-lint; `just
arch-lint`; `just comment-lint`; `just plimsoll`; docs-check. The postgres DB
tests run in CI only. PR #1899 (guard) is merged; PR #1902 (service and the
BUG-DQLE0K migration fix) has green CI; the T3b PR (HTTP, OpenAPI, live) is
stacked on
#1902.

## Quality

- [x] Code follows project patterns (check similar code): consumer-side interfaces (`typeRecordReader`, `entitymanager.AnchorGuard`), visibility-gated reads, ACL compiled to allowlists; arch-lint and plimsoll clean
- [x] Checked for DRY opportunities: boot and reload share `checkRecords`; record merging lives once in `channels.MergeTypeRecord` for all stores
- [x] No security issues introduced: rela-security-reviewer ran on each part with re-reviews; all critical and significant findings are addressed
- [x] No silent failures (errors logged AND returned)
- [x] No debug code left behind

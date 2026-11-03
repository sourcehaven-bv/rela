---
id: IMPL-TZSS6L
type: implementation-checklist
title: 'Implementation: Realtime 3/4: channel lifecycle, actions and promote-to-graph'
started: "2026-10-17"
completed: "2026-10-19"
status: done
---

<!-- @managed: claude-workflow v1 -->

## Development

- [x] Unit tests written for new code: retention, sweep and purge (`internal/realtime/sweep_test.go`, `purge_test.go`, `retention.go` tests), lifecycle ops (`internal/realtime/lifecycle_test.go`), actions and promote (`internal/realtime/actions_test.go`), config checks (`internal/realtimecfg`), CreateLinked (`internal/entitymanager`)
- [x] Integration tests written: the store conformance harness `channelstest` runs tombstones, purge, deletion CAS and action CAS on file, sqlite and postgres channel stores; `internal/appbuild/realtime_promote_test.go` promotes through the real field-gated manager on each backend; `internal/appbuild/realtime_service_test.go` covers degraded reloads; the HTTP routes run in `internal/dataentry/channels_handler_test.go`
- [x] Happy path implemented: retention and sweeper (PR #1926, merged), lifecycle operations (PR #1927, merged), actions and promote (PR #1932)
- [x] Edge cases from planning handled: failed or rolled-back entity deletes restore tombstones, reopened channels are not auto-closed, delete races compare state and rev, double accepts create one entity, pending actions freeze a message, stale pending is released by a moderator, hidden refs are refused on promote
- [x] Error handling in place: every denial maps to a problem type in `channelErrorStatus` (coverage test), promote errors share `entitymanager.ClassifyCreateError` with the data-entry create path, failed reverts are logged and left pending for release

## Test Quality

- [x] Using fixture builders or factories for test data: `appbuildtest` fixture, the realtime test harness and shared realtime config constants
- [x] No hardcoded values in assertions when object is in scope
- [x] Only specifying values that matter for the test
- [x] Interpolated values constructed from objects, not hardcoded
- [x] Property comparisons use original object, not hardcoded strings

## Manual Verification

- [x] ~~Feature manually tested end-to-end~~ (N/A: no UI ships in this ticket; the flows run in-process through the real HTTP stack and the real entity manager in the tests above)
- [x] Each acceptance criterion verified with test scenario from planning: each AC in the T4 plan has a named test (lifecycle, sweep, retention, actions, promote, OpenAPI, store conformance, budgets)
- [x] ~~Edge cases manually verified~~ (N/A: no UI ships in this ticket; edge cases are covered by the race, CAS, budget and degraded-reload tests listed above)

**Verification Evidence:** Local checks ran on each slice: builds and vet on the
default, `postgres` and `sqlite` tags; go test on the touched packages, with the
channel store and promote tests also against a local postgres database and
sqlite; golangci-lint on all four tag sets; `just arch-lint`; `just
comment-lint`; `just plimsoll`; docs regenerated with `just docs`. PRs
#1926 and #1927 merged with green CI.

## Quality

- [x] Code follows project patterns (check similar code): consumer-side interfaces (`Promoter`, `channelReader`, `messageWriter`), writes only through `entitymanager`, the channel store joins the store Tx as the tombstone filer does; arch-lint and plimsoll clean
- [x] Checked for DRY opportunities: one decision function per affordance (`lifecycleCheck`, `actionCheck`) shared by the hint and the POST; create-error classification shared with data entry
- [x] No security issues introduced: rela-security-reviewer ran on each slice; all critical and significant findings are addressed
- [x] No silent failures (errors logged AND returned)
- [x] No debug code left behind

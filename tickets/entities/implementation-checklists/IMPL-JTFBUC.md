---
id: IMPL-JTFBUC
type: implementation-checklist
title: 'Implementation: Realtime 3/5: entity-change triggers and auto lifecycle'
started: "2026-10-19"
completed: "2026-10-19"
status: done
---

<!-- @managed: claude-workflow v1 -->

## Development

- [x] Unit tests written for new code: trigger engine, seen-set, queue, retries and recovery (`internal/realtime/trigger_test.go`, `seen_test.go`, `system_test.go`), auto.when (`internal/realtime/trigger_auto_test.go`), load checks (`internal/realtimecfg`), append RequireOpen (`channelstest`)
- [x] Integration tests written: the realtime worker runs over a real store watcher, channel store and KV in `internal/appbuild/realtime_triggers_internal_test.go`, including two workers sharing one leader lock, takeover and a round budget; postgres-tagged tests cover the held advisory lock and pgchannels
- [x] Happy path implemented: triggers (PR #1933) and auto.when (T5b PR stacked on #1933)
- [x] Edge cases from planning handled: silent baseline for new or edited rules, when_closed skip or post, delete then restore posts again, per rule and entity cooldown, a shared 64-attempt cap, time-bounded retries, corrupt index or shard recovery, lost leader session, stale queued posts, trigger posts landing before an auto close, a manual reopen standing
- [x] Error handling in place: permanent channel errors give a post up at once, transient ones retry with backoff for 5 minutes and are then logged and marked seen; a failing store stops the round early

## Test Quality

- [x] Using fixture builders or factories for test data: the realtime test harness and the appbuild trigger fixture
- [x] No hardcoded values in assertions when object is in scope
- [x] Only specifying values that matter for the test
- [x] Interpolated values constructed from objects, not hardcoded
- [x] Property comparisons use original object, not hardcoded strings

## Manual Verification

- [x] ~~Feature manually tested end-to-end~~ (N/A: no UI ships in this ticket; the worker runs end to end in-process over the real store, watcher and channel store in the appbuild tests)
- [x] Each acceptance criterion verified with test scenario from planning: each T5 acceptance criterion in plan section 8 has a named test in the files above
- [x] ~~Edge cases manually verified~~ (N/A: no UI ships in this ticket; edge cases are covered by the takeover, cooldown, retry, recovery and budget tests listed above)

**Verification Evidence:** Local checks ran on each slice: build and vet on the
default, `postgres` and `sqlite` tags; `go test -race` on the touched packages;
the postgres-tagged tests against a local database and the sqlite appbuild
tests; golangci-lint on all four tag sets; `just arch-lint`; `just
comment-lint`; `just plimsoll`; docs regenerated with `just docs`. PR #1933
passed all 29 CI checks.

## Quality

- [x] Code follows project patterns (check similar code): consumer-side interfaces (`EntitySource`, `KV`), reads through `visibility.NewAllowAllReader` under the `system:realtime` principal, state in `state.KV`, one leader lock per D1; arch-lint and plimsoll clean
- [x] Checked for DRY opportunities: triggers and auto rules share one engine; person requests, auto.after and auto.when share `opFrom` and `applyTransition`; the load-time and runtime message checks share one helper
- [x] No security issues introduced: rela-security-reviewer ran on both slices; all critical and significant findings are addressed
- [x] No silent failures (errors logged AND returned)
- [x] No debug code left behind

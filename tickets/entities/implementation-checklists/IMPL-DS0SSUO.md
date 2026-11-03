---
id: IMPL-DS0SSUO
type: implementation-checklist
title: 'Implementation: Inbox 6: cross-process live delivery'
started: "2026-11-02"
completed: "2026-11-02"
status: done
---

<!-- @managed: claude-workflow v1 -->

## Development

- [x] Unit tests written for new code (`internal/realtime/hints_test.go`,
`internal/channels/filechannels/hints_test.go`,
`internal/store/pgstore/notify_test.go`,
`internal/appbuild/channelhints_internal_test.go`)
- [x] Integration tests written (test full flow, not just units):
`internal/appbuild/channelhints_test.go` (two Services over one project, file
and postgres builds), `channelhints_fs_test.go` (AC8),
`internal/cli/channel_hints_test.go` (AC5d, `rela channel post` in a subprocess,
file and postgres builds)
- [x] Happy path implemented
- [x] Edge cases from planning handled
- [x] Error handling in place (errors surfaced, not swallowed): send errors
are logged at most once a minute and never fail a write, by design (AC8)

## Test Quality

- [x] Using fixture builders or factories for test data
- [x] No hardcoded values in assertions when object is in scope
- [x] Only specifying values that matter for the test
- [x] Interpolated values constructed from objects, not hardcoded
- [x] Property comparisons use original object, not hardcoded strings

## Manual Verification

- [x] Feature manually tested end-to-end
- [x] Each acceptance criterion verified with test scenario from planning
- [x] Edge cases manually verified

**Verification Evidence:**

AC9, file backend: a temp project with one `item` and a `room` channel type.
rela-server ran on it (log: `realtime: channel subscribers resync
reason="follower start"`). A client opened `/api/v1/_events`, subscribed
`/api/v1/items/ITEM-001/_channels/room`, and `rela channel post ITEM-001 room
text` ran as a separate process. The stream received `event: live:changed`
`{"key":"head"}` for each post, the second one 420 ms after the CLI process
exited. Before this change rela-server sent no frame for a post from another
process. The postgres two-server manual check was not done; the postgres-tagged
integration tests (two Services on one schema, and the CLI subprocess test)
cover the same path against a local PostgreSQL.

Postgres benchmark (`BenchmarkChannelPost`, local PostgreSQL, 1000 posts per
run, 5 runs each, Apple M-series): with hints 0.83 to 0.89 ms/op, with no hint
endpoint 0.75 to 0.82 ms/op. A first run gave the opposite order (0.60 vs 0.80
ms/op), so the difference is within run-to-run noise.

DB-gated tests ran locally against Postgres.app (`RELA_TEST_DATABASE_URL`):
`TestNotify_*`, existing listener tests, `TestChannelHints_*` and
`TestChannelCLI_PostReachesOtherProcess` with `-tags postgres`.

**Deviations from the plan:**

- Flush at exit: `Services.Close` flushes queued hints (at most 3 s) instead
of each channel write command calling `Flush`. Every CLI command defers
`svc.Close()`, so this covers all of them with one call site.
- `ConfirmTombstones` sends no mark: it changes no live channel, and a
resync on every entity create would be costly.
- Restoring a tombstoned batch after a failed delete marks every channel
(`MarkAll`, a resync hint): the undo does not have the batch's keys at hand.
- A rename lists the entity's channels before the rekey and marks them
under both ids (moved). If the list fails, it marks every channel.
- Integration test for a session move between processes: not added as a
separate case. A session move uses the same moved kind as rename and delete,
which the integration test covers end to end; the unit tests cover the session
move mark.
- A test subscriber must exist before the follower starts: the follower's
start resync otherwise lands on a later subscriber and subsumes the first real
hint. Tests subscribe first. Production consumers treat a resync as a full
recheck, so this costs nothing there.
- `rela channel watch --poll` help text now says the poll is a recheck.

## Quality

- [x] Code follows project patterns (check similar code): `FollowChannels`
mirrors `WatchLeases`; the relay is shared state like `realtimeLocal`
- [x] Checked for DRY opportunities
- [x] No security issues introduced: hints carry channel keys and
principal keys only, never message content; decode validates every key
- [x] No silent failures (errors logged AND returned): send errors are
logged and deliberately not returned (AC8)
- [x] No debug code left behind

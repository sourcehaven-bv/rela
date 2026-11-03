---
id: PLAN-69I0GGV
type: planning-checklist
title: 'Planning: Inbox 6: cross-process live delivery'
started: "2026-11-02"
completed: "2026-11-02"
status: done
---

<!-- @managed: claude-workflow v1 -->

## Understanding

- [x] Problem/requirements clearly understood
- [x] Scope defined (what's in/out documented below)
- [x] Acceptance criteria documented with specific test scenarios

**Problem.** Live channel delivery is process-local. `realtime.Local` holds the
change sets of live subscribers, and only writes made in the same process mark
them (`internal/realtime/local.go`). A post, edit, session move or lifecycle
move made by another process (stdio `rela mcp`, `rela channel`, a second
rela-server on postgres) reaches open browsers only when they fetch the channel
again (`docs/realtime.md`, "Agents and other processes"). The inbox and busy
boards catch up through a periodic safety replay (about 12 s on files, 14 s on a
database). A coding agent that posts through stdio `rela mcp` while people watch
the channel in the web app is the motivating case.

**Terms.**

- **Hint**: a small record that says "these channels changed, of this kind".
It carries no channel content. The receiver replays its subscriptions as each
subscriber, as it does for a local write, so a hint cannot show anything the
subscriber could not already read.
- **Feed**: the transport that carries hints between processes of one
project.
- **Resync**: the existing `Subscription` flag that makes a subscriber recheck
every channel it follows.

**Scope: what happens per backend.**

| Change | Files (default) | PostgreSQL | SQLite | Memory |
| --- | --- | --- | --- | --- |
| Message append, edit, removal, action | Hint through a journal file; live in rela-server, desktop, stdio `rela mcp` and `rela channel watch` within about 0.25 s plus one hub tick | Hint through NOTIFY; live in every process within one hub tick | Unchanged: one process only | Unchanged: one process only |
| Session move, create, lifecycle move, delete, timer, sweep | Same, with the moved kind, so busy boards refresh within one tick | Same | Unchanged | Unchanged |
| Anchor rename or delete (rekey, tombstone), tombstone confirm, retention purge | Same, moved kind (these sites do not mark today; the plan adds the marks) | Same | Unchanged | Unchanged |
| Inbox (mentions, replies, read marks, waiting) | Hints carry addressed principal keys and read-mark readers; the inbox replays at its next grid step (4 s) instead of the 12 s safety turn | Same, grid step 2 s instead of 14 s | Unchanged | Unchanged |
| Typing, ephemeral signals | Stay process-local (by design, `signals_not_kept`) | Same | n/a | n/a |

Why each is included:

- Files: the default backend, and the one where stdio `rela mcp` and
`rela channel` share a project with rela-server or the desktop app. Without it
the motivating case is not fixed.
- PostgreSQL: the production backend, and the only one with several
rela-server processes. Agents there may use stdio `rela mcp` or a second server.
- SQLite: excluded. It admits one process (DEC-LFSYNY); agents use remote MCP,
which runs inside rela-server and is already live.
- Memory: excluded. A channel store in process memory cannot be shared.
- The "Files" column applies to the default build. In the postgres build, a
store that is not pgstore gets in-memory channels (`storeChannelsFor`,
`channels_postgres.go`), so it has no feed.

**In scope.**

1. A hint record in `internal/realtime` for every durable mark. It has one of
four shapes: channel keys with a kind (message or moved); one channel key with
its kind and the addressed principal keys; one reader with the keys whose read
marker moved; or a resync. Each hint has an origin id. Ephemeral marks are never
sent.
2. `Local` queues a hint after each durable mark when it has a sender. A
background goroutine sends it. Hints from other processes come in through a new
receive path that marks local subscribers and never sends again.
3. The write sites that do not mark today get marks: the alias hooks
(`Rekey`, `Tombstone`), `ConfirmTombstones`, the guard's `Tombstone` and
retention `Purge`.
4. A file feed for the file channel store: an append-only journal in the
channel directory, followed by polling its size.
5. A PostgreSQL feed: `pg_notify` on a second channel name, `rela_channels`,
received on the LISTEN connection pgstore already holds.
6. One follower per `Local`, started by long-running hosts: rela-server, the
desktop app, stdio `rela mcp` and `rela channel watch`, next to `WatchLeases`.
Short-lived commands that write (`rela channel post`) flush the sender before
they exit.
7. Resync when the feed reports a gap: a pg listener reconnect, a journal
rotation or truncation, an unreadable line, an oversized hint, a full queue, a
follower start. Gap resyncs are limited to one per 5 s.
8. Docs: `docs/realtime.md` (process locality, busy table, inbox timing, MCP
watch), `docs/postgres-backend.md` (LISTEN channels, what hints expose).

**Out of scope.**

- SQLite and memory backends (see above).
- Typing and ephemeral signals across processes, and rate limits across
processes. Both stay process-local by design.
- Removing the safety nets: the MCP watch channel poll (2 s / 5 s), the inbox
and busy safety replay and the `rela channel watch` `--poll` stay, because hints
are best effort.
- Exactly-once or ordered delivery. A hint lost without a detectable gap (a
process that dies between the store write and the send) shows at the next
refetch, or at the next safety turn for inbox and busy boards.
- Network filesystems for the file feed (O_APPEND is not atomic there; the
file channel store already assumes a local disk).

**Acceptance Criteria:**

1. Files: with two assemblies over one project directory (two processes in
the manual test), a post through one wakes a `SubscribeLive` subscription of the
other with the channel key in `Keys` and `Durable`, not in `Moved`.
2. Files: a session move through one reaches the other's `Moved` set.
3. Files: a post that mentions `integration:x` reaches the other's
`TakeAddressed`; a mark read reaches its `TakeReads` for that reader and leaves
other subscribers' `Keys` and `Durable` empty.
4. A process never acts on its own hints (origin check), and a received hint
is never sent again (no echo loop).
5. Ephemeral marks (typing, signals, expiry, the typing drops in
`EntityRenamed`) send no hint. 5a. Files: a rename or delete of a channel anchor
in one process reaches the other's subscribers with the moved kind. 5b. A mark
never waits on the network: the write returns while the sender is blocked. A
full queue turns into one resync hint. 5c. A schema reload keeps exactly one
follower per `Local`, and hints keep flowing through the new generation's store.
5d. `rela channel post` delivers its hint before the process exits.
6. File feed: rotation, truncation, a torn line or a missing file make the
follower resync, never crash or skip silently; a follower that starts reads only
lines written after it started, then resyncs once.
7. PostgreSQL (DB-gated): a hint sent through one store reaches a follower on
a second store over the same schema; a hint from another schema is dropped; a
listener reconnect makes the follower resync after LISTEN succeeds; a listener
that failed at Open is reported as "feed unavailable"; the entity change feed on
`rela_changed` behaves as before.
8. A send failure never fails the write. It is logged, rate-limited.
8a. Gap resyncs run at most once per 5 s per `Local`, and each is counted and
logged.
9. Manual: rela-server on the file backend with a channel open in the
browser; `rela channel post` and stdio `rela mcp` `post_message` show the
message without a reload. The same on a postgres build with two rela-server
processes.
10. Docs describe the per-backend behaviour above.

## Research

- [x] ~~For larger features: run `/research` to create a structured research doc~~ (N/A: RES-B0MJAZ section (k) and slice 6 already decide the shape; this plan records the deviations)
- [x] Searched for existing libraries that solve this problem
- [x] Checked codebase for similar patterns or reusable code
- [x] Looked for reference implementations in other projects
- [x] Reviewed relevant rela concepts for prior art

**Research Doc:** RES-B0MJAZ (section (k) "Several server processes", slice 6).

**Existing Solutions:**

- `internal/store/pgstore/feed.go`, `listener.go` (TKT-WZYWM9, TKT-9TOEBH):
the entity change feed. One constant NOTIFY channel, schema and origin in the
payload, one dedicated LISTEN connection per store, catch-up on reconnect.
Reused: the connection, the schema filter, the reconnect loop.
- `internal/lease/wake.go` (`Service.Watch`): cross-process wake by polling
a version, started by long-running hosts through `appbuild.WatchLeases`. Reused:
the host wiring pattern.
- `internal/realtime/local.go`: `markAs`, `markAddressed`, `markFor`, the
resync flag. Most write sites already report their kind there. The alias hooks,
`ConfirmTombstones`, the guard's `Tombstone` and retention `Purge` do not
(`appbuild/channels.go:131,153,156,228`, `channelguard.go:205`,
`realtime/retention.go:341`); the plan adds marks there, so `Local` becomes the
one seam that sees every durable channel change.
- `internal/channels/filechannels`: `RootedFS.OpenForAppend`, `tail.go`
reading at an offset (`io.ReaderAt`), `_`-prefixed store directories that walks
skip (`file.go:1022,1103`).
- Libraries: fsnotify is already used (`internal/storage/watcher.go`) but a
recursive watch of every channel directory costs one descriptor per directory
with kqueue on macOS and is unreliable on some filesystems. A single journal
file polled by size needs no library.
- Reference designs: PostgreSQL LISTEN/NOTIFY as a cache-invalidation hint
with a resync on reconnect (common pattern, e.g. Hasura, PostgREST schema cache
reload); append-only journal tailing (like `tail -F`, which also detects
rotation by file identity).

## Approach

- [x] Technical approach chosen and documented
- [x] Approach builds on existing patterns (not reinventing)
- [x] Alternatives considered (document why rejected)
- [x] Dependencies identified (packages, APIs, types)

**Technical Approach:**

1. `internal/realtime/hints.go` (new).
   - Consumer-side interfaces: `HintSender { SendHint(ctx, payload []byte)
error }` and `HintSource { FollowHints(ctx, deliver func([]byte), gap func())
error }`. The names avoid `changefeed` and `pgstore/feed.go`.
   - `hint` struct, JSON encoded, with origin and one of four kinds:
     - `m` (message) or `v` (moved): a list of channel keys.
     - `a` (addressed): one channel key, its kind (`m` or `v`), and the
addressed principal keys. It maps to one `markAddressed(targets, key, kind)`
call.
     - `r` (read): one reader (user, org) and that reader's keys. It is
applied only through `markFor`, never through `markAs`.
     - `s` (resync).
   - Caps: at most 64 keys, 64 addressed keys, 7000 bytes per hint. A mark
past a cap becomes a resync hint.
   - Decoding validates each key and reader with the existing `channels`
validators. A hint that fails decoding is treated as a gap (resync), never as
partial data.
2. `Local` changes (`local.go`).
   - `NewLocalWithHints(clock, sender)` (nil sender rejected); `NewLocal`
keeps no sender. A random origin id per `Local`.
   - The internal mark functions (`markAs`, `markAddressed`, `markFor`) take
a `send bool` argument. Public mark methods pass true; the receive path and the
typing drops in `EntityRenamed` pass false. No flag on `Local`, so a concurrent
local write is never affected.
   - Marks only enqueue. A sender goroutine owns a bounded queue (256). It
merges entries over about 20 ms into one hint per kind (and per addressed key or
reader), and sends with a 2 s timeout. A full queue drops the entries and queues
one resync hint. A send error is logged at most once a minute and never
returned. So no mark site, including those under the outputs lock
(`outputs_runner.go:108`, `service.go:537`), waits on the network, and
`notePosted`'s two marks become one send.
   - `Flush(ctx)` drains the queue; `Close(ctx)` flushes and stops the
sender. Short-lived CLI commands that write call `Flush` before exit.
   - `FollowHints(ctx, src)` is single-flight per `Local`, with a `running`
guard like `realtime_worker.go:369-375`. A second call returns at once. It
resyncs every subscriber once at start. For each payload it drops its own origin
and applies the hint through the internal mark functions with `send=false`.
`gap` and `s` hints set resync on every subscriber (new `markResync`), at most
once per 5 s; extra gaps inside the window are folded into one resync at its
end. Each resync is counted and logged.
3. File feed: `internal/channels/filechannels/hints.go` (new).
   - Journal `<root>/_feed/changes.jsonl`, mode 0600. `SendHint` appends one
line with one `OpenForAppend` write.
   - Rotation runs under the store's file lock. Past 1 MiB the writer renames
the journal to `changes.old.jsonl`. If the rename fails (Windows with an open
handle) and the journal is past a 4 MiB hard cap, the writer truncates it under
the lock instead. Followers see the shrink and resync. No Windows test runner
exists in CI, so the failed-rename path is tested with an injected rename error.
   - `FollowHints` polls `Stat` every 250 ms. It remembers the file identity
(`os.SameFile`) and offset, reads new complete lines with `ReadAt`, and reports
a gap on identity change, shrink, or a line it cannot use. The shrink check and
line validation stay, because a new file can reuse the old inode number. It
starts at the current size.
   - Built only when the store has a real file lock (not `MemFS`).
4. PostgreSQL feed: `internal/store/pgstore/notify.go` (new), `listener.go`.
   - pgstore exposes a generic pair, limited to a fixed list of channel
names (`rela_channels` only for now): `Notify(ctx, st, channel, body)` and
`Listen(st, channel, deliver, gap) (cancel, error)`. The channel concept stays
out of pgstore.
   - The schema is resolved at Open on its own, not only by
`startListener`. If the listener failed at Open, `Notify` and `Listen` return
`ErrListenerUnavailable`, and the host logs "channel hints unavailable".
   - The listener routes on `n.Channel` first. For `rela_channels` it splits
the payload on the first separator only and drops other schemas. A channel hint
never falls through to the entity catch-up.
   - On reconnect: LISTEN on both channels, then call `gap`, then run the
entity catch-up. `deliver` must not block (it runs on the listener goroutine
that also runs the catch-up); the realtime side hands it to a buffered channel
and turns an overflow into a gap.
   - The entity feed on `rela_changed` keeps its format and filters.
5. Wiring: `internal/appbuild/channels.go`, `channels_postgres.go`,
`piles.go`, new `channelhints.go`.
   - The shared state next to `realtimeLocal` holds a `hintRelay`: one
sender and one source that `Local` uses for its whole life. Each assembly
attaches its store's file or pg hint endpoints to the relay (atomic swap). The
relay's source re-follows the new endpoint and reports a gap on swap. An old
generation's `Close` detaches only its own endpoint. So a store rebuilt by a
reload never leaves `Local` sending on a closed listener.
   - The shared `realtime.Local` is built with `NewLocalWithHints` when the
backend has hints (file store, pgstore); otherwise with `NewLocal`.
   - `channelAliases` calls `Local.Mark` (moved) for the keys that `Rekey`
and `Tombstone` change. Where the store does not return the keys, it queues a
resync hint. The same for `ConfirmTombstones`, the guard's `Tombstone` and
retention `Purge`.
   - `appbuild.FollowChannels(ctx, s) <-chan struct{}` next to
`WatchLeases`, called from `cmd/rela-server/serve.go`,
`cmd/rela-desktop/main.go`, `internal/cli/mcp.go` and
`internal/cli/channel_watch.go`. Because `FollowHints` is single-flight,
overlapping generations in rela-server (`serve.go:281-307`) and a generation
held by `haltScheduler` do not start a second follower or stop the first.
   - `rela channel post` (and the other channel write commands) call
`Local.Flush` before exit.
6. Docs as listed under Documentation Impact.

**Files to modify:**

- `internal/realtime/local.go`, `retention.go`, new
`internal/realtime/hints.go`, tests
- new `internal/channels/filechannels/hints.go`, tests
- new `internal/store/pgstore/notify.go`, `listener.go`, `open.go`,
`pgstore.go`, tests
- `internal/appbuild/channels.go`, `channelguard.go`, `channels_postgres.go`,
`channels_nostore.go`, `piles.go`, `realtime_service.go`, new `channelhints.go`,
integration tests
- `cmd/rela-server/serve.go`, `cmd/rela-desktop/main.go`,
`internal/cli/mcp.go`, `internal/cli/channel_watch.go`, the channel write
commands in `internal/cli`
- `docs/realtime.md`, `docs/postgres-backend.md`, `docs/mcp-server.md` if it
states the poll timing
- `.testcoverage.yml` only if a new package needs a floor (none expected)

**Alternatives considered:**

| Alternative | Why rejected |
| --- | --- |
| Send `Local.Mark` (moved kind) for every remote change, as the TKT-W5ZSYJ note suggests | The sender knows the kind at its mark site, so the hint can carry it. Moved for every message would replay busy boards on each remote post. |
| Send from the store backends (inside the pg transaction, under the file lock) | Atomic with commit, but the kind and the addressed principals are known only in the realtime service, and it touches every mutating method of two backends. Local is the one seam every write site already reports to. |
| Reuse `rela_changed` with a new payload kind (RES-B0MJAZ (k)) | The current and older pgstore listeners treat an unknown payload as unparseable and run a catch-up query for each one. A second channel name on the same LISTEN session costs no connection and leaves the entity feed untouched. |
| A LISTEN connection of its own in pgchannels | One more held connection per process and schema, the cost TKT-9TOEBH removed. |
| fsnotify on channel directories | One watch descriptor per channel directory, unreliable on some filesystems. |
| A version counter file (like leases) | Says only "something changed", so every subscription of every viewer would resync on each post. |
| Poll the head of each subscribed channel | One file lock per followed channel per tick. |

**Dependencies:** none new. Uses `encoding/json`, `crypto/rand`, pgx (already
present).

## Security Considerations

- [x] Input sources identified (user input, config, external APIs)
- [x] Input validation approach defined (allowlist preferred over blocklist)
- [x] Security-sensitive operations identified (file access, auth, crypto)
- [x] Error handling doesn't leak sensitive information

**Input Sources & Validation:**

- Hints from the journal file and from NOTIFY. Writers are other rela
processes with access to the project directory or the database. A forged hint
cannot disclose anything: every replay runs as the subscriber and re-gates.
Decoding validates keys and readers with the `channels` validators, caps counts
and bytes, and treats any failure as a resync.
- Availability: anyone who can append to `.rela/channels/_feed` or NOTIFY
with a chosen schema prefix can force resyncs, and a resync replays every
channel, busy and inbox subscription. Gap and resync hints are therefore limited
to one resync per 5 s per `Local`, counted and logged. Plain `m` and `v` hints
for many keys are capped at 64 keys each and cost the same as local writes.
- NOTIFY payloads from other schemas on the same database are dropped by the
schema prefix, as the entity feed does.
- Exposure: NOTIFY delivers each payload to every listener on the database,
in every schema. Hints carry channel keys (entity ids) and addressed and reader
principal keys. The entity feed already sends entity ids this way. Hashing the
principal keys would not help, because the salt (the schema) is public and
principal ids are guessable. `docs/postgres-backend.md` states this: schemas
that share a database share hint metadata, so tenants that must not see each
other's ids need separate databases.

**Security-Sensitive Operations:**

- File access goes through `storage.RootedFS` under the channel directory,
mode 0600. The journal holds channel keys (entity ids) and principal keys, the
same data the channel directory and mention rows already hold. Rotation bounds
it to about 2 MiB.
- No channel content, message text or property value is ever in a hint.
- Errors are logged without payloads.

## Test Plan

- [x] Test scenarios documented for each acceptance criterion
- [x] Edge cases identified and documented
- [x] Negative test cases defined (invalid input, error conditions)
- [x] Integration test approach defined (not just unit tests)

**Test Scenarios:**

- AC1-5: `internal/realtime` unit tests with an in-memory fake sender and
source that connect two `Local`s: message vs moved kinds, addressed, reads,
origin drop, no re-send of received hints, ephemeral sends nothing (including
the `EntityRenamed` typing drops), overflow past caps sends resync.
- AC3: a remote mark read leaves other subscribers' `Keys` and `Durable`
empty and fills only that reader's `TakeReads`.
- AC5b: a blocking fake sender; the mark returns at once; a full queue
yields one resync hint; `notePosted` sends one hint. `Flush` and `Close` drain
the queue.
- AC6: `filechannels` tests over a temp dir: send/follow round trip, partial
last line waits for its newline, rotation under the lock, a failed rename past
the hard cap truncates, truncation, torn line, missing file then created, start
offset. Concurrent appends from separate processes (the test re-executes its own
binary as helper processes), not goroutines sharing one handle.
- AC7: pgstore DB-gated tests (`RELA_TEST_DATABASE_URL`, run locally with
Postgres.app and in CI's Postgres job): two stores on one schema, a foreign
schema, a forced reconnect (terminate the listener backend) gives a gap only
after both LISTENs succeed, a listener that failed at Open returns
`ErrListenerUnavailable`, a payload with the separator inside the body, the
existing listener tests stay green.
- AC1-3, 5a integration: `internal/appbuild` test with two `Services` over
one temp project (file backend): post, move a session, mark read, and rename and
delete a channel anchor through one service; the other's `SubscribeLive`
receives the keys with the right kind. A postgres-tagged twin over one schema.
- AC5c: a schema reload in an `appbuild` test keeps one follower (a second
`FollowChannels` returns at once) and hints still arrive after the swap.
- AC5d: a CLI test runs `rela channel post` as a subprocess against a
project with a follower in the test process; the hint arrives.
- AC8: a failing fake sender; the post succeeds and the log has one line.
- AC8a: a burst of gap and `s` hints yields one resync per 5 s window, with
a fake clock.
- Post latency on postgres: a DB-gated benchmark of `PostMessage` with and
without hints; the result goes in the implementation checklist.
- AC9: manual, recorded in the implementation checklist.

**Edge Cases:**

- A hint for a channel no subscriber follows: marks cost nothing more.
- Several keys in one mark (sweep, retention): one hint, or resync past 64.
- Bursts: the sender merges marks over about 20 ms; the hub coalesces by
tick.
- Follower start while writers run: start at current size, resync once.
- Two writers rotating at once: rotation runs under the store's file lock,
so only one renames. The follower sees an identity change and resyncs.
- Overlapping assemblies during a reload: one follower; the relay swaps
endpoints and reports a gap.
- Process stops between write and send: no gap visible; documented.
- Long principal keys or many mentions: caps and the 7000-byte limit (pg
limit is 8000) turn the hint into a resync.

**Negative Tests:**

- Malformed JSON, unknown kind, invalid key, oversized line: resync, no
panic, no partial marks.
- Payload from another schema: dropped.
- Feed send error: write succeeds.
- `NewLocalFeed` with a nil feed: error.

## Risk Assessment

- [x] Technical risks assessed with mitigations
- [x] Security risks assessed (see Security Considerations)
- [x] Effort estimated (xs/s/m/l/xl)

**Risks:**

- Write latency: none on the write path, because marks only enqueue. The
sender does one NOTIFY round trip (one pool connection) or one file append per
merged batch. Mitigation: 2 s timeout, no fsync on the journal, errors never
fail the write; the postgres benchmark measures the effect.
- Hints lost at exit of a short-lived command. Mitigation: `Flush` before
exit in channel write commands.
- Replay load in rela-server grows with remote writes, as if they were local.
This is the intended behaviour; the hub already coalesces by tick and caps
rounds.
- Lost hints without a gap signal. Mitigation: resync on every detectable gap;
existing safety replays and MCP polls stay.
- Regression in the pgstore entity feed from routing two channels on one
connection. Mitigation: route by `n.Channel` before parsing; existing listener
tests and a new test for both channels.
- Mixed versions during a rolling deploy: an old process neither sends nor
follows; behaviour for it is as today.
- Windows rename over an open journal can fail. Mitigation: the reader opens
per poll and closes; a failed rotation is retried on the next send; past the 4
MiB hard cap the writer truncates under the lock.

**Effort:** m (realtime and file feed), plus pg listener work: l overall.

## Documentation Planning

- [x] User-facing docs identified (skip if internal refactor)
- [x] Docs-checklist will be created when entering implementation

**Documentation Impact:**

- [x] `docs/realtime.md`: "Agents and other processes" (process locality),
the live-frames paragraph before it, the busy-board backend table, inbox live
timing, the mention wake and MCP watch wording.
- [x] `docs/postgres-backend.md`: the listener now LISTENs on two channels;
hint metadata is visible to every listener on the database.
- [x] `docs/mcp-server.md` (or the MCP guide): watch timing, if stated there.
- [ ] ~~CLI reference~~ (N/A: no CLI change)

## Design Review

- [x] Run `/design-review` before starting implementation
- [x] All critical/significant findings addressed in plan

**Design Review Findings:** 11 findings (7 significant, 2 minor, 2 nit), all
addressed in this plan.

1. Read marks applied as message changes: new `r` and `a` hint kinds,
applied only through `markFor` and `markAddressed` (Approach 1).
2. Durable writes that never mark: marks added in the alias hooks,
`ConfirmTombstones`, the guard and retention `Purge` (Approach 5).
3. Synchronous network sends at mark sites: sender goroutine with a bounded,
merging queue; `Flush` for CLI commands (Approach 2).
4. Feed per assembly vs shared `Local`: `hintRelay` in shared state;
single-flight follower (Approach 2, 5).
5. Postgres listener edge cases: schema resolved apart from the listener,
`ErrListenerUnavailable`, LISTEN then gap then catch-up, routing on `n.Channel`
(Approach 4).
6. Replay storms and NOTIFY exposure: resync limit, exposure documented
(Security).
7. Journal rotation: rotation under the file lock, 4 MiB hard cap, shrink
check kept (Approach 3).
8. Re-sending received hints: `send` argument on internal mark functions
(Approach 2).
9. Names: `HintSender`, `HintSource`; generic pgstore `Notify`/`Listen`.
10. Missing tests: added under Test Scenarios.
11. Backend table: build limit stated; `rela channel watch` included.

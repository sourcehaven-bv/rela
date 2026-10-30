---
id: PLAN-C9AHJ7
type: planning-checklist
title: 'Planning: Busy board live refresh on session moves'
started: "2026-10-30"
completed: "2026-10-30"
status: done
---

<!-- @managed: claude-workflow v1 -->

## Understanding

- [x] Problem/requirements clearly understood
- [x] Scope defined (what's in/out documented below)
- [x] Acceptance criteria documented with specific test scenarios

**Problem.** TKT-21G6JS added the `busy:` kanban card field. The board asks for
`?busy=<channel type>` and each row carries `_busy`. The board's live
subscription is replayed only by a full round, which a graph write, a lease
change or a reload starts. A session move that writes no graph output starts
only a topic round, and a topic round never selects a list subscription
(`roundFilter.selects`, `internal/dataentry/live_topics.go:137`). So an open
board shows such a move only after a reload or the next unrelated graph write.
`TestLive_BusyListIgnoresChannelRound` pins that gap today.

The case that motivates the ticket is the agent-runner PoC. It posts through
stdio `rela mcp --run-as integration:runner`, a separate process, on the default
file backend. Its writes never reach rela-server's `realtime.Local`. So an
in-process wake alone does not fix the PoC board. This plan adds two parts: a
wake for moves made inside the server process, within one tick, and a staggered
safety replay that bounds staleness for moves from any other process, on every
backend.

The TKT-21G6JS design review (findings 1 to 3, `.ignored/dr-21g6js.tsv`) moved
the live wake here and named three conditions:

- *Live wake.* Selecting by channel type alone would replay every busy board
on every write to any channel of that type, including message appends and
channels on anchors the subscriber cannot see. Selection must go through the
anchors on the subscriber's own gated page.
- *Fallback oracle.* When a round has no room or time for a replay,
`liveHub.fallback` sends a refetch frame without a check
(`internal/dataentry/live.go:904`). If a change the subscriber cannot see chose
that replay, the frame tells the subscriber the change happened. The inbox is
exempt for this reason (`live_inbox.go:46`). A busy board needs the same
treatment in the rounds where a change can choose it.
- *Syncing gap.* While a PUT takes a subscription's baseline, the
subscription is `syncing` and rounds skip it. A changed round sets `recheck` so
the PUT leaves it pending for the next tick. A topic round sets `recheck` only
on presence, channel and inbox subscriptions (`live.go:850`), because "a change
of a channel moves no other response". That stops being true for a list that
carries `_busy`.

**How live views work today (from the code).**

- *Rounds.* The live worker (`liveHub.run`, `live.go:630`) runs on a fixed
500 ms clock. A graph write, lease wake or reload marks the round dirty, and the
next tick runs a full round: every subscription is replayed as its principal and
its hash compared. A channel change does not dirty the round. It runs a topic
round (`replayRound` with a `roundFilter`) over what changed. The ticker drops
ticks while a round runs, so rounds never overlap.
- *Channel signals.* `realtime.Local` keeps one change set per subscriber.
The hub's subscription comes from `Local.SubscribeLive`
(`internal/realtime/local.go:160`) and offers `Take` (every changed key,
ephemeral included), `TakeReads` (moved read markers per reader), `TakeDurable`
(keys a stored write changed: posts, edits, removals, actions, session and
lifecycle moves) and `TakeAddressed` (mention and reply targets). Past
`realtimelimits.MaxSubscriberChanges` (1024) keys the subscriber gets `resync`
instead.
- *Topics.* A channel subscription learns its topic from its own 200 replay
through a `topicProbe` in the request context (`recordTopic`, `applyTopic`). A
topic never comes from the client, and a refused replay clears it, so a channel
change never replays a subscriber who may not read the channel.
- *Inbox.* An inbox replay records an `inboxWake` (principal key, reader,
waiting types, channel keys of its window) through the same probe
(`recordInbox`, `live_inbox.go:193`). `selectsInbox` matches the round's durable
keys, addressed keys and moved markers against it. Inbox replays run only on the
inbox grid. An inbox subscription that a round has no room or time for goes to
pending with no frame (`fallback`, `live.go:912`), and the rotating cursor
(`replayRound`, `live.go:745`) makes sure it is replayed in turn.
- *Inbox safety replay.* `safetyDue` (`live_inbox.go:174`, called from
`collect` at `live.go:856`) takes every inbox subscription with a wake once per
safety period, so a write in another process reaches the badge. The period is
the largest multiple of the inbox grid at or below `liveInboxSafetyMax` (28
ticks): 24 ticks (12 s) over the file channel store, 28 ticks (14 s) over a
database. Each principal has a fixed slice from a stable hash of its share key,
and each grid tick replays the slices whose turn it is (`inboxClock.tickAt`,
`inboxSlice`). Which pairs a safety turn takes depends only on the clock and the
principal, never on a change.
- *Syncing.* `liveSub.syncing` is set by `replace` and cleared by
`baseline`. `collect` skips a syncing subscription and sets `recheck` per the
rule above; `baseline` then adds it to pending.
- *Busy read.* `serveRowBusy` (`internal/dataentry/busy_rows.go:43`) runs
inside `serveListRowExtras` on rows that already passed the row gate and
redaction. It calls `realtime.BusyReader.Busy` once per page, which gates each
channel again through the anchor reader and the type's `read:` role. A family
channel (`anchor` default) shows on every face row of its entity; a face channel
(`anchor: face`) only on the row of its face.
- *Board subscription.* `KanbanView.vue` (lines 236 to 254) loads every page
but subscribes with `page=1&per_page=100`. Its lease comment already says that
cards past the first 100 refresh only on entity changes.

**Terms used below.**

- *Busy subscription*: a live list subscription whose path carries `busy`.
- *Busy wake*: what a busy subscription's last 200 replay reports it listens
to. It holds the channel type the read served and the anchors of the rows on the
page. It is server-derived, like a topic.
- *Moved key*: the key of a channel whose stored write may change what a busy
read answers: a session move, a create, a lifecycle move, a delete, a tombstone
or restore, a timer write.
- *Message-only write*: a stored write that cannot change a busy answer: a
post that moved no session and created no channel, an edit, a removal, an action
or an outputs progress write.
- *Topic round*: a round with a `roundFilter` (`only != nil`). A change or a
safety turn chose its pairs. A *full round* (`only == nil`) takes every
subscription, whatever changed.

**Scope (in):**

- `internal/channels`: `Message.CreatedChannel`, a never-stored flag both
stores set on the message `Append` returns when the append created the implicit
channel.
- `internal/realtime`: a moved set on `SubscribeLive` subscriptions, read
with `Subscription.TakeMoved()`. Every stored write is moved by default; the
message-only sites say so explicitly.
- `internal/dataentry` live path:
  - a busy subscription records a busy wake from its own 200 replay;
  - a topic round selects a busy subscription only when a moved key has the
wake's channel type and an anchor that is on the wake's page, or when the
channel subscription lost count (resync);
  - the safety replay also takes each busy subscription with a wake, on its
principal's slice;
  - a busy subscription gets no fallback frame in a topic round; it goes to
pending instead, like an inbox subscription. In a full round it keeps the
fallback frame, like any list;
  - a syncing busy subscription is marked for a recheck when a round carries a
moved key of its type, or a resync.
- Tests for each of these, including the ACL cases, on the manual ticker.
- Docs: the "When it updates" paragraph of the realtime guide's busy
indicator section, and the live views and `?busy=` paragraphs of the data-entry
API reference.

**Scope (out):**

- A cross-process wake within one tick. A move made in another process
(stdio `rela mcp`, `rela channel`, a second server replica) never reaches this
process's `Local`. The safety replay bounds that delay to one safety period. A
faster path is TKT-YK30D1's postgres poke, which should feed the moved set (Q3).
- The SPA. The board already subscribes with `busy` in its params and
refetches on `live:changed`. No frontend change. Cards past the first 100 stay
outside the subscription, as they are for leases.
- The MCP watch tool and other consumers of `realtime.Subscription`. They
keep `Take`, `TakeDurable` and the rest as they are.
- Busy state on views (`_views`), calendars, gantts and list columns
(TKT-TU3XH4).
- Narrowing timer writes to moves only. A reminder counts as moved. See Q1.
- The inbox grid for busy subscriptions. See Approach, alternative 5.

**Acceptance Criteria:**

1. **The live subscription reports moved keys.** A `SubscribeLive`
subscription's `TakeMoved()` returns, sorted, the keys of stored writes that may
move a session or a lifecycle since the last call, and empties the set. A post
or trigger post whose returned message has `MovedSession` or `CreatedChannel`, a
channel create, a lifecycle transition, a delete, and every `Local.Mark` call
(sweep, timers, retention) are moved. A post that moved nothing, an edit, a
removal, an action, a promote and an outputs progress write are durable and not
moved. Typing and ephemeral signals are neither. Every moved key is also in
`Take` and `TakeDurable`. A plain `Subscribe` subscription returns nil. An
overflow of the shared limit resyncs and clears the moved set. Both stores set
`CreatedChannel` on the first append to an implicit channel only. Test:
`internal/realtime/live_subscription_test.go` `TestLocal_MovedKeys` table: "post
that moves"; "first post creates the channel"; "post that does not move";
"edit"; "remove"; "action"; "create"; "close"; "delete"; "Local.Mark"; "typing";
"ephemeral signal"; and `TestSubscribeLive` new cases "TakeMoved empties the
set", "a plain subscription takes no moved keys" and "overflow clears moved".
`TestPostMoved` covers the helper. `internal/channels/channelstest` case "append
reports CreatedChannel": true with seq 1 on the implicit create; false on the
second append and on the first append to an explicitly created named channel.
2. **A busy subscription learns its wake from its own replay.** After a 200
replay whose busy read was served, the subscription holds a busy wake: the
served channel type, the ids of the page's rows (for family channels) and their
`id@face` addresses (for face channels). A replay whose busy read was not served
(realtime off, unknown type, a type removed from `realtime.yaml`, no attached
row, store error) or an empty page clears the wake, though the list answers 200.
A non-200 answer clears it too. A replay that did not finish keeps it. A
non-busy list subscription never holds one. Test:
`internal/dataentry/live_busy_test.go` (new) `TestLive_BusyWakeSetFromReplay`
(served; empty page; realtime off; unknown type; "read revoked": the policy is
swapped so alice may no longer read `ticket`, the full round answers 403 or 404,
and the wake is nil; unfinished replay keeps it), and `TestRecordBusy` (a family
row and a face row give the expected keys; no probe on the context is a no-op).
3. **Selection matches moved keys on the page.** `roundFilter.selects` takes
a busy subscription when the round has a moved key whose type equals the wake's
type and whose anchor is on the page: a key with an empty face matches a row id,
a key with a face matches the row's `id@face`. It takes it on resync when it has
a wake. It never takes one without a wake. A durable key that is not moved, a
moved key of another type, and a moved key on an anchor that is not on the page
do not select it. Test: `live_busy_test.go` `TestRoundFilter_SelectsBusy` table
with each of those rows.
4. **An in-process session move frames the board.** Alice subscribes to
`/api/v1/tickets?busy=run` and to TKT-1's `run` channel. Each step posts through
the server's realtime service, then steps one tick:
   - "move that changes nothing": alice posts `go` (the first post, which
creates the channel and moves new to queued, not a working state). The board is
replayed once and gets no frame; the channel subscription frames in the same
round.
   - "claim": the runner posts `claim` (queued to planning, no outputs). The
board frames, and the refetch shows TKT-1 busy.
   - "plan": the runner posts `plan` (planning to plan-review). The board
frames and TKT-1 is not busy.
   - "first post starts working": on a board with `busy=job`, alice posts
`log` on TKT-1's `job` channel. The post creates the channel in its working
initial state and moves nothing, so only `CreatedChannel` marks it moved. The
board frames and TKT-1 is busy.
   - "create": on a board with `busy=task`, alice creates a `task` channel
on TKT-1. It starts working; the board frames and TKT-1 is busy.
   - "close" and "delete": alice closes, then deletes, TKT-1's `job`
channel. Each frames the `busy=job` board and TKT-1 is not busy after.
   - "Local.Mark": a `job` channel seeded straight in the store, then
`Local.Mark` on its key (what a timer or sweep calls), frames the board.

Test: `live_busy_test.go` `TestLive_BusyListWakesOnSessionMove` with those
subtests. It replaces `TestLive_BusyListIgnoresChannelRound` in `live_test.go`,
deleted in the same commit, since it asserts the opposite.
5. **Message-only writes and unrelated moves do not replay the board.** With
`countPrincipalReplays`, none of these replays alice's busy board: a post in
`run` that moves nothing, an edit, a removal, a typing start, a move in a `chat`
or `instant` channel while the board asks for `run`, and a `run` move on TKT-2
while the board shows only TKT-1 (`per_page=1`). Each case steps one tick in
which alice's TKT-1 `run` channel subscription frames, or is replayed, in the
same round. Test: `TestLive_BusyListIgnoresMessageOnly` subtests for each.
6. **ACL: a hidden anchor does not wake.** Under a policy where bob cannot
read TKT-2, alice and bob subscribe to the same busy URL.
- "hidden row": a move on TKT-2's `run` channel replays and frames
alice's board and replays bob's board zero times, in the same stepped tick.
Bob's wake, read through a test accessor under `c.mu` as `hashOf` does, has no
TKT-2 key; alice's has one.
- "face not on the page": bob's board in the published world shows
`PAGE-1@published`. A `facerun` move on `PAGE-1@draft` is made by a direct store
write and `Local.Mark` on its key, since no busy principal may post in the draft
world. In the same tick a `run` move on TKT-1 replays bob's second subscription,
`/api/v1/tickets?busy=run`, once. Bob's page board is replayed zero times.

Test: `TestLive_BusyHiddenAnchorNoWake` with those subtests.
7. **ACL: a principal-specific fold is not cached across principals.**
Alice and bob subscribe to the same busy URL with `busy=instant`. Both see
TKT-1; only alice, its owner, may read its `instant` channel. A move on TKT-1's
`instant` channel replays both boards, once each, as their own principals.
Alice's hash moves and she gets a frame. Bob's hash stays the same, he gets no
frame, and his `_busy` for TKT-1 is `[]` before and after. Alice's frame is the
positive control in the same stepped tick. Test:
`TestLive_BusyWakePerPrincipal`.
8. **No fallback frame in a topic round.** With `roundCap = 1` or
`roundBudget = -1`, a move that selects alice's busy board and another
subscription overflows the topic round. The busy board gets no frame and goes to
pending. When the limits are restored, the next tick replays it and frames it
because `_busy` moved. A full round that overflows (a graph write) gives the
busy board its fallback frame, like a plain list. A plain list without `busy`
gets its fallback frame in both kinds of round, as today. Test:
`TestLive_BusyOverflowNoFallbackFrame` (subtests "round cap", "round budget",
"full round frames"); the existing fallback tests in `live_hub_test.go` stay
green.
9. **A move during the baseline is not lost.** A busy subscription's PUT is
held while it takes its baseline. A `claim` move lands in that window. When the
PUT finishes, the subscription is pending, and the next tick replays and frames
it. A round with only message-only writes, or with moved keys of another type
only, sets no recheck on it. Test: `TestLive_BusyListRecheckWhileSyncing` and
`live_hub_test.go` `TestLiveHub_CollectRechecksBusy` table ("moved key of its
type", "resync", "message-only", "other type", "full round").
10. **Cost is bounded.** One replay per (principal, path) per round, however
many moves the round carries: 20 moves across TKT-1 and TKT-2 within one tick
replay alice's board exactly once on the next step. A busy wake holds at most
two keys per row of the page, so at most 200 for the largest page. Selection per
subscription costs at most min(wake anchors, round anchors of the type) map
lookups. Test: `TestLive_BusyWakeCoalesces`; `TestRecordBusy` asserts the key
count for a full page.
11. **Docs.** The realtime guide and the API reference say which session
changes refresh an open board within a tick, that a move from another process
reaches it within the safety period (about 12 s on files, 14 s on a database)
with or without outputs, that only the first 100 cards are watched, and that a
busy list gets no refetch frame from an overflowed topic round. Test: reviewed
on the docs checklist; `just docs` regenerates `docs/realtime.md`.
12. **A move outside `Local` reaches the board on its safety turn.** Alice
subscribes to `busy=instant`. An `instant` channel on TKT-1 is seeded straight
in the store with no `Local.Mark`, as another process would. No replay of the
board runs before alice's safety turn. On her turn the board is replayed once
and frames, since TKT-1 became busy. On the next safety turn, with nothing
changed, it is replayed once and gets no frame. A busy subscription without a
wake is not taken by the safety replay. Test: `TestLive_BusySafetyReplay` on the
manual ticker with `inboxSliceOf` fixed, modelled on `testInboxSafetyReplay`
(`live_inbox_test.go:702`), run for both grids (`LiveInboxGrid`,
`LiveInboxGridScan`).

## Research

- [x] ~~For larger features: run `/research` to create a structured research doc~~ (N/A: the design follows the inbox and topic patterns; choices are under Approach)
- [x] Searched for existing libraries that solve this problem
- [x] Checked codebase for similar patterns or reusable code
- [x] Looked for reference implementations in other projects
- [x] Reviewed relevant rela concepts for prior art

**Research Doc:** N/A

**Existing Solutions:**

- Libraries: none needed. No new Go or npm package.
- Codebase patterns reused:
  - `inboxWake`, `recordInbox` and `selectsInbox`
(`internal/dataentry/live_inbox.go`): a wake set reported by the subscription's
own 200 replay through `topicProbe`, matched against the round's keys. The busy
wake copies the shape.
  - The inbox safety replay (`safetyDue`, `inboxClock`, `inboxSlice`): a
fixed-clock, per-principal stagger that bounds cross-process staleness with no
oracle. Busy subscriptions join it unchanged.
  - `applyTopic` (`live_topics.go:182`): a 200 takes the probe's value, any
other status clears it, an unfinished replay keeps it. The busy wake follows the
same three rules.
  - The inbox's no-fallback rule in `liveHub.fallback` and the rotating cursor
in `replayRound`: a deferred replay frames only when its own hash moves, and the
rotation makes sure it runs.
  - `Local.markWhere` with the `durable` flag
(`internal/realtime/local.go:396`): one call adds a key to several sets of each
subscriber, all under one shared limit. The moved set is one more such set.
  - `channels.Message.MovedSession` (`internal/channels/channels.go:387`):
the store already reports whether an append moved the session.
`Message.ReplyTarget` (`channels.go:393`) is the precedent for a never-stored
field on the returned message, which `CreatedChannel` follows. Both stores
already compute `created` in `appendTarget` (`filechannels/ops.go:41`,
`sqlchannels/sql.go:455`) and only discard it.
  - `realtime.BusyReader.Busy` and its family versus face mapping
(`internal/realtime/busy.go`, `busyRun.collect`): the matching rule in AC 3
mirrors it.
- Reference implementations: Phoenix LiveView and Hasura live queries both
separate "what may have changed" (a cheap, over-approximate invalidation key)
from "what changed for this viewer" (a re-render or re-query as the viewer).
Hasura also polls on an interval as the floor. rela's live views already work
this way: selection is a cheap over-approximation, the per-principal replay and
hash decide the frame, and the safety replay is the floor.
- rela prior art: TKT-HJ5Z9Y (live views), TKT-ZU7P13 (topic rounds),
TKT-9YRRM5 (live inbox, its safety replay and its no-fallback rule), TKT-21G6JS
(`busy:`, `?busy=`, `_busy`, and the review that moved this work here),
TKT-YK30D1 (cross-process channel pokes on postgres).

## Approach

- [x] Technical approach chosen and documented
- [x] Approach builds on existing patterns (not reinventing)
- [x] Alternatives considered (document why rejected)
- [x] Dependencies identified (packages, APIs, types)

**Technical Approach:**

1. **`CreatedChannel` in `internal/channels`.**
   - `Message` gains `CreatedChannel bool` with `json:"-"`, documented like
`ReplyTarget`: "Zero: always, except on the message Append returns, where it
reports that the append created the implicit channel. Never stored."
   - `filechannels` sets it from the `created` that `appendTarget` returns
(`ops.go:41`). `sqlchannels` does the same at `sql.go:455`, which covers
pgchannels and sqlitechannels.
   - A `channelstest` case pins it (AC 1), so every store must set it.

2. **Moved keys in `realtime.Local`.**
   - `Subscription` gains `moved map[channels.Key]struct{}`, allocated only by
`SubscribeLive`, next to `durable`. It shares `max` with `changed`. Every moved
key is also durable and changed. Every place that clears `durable` on resync
also clears `moved`.
   - `TakeMoved() []channels.Key` returns the set sorted and empties it, as
`TakeDurable` does. It returns nil for a subscription not made by
`SubscribeLive`, and nil after an overflow.
   - `markWhere(pick, keys, durable bool)` becomes
`markWhere(pick, keys, kind markKind)` with three kinds: ephemeral, message
(durable, not moved) and moved (durable and moved). `mark` keeps its name and
means moved. A new `markMessage(keys...)` means message. `markEphemeral` is
unchanged.
   - `markAddressed(targets, key)` takes the kind of its key mark, so a post
or edit that addresses someone can still be message-only.
   - The default is moved on purpose. A site that is not reclassified
over-wakes busy boards (cost) and never leaves one stale (correctness).
   - `Local.Mark`, the `ChangeMarker` entry for the sweep, timers, retention
and `applyTransition`, stays moved. Its godoc says so, and says why a timer
reminder counts too (Q1).

3. **Reclassify the message-only sites.** One helper decides a post:

   ```go
   // postMoved reports whether the appended message m may change a busy
   // answer: its append moved the session, or created the implicit channel,
   // which may start in a working state.
   func postMoved(m channels.Message) bool { return m.MovedSession || m.CreatedChannel }
   ```

Each site changes `mark` to `markMessage`, or passes the message kind:
- `Local.notePosted` (`service.go:500`): moved when `postMoved(m)`,
message otherwise.
- `systemWriter.post` (`system.go:62`): the same helper on the message
the append returns (today it is discarded).
- `Service.Edit` (`ops.go:101`) and `Service.Remove` (`ops.go:229`).
- `Service.Action` and `ReleaseAction` (`actions.go:70`, `79`, `93`,
`145`), `promoteFailed` (`promote.go:217`).
- The outputs engine's notice writes (`outputs.go:854`, `1002`). The
`on_final` close goes through `applyTransition` and stays moved.
- Unchanged, so moved: `Create` (`service.go:775`), `Lifecycle` delete and
transitions (`lifecycle.go`), `applyTransition` (`sweep.go`), timers
(`timers.go:708`), retention (`retention.go:381`, `434`).
- `EntityRenamed` keeps `mark`. A rename is a graph write and starts a full
round anyway.

4. **The hub reads moved keys.** `channelChanges`
(`live_topics.go:67`) gains `TakeMoved() []channels.Key`;
`*realtime.Subscription` already satisfies it after step 2. `liveHub.run` takes
it next to `TakeDurable` and calls `f.addMoved(moved)`. The topic round
condition is unchanged: every moved key is also a changed key, so `len(keys) >
0` already holds. `fakeChanges` in `live_channels_test.go` gains the method.

5. **The busy wake (`internal/dataentry/live_busy.go`, new).** The file
opens with a package comment that states the rules in AC 2 to 9 and 12, as
`live_inbox.go` does for the inbox.

   ```go
   // busyWake is what a busy list subscription listens to. Its replay
   // reports it on a 200 only; it never comes from the client.
   type busyWake struct {
       typ     string          // the channel type the busy read served
       anchors map[string]bool // row ids and id@face addresses on the page
   }
   ```

   - `recordBusy(ctx, typ string, rows []*entity.Entity)` fills
`topicProbe.busy` when the context has a probe and `rows` is not empty. For each
row it adds `row.ID` and `entity.FormatStateRef(row.ID, row.Face)`. On a type
without faces both are the same string.
   - `serveRowBusy` calls it after `Busy` returns `ok` with no error, with
the rows it was given. Those rows passed the row gate and redaction in the list
handler, so a hidden entity is never in a wake.
   - `roundFilter.addMoved(keys)` builds `f.moved map[string]map[string]bool`
once per round: channel type to anchors, where the anchor is the id for an empty
face and `id@face` otherwise.
   - `roundFilter.selectsBusy(s)`: false without a wake; true on
`f.allChannels`; otherwise true when the wake's anchors and `f.moved[w.typ]`
share a member. It ranges over the smaller of the two sets and looks each member
up in the other. A wake holds at most 200 anchors, so `collect`'s hold time on
`h.mu` and `c.mu` no longer scales with subscriptions times moved keys.

6. **Subscription fields (`live.go`, `livepath.go`).**
   - `livePath.busyType()` returns the first `busy` value of a list path, or
"", as `wantBusy` reads it.
   - `liveTarget` and `liveSub` gain `busy string` from `busyType()`, set in
`handlePut` and `replace` like `inbox`. `liveSub` gains `busyWake *busyWake`.
   - `replayResult` gains `busy *busyWake`, copied from the probe in
`replayHash`.
   - `applyTopic` handles a busy subscription before the `!s.channel` return:
an unfinished replay keeps the wake, a 200 takes `res.busy` (nil when the read
was not served), any other status clears it.
   - `roundFilter.selects` routes a busy subscription to `selectsBusy`, after
the presence check and before the channel check. A list path is never a
presence, channel or inbox path, so no shape takes two branches.
   - A test accessor `busyWakeOf(user)` in `live_test.go` reads a
subscription's wake under `h.mu` and `c.mu`, like `hashOf`.

7. **Safety replay for busy boards (`live_inbox.go`).** `safetyDue` becomes:

   ```go
   return (s.inbox && s.wake != nil || s.busyWake != nil) && now.slices&(1<<h.inboxSlice(fp)) != 0
   ```

   - Its godoc, the safety paragraph of the `live_inbox.go` package comment
and the `liveInboxSafetyMax` comment say that busy boards share the safety
replay, and why: a move in another process never reaches `Local`.
   - No new clock, slice or constant. A busy board takes its principal's
slice, so a principal's inbox and boards share one round.
   - A safety turn runs on a grid tick as a topic round. Busy pairs then get
no fallback frame (step 8). A busy subscription is not deferred to the grid like
an inbox one; the safety turn already falls on a grid tick.
   - *Why it adds no oracle.* Which pairs a safety turn takes depends only on
the wall clock and a hash of the principal's share key. No change, seen or
hidden, chooses it. The replay frames only when the subscriber's own hash moves.
   - *Why its cost is bounded.* Each busy subscription with a wake is
replayed once per safety period: every 24 ticks (12 s) over the file channel
store and every 28 ticks (14 s) over a database. Replays of one principal's tabs
coalesce into one pair. The slices spread the boards over 3 grid ticks (files)
or 7 (database), so a grid tick replays about a third or a seventh of them.
Example: 20 principals each with an open 100-card board on files is about 1.7
list replays a second on average, each with about 100 directory reads in
`ListWaiting`. The cost scales with open boards, not with writes, and the round
cap, the budget and `liveParallel` still bound each round.

8. **No fallback frame in a topic round (`live.go:904`).** `fallback` gains a
`topic bool` argument; `replayRound` passes `only != nil` at both call sites.
The frame rule becomes:

   ```go
   if !s.inbox && !(s.busy != "" && topic) {
       ref.conn.markChanged(ref.key)
   }
   ```

The subscription still goes to pending in every case. In a full round, `collect`
takes every non-syncing subscription whatever changed, so whether a busy pair is
in the round, and where the cap cuts it, depends on no channel change. Its
fallback frame carries only "a full round overflowed", which every plain list
already gets. In a topic round a move, possibly on a channel the subscriber
cannot read, chose the pair, so a frame would tell. The inbox keeps its rule in
both kinds of round, since its grid deferral is separate. The godoc of
`fallback` and the comment in `replayRound` name both shapes and the round-type
rule.

9. **Syncing recheck (`live.go:850`).** The condition becomes:

   ```go
   s.recheck = s.recheck || changed && (only == nil || s.presenceID != "" || s.channel || s.inbox ||
       s.busy != "" && only.movesBusy(s.busy))
   ```

`movesBusy(typ)` is true on `allChannels` or when `f.moved[typ]` is not empty. A
syncing subscription has no wake yet, so the type is the only thing to match on.
The type is configuration, not a secret, and a recheck only schedules a replay
that frames when its own hash moved. The comment above `collect` is updated: a
change of a channel moves no other response "except a busy list's `_busy`".

10. **Cross-process.** `Local` is per process. Moves made inside the
rela-server process wake a board within one tick, on every backend: web posts,
remote MCP (which runs inside rela-server), triggers, timers, sweeps and
outputs. A move from another process (the PoC's stdio `rela mcp`, `rela
channel`, a second replica) reaches the board on its safety turn, within 12 s on
files and 14 s on a database, with or without outputs. A move with graph outputs
still starts a full round at once, as today. TKT-YK30D1 adds a postgres poke
into `Local`; its keys should enter through `Local.Mark`, the moved kind, since
a poke cannot tell a message from a move. The plan records that as a note on
TKT-YK30D1, not as work here.

**Alternatives considered:**

1. *Select every busy subscription by channel type.* Rejected by the
TKT-21G6JS review (finding 1): every write to any channel of the type replays
every busy board, including channels on hidden anchors.
2. *Select on durable keys filtered by page anchors.* Rejected: every message
append on a visible card's channel would replay the board. The runner posts
progress and text often, so an open board would replay the whole list pipeline
on most ticks while an agent works in the server process. On the file backend
that is one channel directory read per row each time.
3. *Opt-in moved marks (mark only the sites known to move a session).*
Rejected: a missed site, now or in a later change, leaves the board stale with
no test failing. The default-moved rule turns the same mistake into extra
replays.
4. *Compute a `_busy` delta in the hub instead of replaying.* Rejected: it
would bypass the replay-and-hash rule that keeps a frame equal to what polling
shows, and would need its own copy of the read gates.
5. *Put busy replays on the inbox grid.* Rejected: the grid exists because a
full inbox read is expensive and its wake is broad (every waiting type). A busy
wake is narrow (moved keys on page anchors), round starts already follow the
fixed clock, and the grid would add up to 2 to 4 s of latency. Only the safety
turn, which is a grid tick anyway, is shared.
6. *Record the wake from `BusyReader`'s own anchor list.* Rejected: it changes
the `busyReader` interface and the realtime API for no gain. The rows the list
handler already holds give the same set, slightly wider for a family channel on
a faced type, which the replay and hash absorb.
7. *Exempt a busy subscription from fallback frames in full rounds too.*
Rejected: a full round's selection and cap cut depend on no channel change, so
the frame reveals nothing a plain list's frame does not. It would only cost the
main kanban board its refetch under load.
8. *Infer a created channel from `Seq == 1`.* Rejected in favour of
`CreatedChannel`. It never misses, since both stores create an implicit channel
only for an unnamed key with no messages, but it also wakes on the first post
into an explicitly created named channel, and it states the intent by inference.
9. *Leave cross-process moves to the docs, or to TKT-YK30D1.* Rejected: the
PoC runs on files in a separate process, and TKT-YK30D1 covers postgres only.
The board the ticket exists for would stay stale.
10. *Sort exempt pairs (inbox, busy) after the others before the cap cut.*
Rejected: it would stop a hidden move from pushing a frame-receiving pair into
fallback, but it starves the exempt pairs under sustained load and does not
remove the effect on how long a round takes. See Security, Timing.
11. *A minimum spacing per busy subscription.* Deferred: the sustained bound
in Risks equals today's graph-write worst case. Add it if measurements show
boards dominating rounds.

**Dependencies:** no new packages. Internal types: `channels.Key`,
`channels.Message.MovedSession`, the new `Message.CreatedChannel`,
`entity.FormatStateRef`, `realtime.Subscription`, the dataentry `topicProbe`,
`roundFilter`, `liveSub` and the inbox safety clock. `just arch-lint` is
unaffected: dataentry already imports `realtime` and `channels`.

**Files to modify:**

- `internal/channels/channels.go`: `Message.CreatedChannel`.
- `internal/channels/filechannels/ops.go`, `internal/channels/sqlchannels/sql.go`:
set it on the returned message.
- `internal/channels/channelstest`: "append reports CreatedChannel".
- `internal/realtime/local.go`: moved set, `TakeMoved`, `markKind`,
`markMessage`, `markAddressed` kind, godoc on `Mark` and `durable`.
- `internal/realtime/service.go`: `postMoved`, `notePosted` kind.
- `internal/realtime/system.go`: trigger post kind through `postMoved`.
- `internal/realtime/ops.go`, `actions.go`, `promote.go`, `outputs.go`:
message-only marks.
- `internal/realtime/sweep.go`: `ChangeMarker` godoc (Mark counts as moved).
- `internal/dataentry/live_busy.go` (new): `busyWake`, `recordBusy`,
`addMoved`, `selectsBusy`, `movesBusy`, package comment.
- `internal/dataentry/live_inbox.go`: `safetyDue`, package comment,
`liveInboxSafetyMax` comment.
- `internal/dataentry/live_topics.go`: `channelChanges.TakeMoved`,
`roundFilter.moved`, `selects`, `topicProbe.busy`, `applyTopic`, package
comment.
- `internal/dataentry/live.go`: `liveSub`, `liveTarget`, `handlePut`,
`replace`, `run`, `collect`, `fallback` and its callers, `replayResult`,
`replayHash`, comments.
- `internal/dataentry/livepath.go`: `busyType`.
- `internal/dataentry/busy_rows.go`: `serveRowBusy` calls `recordBusy`;
godoc.
- Tests:
  - `internal/realtime/live_subscription_test.go`;
  - `internal/dataentry/busy_rows_test.go`: `busyRowsRealtime` gains two
types (see Test Plan);
  - `internal/dataentry/live_busy_test.go` (new), with a manual-ticker busy
env;
  - `live_test.go`: remove `TestLive_BusyListIgnoresChannelRound`, add
`busyWakeOf`;
  - `live_hub_test.go`, `live_channels_test.go` (`fakeChanges`),
`livepath_test.go` (`busyType` cases).
- Docs: `docs-project/entities/guides/GUIDE-realtime.md` (regenerates
`docs/realtime.md`), `docs/data-entry/api-reference.md` (hand-written).

## Security Considerations

- [x] Input sources identified (user input, config, external APIs)
- [x] Input validation approach defined (allowlist preferred over blocklist)
- [x] Security-sensitive operations identified (file access, auth, crypto)
- [x] Error handling doesn't leak sensitive information

**Input Sources & Validation:**

- *Subscription path (client).* `busy` is already an allowlisted live list
key (`liveListKeys`). `busyType()` reads it only to flag the subscription for
the recheck and the fallback rule. It never selects anything by itself, and a
type name is configuration, not a secret.
- *Busy wake (server).* Built only from a 200 replay run as the subscriber,
from rows the list handler gated and redacted, and only when the busy read was
served. It never comes from the client. A refused replay clears it.
- *Moved keys and `CreatedChannel` (server).* Produced by the realtime
service and the channel store on their own writes. They hold channel keys and a
flag only, never content. `CreatedChannel` is never stored or serialized.

**Security-Sensitive Operations:**

- *Row gate: a hidden anchor does not wake (AC 6).* A hidden entity has no
row, so its id is in no wake, and a move on its channels selects no busy
subscription of that principal. A face that is not on the page is not in the
wake either. This is the row-level rule: a hidden entity is nonexistent, and its
session moves select nothing for that subscriber.
- *Channel gate on a visible anchor (AC 7).* A move on a channel the
subscriber may not read, on an anchor it may see, selects the board. The replay
gates the channel again, the answer does not change, and so the hash does not
move and no frame is sent. AC 4's "move that changes nothing" pins that a frame
comes from the hash, not from the selection.
- *No fallback oracle (AC 8).* In a topic round, where a change chooses the
pairs, a busy subscription never gets a frame without a check. In a full round
it gets the same fallback frame as every list, which carries no information
about any change, since a full round takes every subscription.
- *Safety replay (AC 12).* Its selection depends only on the clock and the
principal's share key. It frames only when the hash moves.
- *Per principal (AC 7).* Replays are grouped by principal share key and
path, as before. Each subscription keeps the wake of its own replay. No busy
answer, wake or fold is shared or cached across principals.
- *Timing: this change widens an existing channel.* Round starts follow the
fixed clock, but a round's length and where its cap cuts depend on how many
pairs it replays. After this change two more kinds of move add replays to a
topic round: a move on a channel the subscriber may not read, on a card it can
see, selects its own board; and a move on a card hidden from bob selects alice's
board. A channel or plain subscription that the cap then cuts still gets a
fallback frame, so another principal's frame, or its delay, can now depend on a
session move that principal cannot see. This is the same class as the inbox's
type-wide selection, which `live_inbox.go:40` accepts, and as every full round
that a hidden graph write starts. It needs a round past `liveRoundCap` (256
pairs) or `liveRoundBudget` (10 s) to cut anything, and it reveals at most that
some move happened somewhere in that tick. We accept it rather than claim it is
absent. Sorting exempt pairs last would narrow it (alternative 10).
- *Errors.* A failed busy read is logged server side as today and leaves no
wake; the next full round retries. No error text reaches the client.
- *Cost (AC 10, AC 12).* See Risks.

## Test Plan

- [x] Test scenarios documented for each acceptance criterion
- [x] Edge cases identified and documented
- [x] Negative test cases defined (invalid input, error conditions)
- [x] Integration test approach defined (not just unit tests)

**Test Scenarios:**

| AC | Test |
| --- | --- |
| 1 | `TestLocal_MovedKeys`, `TestSubscribeLive` new cases, `TestPostMoved`, `channelstest` "append reports CreatedChannel" |
| 2 | `TestLive_BusyWakeSetFromReplay`, `TestRecordBusy` |
| 3 | `TestRoundFilter_SelectsBusy` |
| 4 | `TestLive_BusyListWakesOnSessionMove` |
| 5 | `TestLive_BusyListIgnoresMessageOnly` |
| 6 | `TestLive_BusyHiddenAnchorNoWake` |
| 7 | `TestLive_BusyWakePerPrincipal` |
| 8 | `TestLive_BusyOverflowNoFallbackFrame`, existing `live_hub_test.go` fallback tests |
| 9 | `TestLive_BusyListRecheckWhileSyncing`, `TestLiveHub_CollectRechecksBusy` |
| 10 | `TestLive_BusyWakeCoalesces`, `TestRecordBusy` |
| 11 | Reviewed; `just docs` |
| 12 | `TestLive_BusySafetyReplay` |

**Harness.** The `TestLive_Busy*` tests in `live_busy_test.go` run the real HTTP
stack, the live hub, the realtime service and the channel store through a new
`newBusyLiveEnv`. It builds on `newBusyEnv` (`busy_rows_test.go`) but drives the
hub with the manual ticker (`liveStepper.step`), as `newInboxLiveEnv` does
(`live_inbox_test.go:243`), not the wall-clock window of `newBusyLive`. Replays
are counted with `countPrincipalReplays` (`live_inbox_test.go:143`), which keys
by principal and URL, so alice's and bob's replays of one busy URL are told
apart. Every assertion that something was replayed zero times, or got no frame,
follows a stepped tick in which a positive control in the same round was
replayed or framed: alice's TKT-1 `run` channel subscription, alice's board, or
a second board of the same principal (AC 4 to 7). `newBusyLive` stays for the
existing TKT-21G6JS tests.

**Fixture.** `busyRowsRealtime` gains two types. The existing types keep their
YAML anchors untouched, so `chat`, `facerun` and the TKT-21G6JS tests are
unaffected.

```yaml
  job:                      # one per ticket, implicit, starts working
    attach_to: [ticket]
    read: { from: attached }
    roles: { owner: { from: entity.owner } }
    message_types:
      log:    { properties: { body: { type: string } }, post: [owner] }  # does not move
      finish: { properties: { body: { type: string } }, post: [owner] }
    session:
      initial: working
      states:
        working: { label: Working, status: working, on: { finish: { to: done } } }
        done:    { label: Done, final: true }
    lifecycle:
      close:  { roles: [owner] }
      delete: { roles: [owner], retention: 7d }
  task:                     # many per ticket, created explicitly, starts working
    attach_to: [ticket]
    cardinality: many
    read: { from: attached }
    create: { roles: [owner] }
    roles: { owner: { from: entity.owner } }
    message_types:
      log: { properties: { body: { type: string } }, post: [owner] }
    session:
      initial: working
      states:
        working: { label: Working, status: working }
```

`job` gives the first post that creates a working channel without moving it
(`CreatedChannel` alone) and the declared close and delete. `task` gives
"create", which `instant` cannot (it has cardinality one and no `create:`, so
`Service.Create` refuses it with 409 `not_creatable`).

**Edge cases:**

- *Empty page.* No wake; only a full round replays it, which is the only
thing that can add a row.
- *Busy not served* (realtime off, degraded config, unknown type, no
attached row, no principal, store error). No wake. A later full round takes one
when the read is served again.
- *Cards past the first 100.* The board subscribes with
`page=1&per_page=100`, so the wake holds only those cards. A move on a later
card selects nothing; that card refreshes on the next entity change or reload,
as its lease already does.
- *Family channel on a faced type.* Key face "" matches the row id, so a
move frames every face row's board, as `Busy` shows it on every face row.
- *Face channel.* Key `id@face` matches only that face's row.
- *Channel created in a working initial state.* `Create` and a post with
`CreatedChannel` are moved, so the card turns busy live.
- *First post into an explicitly created named channel.* Not
`CreatedChannel`; it is moved only if it moves the session, which is correct,
since the create already woke the board.
- *Close, reopen, archive, delete, tombstone, restore.* Moved; the card stops
or starts being busy live.
- *Timer reminder.* Moved through `Local.Mark`, so it replays the board with
no frame. See Q1.
- *Move from another process.* Reaches the board on its safety turn (AC 12).
- *Moved set overflow.* Resync: every busy subscription with a wake is
replayed, with no fallback frame, like every channel subscription with a topic.
- *Row leaves the page* (filter, sort, delete, ACL change). Each is a graph
write or reload, so a full round replays the board and takes a new wake.
- *Two boards of one principal with different busy types.* Only the board
whose type matches is selected; both share the principal's safety slice.
- *Repeated `busy=`.* The first value counts, as for `wantBusy`; the wake
holds the served type, so selection never disagrees with the answer.
- *Concurrent PUT and move.* AC 9.
- *Typing and ephemeral signals.* Neither durable nor moved.

**Negative Tests:**

- A message-only write, a move of another type, and a move on an anchor off
the page replay nothing (AC 5).
- A hidden anchor's move replays nothing for that principal, and its id is
in no wake of that principal (AC 6).
- A denied channel's move frames nothing for that principal (AC 7).
- An overflowed topic round sends no fallback frame to a busy board (AC 8).
- A non-busy list, and a busy list in a full round, keep the fallback frame
(AC 8).
- A subscription without a wake is never selected by moves or by the safety
replay (AC 3, AC 12).
- A non-200 replay clears the wake (AC 2).

## Risk Assessment

- [x] Technical risks assessed with mitigations
- [x] Security risks assessed (see Security Considerations)
- [x] Effort estimated (xs/s/m/l/xl)

**Risks:**

- *Replay cost on the file backend.* A selected replay runs the full list
pipeline plus one `ListWaiting`, which on files reads one directory per anchor.
Mitigation: only moves select, only on page anchors, one replay per (principal,
path) per 500 ms tick, at most `liveRoundCap` pairs a round. Session moves are
rarer than posts: the PoC moves a run a few times per ticket, and posts are
rate-limited per principal.
- *Sustained moves.* Coalescing is per tick. While moves keep landing on
different visible cards in consecutive ticks, every open board of that type is
replayed every tick. Example: 10 principals with a 100-card board on files and a
move every tick is 20 list replays a second and about 2,000 directory reads a
second, through 4 `liveParallel` slots that PUT baselines share. The bound:
rounds never overlap, each runs at most 256 pairs within 10 s, and each pair is
replayed at most once per round. That is the same worst case as a graph write
every tick, which replays every subscription today. A minimum spacing per busy
subscription is the follow-up if measurements show it (alternative 11).
- *Safety replay cost.* About one list replay per open board per 12 s on
files or 14 s on a database, staggered by principal (Approach step 7). Bounded
by the number of open boards, not by write volume.
- *A reclassified site was wrong.* A move marked as message-only would leave
the board stale until the next safety turn. Mitigation: only the sites in
Approach step 3 become message-only, each in AC 1's table; everything else stays
moved.
- *A later write site forgets to choose.* It gets `mark`, which is moved, so
the cost is extra replays, not a stale board.
- *A future channel store omits `CreatedChannel`.* The `channelstest` case
fails for it. Until fixed, a first post that starts working would wait for the
safety turn.
- *Busy boards deferred in topic rounds.* Under a sustained topic-round
overflow a board waits for its turn in the rotation instead of refetching at
once. Mitigation: the rotation guarantees a turn within ceil(n / 256) ticks for
n pairs; the inbox relies on the same rule. In full rounds busy boards keep the
fallback frame, so the main board under load refetches as before.
- *Plimsoll.* `liveHub`, `App` and `channelsHandler` gain no methods. New
methods sit on `roundFilter` and `Subscription` (seven exported methods after
the change), and functions in `live_busy.go`. `channels.Message` gains one
exported field; check its `max-fields` line.
- *Cross-process expectations.* Moves from another process wait up to one
safety period. Mitigation: the docs say so, with the numbers.

**Effort:** l. The design review added a field in two channel stores with a
conformance case, two fixture types, a manual-ticker test env, the safety replay
and its test, and the round-type fallback rule. The code is still one coherent
change to the live path and fits one PR.

## Documentation Planning

- [x] User-facing docs identified (skip if internal refactor)
- [x] Docs-checklist will be created when entering implementation

**Documentation Impact:**

- [x] ~~docs/metamodel.md - New metamodel features~~ (N/A: no schema change)
- [x] ~~docs/cli-reference.md - New/changed commands~~ (N/A: no CLI change)
- [x] ~~docs/data-entry.md - UI changes~~ (N/A: the board looks the same; only its refresh timing changes, which the realtime guide documents)
- [x] `GUIDE-realtime.md`, busy indicator, "When it updates": a session move,
a channel create, close, reopen, archive or delete made in the server process
updates an open board within a tick, with or without outputs. A move from
another process (stdio `rela mcp`, `rela channel`, another replica) updates it
within about 12 s on files and 14 s on a database. Only the first 100 cards are
watched; cards past the first 100 refresh on the next entity change or reload.
Drop the "reload the page" sentence and update the backend table to match.
- [x] `docs/data-entry/api-reference.md`, `?busy=` paragraph: a session move
on a channel of the busy type, on a row of the page, repeats the list too; a
periodic safety replay catches moves from other processes; the board watches the
page it subscribed to, so cards past the first 100 refresh on the next entity
change or reload.
- [x] `docs/data-entry/api-reference.md`, "Live views": a list with `busy`,
like the inbox, gets no frame without a check when a round chosen by a channel
change skips it; it is checked on a later tick. In a full round it gets the
refetch frame like any list.
- [x] ~~CLAUDE.md - New patterns or conventions~~ (N/A: follows the inbox and topic patterns)
- [x] ~~README.md - Project-level changes~~ (N/A)

## Open questions

- **Q1. Should timer reminders count as moved?** Decided: yes. Timers,
sweeps and retention report through `Local.Mark` with no kind. A reminder then
replays a board with no frame. Reminders are rare, and splitting `ChangeMarker`
would widen a consumer-side interface for little gain. If measured cost says
otherwise, `Timers.write` can pass `step.Outcome.Moved` later.
- **Q2. Should a busy list lose fallback frames in full rounds too?**
Decided: no. It loses them only in topic rounds, where a change chose the pair.
A full round's fallback frame reveals nothing (Approach step 8, alternative 7).
- **Q3. Should TKT-YK30D1's poke feed the moved set?** Decided: yes, as a
note on that ticket. Its keys cannot tell a message from a move, so they enter
through `Local.Mark`, the moved kind. The busy safety replay covers the other
backends, so TKT-YK30D1 need not widen its scope.

## Design Review

- [x] Run `/design-review` before starting implementation
- [x] All critical/significant findings addressed in plan

**Design Review Findings:** <!-- List review-response IDs, e.g., RR-xxxx --> 14
findings (`.ignored/dr-w5zsyj.tsv`): four significant, six minor, four nits. All
four significant findings are addressed in this plan: the busy safety replay
(step 7, AC 12), the round-type fallback rule (step 8, AC 8), the manual-ticker
harness with per-principal counts and positive controls (Test Plan), and the
extended fixture (Test Plan, AC 4). Resolutions per finding are in
`.ignored/dr-w5zsyj-res.txt`.

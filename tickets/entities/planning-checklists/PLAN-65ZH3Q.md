---
id: PLAN-65ZH3Q
type: planning-checklist
title: 'Planning: Sessions 4/3: interrupting timers'
started: "2026-10-22"
completed: "2026-10-22"
status: done
---

<!-- @managed: claude-workflow v1 -->

## Understanding

- [x] Problem/requirements clearly understood
- [x] Scope defined (what's in/out documented below)
- [x] Acceptance criteria documented with specific test scenarios

**Problem:** Slice 1 (TKT-Y6NJ6U, merged) gave a channel type a flat session
state machine driven by typed messages. Nothing moves a session when nobody
posts. The GDPR breach use case needs exactly that: "if the authority is not
notified within 72 hours of detection, escalate". The sessions research
(`.ignored/res-sessions-final.md`, RES-O8I8S7) chose interrupting timers per
state (option a1/a2), stored as `next_due_at` on the channel row (c2), fired by
the realtime worker leader with a compare-and-set (CAS) on (state, generation).
The user decided that a session runs only while its channel is open, and that
timers that come due while the channel is closed are skipped and restart from
the reopen. This is slice 3 of the sessions feature (FEAT-GWF8YH phase 4).

Terms used below:
- **Timer**: an `after:` entry on a session state. When it comes due while
the session is still in that state and the channel is open, it moves the session
to its `to` state. The research calls these interrupting timers.
- **Anchor time**: the time a timer counts from. `since: state` (the
default) counts from when the session entered the state (`entered_at`; for a
session that never moved, the channel's `created_at`, as in slice 1). `since:
session` counts from the channel's `created_at`.
- **Restart anchor**: a time stored per timer name when that timer came due
while the channel was closed or tombstoned. From then on the timer counts from
the restart anchor. A move clears all restart anchors.
- **Resume**: a reopen, or the restore of an open channel from a tombstone.
Both record restart anchors in the same store step.
- **Due time**: when a timer comes due. It is computed from the config and
the channel row (Approach, decision 4), never stored as the truth.
- **`next_due_at`**: the earliest due time of the current state's timers,
stored on the channel row so the leader can find due sessions with one indexed
query. It is an index hint, kept in step with the row by every session write and
repaired by a periodic recompute.
- **Fire**: the store step that moves a session because a timer came due.
- **Reschedule**: a store step that only rewrites `next_due_at`, without a
move.
- **Round**: one pass of the timer loop: list due sessions of every timer
type, fire or reschedule them, and compute when to wake next.
- **Recompute**: a pass that lists every open channel of the session types
and corrects stale or missing `next_due_at` values.

**Scope:**
- In: `after:` on a session state in `realtime.yaml`: a map from timer name
to `{period, since: state|session, to, label}`. Load-time validation, one
problem per key.
- In: two new session fields on the channel row (`next_due_at` and the
restart anchors) in every channel store backend: `filechannels` (disk and
MemFS), `sqlitechannels` and `pgchannels` over `sqlchannels`. sqlite rung v25
plus a post-ladder index, postgres migration `0032_channel_session_timers.sql`,
and `meta.yaml` fields.
- In: `channels.Store` gains `SetSession` (fire or reschedule without a
message) and `ListSessionsDue` (the due query over a set of types).
`SessionStep` gains a required `Due` and an optional `Timer`. `NewChannel` and
`Transition` gain an optional `SessionDue`, and `Untombstone` takes a resume
callback, so that every write that changes the answer keeps `next_due_at` and
the restart anchors current under the store lock.
- In: a small timer component, `realtime.Timers`, run by the realtime worker
leader. It has a wake signal of its own type (`realtime.DueWake`), a ticker
fallback, an injected clock, a recompute after a config change and every 10
minutes, stranded handling, freezing while the channel is closed, and failure
isolation per channel.
- In: notices with a `timer` prop and audit records with `triggered_by:
timer:<type>/<state>/<name>`, written as `system:realtime`.
- In: the next due time on the wire: `due_at` and `timers` in the session
block of the HTTP head, page and list answers, MCP `read_channel`,
`list_channels` and `post_message`, the CLI header, the OpenAPI `ChannelSession`
schema and the frontend wire-field mirror (types only).
- In: budget tests for the filechannels due scan, the timer round and the
recompute.
- In: docs in `docs-project/entities/guides/GUIDE-realtime.md` and the
`read_channel` row of `GUIDE-mcp-server.md`.
- Out (later slices): session-level reminder timers with `in:` and `every:`
(slice 4), which will also revisit repeating timer cycles; `outputs:`, `run_as`,
`on_final: close` (slice 5); `status:`, `waiting_for:` (slice 6); `count()`
guards (slice 7). Out: any UI component (slice 2 owns the session header and may
still be in review; this slice touches only the wire types). Out: SLA pause and
entity-relative deadlines (would move timers to a timer table, option c3).
- Out: `run_as`. A timer fire in this slice writes only to the channel store
(state, notice, audit), as the trigger engine and the `auto.after` sweep already
do as `system:realtime`. It makes no graph write and evaluates no ACL, so it
needs no granted identity yet. Decision 3 of the research (`run_as` granted in
`acl.yaml`, missing grant fails at load) applies to graph outputs, and slice 5
adds the key together with `outputs:`. Until then `session.run_as` stays an
unknown key and is refused by the key checker.

**Example (the shape this slice accepts; the `breach-response` type in
`testdata/valid.yaml` gains it, and the docs use it):**

```yaml
    session:
      label: GDPR breach response
      initial: detected
      states:
        detected:
          label: Detected
          on:
            contained: { to: contained, roles: [lead] }
            dismiss:   { to: dismissed }
            escalated: { to: contained, by: trigger }
          after:
            no-containment: { period: 24h, to: overdue, label: Not contained within a day }
        contained:
          label: Contained
          on:
            assessed: { to: assessed, when: "message.risk != 'none'" }
          after:
            notify-deadline: { period: 72h, since: session, to: overdue, label: 72 hours since detection }
        assessed:
          label: Risk assessed
          on:
            notified: { to: notified }
          after:
            notify-deadline: { period: 72h, since: session, to: overdue, label: 72 hours since detection }
        overdue:
          label: Notification overdue
          on:
            notified: { to: notified }
        notified:  { label: AP notified, final: true }
        dismissed: { label: Dismissed, final: true }
```

**Acceptance Criteria:**
1. The example compiles. `SessionState.Timers()` returns the state's timers
sorted by name, each with `Name`, `Label` (defaults to the name), `To`, `Since`
(`state` by default) and a parsed `Period`. `Session.HasTimers()` is true.
`overdue`, which has `on:` and no `after:`, and a state with only `after:` both
load. Test `TestParse_SessionTimers` in `internal/realtimecfg/load_test.go`;
`TestParse_Golden` updated.
2. Each invalid timer reports its key path and reason, all in one load.
Table-driven `TestParse_SessionTimersInvalid`, one row per rule in Approach,
decision 1: `after` on a final state; a non-final state with neither `on` nor
`after`; more than 8 timers; a bad timer name; missing `period`; a period that
does not parse; a fixed period under 1m; `since` other than `state` or
`session`; the key `from` (refused, "use since"); missing `to`; undeclared `to`;
a timer self-loop (`since: state`); a timer self-loop (`since: session`); a
two-state timer cycle mixing `since: state` and `since: session` (the reason
names the cycle path).
3. A cycle that passes through a message transition loads (A `after:` to B,
B `on:` a message to A), as does a calendar period (`1mo`). Test rows in
`TestParse_SessionTimers`.
4. Adding, removing, renaming a timer and changing its period passes
`CheckReload`. Test rows added to `TestCheckReload_SessionChangesAllowed`.
5. The compiled timers are immutable: `SessionState.timers` is unexported
and `Timers()` returns a copy. Test row in `TestCompiled_SessionIsReadOnly`.
6. Store conformance, in `channelstest.RunAll` (file on disk, file on MemFS,
sqlite, postgres), new `channelstest/timers.go`:
   - `testSessionDueOnAppend`: an append whose step's `Due` returns T stores
`NextDueAt` T. A move clears the restart anchors and stores the new `Due`
result. A step without `Due` is `ErrInvalidMessage` and stores nothing. The
slice-1 conformance steps gain a `Due` that returns zero.
   - `testSessionDueOnCreateMany`: `CreateMany` with `SessionDue` stores its
result.
   - `testSessionResumeOnReopen`: `SetLifecycle` to open with
`SessionDue` stores the returned `NextDueAt` and restart anchors in the same
step. A close with no `SessionDue` leaves both as they were.
   - `testSessionResumeOnUntombstone`: `Untombstone` calls the resume
callback once per restored open channel and stores its result; it does not call
it for a closed one; a nil callback changes nothing.
   - `testSetSessionFire`: on a session in `contained` at generation 1, a
step whose decider moves to `overdue` with `Timer` `notify-deadline` appends one
notice at `last_seq + 1` (type, author and `TriggeredBy` from the step; props
`from`, `to`, `generation`, `timer`; no `cause`). It sets the session to
(`overdue`, 2, `at`), clears the restart anchors, stores the `Due` result,
raises rev, and writes one audit record (op `session`, `By` the notice author,
`Seq` the notice seq, `TriggeredBy` from the step). The returned channel and
`Head` agree.
   - `testSetSessionReschedule`: a decider that returns no move writes no
message and no audit record, stores the `Due` result, and leaves rev unchanged.
   - `testSetSessionCAS`: two concurrent `SetSession` calls whose deciders
move only from (`contained`, 1) produce exactly one move and one notice.
   - `testSetSessionRefusals`: a missing channel is `ErrNotFound` and
nothing is created. A move without `Timer` is `ErrInvalidMessage`. An invalid
state name is `ErrInvalidMessage`. An `Append` whose step sets `Timer` is
`ErrInvalidMessage`. In each case nothing is stored.
   - `testListSessionsDue`: returns the live, open channels of the given
types whose `NextDueAt` is set and at or before `until`, ordered by `NextDueAt`
then key (anchor id, face, type, name), at most `limit`. Ties on `NextDueAt`
come back in key order on every backend. It leaves out other types, closed and
archived channels, tombstoned channels, channels with no due time, and channels
due after `until`. An empty type list or an invalid type is `ErrInvalidKey`; a
limit of 0 or less is `ErrInvalidMessage`.
   - `testSessionScheduleSurvivesMoves`: `NextDueAt` and the restart anchors
are kept across `SetLifecycle` close, `Rekey`, and `Tombstone`.
7. Schema steps: `TestOpenV24Database` in `internal/sqlitedb` writes a real
v24 database file (the v24 `channels` DDL, kept as a test fixture, and
`user_version` 24) and opens it through `Open`. Open succeeds, the columns and
`channels_session_due_idx` exist with the shape of a fresh database, an existing
row reads with no due time and no restart anchors, opening a second time is
harmless, and `user_version` is 25. Postgres: conformance runs on the migrated
schema, and `TestLegacyRowHasNoSchedule` in `pg_test.go` inserts a row without
the new columns and reads it the same way.
8. The due query uses the index: `TestListSessionsDue_UsesIndex` in
`internal/channels/sqlitechannels` checks `EXPLAIN QUERY PLAN` for
`channels_session_due_idx`, in the style of
`sqlitestore/graphquery_explain_test.go`.
9. filechannels: `TestMetaWithoutScheduleReadsEmpty` (a slice-1
`meta.yaml` reads no due time and no restart anchors).
`TestListSessionsDue_Budget` in `file_test.go`, with a counting `storage.FS`
wrapper, two timer types and 10 and then 50 entities: one call for both types
makes one tree walk. That is one `ReadDir` of the root, one per entity directory
and one per face directory, and one `meta.yaml` read per channel of the listed
types. It makes no log segment read, no audit read and no read of another type's
`meta.yaml`, and takes no store lock. `TestListSessionsDue_DamagedMeta`: a
damaged `meta.yaml` is skipped with a warning, and the other channels are still
listed.
10. Due computation, `internal/realtime/timers_test.go`,
`TestTimerDue` table: `since: state` counts from `entered_at`; `since: session`
counts from `created_at`; a session that never moved counts `since: state` from
`created_at`; a restart anchor after the anchor time wins; final and stranded
states have no due time; with two timers the earliest wins and a tie goes to the
first name; a due time is normalized to microseconds (`channels.NormTime`).
`TestResumeSchedule` table: a timer that came due before the resume gets the
resume time as its restart anchor; a timer not yet due keeps its deadline; two
close/reopen cycles keep a restarted deadline that was not due at the second
reopen (P=72h from session at T0: close T0+10h, reopen T0+100h gives T0+172h;
close T0+110h, reopen T0+120h keeps T0+172h); with two timers, each keeps its
own restart anchor.
11. Timer rounds, `timers_test.go`, over the file store on MemFS with the
test clock:
    - `TestTimers_Fires`: a session in `detected` entered at T0 moves to
`overdue` in the first round at or after T0+24h, not before. The notice has
`timer: no-containment`. The audit record has `By` `system:realtime` and
`TriggeredBy` `timer:breach-response/detected/no-containment`. The key is marked
for live subscribers.
    - `TestTimers_FromSession`: in `assessed`, entered at T0+30h on a
channel created at T0, the timer fires at T0+72h.
    - `TestTimers_MessageWins`: a `notified` post that lands before the
round moves the session; the round's fire fails the CAS and does nothing (one
notice in the log).
    - `TestTimers_LeaderOverlap`: two `Timers` over one store run a round
at the same clock time; exactly one fire is stored.
    - `TestTimers_ClosedFreezes`: a channel closed before its due time is
not fired while closed. Reopened after the due time, it fires at reopen +
period. Reopened before the due time, it fires at the original due time. Two
close/reopen cycles keep the restarted deadline (the AC 10 example) and fire at
T0+172h.
    - `TestTimers_RestoreFromTombstone`: an open channel tombstoned across
its due time and restored is treated as reopened at the restore: it fires at
restore + period, not in the first round after the restore.
    - `TestTimers_Stranded`: a session stored in a state the config no
longer declares is never fired. The recompute clears its `next_due_at` and logs
one warning per (type, state) per loaded config.
    - `TestTimers_FinalNoTimers`: a session moved into a final state by a
timer has no due time.
    - `TestTimers_ReloadChangesPeriod`: after a config swap that shortens
`no-containment` to 1h, the recompute reschedules, and a session entered more
than 1h ago fires in that round. After a swap that removes the timer, the
recompute clears `next_due_at`.
    - `TestTimers_RecomputeRepairsStaleHint`: an open channel with a due
timer and a stored `next_due_at` of zero (written under an older config) is
repaired by the periodic recompute and fired. The second pass one ceiling after
a config change also repairs it.
    - `TestTimers_ChainFiresWithoutCeiling`: a fire into a state whose
`since: session` timer is already past sets `More`, and the next round (at the
floor) fires it.
    - `TestTimers_FailingRowIsolated`: a channel whose `SetSession` fails
is logged and skipped, later due channels of the same type still fire in the
same round, and the failing key is not retried until its backoff ends. `Reset`
clears the backoff.
    - `TestTimers_Degraded`: under a degraded snapshot the round fires and
recomputes nothing. It fires on the first round after the config is valid again.
    - `TestTimers_RoundCap`: with more due sessions than
`MaxTimerWritesPerRound`, a round writes exactly the cap, reports `More`, and
the next round continues.
    - `TestTimers_NextWake`: `Next` is the earliest due time after now among
the listed sessions and the `next_due_at` of each write; zero when there is
none.
12. Budget: `TestTimers_Budget` in `timers_test.go`, with a counting store
wrapper like `TestSweep_Budget`:
    - a round makes exactly one `ListSessionsDue` call, for all timer types
together, and none when no type has timers;
    - with 200 channels that need a reschedule after a config change, the
recompute calls `ListByLifecycle(open)` exactly once per session type for that
config and drains the writes over four rounds;
    - a round on an unchanged config before the 10-minute recompute makes no
`ListByLifecycle` call;
    - `TestTimers_NoRewriteWhenUnchanged`: a round over sessions whose
stored `next_due_at` already equals the derived due time makes zero `SetSession`
calls.
13. Service: `TestSessionPost_SetsDue` (a post on a timer type stores the
due time and notes it on the `DueWake`); `TestSessionCreate_SetsDue` (`Create`
of a `cardinality: many` timer type stores it); `TestSessionReopen_Resumes` (a
reopen through the service records the restart anchors in the same step);
`TestSessionTriggerPost_SetsDue` in `trigger_test.go`.
14. Worker and wake: `TestTimerDelay` in `internal/appbuild` (pure: the
delay is the time until `Next`, clamped to [1s, 60s]; `More` gives the floor; a
zero `Next` gives the ceiling). `TestRealtimeWorker_TimerWake` (a wake with a
due time earlier than the current target shortens the sleep; a later one does
not start a round). `TestDueWake` in `internal/realtime` (`Note` keeps the
earliest time and coalesces pings; `Take` returns and clears it).
15. Wire: `TestChannelWireGolden` regenerated (`channel_wire_fields.json`
gains `due_at`, `timers` and `ChannelSessionTimer`). Mirrored fields pass
`frontend/src/types/wireFields.test.ts`.
`TestChannelsHandler_SessionTimersInReads` checks `due_at` and `timers` on the
head, page and list for an open channel. On a closed channel the timers are
listed and no `due_at` appears. Under a degraded snapshot the timers are listed
and no `due_at` appears. A final state has neither.
16. OpenAPI: `TestOpenAPI_ChannelSession` extended for `due_at` and
`timers` referencing `ChannelSessionTimer`.
17. Collection cost: `channels_budget_test.go` with a timer type in the
fixture; the list endpoint's store-call budget is unchanged.
18. MCP: `TestReadChannel_IncludesSessionTimers` and
`TestPostMessage_ReturnsSessionDue` in `tools_channels_test.go`.
19. CLI: `TestRenderRead_SessionTimer` in `channel_render_test.go` prints
"next: no-containment to overdue at <time>" in the session line, and renders a
timer notice as "moved from detected to overdue (timer no-containment)".
20. Docs: the guide example is the `breach-response` block of
`testdata/valid.yaml` (AC 1 pins it), and `just docs` regenerates
`docs/realtime.md` and `docs/mcp-server.md` without diff.

## Research

- [x] For larger features: run `/research` to create a structured research doc
- [x] ~~Searched for existing libraries that solve this problem~~ (N/A: covered by the research doc; the timer loop reuses internal packages)
- [x] Checked codebase for similar patterns or reusable code
- [x] Looked for reference implementations in other projects
- [x] Reviewed relevant rela concepts for prior art

**Research Doc:** RES-O8I8S7 (`.ignored/res-sessions-final.md`). Decisions of
2026-10-21: flat states with per-state interrupting timers; the session is
separate from the lifecycle and runs only while the channel is open; timers due
while closed are skipped and restart from the reopen; timer graph writes use a
per-type `run_as` (deferred to slice 5 with the graph writes); `next_due_at` on
the channel row (c2), with a timer table (c3) as the upgrade path.

**Existing Solutions:**
- `SessionStep`, `DecideSession`, `ApplySession`, `SessionNotice`,
`SessionAudit` (`internal/channels/session.go`): the decider-under-lock step
from slice 1. `SetSession` reuses it without a causing message.
- `Sweeper.RunLifecycle` and `auto.after` (`internal/realtime/sweep.go`):
indexed per-type listing, CAS through `applyTransition`, `ErrConflict` skipped,
degraded config skips automatic steps, `auto:<type>/<op>` labels. The timer
round follows the same shape and label style.
- `Store.DueTombstones` and `channel_tombstones_due_idx`: an indexed "due by
time" listing polled by the leader.
- The realtime worker (`internal/appbuild/realtime_worker.go`): leader lock,
takeover check every 10s, at-least-once overlap. The `tombstoneConfirmer.wake`
channel is the model for `DueWake`.
- `lease.Hub` and `NextExpiry` (`internal/lease/wake.go`): a wake signal plus
sleep until the nearest expiry.
- `realtimecfg.Period` (`ParsePeriod`, `AddTo`, `Fixed`): the period syntax
of `auto.after`, reused for `period:`.
- `TestSweep_Budget` (`internal/realtime/purge_test.go`): the counting store
wrapper the timer budget test copies.
- `mentionsDDL` and `searchDDL` (`internal/sqlitedb/sqlitedb.go`): indexes
on columns a rung adds run after the ladder. The due index does the same.
- `channel_messages_pending_idx ... WHERE pending_at > 0`: the partial index
pattern for the due index.
- Prior art (research table): XState `after:` ("the timer belongs to the
state", cancelled on exit, which the CAS on (state, generation) gives by
construction); BPMN interrupting boundary timers; Temporal's durable timer
queue, where a fire is just an event judged against the current state.

## Approach

- [x] Technical approach chosen and documented
- [x] Approach builds on existing patterns (not reinventing)
- [x] Alternatives considered (document why rejected)
- [x] Dependencies identified (packages, APIs, types)

**Technical Approach:**

1. **Config.** `raw.go`: `rawSessionState.After map[string]rawSessionTimer`;
`rawSessionTimer{Period, Since, To, Label string}`. Compiled, in
`realtimecfg/session.go`:

   ```go
   type TimerSince string // "state" (default) | "session"
   type SessionTimer struct {
       Name, Label, To string
       Period          Period
       Since           TimerSince
   }
   func (st SessionState) Timers() []SessionTimer // sorted by name, a copy
   func (s *Session) HasTimers() bool
   ```

`SessionState` gains unexported `timers []SessionTimer`. Timer order is by name,
so no YAML node walk is needed and merge keys under `after` are harmless. The
anchor key is `since`, not `from`, because `from:` on a lifecycle op already
means "source states". `keys.go` refuses `states.*.after.*.from` with "use
since: state or since: session". Validation, one problem per key:
- `states.<s>.after` on a final state: "a final state has no timers; drop
after or remove final: true".
- The slice-1 rule for non-final states becomes: "a state that is not
final needs a transition under on or a timer under after; mark it final: true if
the session ends there".
- More than `realtimelimits.MaxSessionTimers` (8): "at most 8 timers per
state".
- Timer name: "timer name must match ^[a-z][a-z0-9-]{0,62}$".
- `period`: missing: "name how long the timer waits, such as 72h"; bad
syntax: the existing period message; a fixed period under
`realtimelimits.MinSessionTimer` (1m): "a timer waits at least 1m". A calendar
period (`mo`, `y`, where `Period.Fixed` reports false) always passes, since its
shortest length is 28 days.
- `since`: "since must be state or session".
- `to`: missing: "name the state this timer moves to"; undeclared: the
unknown-state message.
- Any cycle made only of `after:` edges is refused, self-loops included,
at the timer that closes it: "timers form a loop (assessed -> overdue ->
assessed) that would move the session forever without a message; break it with a
transition under on". A repeating timer loop has no use case before slice 4's
bounded reminders, and each move appends a notice and an audit record. One
depth-first search over at most 64 states and the timer edges. A cycle through
an `on:` transition is allowed.
- `CheckReload`: unchanged. Timers do not affect how channels are keyed.

2. **Store model** (`internal/channels/session.go`):

   ```go
   type Session struct {
       State      string
       Generation int64
       EnteredAt  time.Time
       // NextDueAt is the index hint for the due query. Zero: no due time.
       NextDueAt  time.Time            `json:"next_due_at,omitzero"`
       // Restarts holds, per timer name, the restart anchor of a timer that
       // came due while the channel was closed or tombstoned.
       Restarts   map[string]time.Time `json:"restarts,omitempty"`
   }
   // SessionSchedule is what a schedule callback returns. A nil Restarts
   // keeps the stored anchors.
   type SessionSchedule struct {
       NextDueAt time.Time
       Restarts  map[string]time.Time
   }
   // ScheduleFunc is called under the lock with the channel as it will be
   // committed. It must be pure and fast, like SessionStep.Decide.
   type ScheduleFunc func(next Channel) SessionSchedule

   type SessionStep struct {
       ... // slice 1 fields
       Due   ScheduleFunc // required
       Timer string       // names the timer behind a SetSession move; Append refuses it
   }
   type NewChannel struct { ...; SessionDue ScheduleFunc } // optional
   type Transition struct { ...; SessionDue ScheduleFunc } // optional; used on reopen
   ```

Helpers, shared by both backends:
- `ApplySession` clears `NextDueAt` and `Restarts`, because a move leaves
the state the timers belonged to. Today it replaces the whole struct, which
keeps that behaviour explicit.
- `ApplySchedule(c *Channel, f ScheduleFunc)`: with `f` nil it does
nothing. Otherwise it stores `NormTime(NextDueAt)` and, when non-nil, the
normalized `Restarts`. The store caps `Restarts` at
`realtimelimits.MaxSessionTimers` entries with valid names (`ErrInvalidMessage`
otherwise).
- `TimerNotice(step, seq, at, move, gen, rev)`: props `from`, `to`,
`generation`, `timer`.
- `TimerAudit(k, step, notice, move, gen)`: `By` the notice author, `Seq`
the notice seq, `TriggeredBy` `step.AuditTriggeredBy`.
- `PrepareSessionStep` refuses a nil `Due` and checks `Timer` (empty or a
valid name). `Append` refuses a step with `Timer` set.

Two new `Store` methods, the seam the slice-1 plan reserved:

```go
// SetSession runs step against the channel's session without a message:
// a timer fire when the decider returns a move, a reschedule otherwise.
// On a move it appends a notice at last_seq+1 (see TimerNotice), applies
// the move at `at`, and writes the session audit record. A move without
// step.Timer, or with an invalid state name, is ErrInvalidMessage. In
// every case it then applies step.Due. A move raises rev; a reschedule
// does not, because next_due_at is not shown to readers. A missing
// channel is ErrNotFound; it never creates one.
SetSession(ctx context.Context, key Key, at time.Time, step SessionStep) (Channel, error)

// ListSessionsDue returns the live, open channels of the given types
// whose Session.NextDueAt is set and at or before until, ordered by
// NextDueAt and then by key, at most limit. types must hold at least one
// valid type, else ErrInvalidKey; a limit of 0 or less is
// ErrInvalidMessage. The database backends read an index. The file
// backend walks the tree once per call: one directory read of the root,
// of each entity directory and of each face directory, and one meta.yaml
// read per channel of the given types. It skips a damaged meta.yaml with
// a warning.
ListSessionsDue(ctx context.Context, types []string, until time.Time, limit int) ([]Channel, error)
```

`SetLifecycle` applies `t.SessionDue` in its step when set. The service sets it
on every reopen of a session type; close and archive set none. `Untombstone(ctx,
h, at, resume ScheduleFunc)` applies `resume` to each restored channel that is
open. `Nil: accepted`; nothing changes then. Its three callers
(`appbuild/channelguard.go` twice, `realtime/retention.go`) pass
`realtime.ResumeSchedule(cfg)`.

Plimsoll: both `*Store` types are at `max-exported-methods=27`. That becomes 29,
with the note "27 -> 29 (TKT-INM3AZ): SetSession and ListSessionsDue", as a
documented required-interface exception. `filechannels.Store` has 40 methods in
total (the default cap) and would reach 42, so `occupied` and `fileBatch` become
free functions over `*disk`. `sqlchannels.Store` has 39 and would reach 41, so
`checkParent` becomes a free function taking a `Querier`. `just plimsoll` is in
the implementation checklist.

3. **Backends and migrations.**
   - `sqlchannels`: `chanCols` and `scanChannel` read `session_next_due_at`
and `session_restarts` (JSON text, empty for none). `bumpSession` writes both.
`Append` calls `ApplySchedule` after `DecideSession` and `ApplySession` in its
existing transaction. `insertChannel` (from `CreateMany`), `SetLifecycle` and
`Untombstone` apply their callbacks in their existing transactions.
`SetSession`: in one transaction under the entity lock it reads the row,
decides, inserts the notice on a move, runs one `UPDATE channels` (last_seq,
rev, all session columns) and inserts the audit row. `ListSessionsDue`: `SELECT
chanCols FROM channels WHERE tomb = '' AND type IN (?, ...) AND state = 'open'
AND session_next_due_at > 0 AND session_next_due_at <= ? ORDER BY
session_next_due_at, anchor_id, anchor_face, type, name LIMIT ?`. Its own query,
because `queryChannels` fixes another order.
   - Index, on both databases: `channels_session_due_idx ON channels (type,
state, session_next_due_at) WHERE session_next_due_at > 0 AND tomb = ''`. The
query repeats the partial predicate literally, so both planners can use it, one
index range per listed type. `state` is in the key, so closed channels with past
due times are skipped by the range scan, not filtered after it. When the
migration runs, every row has 0, so the index starts empty and builds at once.
   - `internal/store/pgstore/migrations/0032_channel_session_timers.sql`:
`ALTER TABLE channels ADD COLUMN IF NOT EXISTS session_next_due_at BIGINT NOT
NULL DEFAULT 0`, `... session_restarts TEXT NOT NULL DEFAULT ''`, and `CREATE
INDEX IF NOT EXISTS` for the index, with a header in the style of 0031. If
another branch takes 0032 first, renumber at rebase: the ladder must stay
contiguous.
   - `sqlitedb`: `channelsDDL` gains `session_next_due_at INTEGER NOT NULL
DEFAULT 0` and `session_restarts TEXT NOT NULL DEFAULT ''`, but NOT the index.
`schemaSQL` runs before the ladder on every `Open`, and on a v24 file the index
would name a column rung 25 has not added yet. The index goes into a new
post-ladder constant, `channelTimersDDL`, executed after `c.migrate` next to
`mentionsDDL` and `searchDDL`, with `CREATE INDEX IF NOT EXISTS`.
`schemaVersion` becomes 25. The new rung `to: 25`, `addChannelSessionTimers`,
probes each column like `addChannelSessionColumns` and adds the missing ones. AC
7 opens a real v24 file through `Open`, which is what catches an index placed
before the ladder.
   - `filechannels`: `meta` gains `SessionNextDueAt` and `SessionRestarts`
(yaml, omitempty), and `setSession` and `channel` map them. `Append`,
`CreateMany`, `SetLifecycle`, `Untombstone` and the new `SetSession` apply their
callbacks before `writeMeta`. `SetSession` follows `Append`'s order under the
lock: `meta.yaml`, then the notice record, then the audit record. The crash
window is the one the package doc already describes; the doc gains one sentence
for the timer fire. `ListSessionsDue` uses a new `disk.visitTypes(set,
skipDamaged, fn)` that walks the tree once and descends only into type
directories in the set. It skips and logs a damaged `meta.yaml` instead of
failing the walk; the existing visitors keep their behaviour. It then filters,
sorts by (due, key) and truncates. It takes no lock. AC 9 pins the directory
reads and the file reads.

4. **Due computation** (`internal/realtime/timers.go`, pure):

   ```go
   // timerDue is when t comes due for ch, whose current session is cur.
   func timerDue(t realtimecfg.SessionTimer, cur current, ch channels.Channel) time.Time {
       anchor := cur.EnteredAt // since: state; created_at for a session that never moved
       if t.Since == realtimecfg.TimerSinceSession {
           anchor = ch.CreatedAt
       }
       if r, ok := ch.Session.Restarts[t.Name]; ok && r.After(anchor) {
           anchor = r
       }
       return channels.NormTime(t.Period.AddTo(anchor))
   }
   // nextTimer is the current state's earliest-due timer (a tie goes to the
   // first name). ok is false for a final or stranded state, or a state
   // without timers.
   func nextTimer(s *realtimecfg.Session, ch channels.Channel) (t realtimecfg.SessionTimer, due time.Time, ok bool)
   ```

   - `since: session` counts from `created_at`. There is no separate start
column and no arming. The documented reload rule ("adding or shortening a timer
can fire it in the next round") applies to every session, old or new.
`currentSession` and the wire `entered_at` stay as in slice 1.
   - Freeze: the lifecycle is not an input to `timerDue`. A closed channel
still has a due time, but the due query skips it.
   - Restart, `ResumeSchedule(cfg) channels.ScheduleFunc`: called under the
lock on a resume (a reopen or a restore of an open channel), with `at` as the
resume time. For each timer of the current state whose due time, with its
current restart anchor, is at or before `at`, it sets `Restarts[name] = at`. A
timer not yet due keeps its anchor and its deadline. It then returns the new
`NextDueAt`. A 72-hour legal deadline does not stretch because someone closed
the channel for an hour, and repeated close/reopen cycles keep each timer's
restarted deadline, because the anchor is stored per timer rather than derived
from the last reopen (AC 10, AC 11).
   - Tombstoned span: treated like closed. The restore of an open channel is
a resume at the restore time; a closed channel is resumed at its later reopen.
   - `sessionDue(s) channels.ScheduleFunc` for steps and creates returns
`nextTimer`'s due time, or zero, and nil `Restarts`. Every session step and
every `CreateMany` on a session type sets it, also for types without timers.
   - Comparisons between a derived due time and a stored one use
`channels.NormTime` on both sides, so a microsecond-truncated stored value never
looks different (AC 12 `NoRewriteWhenUnchanged`).

5. **Writes that keep `next_due_at` current.**
   - Message moves and posts: `newSessionPost` sets `step.Due`. The decider
is unchanged.
   - Implicit create (first append) and `Service.Create` (`CreateMany`
with `SessionDue`).
   - Trigger posts (`systemWriter.post`): same step as user posts.
   - Reopen (`Service` lifecycle path and any automatic reopen): `Transition.
SessionDue = ResumeSchedule`. Close and archive: no write.
   - Restore from tombstone: `Untombstone(..., ResumeSchedule)`.
   - Timer fires and reschedules: `SetSession` with `Due` (decision 6).
   - Config reload, leader start and every 10 minutes: the recompute
(decision 7).
   - After a step, the service and the system writer pass
`step.Outcome.Session.NextDueAt` to `DueWake.Note` when it is non-zero.

Invariant: the stored `next_due_at` is never later than the true due time, with
one exception: a write made under an older config (a post that loaded the old
snapshot, or a process that has not reloaded yet) can store a stale value,
including zero. The recompute repairs it within one ceiling after the leader
sees a config change and otherwise within 10 minutes. An earlier value is always
safe: the round re-derives the due time from the row, finds it not yet due, and
rewrites it.

6. **Timer loop.** A new `realtime.Timers` in `timers.go`. It is kept apart
from the 1400-line trigger engine and shares only the store and the config.

   ```go
   type TimerDeps struct {
       Store  timerStore    // consumer-side: ListSessionsDue, ListByLifecycle, SetSession
       Config ConfigSource
       Marker ChangeMarker
       Clock  Clock
   }
   func NewTimers(d TimerDeps) (*Timers, error) // Nil: every field rejected
   func (t *Timers) Reset()                     // leader start and stop: drop all memory
   func (t *Timers) RunRound(ctx context.Context) (TimerReport, error)
   type TimerReport struct {
       Fired, Rescheduled, Stranded, Failed int
       More bool      // writes were left for the next round, or a write is already due
       Next time.Time // earliest known due time after now
   }
   ```

`RunRound`: load the snapshot once. If realtime is off or the config is
degraded, do nothing. Run a recompute batch when one is pending (decision 7).
Then make one `ListSessionsDue(timerTypes, now + TimerCeiling, limit)` call for
all types with timers, where `limit` is `MaxTimerWritesPerRound + 1` plus the
number of keys in backoff, capped at `MaxTimerListPerRound`. For each listed
channel not in backoff, compute `nextTimer`:
- due at or before now: fire;
- otherwise, a normalized due time that differs from the stored one:
reschedule;
- otherwise, fold the due time into `Next`.

After each `SetSession`, the round reads the committed
`Outcome.Session.NextDueAt`. A value at or before now sets `More`, so a chain of
timers continues at the floor rather than at the ceiling. A later value is
folded into `Next`. Writes stop at `MaxTimerWritesPerRound` (64) and set `More`.

A fire is `SetSession(key, now, step)` with:
- `Decide`: a CAS. It moves only when, under the lock, the channel is
open, the current (state, generation) equals the listed one, and `nextTimer` on
the committed row still picks this timer with a due time at or before `now`. The
move is `{cur.State, timer.To}`. Anything else returns no move, and the step
degrades to a reschedule.
- `Timer`: the timer name. `NoticeType`: `session`. `NoticeBy`:
`systemAuthor()`. `TriggeredBy`: `session:<type>` (the store-only notice marker
from slice 1). `AuditTriggeredBy`: `timer:<type>/<state>/<name>`, matching
`auto:<type>/<op>`. `Due`: `sessionDue(s)`.

A reschedule is the same call with a decider that never moves. `ErrConflict` and
`ErrNotFound` are skipped. Failure isolation: any other error on one channel is
logged with its key, counted in `Failed`, and the key goes into an in-memory
backoff for `TimerBackoff` (5 minutes, at most `MaxTimerBackoffKeys` keys,
oldest dropped). The round continues with the next channel, so one failing row
never blocks a type. `Reset` clears the backoff. A damaged `meta.yaml` costs
only that channel (decision 3). After a write the round calls
`Marker.Mark(key)`.

Semantics:
- The move happens exactly once per (state, generation). The attempt is
at least once. If a commit is lost, or two leaders overlap for up to 10s, the
second attempt sees the new generation and moves nothing (AC 11
`LeaderOverlap`).
- The fire time is `now` of the round. It is the notice time and the new
state's `entered_at`.
- A message and a timer racing: whichever the store serializes first
wins; the other is judged against the new state.

`realtime.DueWake`, a small type of its own in `duewake.go`: `Note(t time.Time)`
keeps the earliest noted time under a mutex and pings a channel with capacity 1;
`C() <-chan struct{}`; `Take() time.Time` returns and clears the earliest time.
It is built once at wiring (in `refHolders`, so services rebuilt by a schema
reload share it), and is a required field of the service and system-writer deps
and of the worker parts. `Local` does not change. Non-leader processes note due
times that nobody reads; that costs nothing.

Host, in `realtime_worker.go`: `realtimeWorkerParts` gains `timers` and
`dueWake`, and `buildRealtimeWorker` constructs `Timers` with `Clock: time.Now`.
`lead()` calls `timers.Reset()` and runs one round at start. A `time.Timer` is
set to `timerDelay(rep.Next, rep.More, now)`. That pure function clamps to
[`TimerFloor` 1s, `TimerCeiling` 60s]; `More` gives the floor and a zero `Next`
gives the ceiling. Two new `select` cases:
- the timer fires: run a round and reset the timer;
- `dueWake.C()` fires: `Take()` the noted time; if it is before the
current target, move the target there, but never closer than the floor from the
last round. A wake with a later due time starts no round.

Moves therefore never cause scans. Scans happen at due times and at the ceiling,
one walk per round on the file tier. Moves made in another process (CLI, MCP, a
second rela-server) reach the leader through the ceiling query, so such a timer
fires at most 60s late; the minimum fixed period is 1m. Clock injection:
`Timers` takes `Clock`, and the host's scheduling arithmetic is the pure
`timerDelay`. Tests drive `RunRound` with the test clock and do not rely on real
timers.

Principal: `lead()` already puts `system:realtime` on ctx. The notice author and
the audit `By` are `systemAuthor()`. No ACL is evaluated: a timer is config
acting on the channel store, like `auto.after`.

7. **Recompute, reload and stranded states.** A recompute pass starts:
   - on the first round after leader start;
   - on the first round that sees a new config pointer, and a second time
one `TimerCeiling` later, to catch writes made under the old config while the
reload spread;
   - every `TimerRecompute` (10 minutes, with a tenth of jitter either way,
like `reconcileDelay`), to repair hints written by processes that had not
reloaded yet.

A pass lists once: one `ListByLifecycle(typ, open, now+1µs)` per session type,
for the config it started with. It keeps the channels that need a write in
memory and drains them in batches that share the round cap with fires, across
rounds. A pending pass is dropped when the config pointer changes (a new pass
starts) and on `Reset`. Each queued write re-derives under the lock, so a
channel that changed since the listing gets the right value. The pass decides
per channel:
- stranded (state not declared): count it per state. If it has a stored
due time, queue a reschedule to clear it. Log one warning per (type, state) with
the count: "realtime: sessions are in a state realtime.yaml no longer declares;
declare it again with a transition out". This is the reload log slice 1
deferred. It logs once per loaded config, not on every periodic pass.
- types with timers: a normalized derived due time that differs from the
stored one: queue a reschedule.

Reload rules, documented:
- a timer counts from its anchor, so adding or shortening a timer can
fire it in the next round, for old and new sessions alike;
- removing a timer, or the whole `after`, clears the due time;
- removing `session:` leaves the stored fields; that type is no longer
listed, and the fields come back with the block;
- the leader sees a reload at the latest after one ceiling (60s). The
wire shows the new due time at once, because the view derives it (decision 8).

Stale `next_due_at` on a type that lost all timers is never queried. The next
recompute after timers return fixes it.

8. **Wire and rendering.** `SessionView` gains `DueAt *time.Time
json:"due_at,omitempty"` and `Timers []SessionTimerView
json:"timers,omitempty"`, with `SessionTimerView{Name, Label, To, Since string;
DueAt *time.Time}`. They are derived by `nextTimer` and `timerDue` from config
and the row, never from the stored hint:
   - `Timers` lists the current state's timers for every reader (this is
config);
   - `DueAt` (session level and per timer) is set only for a stored, open
channel under a config that is not degraded;
   - a closed channel lists its timers without due times (frozen);
   - under a degraded config the timers are listed without due times,
because the leader does not fire them;
   - a final or stranded state, or a channel not yet stored, shows none.

`sessionView` takes the snapshot's `Degraded` flag; its callers already hold the
snapshot. The view stays pure, so list reads make no extra store calls (AC 17).
Rest of the surface:
- `apiwire/v1`: `ChannelSession.DueAt string json:"due_at,omitempty"`,
`Timers []ChannelSessionTimer`. `dataentry/channels_wire.go` maps them.
- `frontend/src/types/channels.ts`: mirror the types and extend
`CHANNEL_WIRE_FIELDS`. Types only; slice 2 renders them.
- `internal/openapi/channels.go`: a `ChannelSessionTimer` schema and the
two properties, plus the `timer` notice prop in the message description.
- MCP: `read_channel` and `list_channels` carry the fields through
`ChannelView`, and `post_message` through `messageResult.Session`. The
`read_channel` description mentions `due_at`. A fire raises rev, so
`read_channel` with `watch` wakes on it.
- CLI: `sessionText` appends "next: <timer> to <state> at <time>".
Notices with a `timer` prop render "moved from X to Y (timer <name>)".

9. **Limits** (`internal/realtimelimits`, server-owned, not in `limits:`):
`MaxSessionTimers = 8` (per state), `MinSessionTimer = time.Minute` (fixed
periods only), `TimerFloor = time.Second`, `TimerCeiling = time.Minute`,
`TimerRecompute = 10 * time.Minute`, `MaxTimerWritesPerRound = 64`,
`MaxTimerListPerRound = 256`, `TimerBackoff = 5 * time.Minute`,
`MaxTimerBackoffKeys = 128`.

**Alternatives rejected:**
- Derived scan per (type, state, timer), like `auto.after` (c1): one query
per timer per tick, minute precision, and `since: session` needs its own index.
The research chose c2.
- A separate timer table (c3): two writes to keep consistent per transition
and a cancel path that can leak. It remains the upgrade path for SLA pause.
- `next_due_at` as the truth: every reload would need writes before the
wire could show a due time. Deriving the due time and keeping `next_due_at` as a
hint keeps the wire right at once.
- Deriving the restart from the last `ReopenedAt` only: a second
close/reopen loses the first restart point and stretches the deadline. The
per-timer restart anchor, written in the resume step, keeps it.
- An accumulated paused duration: it shifts every timer by the closed time
(SLA pause), which is not the decided rule; only timers that came due restart.
- A `started_at` column with "first seen" arming for old channels: it
shields only generation-0 sessions, rewrites the displayed `entered_at`, and
adds arm writes and a rev special case. `since: session` counts from
`created_at` instead.
- An optional `Due` on steps: a move without it would zero the hint and the
timer would never fire.
- Re-listing every open channel each round while a capped recompute drains:
quadratic reads. The pass lists once and drains from memory.
- An in-memory heap in the leader for filechannels (research note): it is
backend-specific and still needs a periodic rescan for moves in other processes.
A wake carrying the due time, with one walk per round at due times and at the
ceiling, gives the same bound for every backend.
- One `ListSessionsDue` call per type: one tree walk per type per round on
the file tier. The method takes a set of types.
- Waking the loop on every channel change (`Local.Subscribe`): every post
would cause a scan on the file tier. Putting the wake in `Local` was also
rejected: it has one writer side and one reader, the `tombstoneConfirmer.wake`
shape.
- Allowing timer-only cycles: a channel would append 1440 notices and audit
records a day forever. Slice 4 brings bounded reminders.
- Timers keyed by duration (`after: { 72h: {to: ...} }`, the research
sketch): no stable name for the audit label, no place for a label, and two
timers with one period are impossible. A per-state `timers:` list was rejected
because slice 4 puts reminder `timers:` at the session level, and the ticket
names `after:`.
- `from:` as the anchor key: it already means "source states" on lifecycle
ops in the same file.
- `after:` on a transition: a transition is keyed by a message type, and a
timer has no message.
- Restarting every timer from the reopen: it stretches deadlines that had
not come due. The decision says due timers restart.
- `run_as` now: there are no graph writes yet; adding an identity that
grants nothing would be a dead check.
- `jobs.Queue`: it has no delayed jobs, and a fire has no external side
effect (`jobs.md`).
- The notice `triggered_by` set to the timer label: it would break the
store-only `session:` notice marker that protects notices from forgery and
edits. The notice keeps `session:<type>` and gains the `timer` prop; the audit
record carries `timer:<type>/<state>/<name>`.

**Files to modify:**
- `internal/realtimecfg`: `raw.go`, `keys.go` (`from` refusal), `session.go`
(timers, validation, cycle check), `testdata/valid.yaml` (+ `load_test.go`,
`compiled_test.go`, `reload_test.go`).
- `internal/realtimelimits/limits.go`.
- `internal/channels`: `session.go` (fields, `SessionSchedule`,
`ScheduleFunc`, `ApplySchedule`, `TimerNotice`, `TimerAudit`, `ApplySession`),
`channels.go` (`NewChannel.SessionDue`, `Transition.SessionDue`), `store.go`
(two methods, `Append`, `SetLifecycle` and `Untombstone` contracts),
`channelstest/channelstest.go`, `channelstest/session.go` (steps gain `Due`) and
new `channelstest/timers.go`.
- `internal/channels/sqlchannels/sql.go`, `retention.go`;
`internal/channels/filechannels` `file.go` (`visitTypes`), `ops.go`,
`retention.go` (free-function moves, `Untombstone`) (+ `file_test.go`);
`internal/channels/pgchannels/pg_test.go`;
`internal/channels/sqlitechannels/sqlite_test.go`.
- `internal/sqlitedb`: `sqlitedb.go` (columns, `channelTimersDDL` after the
ladder), `migrate.go` (+ new test with a v24 fixture).
- `internal/store/pgstore/migrations/0032_channel_session_timers.sql`.
- `internal/realtime`: new `timers.go` and `duewake.go` (+ tests),
`session.go` (`Due` on steps, views), `service.go` (`Create`, `noteDue`),
`lifecycle.go` (reopen `SessionDue`), `retention.go` (`Untombstone`),
`system.go`, `render.go` (+ `session_test.go`, `trigger_test.go`,
`lifecycle_test.go`).
- `internal/appbuild`: `realtime_worker.go`, `realtime_service.go`
(`DueWake` wiring), `channelguard.go` (`Untombstone`) (+ worker test).
- `internal/apiwire/v1/channels.go`; `internal/dataentry/channels_wire.go`
(+ golden files, handler and budget tests).
- `internal/openapi/channels.go` (+ test).
- `frontend/src/types/channels.ts`.
- `internal/mcp/tools_channels.go` (+ test).
- `internal/cli/channel_render.go` (+ test).
- `docs-project/entities/guides/GUIDE-realtime.md`,
`GUIDE-mcp-server.md`; `docs/realtime.md` and `docs/mcp-server.md` by `just
docs`.

## Security Considerations

- [x] Input sources identified (user input, config, external APIs)
- [x] Input validation approach defined (allowlist preferred over blocklist)
- [x] Security-sensitive operations identified (file access, auth, crypto)
- [x] Error handling doesn't leak sensitive information

**Input Sources & Validation:**
- `realtime.yaml` (operator config, not secret): timer names follow the name
grammar; `to` must be a declared state; periods parse with the existing
allowlist syntax and are bounded below by 1m (fixed periods) and above by the
`Period` count cap; at most 8 timers per state; every cycle made only of timers
is refused.
- No request carries a timer, a due time or a restart anchor. The fields are
written only by store steps whose callbacks the service builds from config.
`SetSession` has no HTTP, MCP or CLI path: only the worker calls it.
- `Append` refuses a step with `Timer`, so a post cannot pose as a timer
fire. The notice marker rule from slice 1 still holds. The store caps and
validates the restart anchors it stores.

**Security-Sensitive Operations:**
- Who moves a session by timer: only config. The move is written as
`system:realtime`, with no ACL evaluation and no graph write, as `auto.after`
and triggers do. No principal's authority is borrowed, so no write can be
laundered. Graph outputs and `run_as` arrive together in slice 5, with the
load-time grant check from decision 3.
- Disclosure: `due_at` and `timers` are derived from config and from row
times every channel reader already sees (`entered_at`, `created_at`, the
lifecycle times). Configuration is not secret (CLAUDE.md). The restart anchors
are not on the wire; the due time reveals them only to readers of the channel.
- Denial of service: the 1m floor, the timer-cycle refusal, the per-round
write and list caps, the 1s floor between rounds, the backoff for failing rows
and leader-only execution bound the write rate. The filechannels scan is one
walk per round, pinned by AC 9.
- The decider and the schedule callbacks run under the store lock and do no
I/O.
- Errors: config problems name keys and values. Store errors in the round
are logged with the channel key only, as the sweep does.

## Test Plan

- [x] Test scenarios documented for each acceptance criterion
- [x] Edge cases identified and documented
- [x] Negative test cases defined (invalid input, error conditions)
- [x] Integration test approach defined (not just unit tests)

**Test Scenarios:** one test or table row per acceptance criterion, named there.
Layers:
- realtimecfg unit tests (AC 1 to 5);
- store conformance on four backends (AC 6), postgres gated on
`RELA_TEST_DATABASE_URL` and run with `just test-postgres`;
- opening a real v24 sqlite file, index use and the file tier (AC 7 to 9);
- pure due and resume computation (AC 10);
- timer rounds over the file store on MemFS with the test clock and swapped
config snapshots (AC 11 to 12);
- service and trigger tests (AC 13);
- worker scheduling and the wake type (AC 14);
- HTTP golden, frontend mirror, OpenAPI, budget, MCP and CLI (AC 15 to 19);
- docs (AC 20).

Race detector on; `LeaderOverlap` and `testSetSessionCAS` exercise the
concurrency.

**Edge Cases (each has a test):**
- A message and a timer race (AC 11 `MessageWins`).
- Two leaders fire at once (AC 11 `LeaderOverlap`, AC 6 `CAS`).
- Closed, then reopened before or after the due time, and two close/reopen
cycles (AC 10, AC 11 `ClosedFreezes`).
- Restore from a tombstone (AC 6, AC 11 `RestoreFromTombstone`).
- `since: session` in a later state (AC 11 `FromSession`).
- A chain of timers whose next one is already due (AC 11 `ChainFires...`).
- Two timers in one state, tie on due time; ties in the due listing (AC 6,
AC 10).
- Stranded and final states (AC 11).
- Reload shortens or removes a timer; a stale zero hint from an older
config (AC 11).
- Degraded config, in the round and on the wire (AC 11, AC 15).
- The round cap and continuation; the recompute drained across rounds (AC
11, AC 12).
- A failing row and a damaged `meta.yaml` (AC 9, AC 11).
- A legacy `meta.yaml`, a legacy postgres row and a real v24 sqlite file (AC
7, AC 9).
- Not covered by a test: the 60s cross-process latency (a property of the
ceiling, documented) and a crash inside the filechannels fire window (same
window as slice 1, documented in the package doc).

**Negative Tests:** AC 2 (every load rule, including every timer cycle and the
`from` key), AC 6 (a step without `Due`, `SetSession` on a missing channel, a
move without `Timer`, an invalid state name, `Timer` on an `Append`, an empty
type list, a limit of 0), AC 11 (no fire before the due time, while closed,
after a message moved the session, in a stranded state, under a degraded
config).

## Risk Assessment

- [x] Technical risks assessed with mitigations
- [x] Security risks assessed (see Security Considerations)
- [x] Effort estimated (xs/s/m/l/xl)

**Risks:**
- A runaway loop of moves. Mitigation: every timer-only cycle is refused at
load, plus the 1m floor, the per-round cap and the 1s round floor (AC 2, AC 11).
- The filechannels scan grows with the number of entity directories, face
directories and channels of the timer types, because it walks the whole tree.
Mitigation: one walk per round for all timer types, rounds only at due times and
at the 60s ceiling, never per move (AC 9, AC 12). The fs and desktop tiers are
small; the database tiers use the index (AC 8).
- sqlite `Open` on an existing database fails if the due index runs before
the ladder. Mitigation: the index is post-ladder, and AC 7 opens a real v24 file
through `Open`.
- Stale hints from writes under an older config. Mitigation: the recompute
runs twice after a config change and every 10 minutes (AC 11
`RecomputeRepairsStaleHint`).
- Changing `Untombstone` and `SetLifecycle` touches existing callers.
Mitigation: both callbacks are optional, nil keeps today's behaviour, and
conformance pins both (AC 6).
- Making `Due` required touches the slice-1 steps and tests. Mitigation:
the conformance steps gain a zero `Due`; production steps already go through
`newSessionPost`.
- Migration numbering collides with another branch (0032, v25). Mitigation:
renumber at rebase; the 0031 header already states the rule.
- A plimsoll cap on both store types. Mitigation: the free-function moves
in Approach 2 and `just plimsoll`.
- Merge conflicts with slice 2 in `frontend/src/types/channels.ts` and the
wire golden files. Mitigation: append-only edits; regenerate the golden files
after rebasing.
- The worker's `lead()` select grows by two cases. Mitigation: all logic is
in `realtime.Timers` and the pure `timerDelay`; the worker only routes.

**Effort:** l

## Documentation Planning

- [x] User-facing docs identified (skip if internal refactor)
- [x] Docs-checklist will be created when entering implementation

**Documentation Impact:** Enhancement. Edits go into
`docs-project/entities/guides/GUIDE-realtime.md`; `just docs` regenerates
`docs/realtime.md`.
- "Sessions": a "Timers" part with the example, the `after:` keys table
(`period`, `since`, `to`, `label`), how anchors work (`since: state` versus
`since: session`, which counts from the channel's creation), what happens while
the channel is closed or its entity's channels are tombstoned (frozen; timers
that came due restart from the reopen or restore; deadlines that were not due
are kept, also across repeated closes), final and stranded states, the refusal
of timer-only cycles, exactly-once moves, firing precision (within a second for
moves in the leading process, within a minute otherwise), the timer notice
(`timer` prop) and the audit `triggered_by: timer:<type>/<state>/<name>`, and
that timers do not write to the graph yet.
- "Checks when the file loads": the timer rules.
- "Limits": the timer limits.
- "HTTP API" and "Agents and other processes": `due_at` and `timers`, and
that `due_at` is absent while the channel is closed or the config is degraded.
- "Reloading": the reload rules from Approach 7, the stranded warning, and
that timers do not fire under a degraded config.

`GUIDE-mcp-server.md`: the `read_channel` row mentions `due_at`. Code: godoc on
`SetSession`, `ListSessionsDue` (with its real file-tier cost), `ScheduleFunc`,
`ResumeSchedule`, `Timers`, `DueWake` and the filechannels package doc sentence.

## Design Review

- [x] Run `/design-review` before starting implementation
- [x] All critical/significant findings addressed in plan

**Design Review Findings:** the 16 findings of `/design-review`
(`.ignored/dr-inm3az.tsv`) are recorded as review-response entities linked to
TKT-INM3AZ via `has-review-response`. All are addressed in this plan:
- the sqlite due index moved after the ladder, and AC 7 opens a real v24
file;
- a per-timer restart anchor replaces the last-reopen rule, with two-cycle
tests;
- the recompute lists once per config and drains across rounds, and runs
again one ceiling after a config change and every 10 minutes;
- `ListSessionsDue` takes a set of types, with one walk per round, and AC 9
counts directory reads;
- every timer-only cycle is refused;
- round writes feed `Next`, failing rows are isolated with a backoff, and a
damaged `meta.yaml` skips only that channel;
- restore from a tombstone is a resume;
- `started_at` and arming are dropped;
- `Due` is required, ties are ordered by key, and a limit of 0 or less is
an error;
- due times are compared normalized, and calendar periods pass the minimum;
- the audit label is `timer:<type>/<state>/<name>`;
- the wake is a `DueWake` type of its own, and the anchor key is `since`;
- under a degraded config the wire omits `due_at`.

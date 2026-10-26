---
id: PLAN-61IBGQ
type: planning-checklist
title: 'Planning: Sessions 4/4: non-interrupting timers'
started: "2026-10-24"
completed: "2026-10-24"
status: done
---

<!-- @managed: claude-workflow v1 -->

## Understanding

- [x] Problem/requirements clearly understood
- [x] Scope defined (what's in/out documented below)
- [x] Acceptance criteria documented with specific test scenarios

**Problem:** Slices 1 to 3 (TKT-Y6NJ6U, TKT-190E1N, TKT-INM3AZ, the last in
review as PR #1967) gave a channel type a flat session state machine, moved by
typed messages and by interrupting `after:` timers. The GDPR breach use case
also needs a reminder that does not move the session: "post a status update
every 30 minutes while the breach is detected, contained or assessed". The
sessions research (RES-O8I8S7, `.ignored/res-sessions-final.md`, decision 1,
option a2) chose session-level timers with `in: [states]`, so one reminder
covers several states without being copied into each. Decision 2 still holds: a
session runs only while its channel is open, and timers that came due while the
channel was closed are skipped and restart from the reopen. This is slice 4 of
the sessions feature (FEAT-GWF8YH phase 4).

Terms used below:
- **Reminder**: one entry of `session.reminders`. While the session is in one
of its `in:` states and the channel is open, it posts a message every `every`.
It never moves the session. The research calls these non-interrupting timers. In
config, Go, guards and on the wire the word is "reminder"; "timer" keeps meaning
the per-state `after:` entries of slice 3.
- **Span**: the time a session stays inside a reminder's `in:` states without
leaving them. A move from one `in:` state to another, or a re-entry, stays in
the span. A move to a state outside `in:` ends it.
- **Reminder anchor (`since`)**: the time the reminder's slots count from. It
is the start of the span, or the resume time after a restart.
- **Slot**: a time a reminder may post: `since + first + k * every` for
k = 0, 1, 2, ...
- **Next slot**: slot 0 when the reminder has not posted in this span,
otherwise the first slot strictly after the last post.
- **Reminder record**: what the channel row stores per reminder: `since`,
`last` (the time of the last post in this span, zero for none) and `count`
(reminders posted in this span).
- **Reminder message**: the message a reminder posts. Its type is a message
type the operator declares; its `triggered_by` carries the store-only
`reminder:` marker.
- **Reminder counter**: the record's `count`, readable in transition guards
as `session.reminders['<name>']`.
- **Slack**: `ReminderSlack` (30s). A reminder's contribution to
`next_due_at` is its next slot plus the slack, so the leader wakes for several
nearby slots once.
- Timer, anchor time, restart anchor, resume, due time, `next_due_at`, fire,
reschedule, round and recompute keep their meaning from the 4/3 plan
(`.ignored/plan-inm3az-body.md`).

**Scope:**
- In: `session.reminders` in `realtime.yaml`: a map from reminder name to
`{in, every, first, max, label, post: {type, props}}`. Load-time validation, one
problem per key, and refused keys with a reason.
- In: one new session field on the channel row, the reminder records, in
every channel store backend: `filechannels` (disk and MemFS), `sqlitechannels`
and `pgchannels` over `sqlchannels`. sqlite rung v26, postgres migration
`0033_channel_session_reminders.sql`, and a `meta.yaml` field. No new index:
reminders share `next_due_at` and its index.
- In: `channels.SessionStep` gains an optional `Remind` callback that returns
the reminder message and the reminder's new record, so that `Store.SetSession`
can append the message and store the record in one store step. `SessionSchedule`
gains `Reminders`. No new `Store` method.
- In: the timer loop (`realtime.Timers`) posts reminders next to interrupting
timers, from the same due listing and within the same round cap. `next_due_at`
is the earliest of the timers' due times and the reminders' next slots plus the
slack.
- In: the reminder counter in transition guards (`session.reminders`).
- In: reminder messages protected from edit, author delete and forgery.
Moderators may remove them, with the audited reason other removals have.
- In: the reminders on the wire: `reminders` in the session block of the HTTP
head, page and list answers, MCP `read_channel`, `list_channels` and
`post_message`, the CLI header, the OpenAPI `ChannelSession` schema and the
frontend wire-field mirror (types only).
- In: budget tests for many channels with one-minute reminders, and for a
reminder backlog ahead of a deadline.
- In: docs in `docs-project/entities/guides/GUIDE-realtime.md` and the
`read_channel` row of `GUIDE-mcp-server.md`.
- Out (later slices): `outputs:`, `run_as`, `on_final: close` (slice 5);
`status:`, `waiting_for:` (slice 6); `count()` guards and the menu filter (slice
7). Out: any UI component and frontend helpers, as in slice 3; a reminder
message already renders in the SPA as an ordinary message of its declared type
from the system author. A frontend `isReminder` helper is added by the slice
that renders reminders. Out: reminders in the schema endpoint descriptor (slice
3 left `after:` timers out of it too). Out: calendar periods (`mo`, `y`) for
reminders. Out: reminder messages that send mail or push notifications.
- Out: an empty `post:` for message types only reminders or triggers post.
That concerns triggers as much as reminders and is a separate change.

**Example (the `breach-response` type in `testdata/valid.yaml` gains the
`reminder` and `escalate-stalled` message types, the `escalate-stalled`
transition and `reminders:`; the docs use it):**

```yaml
    message_types:
      # ... the slice 1 to 3 types ...
      reminder:         { properties: { body: { type: markdown, required: true } }, post: [lead] }
      escalate-stalled: { properties: { note: { type: string } }, post: [dpo] }
    session:
      label: GDPR breach response
      initial: detected
      states:
        detected:
          # ... as in slice 3 ...
        contained:
          label: Contained
          on:
            assessed:         { to: assessed, when: "message.risk ~= 'none'" }
            escalate-stalled: { to: overdue, when: "session.reminders['status-update'] >= 3" }
          after:
            notify-deadline: { period: 72h, since: session, to: overdue, label: 72 hours since detection }
        # ... assessed, overdue, notified, dismissed as in slice 3 ...
      reminders:
        status-update:
          label: Status update
          in: [detected, contained, assessed, overdue]
          every: 30m
          first: 10m
          max: 48
          post:
            type: reminder
            props: { body: "Post a status update: what changed since the last one?" }
```

**Acceptance Criteria:**
1. The example compiles. `Session.Reminders()` returns the reminders sorted
by name, each with `Name`, `Label` (defaults to the name), `In()` (a copy, in
file order), `Every`, `First` (defaults to `Every`), `Max` (0 for no limit),
`PostType` and `Props()` (a copy). `Session.RemindersIn(state)` returns those
whose `in:` holds the state, and `Session.Reminder(name)` finds one.
`Session.HasSchedule()` is true for a session with timers or reminders, and
`HasTimers()` keeps its meaning. A state with `on:` and no `after:` that only a
reminder covers loads. A reminder type that declares `actions` loads, and its
actions stay declared. Test `TestParse_SessionReminders` in
`internal/realtimecfg/load_test.go`; `TestParse_Golden` updated.
2. Each invalid reminder reports its key path and reason, all in one load.
Table-driven `TestParse_SessionRemindersInvalid`, one row per rule in Approach,
decision 1: more than 8 reminders; a bad reminder name; missing or empty `in`;
an undeclared state in `in`; a final state in `in`; a state listed twice in
`in`; missing `every`; an `every` that does not parse; a calendar `every`; an
`every` under 1m; a `first` that does not parse, is a calendar period or is
under 1m; `max` under 1 or over 1000; missing `post`; missing `post.type`; an
undeclared `post.type`; an ephemeral `post.type`; a `post.type` that keys an
`on:` entry of some state; a `post.props` key the type does not declare; a `{{`
in a prop; a literal on a property that is not string, markdown or enum; a
required property not set; a prop over its `max`; props over `message_bytes`; a
type whose `when:` the props do not meet; a type whose `when:` calls `today()`.
3. Refused keys, each with its reason: `session.timers`,
`session.states.<s>.reminders`, `session.reminders.<n>.to`,
`session.reminders.<n>.period` and `session.reminders.<n>.since`. Rows in
`TestParse_Invalid`.
4. A guard naming the counter compiles only for a declared reminder:
`session.reminders['status-update'] >= 3` loads; `session.reminders['nope']` is
"does not compile: unknown attribute ..."; a session without reminders declares
an empty `session.reminders` record. A reminder with a bad `every` whose name is
valid gives only its own `every` problem: a guard naming it reports no second
error. Rows in `TestParse_Session` and `TestParse_SessionInvalid`.
5. Adding, removing and renaming a reminder and changing `in`, `every`,
`first` or `max` pass `CheckReload`. Rows in
`TestCheckReload_SessionChangesAllowed`.
6. The compiled reminders are immutable: the slice and map fields are
unexported and every accessor returns a copy. Row in
`TestCompiled_SessionIsReadOnly`.
7. Store conformance, in `channelstest.RunAll` (file on disk, file on MemFS,
sqlite, postgres), new `channelstest/reminders.go`:
   - `testSetSessionRemind`: on a session in `contained` at generation 1, a
step whose decider returns no move and whose `Remind` returns a post appends one
message at `last_seq + 1` with the post's type, props and `TriggeredBy`, the
step's `NoticeBy` as author and `at` as time. It stores the post's `Record`
under its `Name` before calling `Due`: the channel `Due` receives already holds
it. It stores the `Due` result, raises rev, writes no audit record and leaves
state and generation as they were. `Outcome.Reminded` is true. The returned
channel and `Head` agree.
   - `testSetSessionMoveBeatsRemind`: when the decider moves, `Remind` is not
called (a counting callback), and only the notice is appended.
   - `testSetSessionRemindCAS`: two concurrent `SetSession` calls whose
`Remind` posts only while the committed record has a zero `Last` append exactly
one reminder message.
   - `testSetSessionRemindRefusals`: a post whose `TriggeredBy` does not
start with `reminder:`, an invalid type name, an invalid reminder name, a record
with a negative count, a record whose `Last` is after `at`, and props over
`MaxMessageBytes` are refused (`ErrInvalidMessage`, `ErrTooLarge`), and nothing
is stored. An `Append` whose `TriggeredBy` starts with `reminder:`, and an
`Append` step with `Remind`, are `ErrInvalidMessage`.
   - `testSessionRemindersStored`: the reminder records round-trip through
the `Due` of an `Append` step, `CreateMany` with `SessionDue`, `SetLifecycle`
reopen and `Untombstone`. `Since` and `Last` given with nanoseconds and a
non-UTC location read back `time.Equal` to their `NormTime` on all four
backends. A schedule with a nil `Reminders` keeps them; a move clears them
unless `Due` returns them. More than 8 records after `Due`, an invalid name, or
a negative count is `ErrInvalidMessage` and nothing is stored.
   - `testSessionScheduleSurvivesMoves` extended: the records are kept across
`SetLifecycle` close, `Rekey` and `Tombstone`.
8. Schema steps: `TestOpenV25Database` in `internal/sqlitedb` writes a real
v25 database file (the v25 `channels` DDL as a fixture, `user_version` 25) and
opens it through `Open`. The column exists with the shape of a fresh database,
an existing row reads with no reminder records, opening twice is harmless, and
`user_version` is 26. Postgres: conformance runs on the migrated schema, and
`TestLegacyRowHasNoReminders` in `pg_test.go` inserts a row without the column
and reads it the same way.
9. filechannels: `TestMetaWithoutRemindersReadsEmpty` (a slice-3
`meta.yaml` reads no records). `TestListSessionsDue_Budget` still holds with
reminder types in the fixture (one walk per call, no log segment read).
10. Slot computation, `internal/realtime/reminders_test.go`,
`TestNextSlot` table: an unset session in an `in:` initial state counts from
`created_at`; a session that moved into the span counts from the move; with a
zero `last` the next slot is `since + first`; otherwise it is the first slot
strictly after `last`, so slots missed while the leader was down are skipped
(with `every` 30m and `first` 30m, a post at `since + 2h40m` gives the next slot
`since + 3h`); with `max` reached there is none; a reminder not active in the
state has none; a stranded state has none. Reload rows: `every` lengthened from
1m to 30m after 100 posts gives a next slot at most 30m after `last`, not 50h
later; `every` shortened gives the first new-grid slot after `last`; a changed
`first` moves the grid, and the next slot is still the first one after `last`.
`TestNextDue_Both`: `next_due_at` is the earlier of the first timer due time and
the first reminder's next slot plus `ReminderSlack`, in both orders and on a
tie; the result is normalized (`channels.NormTime`).
11. Carry, resume and pruning, `reminders_test.go`:
    - `TestStepScheduleReminders` table: a move between two `in:` states
keeps `since`, `last` and `count`, and stores an implicit record explicitly; a
re-entry keeps them; a move out of `in:` drops the record; a move into `in:`
from outside starts a new span (no record, `since` is the move time); a message
that moves nothing keeps the records; a move that also carries `since: session`
restart anchors (slice 3) keeps both.
    - `TestResumeScheduleReminders` table: a reminder whose next slot came
due before the resume restarts (`since` = resume time, `last` zero, `count`
kept); one not yet due keeps its record; two close and reopen cycles keep a slot
that was not due at the second reopen; interrupting timer anchors are resumed as
in slice 3 in the same call.
    - `TestSchedulesPruneUndeclared`: every schedule that returns a non-nil
map (step, reminder step, resume) keeps only records of reminders the current
config declares. A session with 8 stored records whose 8 reminders a reload
renamed gets its first reminder under the new names without an error and without
backoff, and stores at most 8 records.
12. Timer rounds, `timers_test.go`, over the file store on MemFS with the
test clock:
    - `TestTimers_RemindPosts`: a session entered `detected` at T0 gets its
first reminder in the first round at or after T0+10m, then one per 30m. Each
message has type `reminder`, the configured body, author `system:realtime` and
`triggered_by` `reminder:breach-response/status-update`. The counter rises by
one per message, state and generation do not change, and the key is marked for
live subscribers.
    - `TestTimers_RemindAcrossStates`: a move from `detected` to `contained`
at T0+25m keeps the phase: the next reminder is at T0+40m and the counter
continues.
    - `TestTimers_RemindStopsOutsideIn`: a move to `notified` stops the
reminders and the counter reads 0; a later move back into an `in:` state (in a
test config with such a transition) starts at 0 again.
    - `TestTimers_RemindMax`: after `max` reminders no more are posted; the
wire shows `count` = `max` and no `due_at`.
    - `TestTimers_RemindSkipsMissed`: rounds stopped for 2h10m give one
reminder at restart, not five, and the next one on the original grid.
    - `TestTimers_RemindTwoDue`: two reminders of one channel due in the
same round give one post in that round (one step per channel per round) and the
second in the next round, which `More` schedules at the floor.
    - `TestTimers_RemindClosedFreezes`: no reminder while closed; reopened
after the slot, the next reminder is at reopen + `first`; reopened before it, at
the original slot. The counter is kept.
    - `TestTimers_RemindRestoreFromTombstone`: a restore is a resume.
    - `TestTimers_RemindAndTimerSameRound`: when an interrupting timer and a
reminder are both due, the timer fires first; the reminder is judged against the
new state in the next round (it posts there when the new state is in `in:`, and
not when it is not).
    - `TestTimers_RemindLeaderOverlap`: two `Timers` over one store run a
round at the same clock time; exactly one reminder message is stored.
    - `TestTimers_RemindMessageRace`: a post that moves the session out of
`in:` before the round's write makes the reminder step post nothing.
    - `TestTimers_RemindReload`: a swap that shortens `every` reschedules in
the recompute; a swap that lengthens `every` from 1m to 30m after many posts
posts again within 30m of the last post; a swap that changes `first` keeps at
most one `every` between posts; a swap that removes the reminder clears its due
time and drops its record at the next write; a swap that adds a reminder to a
state of an old session posts once in the next round and then follows the grid.
    - `TestTimers_RemindDegraded`: under a degraded snapshot no reminder is
posted; the first round after the config is valid again posts it.
    - `TestTimers_RemindPostFails`: with props that fail `validateProps`
(an enum value the metamodel no longer has, injected through the snapshot), the
round counts the failure in `Failed`, holds the channel back for `TimerBackoff`,
does not set `More` for it, and posts nothing. `Reset` clears the backoff.
    - `TestTimers_ReminderBacklogBound`: 200 due reminders whose hints sort
ahead of a due interrupting timer. Rounds 1 to 3 post 64 reminders each; round 4
posts the remaining 8 and fires the timer. Rounds run one floor apart, so the
deadline is ceil(200/64) floor rounds late, the documented bound.
    - `TestTimers_ReminderSlack`: a channel whose only due time is a reminder
slot at S stores `next_due_at` S + `ReminderSlack` and reports `Next` S +
`ReminderSlack`; a round at S + 10s, run for another channel's timer, posts it
(slot at or before now); an interrupting timer's hint stays exact.
13. Guard: `TestSessionPost_ReminderGuard` in `session_test.go`: the dpo
posts `escalate-stalled` in `contained` after two reminders and nothing moves;
after the third reminder the same post moves the session to `overdue`. The guard
reads the committed counter under the store lock: with the third reminder
written between the call's head read and the append, the post moves the session.
A row with a stored record of 5 whose reminder a reload took out of
`contained`'s `in:` reads 0, and the post does not move.
14. Protection: `TestReminderMessageProtected`: on a reminder message, edit
and author delete answer 409 `reminder_message`; `_actions` holds no `edit` or
`delete`. A moderator sees `remove` and the message type's declared actions; a
moderator remove with a reason succeeds, writes the usual removal audit record,
and leaves the counter unchanged. A member's own post of type `reminder` (no
marker) is an ordinary message: it may be edited, and it does not raise the
counter.
15. Budget: `TestTimers_ReminderBudget` in `timers_test.go`, with the
counting store wrapper of `TestTimers_Budget`: 200 channels with `every: 1m`
reminders on staggered phases, the loop driven by `NextTimerRound` and the test
clock for 10 simulated minutes. Every channel gets exactly one reminder per
slot, none before its slot. Each round makes one `ListSessionsDue` call, and
there are at most 6 rounds per simulated minute (2 slack rounds plus
ceil(200/64) drain rounds). The existing budget rows still hold.
16. Service and wake: `TestSessionPost_NotesReminderWake`: a post on a type
with a reminder notes `next_due_at` (the next slot plus `ReminderSlack`) on the
`DueWake`; `TestSessionCreate_SetsReminderDue`: `Create` of a `cardinality:
many` type whose initial state has a reminder stores its due time.
17. Wire: `TestChannelWireGolden` regenerated (`channel_wire_fields.json`
gains `reminders` and `ChannelSessionReminder`, `channel_wire.json` a sample
reminder message). The mirrored fields pass
`frontend/src/types/wireFields.test.ts`.
`TestChannelsHandler_SessionRemindersInReads` checks `reminders` on the head,
page and list for an open channel (with `count` and `due_at` equal to the slot,
not the hint); on a closed channel and under a degraded snapshot the reminders
are listed with `count` and without `due_at`; in a state outside `in:` there are
none.
18. OpenAPI: `TestOpenAPI_ChannelSession` extended for `reminders`
referencing `ChannelSessionReminder`, and the reminder marker in the message
description.
19. Collection cost: `channels_budget_test.go` with a reminder type in the
fixture; the list endpoint's store-call budget is unchanged.
20. MCP: `TestReadChannel_IncludesSessionReminders` and
`TestPostMessage_ReturnsSessionReminders` in `tools_channels_test.go`.
21. CLI: `TestRenderRead_SessionReminder` in `channel_render_test.go` prints
"reminder status-update: 2 of 48, next at <time>" in the session line, and
renders a reminder message with the "reminder" note.
22. Docs: the guide example is the `breach-response` block of
`testdata/valid.yaml` (AC 1 pins it), and `just docs` regenerates
`docs/realtime.md` and `docs/mcp-server.md` without diff.

## Research

- [x] For larger features: run `/research` to create a structured research doc
- [x] ~~Searched for existing libraries that solve this problem~~ (N/A: covered by the research doc; reminders reuse the slice 3 timer loop and store step)
- [x] Checked codebase for similar patterns or reusable code
- [x] Looked for reference implementations in other projects
- [x] Reviewed relevant rela concepts for prior art

**Research Doc:** RES-O8I8S7 (`.ignored/res-sessions-final.md`). Decisions of
2026-10-21: flat states plus session-level timers with `in:` and a `count()`
guard (a2); the session runs only while the channel is open, and timers due
while closed are skipped and restart from the reopen; `next_due_at` on the
channel row (c2). The research sketch `timers: { status-update: { every: 30m,
in: [...], post: { type: reminder } } }` is the shape this slice implements,
under the key `reminders:`: the sketch was illustrative, and `timers` already
means the per-state `after:` entries on the wire and in Go.

**Existing Solutions:**
- Slice 3 timer loop (`internal/realtime/timers.go`): one `ListSessionsDue`
per round for all types, CAS deciders, recompute, backoff (`holdBack`), round
cap, `withSessionDue` capturing the left channel, `carriedRestarts`,
`ResumeSchedule`, `Resumption`. Reminders extend each of these rather than
adding a second loop.
- `channels.SetSession`, `TimerNotice`, `ApplySchedule` and the restart
anchors (`internal/channels/session.go`): the store step, `NormTime`
normalization and the JSON-microseconds column pattern (`encodeRestarts`,
`session_restarts`) the reminder records copy.
- Trigger posts (`realtimecfg/validate.go` `triggerProps`,
`triggerMessage`; `realtime/system.go`): literal props checked at load and again
at post, written as `system:realtime`, no rate bucket. The reminder `post:`
reuses both checks. Trigger posts already carry their type's declared actions,
which reminders keep the same way.
- Session notice protection (`isNotice`, `sessionNotice`, the `session:`
marker refused by `PrepareMessage`) and `sessionMessage` (author edit and delete
refused, moderator remove allowed and audited): the model for the `reminder:`
marker and the `reminder_message` 409.
- `DueWake`, `TimerSchedule`, `NextTimerRound` (`internal/realtime/duewake.go`):
unchanged; they read `next_due_at`, which now includes the slack.
- `predicate` bracket access on records (`walk.go` `walkAttrGet` accepts
`rec['key']` with a literal key), so hyphenated reminder names work in guards
and unknown names fail at compile time.
- Prior art (research table): BPMN non-interrupting boundary timer events
with a cycle (`R/PT30M`): fire a side path and stay in the activity, with the
cycle kept while the activity runs; XState `after:` cancelled on exit, which the
span rule mirrors for the `in:` set. Cron-style schedulers skip missed runs
rather than replaying them, which the next-slot rule follows.

## Approach

- [x] Technical approach chosen and documented
- [x] Approach builds on existing patterns (not reinventing)
- [x] Alternatives considered (document why rejected)
- [x] Dependencies identified (packages, APIs, types)

**Technical Approach:**

1. **Config.** `raw.go`: `rawSession.Reminders map[string]rawReminder`;
`rawReminder{Label string; In []string; Every, First string; Max *int; Post
*rawTriggerPost}` (the trigger post shape: `type` and literal `props`).
Compiled, in a new `realtimecfg/reminders.go`:

   ```go
   type SessionReminder struct {
       Name, Label string
       Every, First time.Duration // fixed durations; First defaults to Every
       EveryText    string        // as written, for the wire
       Max          int64         // 0: no limit
       PostType     string
       in           []string          // file order
       inSet        map[string]bool
       props        map[string]string // literal values
   }
   func (r SessionReminder) In() []string              // copy
   func (r SessionReminder) Active(state string) bool
   func (r SessionReminder) Props() map[string]string  // copy
   func (s *Session) Reminders() []SessionReminder          // sorted by name, copy
   func (s *Session) Reminder(name string) (SessionReminder, bool)
   func (s *Session) RemindersIn(state string) []SessionReminder
   func (s *Session) HasSchedule() bool                     // timers or reminders
   ```

Session level, not per state: the point of `in:` is one reminder over several
states. `in` order is read as decoded (a YAML sequence keeps its order), so no
node walk is needed. Reminder names share no namespace with `after:` names: they
are stored in their own map and labelled with their own marker. Validation, one
problem per key:
- More than `realtimelimits.MaxSessionReminders` (8): "at most 8
reminders per session".
- Name: "reminder name must match ^[a-z][a-z0-9-]{0,62}$".
- `in`: missing or empty: "name the states the reminder runs in";
undeclared: the unknown-state message; final: "a final state ends the session; a
reminder there would post until the channel closes"; listed twice: "state x is
listed twice".
- `every`: missing: "name how often the reminder posts, such as 30m";
bad syntax: the period message; calendar: "every must be a fixed duration such
as 30m or 24h; months and years are not supported for reminders"; under
`realtimelimits.MinSessionTimer` (1m): "a reminder waits at least 1m".
- `first`: the same syntax, calendar and minimum rules as `every`.
- `max`: "max must be between 1 and 1000".
- `post` missing: "name the message type the reminder posts under
post.type". `post.type`: missing; not a message type of the channel type;
ephemeral ("a reminder posts a log message; an ephemeral message would reach
only the readers connected to the leading process"); keys an `on:` entry of some
state: "a reminder never moves the session, but a person posting x would; use a
message type no transition is keyed by" (this also keeps a store-written message
out of the deciders). The reserved `session` type is never a declared type, so
it needs no rule.
- `actions` and `promote_to` on the reminder type: allowed, no rule.
Keeping them is trivial: `messageActions` offers a type's declared actions on
any message, and trigger posts by `system:realtime` already get them. The
reminder protection only refuses edit and author delete (decision 7), so an
operator's "acknowledge" action works on reminders.
- `post.props`: `triggerProps` and `triggerMessage`, renamed
`literalProps` and `literalMessage` and shared with triggers, with the reason
texts changed from "a trigger" to "a trigger or reminder".
- `keys.go` refusals: `channel_types.*.session.timers` ("reminders are
declared under session.reminders; a timer that moves the session goes under
after: on its state"); `channel_types.*.session.states.*.reminders` ("reminders
are declared once, under session.reminders, with in:");
`channel_types.*.session.reminders.*.to` ("a reminder posts a message and does
not move the session; for a timer that moves it, use after: on the state");
`...reminders.*.period` ("use every"); `...reminders.*.since` ("a reminder
counts from when the session entered one of its in: states").
- The slice 1 and 3 rule for non-final states is unchanged: a reminder
does not count as a way out of a state.
- Reminders are compiled before the transitions, because the guard
environment needs their names. Every reminder whose name matches the grammar is
declared in the guard environment, whatever its other problems, so a guard
naming it reports no second "unknown attribute" error. `CheckReload`: unchanged.

2. **Counter in guards.** `sessionGuardEnv(props, reminders []string,
entity)` declares `session.reminders` as a `RecordType` with one `IntType` field
per reminder name. Guards read it as `session.reminders['status-update']`; an
unknown name fails to compile at load. `guardHolds` binds it from the channel
row under the store lock: for each declared reminder, the record's `count` when
the reminder is active in the current state, else 0. It reads
`ch.Session.Reminders`, not `current`, because `currentSession` replaces an
unset session's struct. Scope: per reminder and per span. It resets when the
session leaves the reminder's `in:` states and is kept across moves inside them,
re-entries, close and reopen, restore, and reload. A moderator's removal of a
reminder message does not lower it: the counter is session state, not a log
count.

Compatibility with slice 7: `count()` will be a function over messages in the
log; `session.reminders` is a field of the session record. They do not share a
name, and `session.reminders` means "reminders in this span", which a log count
cannot answer once the session left and re-entered the span. Slice 7 may add
`count('reminder')` without changing this field. Only transition guards see the
counter; a message type's own `when:` does not.

3. **Store model** (`internal/channels/session.go`):

   ```go
   // ReminderRecord is one reminder's state in a session span.
   type ReminderRecord struct {
       Since time.Time `json:"since"`          // span start, or the resume time after a restart
       Last  time.Time `json:"last,omitzero"`  // time of the last post in this span; zero for none
       Count int64     `json:"count"`          // reminders posted in this span
   }
   type Session struct {
       ... // slice 1 and 3 fields
       Reminders map[string]ReminderRecord `json:"reminders,omitempty"`
   }
   type SessionSchedule struct {
       NextDueAt time.Time
       Restarts  map[string]time.Time
       // Reminders, when non-nil, replaces the stored records. Nil keeps them.
       Reminders map[string]ReminderRecord
   }
   // ReminderPost is what a SetSession step's Remind returns: the message
   // to append and the reminder's record after the post.
   type ReminderPost struct {
       Name        string         // reminder name
       Record      ReminderRecord // stored under Name before Due runs
       Type        string
       Props       map[string]any
       TriggeredBy string         // must start with ReminderMarker
   }
   const ReminderMarker = "reminder:"
   type SessionStep struct {
       ... // slice 1 and 3 fields
       // Remind is called by SetSession under the lock when Decide returns
       // no move. Pure and fast, like Decide. Append refuses a step that sets it.
       Remind func(cur Channel) (ReminderPost, bool)
   }
   type SessionOutcome struct { ...; Reminded bool }
   ```

Helpers, shared by both backends:
- `NormReminder(rec)` applies `NormTime` to `Since` and `Last`, as
`ApplySchedule` does for restart anchors. Every stored record passes through it,
so the CAS compares equal values on every backend.
- `ApplySession` also clears `Reminders`; the step's `Due` decides which
records carry over, as for the restart anchors.
- `ApplyReminder(c *Channel, post ReminderPost, at)`: checks the name
(`ValidStateName`), a non-negative count and `Last` not after `at`
(`ErrInvalidMessage` otherwise), and stores the normalized record under the
name. It runs before `Due`, so `Due` is a pure function of the committed channel
and needs no flag shared with `Remind`.
- `ApplySchedule` stores a non-nil `Reminders` after normalizing each
record and checking at most `MaxSessionReminders` (8) records, valid names and
non-negative counts (`ErrInvalidMessage` otherwise). The cap applies to the
final map, after `Due`. An empty map stores none.
- `ReminderMessage(step, post, seq, at, rev)` builds the message: the
post's type, props (through `PrepareProps`, so size caps apply) and
`TriggeredBy`, the step's `NoticeBy` as author.
- `PrepareMessage` refuses a `TriggeredBy` that starts with
`ReminderMarker`, as it does for `SessionMarker`, so only the store's reminder
step writes it. `PrepareAppendStep` refuses a step with `Remind`.
`PrepareReminderPost` checks the marker, `ValidName(Type)` and the `TriggeredBy`
size.
- `SetSession`, in both backends: `DecideTimer`; on no move and with
`Remind` set, call it; on a post, `ApplyReminder`, append the reminder message
at `last_seq + 1` (raising rev), then `ApplySchedule(step.Due)`, the one channel
update, and no audit record. A step does one of move, remind or reschedule.
`FillSessionOutcome` sets `Reminded`. The store applies no policy: the slot, the
count and the CAS live in the callbacks.

Plimsoll: no new `Store` method, so the pinned method counts stay. `SessionStep`
grows to 10 exported fields and `Session` to 6, under the field cap.

4. **Backends and migrations.**
   - `sqlchannels`: `chanCols` and `scanChannel` read `session_reminders`,
JSON text with times as Unix microseconds (empty for none), encoded and decoded
like `encodeRestarts`. `bumpSession`, `insertChannel`, `SetLifecycle` and
`Untombstone` write it. `SetSession` inserts the reminder message row as it
inserts a notice.
   - `sqlitedb`: `channelsDDL` gains `session_reminders TEXT NOT NULL
DEFAULT ''`; `schemaVersion` 26; rung `to: 26`, `addChannelReminders`, over
`addChannelColumns` with a `channelReminderColumns` list. No index, so nothing
goes after the ladder.
   - `internal/store/pgstore/migrations/0033_channel_session_reminders.sql`:
`ALTER TABLE channels ADD COLUMN IF NOT EXISTS session_reminders TEXT NOT NULL
DEFAULT ''`, header in the style of 0032. A new file even while PR #1967 is
open: 4/3 stays reviewable as it is, and the ladder rule is the same. If another
branch takes v26 or 0033 first, renumber at rebase.
   - `filechannels`: `meta.SessionReminders map[string]reminderMeta`
(`yaml:"session_reminders,omitempty"`), with times written as normalized UTC
values, mapped by `setSession` and `channel`; `schedule` applies it.
`SetSession` appends the reminder line in the same `appendRecords` call position
as a notice. The crash window is the one the package doc describes: `meta.yaml`
can record a raised count and `last` with no message; a reminder can be lost,
never doubled. The package doc gains that sentence.

5. **Slots and schedules** (new `internal/realtime/reminders.go`, pure):

   ```go
   // reminderRecord is r's record for ch: the stored one, or for a reminder
   // active in the current state without one, {Since: cur.EnteredAt}.
   func reminderRecord(r realtimecfg.SessionReminder, cur current, ch channels.Channel) (channels.ReminderRecord, bool)
   // nextSlot is r's next slot: since + first when last is zero, else the
   // first slot since + first + k*every strictly after last. ok is false
   // when r is not active, max is reached, or the state is final or stranded.
   func nextSlot(r realtimecfg.SessionReminder, cur current, ch channels.Channel) (time.Time, bool)
   // nextReminder is the active reminder whose next slot comes first (a tie
   // goes to the first name).
   func nextReminder(s *realtimecfg.Session, ch channels.Channel) (realtimecfg.SessionReminder, time.Time, bool)
   // nextDue is the value of next_due_at: the earlier of nextTimer's due
   // time and nextReminder's slot plus ReminderSlack, normalized.
   func nextDue(s *realtimecfg.Session, ch channels.Channel) time.Time
   ```

   - The slot grid is derived from config at every read; only `since`,
`last` and `count` are stored. A reload that lengthens `every` or changes
`first` gives at most one `every` between the last post and the next one; a
shorter `every` takes effect at the next grid point after the last post. Missed
slots are skipped, never replayed, and the phase stays on the grid. The next
slot is computed in constant time: `k = floor((last - since - first) / every) +
1`, at least 0.
   - An absent record reads as a span that started when the session entered
its state (the channel's `created_at` for an unset session). By induction this
is the span start, because every move inside the span stores the record
explicitly (below). The one exception is a reminder added by a reload while a
session already sat in an `in:` state: it counts from that state's entry, so it
posts once in the next round and then follows the grid. This is the slice 3
reload rule.
   - A post at `now` returns `Record{Since, Last: now, Count + 1}`. With
`Count == Max` the reminder has no next slot until the span ends.
   - Pruning: every schedule that returns a non-nil `Reminders` map keeps
only records of reminders the current config declares and drops the rest.
Inactive declared records (a reload took the state out of `in:`) are kept until
the next move. So a reload that renames all 8 reminders never pushes the map
over the cap.
   - Moves (`stepSchedule`, which already captures the left channel through
`withSessionDue`): the new `carriedReminders(s, from, to)` returns, for every
reminder active in both the left state and `to`, its record from `from` (an
implicit record made explicit). Reminders not active in `to` are dropped, so
their counter is 0. A reminder active only in `to` starts a new span with no
record. A re-entry is a move inside the span. `stepSchedule` returns `Reminders`
= the carried map, non-nil, so the schedule replaces what `ApplySession`
cleared. Without a move it returns nil and the records are kept.
   - The reminder step's `Due` returns the committed map (which already holds
the posted record), pruned, and `NextDueAt = nextDue`.
   - Every schedule (`sessionDue`, `stepSchedule`, `resumeSession`, the
reminder step's `Due`) sets `NextDueAt = nextDue(...)`. Slice 3's `nextTimer`
stays for the timer fire CAS and the wire. There is one function for the hint;
`fold`, `noteDue`, `Resumption.Note` and the recompute read the hint unchanged.
   - Resume (`resumeSession`, used by `ResumeSchedule`,
`transitionSchedule` and `Resumption`): besides the slice 3 restart anchors,
every active reminder whose next slot is at or before the resume time restarts:
`{Since: at, Last: zero, Count: kept}`, so the next reminder comes `first` after
the reopen or restore. A reminder whose slot was not yet due keeps its record.
It returns the full, pruned record map.
   - Close and archive: no write. A closed channel keeps its `next_due_at`,
and the due query skips it, as in slice 3.
   - Stranded states activate no reminder.

6. **Timer loop.** `realtime.Timers` gains reminders without a new
component:
   - `fireDue` and `recompute` list the types whose session
`HasSchedule()`. The recompute compares the stored hint with `nextDue`.
   - Per listed channel: when `nextTimer` is due at or before now, fire the
timer (slice 3; a move wins over a reminder due in the same round). Otherwise,
when `nextReminder`'s slot is at or before now, run a reminder step for it. A
channel's hint may still be in the future (slot plus slack); the fire condition
reads the slot, so one round posts every reminder whose slot has passed.
Otherwise reschedule when the hint differs. One step per channel per round: a
second due reminder of the same channel posts in the next round, which `More`
brings at the floor.
   - Cap: timer fires, reminder posts and reschedules share the one
`MaxTimerWritesPerRound` (64). There is no reminder sub-cap: a sub-cap cannot
protect a deadline that is not in the listing, and it would slow the drain.
Documented bound: when B due reminders sort ahead of a due timer in the listing,
the timer fires at most ceil(B/64) floor rounds (each `TimerFloor`, 1s) late.
With 1000 overdue reminders that is 16 seconds. AC 12 `ReminderBacklogBound`
pins it.
   - A reminder step is `SetSession(key, now, step)` with a decider that
never moves and `Remind`, a CAS: it posts only when, under the lock, the channel
is open, the reminder is active in the committed state, its committed record has
the listed `Since` and `Last` (compared with `time.Equal` on normalized values),
`Count < Max`, and its next slot is at or before `now`. It returns the post with
`Record{Since, Last: now, Count + 1}`. Anything else posts nothing and the step
degrades to a reschedule. The post: `Type` the reminder's `PostType`, `Props`
the literal props after `validateProps` and `whenHolds` (as `systemWriter.post`
does), `TriggeredBy` `reminder:<type>/<name>`. `NoticeBy` is `systemAuthor()`.
   - A post that fails `validateProps` or `whenHolds` before the store call
(for example after a metamodel change removed an enum value) is logged, counted
in `Failed`, and the channel goes into backoff through `holdBack`, the path of a
failed write. No `More` is set for it, so the loop does not spin at the floor.
   - Semantics: a slot posts at most once (the CAS on `Since` and `Last`).
The attempt is at least once; a lost commit or two overlapping leaders post
nothing the second time. The message time is the round's `now`. A post that
moves the session out of the span before the write wins, and the reminder step
posts nothing.
   - `TimerReport` gains `Reminded int`.
   - Precision: a reminder posts within 30 seconds after its slot, later
only while a backlog drains. `first` and `every` are at least 1m and the leader
lists hints up to `now + TimerCeiling`, so a slot set by another process is seen
by the leader's next round and scheduled at slot plus slack. Interrupting timers
keep their slice 3 precision.
   - Principal and side effects: `lead()` puts `system:realtime` on ctx; no
ACL is evaluated; no rate bucket is charged (as for trigger posts). A reminder
sends no mail and starts no job: it is a channel store write with no external
side effect (`jobs.md`), and live readers learn of it through `Marker.Mark` and
SSE.

7. **Reminder message format and protection.** A reminder message is a log
message of the declared type with the literal props, author `system:realtime`
(`author.system` true on the wire) and `triggered_by: reminder:<type>/<name>`.
Its props carry no count: the type's properties are the operator's, and the
count is in the session block. `isReminder(m)` (server only) is
`strings.HasPrefix(m.TriggeredBy, channels.ReminderMarker)`; only the store can
write the marker, so a person's post of the same type is not a reminder.
   - `Edit` and author `Delete` of a reminder answer 409 `reminder_message`
("a reminder is posted by the session and cannot be edited or deleted; a
moderator may still remove it"), new `TypeReminderMessage` in `errors.go`. The
author check already refuses both, because no caller is `system:realtime`; the
explicit 409 gives the reason. `messageActions` offers no `edit` or `delete` on
a reminder.
   - Moderator `Remove` is allowed with its reason and its removal audit
record, as for a message that moved the session (`sessionMessage`). A
misconfigured `every: 1m` can then be cleaned up. The counter does not drop; the
docs say so.
   - The message type's declared actions and `promote_to` are offered as on
any message (decision 1).
   - Retention and channel delete erase reminders with the channel.
   - No audit record for the post: the channel audit trail records state
changes (lifecycle, session moves, removals), not posts, and trigger posts are
not audited either; the message is the record. The comments rule (comments stay
out of the graph, no audit or versioning) does not apply: reminders are channel
messages, not comments, and nothing enters the graph.

8. **Wire and rendering.** `SessionView.Reminders []SessionReminderView
json:"reminders,omitempty"`, with `SessionReminderView{Name, Label, Every
string; Count int64; Max int64 (omitempty); DueAt *time.Time}`. They are derived
from config and the row, never from the hint:
   - listed for every reminder active in the current state (config plus the
count, which every channel reader can see in the log anyway);
   - `DueAt` is the next slot, not slot plus slack, and is set only on a
stored, open channel under a config that is not degraded, and not once `Count ==
Max`;
   - none in a final, stranded or not-yet-stored state, or outside `in:`.
`SessionView.DueAt` keeps its slice 3 meaning, the next interrupting move;
reminders carry their own `due_at`, so a reader can tell "moves at" from
"reminds at". Rest of the surface:
   - `apiwire/v1`: `ChannelSession.Reminders []ChannelSessionReminder`;
`dataentry/channels_wire.go` maps it.
   - `frontend/src/types/channels.ts`: the mirror type and
`CHANNEL_WIRE_FIELDS` only. No helper and no component change.
   - `internal/openapi/channels.go`: a `ChannelSessionReminder` schema, the
property, and a sentence on reminder messages (`triggered_by` marker, system
author) in the message description.
   - MCP: through `ChannelView` and `messageResult.Session`; the
`read_channel` description mentions reminders. A reminder raises rev, so
`read_channel` with `watch` wakes on it.
   - CLI: `sessionText` appends "reminder <name>: <count>[ of <max>], next at
<time>"; a reminder message renders with a "reminder" note.

Wire shape:

```json
"reminders": [ { "name": "status-update", "label": "Status update", "every": "30m",
                 "count": 2, "max": 48, "due_at": "2026-10-24T10:40:00Z" } ]
```

A reminder message:

```json
{ "seq": 12, "type": "reminder", "author": { "user": "system:realtime", "system": true },
  "triggered_by": "reminder:breach-response/status-update",
  "props": { "body": "Post a status update: what changed since the last one?" } }
```

9. **Limits** (`internal/realtimelimits`, server-owned): `MaxSessionReminders
= 8` (per session), `MaxReminderCount = 1000` (upper bound of `max`),
`ReminderSlack = 30 * time.Second`. `every` and `first` reuse `MinSessionTimer`
(1m). Reminders share `MaxTimerWritesPerRound` (64).
`channels.MaxSessionReminders = 8` is the store's own cap, as
`MaxSessionRestarts` is.

Cost: a one-minute reminder posts 1440 messages a day per channel while the
session stays in its `in:` states. The docs say so and recommend `max`. Writes:
one per channel per round, at most 64 writes per round, rounds at least 1s
apart, so at most 3840 writes a minute from one leader; beyond that reminders
run late, never doubled, and a deadline behind a backlog is late by the bound in
decision 6. Reads: one `ListSessionsDue` per round, and with the slack at most
two reminder-only rounds a minute plus drain rounds (AC 15). The file tier walks
the tree once per round, as in slice 3.

**Alternatives rejected:**
- A reserved system type `reminder` written like a session notice: the SPA
treats an undeclared message type as unknown and reloads the schema for it, so
every reminder would need UI work, and the operator could not choose the text,
label or properties. A declared type renders today.
- The config key `session.timers` (the research sketch): `timers` already
means the per-state `after:` entries on the wire and in Go, so one word would
name two things.
- Reminders per state (`after:`-like): the research chose `in:` so one
reminder spans several states; per-state reminders cannot keep a phase across a
move.
- Re-anchoring on every move inside `in:` (like `since: state`): the 30-minute
status rhythm would restart whenever the state changes, which is not what "every
30 minutes while the breach is open" means.
- Storing a slot index (`next`) instead of `last`: a reload that lengthens
`every` or changes `first` moves the indexed slot far away (every 1m to 30m at
index 100 is 50 hours). `last` keeps at most one `every` of delay.
- Storing only a count and deriving the slot as `since + first +
count * every`: after downtime the loop would post one reminder per missed slot
in quick succession.
- Calendar periods for `every`: slot arithmetic is not constant-time and no
use case needs monthly reminders inside a session.
- A reminder that may also move the session after N posts: that is an
`after:` timer, or a guard on the counter.
- A count prop on the reminder message: it would need a property the
operator did not declare. The session block carries the count.
- Counting reminders with slice 7's `count()` instead of a stored counter:
a log count cannot tell spans apart, costs a log read per decision, and slice 7
is not built yet.
- A new `Store.PostReminder` method: it would duplicate `SetSession`'s lock,
CAS and schedule handling and raise the pinned method counts. `Remind` on the
step reuses them.
- `Due` learning of the post through a flag shared with `Remind`: a backend
may call the callbacks more than once, and a flag from an earlier attempt goes
stale. `Remind` returns the record and the store applies it first.
- A reminder sub-cap inside the round cap: it cannot protect a deadline
outside the listing and slows the drain. A second, timers-only `ListSessionsDue`
per round would protect it fully, at one more query per round; the documented
bound is accepted instead.
- A second derived time for the wake (slot plus slack) next to the hint:
storing slot plus slack as the hint gives one due function.
- Refusing `actions` and `promote_to` on a reminder type: they already work
on system posts, and refusing them would take a feature from operators.
- Refusing moderator removal: nobody could clean up a misconfigured
reminder, and a message that moved the session may already be removed.
- An audit record per reminder: the trail would grow by up to 1440 records a
day per channel with nothing in them a reader cannot see in the log.
- Firing reminders exactly at the slot (no slack): on the file tier dense
slots cause one tree walk per second.
- Folding reminder records into `session_restarts` to avoid a migration: two
meanings in one column, and slice 3's PR would have to change.

**Files to modify:**
- `internal/realtimecfg`: `raw.go`, `keys.go` (five refusals), new
`reminders.go` (types, validation), `session.go` (compile order,
`sessionGuardEnv`, `HasSchedule`), `validate.go` (`literalProps`,
`literalMessage` shared with triggers), `testdata/valid.yaml` (+ `load_test.go`,
`compiled_test.go`, `reload_test.go`).
- `internal/realtimelimits/limits.go`.
- `internal/channels`: `session.go` (`ReminderRecord`, `Session.Reminders`,
`SessionSchedule.Reminders`, `ReminderPost`, `ReminderMarker`, `Remind`,
`Reminded`, `NormReminder`, `ApplyReminder`, `ApplySession`, `ApplySchedule`,
`ReminderMessage`, `PrepareReminderPost`, `PrepareAppendStep`), `channels.go`
(`PrepareMessage` marker), `store.go` (`SetSession` contract),
`channelstest/channelstest.go`, `channelstest/timers.go` and new
`channelstest/reminders.go`.
- `internal/channels/sqlchannels/sql.go`, `retention.go`;
`internal/channels/filechannels/file.go`, `ops.go`, `retention.go` (+
`file_test.go`); `internal/channels/pgchannels/pg_test.go`.
- `internal/sqlitedb`: `sqlitedb.go`, `migrate.go` (+ test with a v25
fixture).
- `internal/store/pgstore/migrations/0033_channel_session_reminders.sql`.
- `internal/realtime`: new `reminders.go` (+ `reminders_test.go`),
`timers.go`, `session.go` (guard binding, views, `isReminder`), `ops.go` and
`msgops.go` (protection), `errors.go`, `render.go`, `service.go` (+
`timers_test.go`, `session_test.go`).
- `internal/apiwire/v1/channels.go`; `internal/dataentry/channels_wire.go`
(+ golden files, handler and budget tests).
- `internal/openapi/channels.go` (+ test).
- `frontend/src/types/channels.ts` (mirror type and wire fields;
  + `wireFields.test.ts`).
- `internal/mcp/tools_channels.go` (+ test).
- `internal/cli/channel_render.go` (+ test).
- `docs-project/entities/guides/GUIDE-realtime.md`, `GUIDE-mcp-server.md`;
`docs/realtime.md` and `docs/mcp-server.md` by `just docs`.

## Security Considerations

- [x] Input sources identified (user input, config, external APIs)
- [x] Input validation approach defined (allowlist preferred over blocklist)
- [x] Security-sensitive operations identified (file access, auth, crypto)
- [x] Error handling doesn't leak sensitive information

**Input Sources & Validation:**
- `realtime.yaml` (operator config, not secret): reminder names follow the
name grammar; `in` names declared, non-final states; `every` and `first` are
fixed durations of at least 1m; `max` is bounded; at most 8 reminders per
session; `post` names a declared, persistent type no transition is keyed by,
with literal props checked as trigger props are (allowlisted property types, no
substitution, size caps, the type's `when:`).
- No request carries a reminder record, a slot or a count. The records are
written only by store steps whose callbacks the service builds from config;
`SetSession` has no HTTP, MCP or CLI path.
- The store refuses the `reminder:` marker on every `Append`, refuses
`Remind` on an `Append` step, and normalizes, caps and validates the records it
stores.

**Security-Sensitive Operations:**
- Who posts a reminder: only config, as `system:realtime`, with no ACL
evaluation and no graph write, like trigger posts and slice 3 timer fires. No
principal's authority is borrowed.
- Forgery: a person may post the reminder's message type if `post:` allows
it, but cannot set `triggered_by`, so the message shows its real author, is not
a reminder, and does not raise the counter. The docs recommend a message type,
and `post:` roles, meant for reminders.
- Actions on reminders: declared actions and `promote_to` run as the person
who invokes them, under their own gates, as on trigger posts. The reminder
author grants nothing.
- Removal: only moderators, with a reason and an audit record; the counter
is unaffected, so a removal cannot change which guards hold.
- Guards: the counter is read from the committed row under the store lock,
so a racing reminder cannot be counted twice or missed. It counts messages every
channel reader already saw, so a guard on it reveals nothing new.
- Disclosure: reminder props are operator text, never entity values; `count`,
`every` and `due_at` derive from config and row times readers already see.
- Denial of service: the 1m minimum, the 8-per-session cap, `max`, the
64-write round cap, the 1s round floor, the 30s slack, the backoff for failing
rows and failing posts, and leader-only execution bound the write and scan rate
(AC 12, AC 15).
- The decider, `Remind` and the schedule run under the store lock and do no
I/O.
- Errors: config problems name keys and values. Round errors are logged with
the channel key only. `reminder_message` names no data.

## Test Plan

- [x] Test scenarios documented for each acceptance criterion
- [x] Edge cases identified and documented
- [x] Negative test cases defined (invalid input, error conditions)
- [x] Integration test approach defined (not just unit tests)

**Test Scenarios:** one test or table row per acceptance criterion, named there.
Layers:
- realtimecfg unit tests (AC 1 to 6);
- store conformance on four backends (AC 7), postgres gated on
`RELA_TEST_DATABASE_URL` and run with `just test-postgres`;
- opening a real v25 sqlite file and the file tier (AC 8, 9);
- pure slot, carry, resume and pruning computation (AC 10, 11);
- timer rounds over the file store on MemFS with the test clock and swapped
config snapshots (AC 12, 15);
- service, guard and protection tests (AC 13, 14, 16);
- HTTP golden, frontend mirror, OpenAPI, list budget, MCP and CLI (AC 17 to
21);
- docs (AC 22).

Race detector on; `testSetSessionRemindCAS`, `RemindLeaderOverlap` and the guard
race in AC 13 exercise the concurrency.

**Edge Cases (each has a test):**
- A move inside `in:` keeps the phase and the counter; a re-entry too (AC
11, AC 12 `RemindAcrossStates`).
- Leaving `in:` resets the counter; re-entering starts at 0 (AC 11, AC 12).
- `max` reached (AC 10, AC 12, AC 17).
- Missed slots after downtime (AC 10 `TestNextSlot`, AC 12
`RemindSkipsMissed`).
- A reload that lengthens or shortens `every` or changes `first` (AC 10, AC
12 `RemindReload`).
- Two reminders of one channel due in one round (AC 12 `RemindTwoDue`).
- 8 stored records whose reminders a reload renamed (AC 11
`TestSchedulesPruneUndeclared`).
- A stored record whose reminder a reload took out of `in:` reads 0 in a
guard (AC 13).
- Closed, reopened before and after the slot, two cycles; restore from a
tombstone (AC 11, AC 12).
- A timer and a reminder due in the same round (AC 12).
- A reminder backlog ahead of a deadline (AC 12 `ReminderBacklogBound`).
- A post that leaves `in:` racing the reminder write (AC 12
`RemindMessageRace`).
- Two leaders (AC 7 CAS, AC 12 `RemindLeaderOverlap`).
- A failing post (AC 12 `RemindPostFails`).
- Times with nanoseconds and a non-UTC location (AC 7
`testSessionRemindersStored`).
- Degraded config, in the round and on the wire (AC 12, AC 17).
- A person's post of the reminder type; a moderator removal (AC 14).
- An unset session with a stored record (AC 10, initial state in `in:`).
- A legacy `meta.yaml`, a legacy postgres row and a real v25 sqlite file (AC
8, AC 9).
- Not covered by a test: a crash inside the filechannels reminder window
(documented in the package doc).

**Negative Tests:** AC 2 and AC 3 (every load rule and refused key), AC 4
(unknown reminder in a guard, no duplicate error for a broken one), AC 7 (marker
on `Append`, `Remind` on an `Append` step, bad post type or name, bad record,
oversized props, too many records), AC 12 (no reminder before its slot, while
closed, outside `in:`, past `max`, under a degraded config, after a racing move,
after a failing post), AC 14 (edit and author delete of a reminder).

## Risk Assessment

- [x] Technical risks assessed with mitigations
- [x] Security risks assessed (see Security Considerations)
- [x] Effort estimated (xs/s/m/l/xl)

**Risks:**
- Log growth from frequent reminders. Mitigation: 1m minimum, `max`, the
documented cost, moderator removal, and retention.
- A reminder backlog delaying a deadline. Mitigation: the shared cap drains
64 writes per round, and the delay is bounded and documented (ceil(B/64) floor
rounds, AC 12 `ReminderBacklogBound`). A timers-only listing is the upgrade path
if the bound proves too loose.
- More file-tier walks with dense reminder slots. Mitigation: the slack in
the hint coalesces them (AC 15).
- The implicit record rule ("absent means the span began at state entry")
breaks if some move inside `in:` forgets to store the record. Mitigation: every
session step goes through `withSessionDue` and `stepSchedule`; AC 11 covers
message moves, timer moves and re-entries.
- CAS values that differ by precision across backends would stop every
reminder. Mitigation: `NormReminder` on every write, microsecond encoding,
`time.Equal`, and the round-trip assertion on four backends (AC 7).
- 4/3 is still in review. Mitigation: this branch builds on it; the
migration is a separate file (v26, 0033), and a rebase after review changes
regenerates the golden files.
- Migration numbering collides with another branch. Mitigation: renumber at
rebase.
- Renaming `triggerProps`/`triggerMessage` touches trigger validation.
Mitigation: the trigger rows of `TestParse_Invalid` keep passing with the
updated reason text.
- Merge conflicts in `frontend/src/types/channels.ts` and the wire golden
files. Mitigation: append-only edits; regenerate after rebasing.

**Effort:** l

## Documentation Planning

- [x] User-facing docs identified (skip if internal refactor)
- [x] Docs-checklist will be created when entering implementation

**Documentation Impact:** Enhancement. Edits go into
`docs-project/entities/guides/GUIDE-realtime.md`; `just docs` regenerates
`docs/realtime.md`.
- "Sessions": a "Reminders" part after "Timers" with the example, the
`reminders:` keys table (`in`, `every`, `first`, `max`, `label`, `post`), the
span rule (kept across moves inside `in:` and re-entries, reset when the session
leaves them), slots and skipped slots after downtime, what a change of `every`
or `first` does (at most one `every` after the last post), what happens while
closed or tombstoned (frozen; a due reminder restarts `first` after the reopen
or restore; the counter is kept), `max`, the counter in guards
(`session.reminders['name']`, bracket form for hyphenated names, 0 outside the
span, not lowered by a removal), the reminder message (declared type with its
actions, system author, `triggered_by: reminder:<type>/<name>`, cannot be edited
or deleted, moderators may remove it, a person's post of the type is not a
reminder), no audit record for the post and no mail, precision (within 30
seconds, later only while a backlog drains), the deadline delay bound behind a
backlog, and the cost of short intervals.
- "Checks when the file loads": the reminder rules and refused keys.
- "Limits": the reminder limits.
- "HTTP API" and "Agents and other processes": `reminders` in the session
block, and that `due_at` is absent while closed, under a degraded config and
after `max`.
- "Reloading": changed `every` and `first`, removed reminders and changed
`in:`.

`GUIDE-mcp-server.md`: the `read_channel` row mentions reminders. Code: godoc on
`ReminderRecord`, `ReminderPost`, `SessionStep.Remind`, `ReminderMarker`, the
`SetSession` contract, `nextSlot`, `nextDue`, the round-cap bound in `Timers`,
and the filechannels package doc sentence.

## Design Review

- [x] Run `/design-review` before starting implementation
- [x] All critical/significant findings addressed in plan

**Design Review Findings:** the 15 findings of `/design-review`
(`.ignored/dr-36z6x3.tsv`) are recorded as review-response entities linked to
the ticket via `has-review-response`. All are addressed in this plan:
- the record stores `last` instead of a slot index, so a reload that
lengthens `every` or changes `first` delays at most one `every`;
- the config key is `session.reminders`, so "timers" keeps meaning `after:`;
- the reminder sub-cap is gone; all writes share the 64-write cap, and the
deadline delay behind a backlog is a documented, tested bound;
- `Remind` returns the record, which the store applies before `Due`;
- schedules drop records of undeclared reminders, so the 8-record cap cannot
block a channel;
- moderators may remove reminders (audited); edit and author delete are
refused;
- a failing post goes into backoff;
- the hint holds slot plus slack, so there is one due function;
- reminder types keep their declared actions and `promote_to`;
- record times are normalized and encoded like restart anchors;
- the missing tests are added;
- the frontend `isReminder` helper is dropped;
- the example type is `escalate-stalled`;
- the guard environment declares every grammatical reminder name;
- the precision claim is "within 30 seconds, later only while a backlog
drains".

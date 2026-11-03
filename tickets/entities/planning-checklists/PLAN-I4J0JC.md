---
id: PLAN-I4J0JC
type: planning-checklist
title: 'Planning: Sessions 4/1: session config and message-driven transitions'
started: "2026-10-21"
completed: "2026-10-21"
status: done
---

<!-- @managed: claude-workflow v1 -->

## Understanding

- [x] Problem/requirements clearly understood
- [x] Scope defined (what's in/out documented below)
- [x] Acceptance criteria documented with specific test scenarios

**Problem:** A channel has a lifecycle (open, closed, archived) but no notion of
where the conversation is. The sessions research
(`.ignored/res-sessions-final.md`) decided that a session is a flat state
machine declared on a channel type. Typed messages are the events. The state is
a snapshot on the channel row with a generation counter. Every transition is
visible as a system message and recorded in the channel audit trail (options a2,
b1, d1). This is slice 1 of 7: the config, the stored state, message-driven
transitions and the state in the channel reads. Today `realtime.yaml` refuses
`session` with "sessions are not supported yet"
(`internal/realtimecfg/keys.go`).

Terms used below:
- **Session state**: the name of the state a channel's session is in.
- **Generation**: a counter on the channel row that rises by one on every
transition. Generation 0 means the session never moved.
- **Unset session**: a channel with generation 0 and no stored state. It reads
as the configured `initial` state.
- **Notice**: the system message a transition appends to the channel log.
- **Decider**: a pure function the service hands to the store's append. The
store calls it under its lock with the current channel, and it returns the move
(from, to) or no move.
- **Stranded**: a stored session state that the current config no longer
declares.

**Scope:**
- In: `channel_types.<name>.session` in `realtime.yaml`: `label`, `initial`
and `states` (in declaration order). Each state has `label`, `final: true`
(optional) and `on:` transitions keyed by message type with `to`, `roles`, `by:
trigger` and `when`.
- In: load-time validation in `internal/realtimecfg`, one problem per key.
- In: session fields (state, generation, entered_at) on the channel row in
every channel store backend: `filechannels` (disk, and MemFS for the memory
tier), `sqlitechannels` and `pgchannels` over `sqlchannels`. Schema steps for
sqlite (v24 rung) and postgres (migration 0031). Conformance tests in
`channelstest.RunAll`.
- In: a posted log message moves the session when, at the moment it is
appended, the channel is open, the current state has an `on:` entry for its
type, the poster passes that entry's `roles` or `by: trigger` rule, and its
`when:` holds. The message, the state change, the notice and the audit record
are one store step, decided under the store lock.
- In: trigger posts (`system:realtime`) move sessions through transitions with
`by: trigger` or with neither `by` nor `roles` (decision d1).
- In: the `session` block on the channel in the HTTP head, page and list
answers, the MCP `read_channel`, `list_channels` and `post_message` results, the
CLI `rela channel` output, the OpenAPI `Channel` schema, and the frontend
wire-field mirror (`frontend/src/types/channels.ts`, types only).
- In: docs: `docs-project/entities/guides/GUIDE-realtime.md` (renders
`docs/realtime.md` via `just docs`) and the `read_channel` and `post_message`
rows in `GUIDE-mcp-server.md`.
- Out (later slices): UI and the `session` description in the schema
endpoint (slice 2); `after:` timers, `next_due_at` (no column now), reload
recompute and the stranded reload log (slice 3); reminder timers (slice 4);
`outputs:`, `run_as`, `on_final: close` (slice 5); `status:`, `waiting_for:`,
handoff (slice 6); `count()` guards and the menu filter (slice 7).
- Out: `entity` in transition guards (user decision; a guard naming `entity`
is refused with an actionable message). Out: more than one transition per
(state, message type).

**Example (the shape this slice accepts; also the docs example and the new
`breach-response` type in `testdata/valid.yaml`):**

```yaml
channel_types:
  breach-response:
    label: Breach response
    attach_to: [datalek]
    roles:
      lead:   { from: entity.owner }
      member: { from: attached }
      dpo:    { permission: privacy.officer }
    message_types:
      text:      { properties: { body: { type: markdown, required: true } }, post: [member] }
      contained: { properties: { note: { type: markdown } }, post: [lead, member] }
      assessed:  { properties: { risk: { type: risk-level, required: true } }, post: [lead, dpo] }
      notified:  { properties: { reference: { type: string, required: true } }, post: [dpo] }
      dismiss:   { properties: { reason: { type: string, required: true } }, post: [lead] }
      escalated: { properties: { note: { type: string } }, post: [lead] }
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
        contained:
          label: Contained
          on:
            assessed: { to: assessed, when: "message.risk != 'none'" }
        assessed:
          label: Risk assessed
          on:
            notified: { to: notified }
        notified:  { label: AP notified, final: true }
        dismissed: { label: Dismissed, final: true }
```

`testdata/schema.yaml` gains `types.risk-level: {values: [none, low, high]}` and
`testdata/acl.yaml` a role granting `privacy.officer` if none does.

**Acceptance Criteria:**
1. The example compiles: `ChannelType.Session()` returns a read-only session
whose states are in declaration order, with `initial`, `final` per state, and
per state one transition per message type with `To`, `Roles`, `By` and a
compiled `When`. Test `TestParse_Session` in
`internal/realtimecfg/load_test.go`; `TestParse_Golden` updated for the new
channel type name list.
2. Each invalid session reports its key path and reason, all in one load.
Table-driven `TestParse_SessionInvalid`, one row per rule in Approach, decision
2: empty `states`, too many states, bad state name, missing `initial`,
undeclared `initial`, initial state marked `final`, final state with `on:`,
non-final state without `on:`, `on:` key not a message type of the channel type,
ephemeral message type, missing `to`, undeclared `to`, undeclared role, role not
in the message type's `post:`, `by` other than `trigger`, `by` together with
`roles`, `when:` that does not compile, `when:` naming `entity` (reason
"transition guards cannot read the entity yet; use a trigger that posts a
message"), message type named `session` in a session type, session-level
`final:` (refused key), merge key under `session.states`.
3. A state named `aux` (a Windows device name, valid in the name grammar)
loads and a session moves into it on every backend. Test row in
`TestParse_Session` plus conformance `testSessionStateNames`.
4. Nested `session:` loads; a top-level `session:` is refused with "a session
belongs under a channel type: channel_types.<name>.session". Test: the `session`
row of `TestParse_Invalid` updated.
5. `session.states` given through a YAML alias (`states: *base`) loads with
the alias's key order. Test row in `TestParse_Session`.
6. Removing a state, renaming `initial`, or removing the whole `session:`
block passes `CheckReload`. Test `TestCheckReload_SessionChangesAllowed`.
7. The compiled session is immutable: its fields are unexported, accessors
return values or copies of slices, and two `ChannelType()` calls share one
`*Session`. Test `TestCompiled_SessionIsReadOnly` in `compiled_test.go`.
8. Store conformance, in `channelstest.RunAll` (file on disk, file on MemFS,
sqlite, postgres):
   - `testSessionStep`: an append whose decider returns (from `detected`, to
`contained`) on an unset session stores the message at seq n and the notice at
n+1 (type and author from the step, `TriggeredBy` from the step, props `from`,
`to`, `generation`, `cause`); sets the session to (`contained`, 1, message
time); raises rev; and writes one audit record with op `session`, `From`
`detected`, `To` `contained`, `Generation` 1, `Seq` n and `By` the poster. The
notice and the audit carry the same non-empty `From`. `Head`, `List`,
`ListEntity` and `ListByLifecycle` return the session.
   - `testSessionNoMove`: a decider that returns no move appends only the
message; the session, the audit trail and the rev step are as for a plain
append.
   - `testSessionDeciderSeesCurrent`: the decider receives the committed
channel: lifecycle state, session state and generation as they are under the
lock, including after a concurrent move, and a closed channel.
   - `testSessionImplicitCreate`: on a cardinality-one channel that does not
exist, the decider sees an open channel with an unset session, and a move
creates and moves the channel in one step.
   - `testSessionInvalidMove`: a decider returning an invalid state name
returns `ErrInvalidMessage` and stores nothing; on a cardinality-one channel
that did not exist, no channel is created.
   - `testSessionSurvivesMoves`: the session is kept across `SetLifecycle`
close and reopen, `Rekey`, and `Tombstone` then `Untombstone`.
   - `testSessionConcurrent`: eight goroutines append with a decider that
moves only from an unset session; exactly one moves it, all eight messages are
stored, the log holds one notice, and the notice follows the lowest-seq message
whose decider saw the unset session.
   - `testSessionNoticeMarker`: an append without a step whose `TriggeredBy`
starts with `session:` is `ErrInvalidMessage`.
   - A named channel that does not exist is `ErrNotFound` with or without a
step (row added to `testNamedMustExist`).
9. Schema steps: `TestMigrateChannelSessions` in `internal/sqlitedb` (a v23
`channels` table gains the three columns with the shape of a fresh database, an
existing row reads as unset, running the rung twice is harmless, `user_version`
is 24). Postgres: conformance runs on the migrated schema, and
`TestLegacyRowHasNoSession` in `pg_test.go` inserts a row without the new
columns and reads it as unset.
10. `filechannels`: `TestMetaWithoutSessionReadsUnset` (a `meta.yaml` from
before this change reads unset) and `TestSessionStepAcrossSegments` (with
`newStore` and `segSize` 2, a step whose message is the last of a segment puts
the notice first in the next one; both read back, and a rev-cursor read returns
both) in `file_test.go`.
11. Service, `internal/realtime/session_test.go`:
    - `TestSessionPost_Transitions`: the lead posts `contained` in
`detected`; `Read` shows `contained`, generation 1, `entered_at` the post time;
the page holds the message and the notice, rendered with `author.system` true
and props `from`, `to`, `generation`, `cause`.
    - `TestSessionPost_GuardFails`: `assessed` with `risk: none` in
`contained` is posted, the session does not move, no notice, no audit record.
    - `TestSessionPost_GuardAbsentProperty`: a guard reading an optional
property the message leaves out fails closed: posted, no move, no error.
    - `TestSessionPost_RoleNarrowing`: a member posts `contained` (allowed by
`post:`) and nothing moves; the lead posts it and it moves. A client ceiling
that denies update on the anchor stops the lead (post refused as today).
    - `TestSessionPost_AnyPoster`: `dismiss` (no `roles`, no `by`) moves the
session for any principal allowed to post it.
    - `TestSessionPost_TriggerOnly`: the lead posts `escalated` (`by:
trigger`); it is posted and nothing moves.
    - `TestSessionPost_NoTransitionHere`: `notified` in `detected` is posted
and moves nothing.
    - `TestSessionPost_FinalState`: in `notified`, no post moves the session.
    - `TestSessionPost_ReEntry`: a transition whose `to` is the current state
raises the generation, resets `entered_at`, and posts a notice.
    - `TestSessionPost_Reply`: a reply of a driving type moves the session.
    - `TestSessionPost_ManyChannels`: two named channels of a
`cardinality: many` session type on one entity move independently.
    - `TestSessionPost_Concurrent`: two leads post `contained` at once on the
file store; both messages are posted, one notice, generation 1, and the notice
follows the lower seq.
    - `TestSessionPost_Degraded`: under a degraded snapshot an entity-property
transition role admits nobody: the post is accepted, nothing moves, and the MCP
result shows the unchanged session.
    - `TestSessionStranded`: a channel stored in `contained`, read under a
config without it, shows `stranded: true`, no transitions; posts are accepted
and move nothing; close and reopen still work.
    - `TestSessionInitialChange`: an unset session reads in the new `initial`
after a config swap.
    - `TestSessionEnteredAt`: an unset session on a stored channel reports
`entered_at` = the channel's `created_at`; on a channel not stored yet, the
guard and the read both see the operation's clock time.
    - `TestSessionLifecycleAndEdits`: close and reopen keep the state; a post
can never move a closed channel; editing the causing message does not change the
session.
    - `TestSessionNoticeProtected`: a notice offers no `_actions`; edit and
remove of a notice, even by a moderator, answer 409 `session_notice`.
12. Triggers, `trigger_test.go`: `TestSessionTriggerPost` (a trigger posting
`escalated` moves the session, audit `By` `system:realtime` and `TriggeredBy`
`trigger:<name>`; a trigger posting `contained`, which has `roles`, moves
nothing) and `TestSessionTriggerWhenClosed` (a `when_closed: post` trigger
posting a driving type into a closed session channel posts the message and moves
nothing).
13. HTTP: `TestChannelWireGolden` regenerated with a session channel in the
samples (`channel_wire_fields.json` gains `session`, `ChannelSession`,
`ChannelSessionTransition`; `channel_wire.json` a sample with a notice);
`frontend/src/types/wireFields.test.ts` passes with the mirrored fields;
`TestChannelsHandler_SessionInReads` checks `session` on the head, the message
page and the list, and its absence on a type without a session.
14. OpenAPI: `TestOpenAPI_ChannelSession` in `internal/openapi` checks that the
`Channel` schema has a `session` property referencing `ChannelSession`, with the
properties of `v1.ChannelSession`.
15. Collection cost: the list endpoint with session channels makes the same
number of store calls as without. Test: a session type in the fixture of
`internal/dataentry/channels_budget_test.go`, budget unchanged.
16. MCP: `TestReadChannel_IncludesSession`, `TestListChannels_IncludesSession`
and `TestPostMessage_ReturnsSession` (after a post that moved it and one that
did not) in `tools_channels_test.go`.
17. CLI: `TestRenderRead_Session` in `internal/cli/channel_render_test.go`
prints the session line in the channel header and renders a notice as "moved
from detected to contained"; `rela channel post` prints the session after the
post.
18. Docs: the example in `GUIDE-realtime.md` is the `breach-response` block of
`testdata/valid.yaml` (AC 1 pins it), and `just docs` regenerates
`docs/realtime.md` and `docs/mcp-server.md` without diff after the guide edits.

## Research

- [x] For larger features: run `/research` to create a structured research doc
- [x] ~~Searched for existing libraries that solve this problem~~ (N/A: covered by the research doc; the engine reuses internal packages)
- [x] Checked codebase for similar patterns or reusable code
- [x] Looked for reference implementations in other projects
- [x] Reviewed relevant rela concepts for prior art

**Research Doc:** `.ignored/res-sessions-final.md` (decisions of 2026-10-21:
flat states, separate from the lifecycle, roles from graph properties,
`next_due_at` for timers later, sessions allowed on `cardinality: many`).

**Existing Solutions:**
- `channels.Store.SetLifecycle` and `SetAction`: state steps with an audit
record under the same lock or transaction. The session step follows them.
- `realtimecfg` message `when:` (`programs.go` `messageEnv`): compiled once at
load, evaluated with `predicatefns.Bind`.
- `internal/statemachine`: conventions only (compile once, index edges,
well-formedness at load). Not reused as an engine: it is entity-keyed and shaped
for the entity write path.
- `systemWriter.post` (`internal/realtime/system.go`): the trigger post path,
extended so trigger posts can move sessions.
- Schema steps: `pgstore/migrations/0030_channel_tombstones.sql` (`ADD COLUMN
IF NOT EXISTS ... NOT NULL DEFAULT`) and the sqlite `addChannelRetention` rung
(probe `pragma_table_info`, then `ALTER TABLE`).
- Prior art is summarized in the research doc. The lesson used here: a
transition is a validated event judged against the current, versioned state.

## Approach

- [x] Technical approach chosen and documented
- [x] Approach builds on existing patterns (not reinventing)
- [x] Alternatives considered (document why rejected)
- [x] Dependencies identified (packages, APIs, types)

**Technical Approach:**

1. **Config types.** `raw.go`: `rawChannelType.Session *rawSession`;
`rawSession{Label, Initial string; States map[string]rawSessionState}`;
`rawSessionState{Label string; Final bool; On map[string]rawTransition}`;
`rawTransition{To string; Roles []string; By string; When string}`. A Go map
loses YAML order, and slice 2 needs it for the stepper, so `Parse` reads the key
order of each `session.states` mapping from the `yaml.Node` it already holds
(helper `mappingKeys` in `session.go`). The helper follows alias nodes (`states:
*base`), and refuses a merge key (`<<`) under `session.states` with "merge keys
are not supported under session.states; write the states out", so the node walk
and the decoded map always agree. The key checker needs no change.

Compiled form, immutable by construction (like `predicate.Program`):

```go
type Session struct { label, initial string; states []SessionState; index map[string]int }
func (s *Session) Label() string
func (s *Session) Initial() string
func (s *Session) States() []SessionState          // copy of the slice
func (s *Session) State(name string) (SessionState, bool)
func (s *Session) Transition(state, msgType string) (Transition, bool)
func (s *Session) Drives(msgType string) bool        // type appears in some on:
type SessionState struct { Name, Label string; Final bool; on map[string]Transition }
func (st SessionState) Transitions() []Transition    // sorted by message type
type Transition struct { MessageType, To string; Roles []string; By TransitionBy; When *predicate.Program }
```

`SessionState.on` is unexported and `Transition.Roles` is returned as a copy by
the accessors, so a caller cannot change shared state. `ChannelType` gains an
unexported `session *Session` and the accessor `Session() *Session` (nil without
a block); `clone.go` shares the pointer instead of copying. Later slices add
fields to `SessionState` (`after`, `status`, `waitingFor`, `outputs`) and to
`Session` (`timers`, `runAs`).

2. **Validation** (new `internal/realtimecfg/session.go`, called from
`validate.go`). All problems are reported in one pass, each with its key path:
   - `session.states`: "a session needs at least one state"; more than
`realtimelimits.MaxSessionStates` (64): "at most 64 states".
   - state name: "state name must match ^[a-z][a-z0-9-]{0,62}$". State names
are never path segments, so Windows device names such as `aux` are allowed; the
store checks the same grammar (decision 3).
   - `session.initial`: missing: "name the state a new session starts in";
undeclared: `unknown state "x"; declare it under states`; marked final: "the
initial state cannot be final: an unset session reads as the initial state, so
no channel could ever move".
   - a final state with `on:`: "a final state has no transitions; drop on or
remove final: true".
   - a non-final state without `on:`: "a state that is not final needs at
least one transition under on; mark it final: true if the session ends there".
(Slice 3 relaxes this for states with `after:`.)
   - `on.<type>`: not a message type of the channel type: `unknown message
type "x"; a transition is keyed by a message type of this channel type`;
ephemeral: "an ephemeral message is never stored, so it cannot move a session".
   - `on.<type>.to`: missing: "name the state this transition moves to";
undeclared: the unknown-state message. `to` may equal the current state (a
re-entry).
   - `on.<type>.roles[i]`: undeclared: the existing `roleRefs` message; not in
the message type's `post:`: `role "x" may not post <type>, so it could never
fire this transition; add it to post or drop it here`.
   - `on.<type>.by`: any value but `trigger`: `by must be trigger; agents are
ordinary principals, so limit people and agents with roles`; together with
`roles`: "name roles or by: trigger, not both: a trigger holds no role".
   - `on.<type>.when`: compiled in `sessionGuardEnv(mt.Properties)`, which
declares `message` (typed as in `messageEnv`), `session` (record: `state`
string, `generation` integer, `entered_at` datetime) and the standard library.
When compiling fails, the source is compiled once more in a probe env that also
declares `entity` as an open record; if that probe compiles and
`prog.UsesWhole("entity")` or `prog.Attributes("entity")` is non-empty, the
problem is "transition guards cannot read the entity yet; use a trigger that
posts a message". Other errors: "does not compile: <error>". The AC 2 row pins
the exact message, so the implementation may pick another detection if the
predicate package offers a cleaner one.
   - message type named `session` in a session type: `"session" is reserved
in a channel type with a session: transitions post notices of that type; choose
another message type name`.
   - `keys.go`: drop the `channel_types.*.session` refusal; the top-level
`session` reason becomes "a session belongs under a channel type:
channel_types.<name>.session"; add `channel_types.*.session.final` with "mark
each final state with final: true".
   - Unreachable states are allowed: keeping an old state declared with an
`on:` out of it is how an operator unsticks stranded sessions.

3. **Store model** (`internal/channels/channels.go`, shared helpers so the
backends cannot drift):

   ```go
   // Session is a channel's session snapshot. The zero value is unset.
   type Session struct {
       State      string    `json:"state,omitempty"`
       Generation int64     `json:"generation,omitempty"`
       EnteredAt  time.Time `json:"entered_at,omitzero"`
   }
   type SessionMove struct{ From, To string }
   // SessionStep lets an append move the channel's session in the same step.
   type SessionStep struct {
       // Decide is called under the store's lock or transaction with the
       // channel as committed (an open, unset channel for an implicit create).
       // It must be pure and fast: no I/O, no locks. It may be called more
       // than once if a backend retries the transaction.
       Decide           func(cur Channel) (SessionMove, bool)
       NoticeType       string // message type of the notice
       NoticeBy         Author // author of the notice (system:realtime)
       TriggeredBy      string // notice TriggeredBy; must start with "session:"
       AuditTriggeredBy string // set when the causing message came from a trigger
       Outcome          *SessionOutcome // optional; filled by the store on success
   }
   type SessionOutcome struct { Session Session; Moved bool; Move SessionMove }
   ```

`Channel.Session Session` (`json:"session,omitzero"`); `NewMessage.Session
*SessionStep`; `AuditRecord.Generation int64` (`json:"generation,omitempty"`);
`AuditSession AuditOp = "session"`. Helpers: `PrepareSessionStep` (decider set,
notice author valid, notice type `ValidName`, sizes capped), `ValidStateName`
(the plain name grammar, without the Windows device-name check that `ValidName`
adds for path segments), `CheckSessionMove(move)` (both names `ValidStateName`,
else `ErrInvalidMessage`), `ApplySession(c *Channel, move, at)` (state = To,
generation + 1, entered_at = at), `SessionNotice(step, cause Message, move,
gen)` (props `from`, `to`, `generation`, `cause`), `SessionAudit(k, step, cause,
move, gen)` (`By` the poster, `From`, `To`, `Generation`, `Seq` the cause).
`PrepareMessage` refuses any `TriggeredBy` that starts with `session:`, so the
notice built by `SessionNotice` inside the store is the only message that can
carry that marker. The `Store` interface gains no method; the `Append` doc
comment describes the step. `Append` does not force `RequireOpen` for a step:
the decider sees the lifecycle state and never moves a channel that is not open,
so `when_closed: post` triggers keep posting.

Plimsoll: `filechannels.Store` has 40 methods (the cap) and `sqlchannels.Store`
39. All shared logic lives in package `channels`; backend helpers are free
functions or methods on `disk` (filechannels) and plain functions taking a
`Querier` (sqlchannels). No new `*Store` method.

Slice 3 seam: timer fires need a step without a user message. That slice adds
`Store.SetSession(ctx, key, step)` over the same decider and helpers (raising
the pinned method counts as a documented required-interface change), plus
`NextDueAt` with its column and index.

4. **Backends.**
   - `sqlchannels` (sqlite and pg): `chanCols`/`scanChannel` read
`session_state`, `session_gen`, `session_entered_at`. In `Append`'s existing
transaction: `appendTarget` (may insert the implicit channel),
`CheckAppendState`, call `Decide(r.c)`, `CheckSessionMove`, insert the message,
insert the notice at seq + 1 when there is a move, one `UPDATE channels` setting
last_seq, rev and the session columns, the create audit if created, the session
audit. Any error rolls back, including the implicit insert. pg serializes by its
entity lock; sqlite by its write transaction.
   - `sqlitedb`: `channelsDDL` gains `session_state TEXT NOT NULL DEFAULT ''`,
`session_gen INTEGER NOT NULL DEFAULT 0`, `session_entered_at INTEGER NOT NULL
DEFAULT 0`; `schemaVersion` 24; rung `to: 24` `addChannelSessionColumns` probes
each column like `addChannelRetention` and adds the missing ones, so a re-run is
harmless.
   - `pgstore/migrations/0031_channel_sessions.sql`: three `ALTER TABLE
channels ADD COLUMN IF NOT EXISTS ... NOT NULL DEFAULT ...`, header in the style
of 0029/0030 (a new file; shipped migrations are never edited). No index:
nothing queries by session state in this slice.
   - `filechannels`: `meta` gains `SessionState`, `SessionGen`,
`SessionEnteredAt` (yaml, omitempty); `meta.channel` maps them. `Append` under
the existing lock: build the target meta (in memory for an implicit create),
call `Decide`, check the move, reserve both revs, write `meta.yaml`, append the
message and notice lines (one `appendLog` call when they share a segment, two
across a boundary), and only then the session audit record. A crash after
`meta.yaml` can leave the session moved with no message, no notice and no audit
record; the trail can miss a record but never names a wrong cause. The package
doc states this exact window. Rekey, tombstone and untombstone move whole
directories, so the fields travel with them.

5. **Deciding a transition** (`internal/realtime`, new `session.go`):
   - `currentSession(ct, ch Channel, now)`: an unset session reads as
`Initial()`. Its `entered_at` is the channel's `created_at`; for a channel that
is not stored yet the service passes a channel built with `CreatedAt = c.now`,
which is the time the append will store. A stored state the config does not
declare is stranded.
   - `decider(ctx, ct, mt, props, now, poster posterRoles) func(Channel)
(SessionMove, bool)`: returns no move when the channel is not open, the state is
final or stranded, the state has no `on:` entry for `mt`, the poster fails the
entry's rule, or the `when:` does not hold. The poster rule: with `roles`, a
user poster passes when `call.can(role, realtimecfg.CapabilityUpdate)` admits
one of them (as `mayPost` does, so the client ceiling applies); a trigger poster
never passes. With `by: trigger`, only a trigger poster passes. With neither,
every poster who was allowed to post passes. The role answers are computed
before the append (they read ACL state, which is I/O), and the closure only
looks them up. A guard that errors or does not yield a boolean fails closed.
   - `Service.Post`: when `ct.Session() != nil` and `Drives(mt.Name)`, append
with a step whose decider is built from the validated props. There is no head
re-read, no retry loop and no new 409: the decision runs against the committed
state under the lock. User posts keep `RequireOpen` as `Service.Post` sets it
today. `PostResult` gains `Session *SessionView`.
   - The service learns the session after the step without a second read:
`SessionStep.Outcome *SessionOutcome` is a pointer the caller sets and the store
fills on success (`Session` after the step, `Moved`, the `SessionMove`).
`Append`'s signature and `Message` stay as they are; `testSessionStep` and
`testSessionNoMove` check the outcome.
   - Ordering: a message is judged against the session state at the moment
it is appended. The store serializes appends per channel, so the move always
follows the lowest-seq matching message. Two concurrent posts of one transition
give two messages and one move.
   - Guard fails, role missing or `by: trigger` on a user post: the message
is still posted (the poster had the `post:` right), nothing moves, no notice, no
audit record. A transition is part of what a message means, not a separate
permission.
   - `systemWriter.post` (triggers): same step with poster kind trigger. It
keeps passing `requireOpen` from `when_closed`; the decider refuses to move a
closed channel, so `when_closed: post` still posts and moves nothing. The audit
`By` is `system:realtime`, `AuditTriggeredBy` is `trigger:<name>`.
   - Notices are protected: `messageActions` offers nothing on a notice, and
`Edit` and `Delete` (own or moderator remove) of a notice answer 409
`session_notice` ("a session notice records a transition and cannot be
changed"). The audit keeps the record anyway; the notice keeps the log readable.
Retention and channel delete still erase notices with the channel.
Identification: reserved type `session` and a `TriggeredBy` with the `session:`
marker, which only the store can write.
   - Degraded config: `call.can` refuses entity-property roles, so a
transition with such a role does not move; the post is still accepted. MCP and
CLI show the session after the post; HTTP clients see it in the head (pushed
over SSE when the rev changes).
   - Edits of the causing message never re-evaluate or undo a transition.
Close and reopen leave the session as it is.

6. **Guard inputs.** `when:` sees `message.<prop>` (validated props, typed as
for the message type's own `when:`; an absent optional property reads as absent
and a comparison on it fails closed), `session.state`, `session.generation`,
`session.entered_at`, and the standard library. No `entity` in this slice (user
decision); refused at load with an actionable message. Adding it later means a
per-`attach_to` compile with the `compileRule` allowlist and an anchor read for
both post paths.

7. **Rendering and wire.**
   - `realtime.SessionView{State, Label string; Generation int64; EnteredAt
time.Time; Final, Stranded bool; Transitions []SessionTransitionView}`,
`SessionTransitionView{MessageType, To string; By string}` (the `on:` entries of
the current state, sorted; config, the same for every reader; empty when final
or stranded). `ChannelView.Session *SessionView` (`json:"session,omitempty"`),
nil for a type without a session. `channelView` takes the channel type and the
clock time; its callers (`Head`, `Read`, `ListAnchor`, `Create`, `Lifecycle`)
hold both.
   - The renderer shows a notice's props (`from`, `to`, `generation`,
`cause`) although `session` is not a declared message type.
   - `internal/apiwire/v1/channels.go`: `ChannelSession` and
`ChannelSessionTransition`; `Channel.Session *ChannelSession`
(`json:"session,omitempty"`); times RFC 3339. `dataentry/channels_wire.go` maps
it. `frontend/src/types/channels.ts` mirrors the two types and adds their fields
to `CHANNEL_WIRE_FIELDS` (types only; no component uses them until slice 2).
   - `internal/openapi/channels.go`: a `ChannelSession` schema, the `session`
property on `Channel`, and a sentence on session notices (type `session`, system
author, props) in the message schema description.
   - HTTP post answer: unchanged (201 with the message). The v1 body is a
message, and wrapping it would break clients; the head carries the session and
SSE pushes it on the notice's rev.
   - MCP: `read_channel` and `list_channels` carry it through `ChannelView`;
`messageResult` gains `session,omitempty` for `post_message`. The `read_channel`
description mentions the session block.
   - CLI: `writeChannelHeader` prints "session: contained (Contained),
generation 1" (plus "stranded" or "final" when set); notices render as "moved
from detected to contained"; `rela channel post` prints the session line after
the message.

Wire shape:

```json
"session": {
  "state": "contained", "label": "Contained", "generation": 1,
  "entered_at": "2026-10-21T09:12:03Z",
  "transitions": [ { "message_type": "assessed", "to": "assessed" } ]
}
```

A notice:

```json
{ "seq": 8, "type": "session", "author": { "user": "system:realtime", "system": true },
  "triggered_by": "session:breach-response",
  "props": { "from": "detected", "to": "contained", "generation": 1, "cause": 7 } }
```

8. **`entered_at` semantics.** After a move: the time of the causing message.
Unset session on a stored channel: the channel's `created_at` (the session is
taken to start with the channel), also for channels that existed before their
type gained a session. Unset session on a channel not stored yet: the
operation's clock time, which equals the `created_at` the append stores.
Generation 0 on the wire marks a session that never moved. Slice 3 seam:
`after:` timers must not count from that `created_at` for generation-0 sessions
of old channels; slice 3 starts them from when its reload recompute first sees
the session.

9. **Config reload and stranded state.**
   - No reload is refused for session changes: sessions do not decide how
channels are keyed, so `CheckReload` and the type record are untouched.
   - An unset session follows a changed `initial` (user decision).
   - A stored state the new config does not declare is stranded: shown with
`stranded: true` and its stored name as label, no transitions, posts accepted
without moving it, lifecycle operations unaffected. The way out is to declare
the state again with an `on:` to a current state.
   - Removing `session:` hides the session from reads; the stored fields stay
and come back with the block.
   - No reload scan or log in this slice; slice 3 builds the per-type scan
for its recompute and logs stranded sessions there.

**Alternatives rejected:**
- Decide outside the store with a (state, generation) expectation, a head
re-read and a bounded retry ending in 409. It needs the lifecycle state in the
expectation, a re-read path that handles a deleted channel, a retry loop in two
post paths, and tests with a conflicting store wrapper. The guard is pure CPU
(props, session, standard library) and the role answers are computed before the
call, so deciding under the lock is correct, keeps the "lowest seq wins" rule by
construction, and is less code. The CLAUDE.md rule forbids slow I/O inside a
`Tx` callback, not pure evaluation; the decider contract says so. The store
still applies no policy of its own: it calls the decider, checks the names and
writes.
- A separate `Store.AppendWithSession`: duplicates the append path in every
backend and breaks the filechannels method cap.
- Forcing `RequireOpen` for steps: breaks `when_closed: post` triggers; the
decider's open check covers it.
- Writing the initial state when a channel is created: needs changes to both
create paths and still needs "unset reads as initial" for channels that existed
before their type gained a session.
- Notice type chosen by the store: the store stays free of config names; the
service passes the type and realtimecfg reserves it.
- A load warning instead of `by: trigger` for trigger-driven transitions: a
warning leaves the hole open; `by: trigger` closes it.
- Short form `contained: contained`: needs a custom decoder the reflection
key checker cannot walk.
- Allowing moderators to remove notices: the log would lose the visible
record of a transition while the session stays moved.

**Files to modify:**
- `internal/realtimecfg`: `raw.go`, `keys.go`, `load.go` (state order),
`compiled.go` (`ChannelType.session`, accessor), new `session.go` (types,
validation, `mappingKeys`, guard env), `programs.go` (`sessionGuardEnv`),
`clone.go`, `validate.go` (call site, reserved name); `testdata/valid.yaml`
(`breach-response`), `testdata/schema.yaml` (`risk-level`), `testdata/acl.yaml`
(`privacy.officer` if missing) (+ `load_test.go`, `compiled_test.go`).
- `internal/realtimelimits`: `MaxSessionStates`.
- `internal/channels`: `channels.go` (`Session`, `SessionMove`,
`SessionStep`, `SessionOutcome`, helpers, `AuditSession`,
`AuditRecord.Generation`, `PrepareMessage` marker check), `store.go` (Append
contract), `channelstest/channelstest.go`.
- `internal/channels/sqlchannels/sql.go`; `internal/channels/filechannels`:
`file.go` (meta, package doc), `ops.go` (Append via `disk` helpers) (+
`file_test.go`); `internal/channels/pgchannels/pg_test.go`.
- `internal/sqlitedb`: `sqlitedb.go`, `migrate.go` (+ new
`channelsessions_test.go`).
- `internal/store/pgstore/migrations/0031_channel_sessions.sql`.
- `internal/realtime`: new `session.go` (+ `session_test.go`), `service.go`,
`system.go`, `render.go`, `read.go`, `ops.go` and `msgops.go` (notice
protection), `errors.go` (`TypeSessionNotice`), `lifecycle.go` and `actions.go`
(`channelView` call sites), `trigger_test.go`.
- `internal/apiwire/v1/channels.go`; `internal/dataentry/channels_wire.go`
(+ `channels_wire_golden_test.go`, `testdata/channel_wire*.json`,
`channels_handler_test.go`, `channels_budget_test.go`).
- `internal/openapi/channels.go` (+ test).
- `frontend/src/types/channels.ts` (types and `CHANNEL_WIRE_FIELDS`).
- `internal/mcp/tools_channels.go` (+ `tools_channels_test.go`).
- `internal/cli/channel_render.go`, `internal/cli/channel.go`
(+ `channel_render_test.go`).
- `docs-project/entities/guides/GUIDE-realtime.md`,
`docs-project/entities/guides/GUIDE-mcp-server.md`; `docs/realtime.md` and
`docs/mcp-server.md` regenerated by `just docs`.

## Security Considerations

- [x] Input sources identified (user input, config, external APIs)
- [x] Input validation approach defined (allowlist preferred over blocklist)
- [x] Security-sensitive operations identified (file access, auth, crypto)
- [x] Error handling doesn't leak sensitive information

**Input Sources & Validation:**
- `realtime.yaml` (operator config, not secret): names checked against
declared states, message types and roles; state names follow the name grammar on
both sides; the state count is capped; merge keys under `session.states` are
refused.
- Posted props: validated by `validateProps` before the guard sees them; the
guard reads only those props, which every channel reader sees anyway.
- Session fields are written only by the store's step; no request carries a
state or generation. The `session:` notice marker is refused on every other
append, so a poster cannot forge a notice.

**Security-Sensitive Operations:**
- Who can move a session: only a principal allowed to post the message type,
narrowed by `roles` (checked with `CapabilityUpdate`, so the client ceiling
applies) or limited to triggers by `by: trigger`. Roles only admit (no runtime
deny, per `acl-ceiling.md`).
- Entity-driven transitions: without `by: trigger`, any poster of the message
type could fake the "entity changed" event. `by: trigger` makes such a
transition unreachable for users and agents; the docs tell operators to use it
for every trigger-driven transition.
- Disclosure: a notice tells readers that the poster's message moved the
session, which can reveal that the poster holds a transition role. Role sources
on restricted properties are refused at load; membership in a permission role is
the same one bit an action result shows.
- No entity data enters a guard, so a guard cannot be an oracle on entity
values.
- The decider runs inside the store lock: it does no I/O, so a slow ACL or
network call cannot hold the lock.
- Errors: config problems name keys and values (config is not secret); store
errors map through `storeError`; `session_notice` names no data.

## Test Plan

- [x] Test scenarios documented for each acceptance criterion
- [x] Edge cases identified and documented
- [x] Negative test cases defined (invalid input, error conditions)
- [x] Integration test approach defined (not just unit tests)

**Test Scenarios:** one test or table row per acceptance criterion, named there.
Layers: realtimecfg unit tests (AC 1 to 7); store conformance on four backends
(AC 8), postgres gated on `RELA_TEST_DATABASE_URL` and run with `just
test-postgres`; schema step and file-tier tests (AC 9, 10); service tests over
the file store on MemFS with the test clock (AC 11); trigger engine tests (AC
12); HTTP handler, golden wire and frontend mirror tests (AC 13, 15); OpenAPI
(AC 14); MCP (AC 16); CLI renderer (AC 17); docs (AC 18). Race detector on.

**Edge Cases (each has a test):**
- First message of a cardinality-one channel moves it (AC 8
`testSessionImplicitCreate`).
- Invalid move on a channel that did not exist creates nothing (AC 8
`testSessionInvalidMove`).
- Named channel missing (AC 8 `testNamedMustExist` row).
- Two named channels of a `many` type (AC 11 `TestSessionPost_ManyChannels`).
- Re-entry (AC 11 `TestSessionPost_ReEntry`).
- Reply of a driving type (AC 11 `TestSessionPost_Reply`).
- Guard on an absent optional property (AC 11
`TestSessionPost_GuardAbsentProperty`).
- Notice crossing a segment boundary (AC 10
`TestSessionStepAcrossSegments`, file tier only, small `segSize`).
- State named like a Windows device (AC 3).
- YAML alias for `states` (AC 5); merge key refused (AC 2).
- `when_closed: post` into a closed channel (AC 12).
- Degraded config (AC 11 `TestSessionPost_Degraded`).
- Not covered by a test: generation overflow (int64, unreachable).

**Negative Tests:** AC 2 (every load rule), AC 4 (top-level refusal), AC 8
(invalid move, forged marker, missing named channel), AC 11 (guard fails, role
missing, trigger-only, no transition here, final, stranded, notice protected),
AC 12 (trigger against a `roles` transition).

## Risk Assessment

- [x] Technical risks assessed with mitigations
- [x] Security risks assessed (see Security Considerations)
- [x] Effort estimated (xs/s/m/l/xl)

**Risks:**
- Append is the hottest channel write; a step runs only for session types
and driving message types and adds one insert for the notice. Mitigation: budget
test (AC 15).
- A decider that panics or blocks inside the store lock. Mitigation: it is
built in one place (`realtime/session.go`), does map lookups and one predicate
evaluation with fixed step budgets, and is covered by tests; the contract
forbids I/O.
- File-tier crash window between `meta.yaml` and the log lines. Mitigation:
audit written last, window documented.
- filechannels is at the plimsoll method cap. Mitigation: helpers on `disk`
and in package `channels`; `just plimsoll` in the implementation checklist.
- Frontend wire mirror drift. Mitigation: `wireFields.test.ts` (AC 13).
- Later slices build on the step and `Session`. Mitigation: slice 3's
`SetSession` reuses the decider and helpers; `outputs` (slice 5) hang off the
move outcome.

**Effort:** l

## Documentation Planning

- [x] User-facing docs identified (skip if internal refactor)
- [x] Docs-checklist will be created when entering implementation

**Documentation Impact:** Enhancement. `docs/realtime.md` is generated from
`docs-project/entities/guides/GUIDE-realtime.md` by `just docs`
(`scripts/generate-docs.sh`), so the edit goes into the guide entity. Changes: a
"Sessions" section after "Lifecycle and periods" (the `breach-response` example,
keys, how a message moves a session, `roles` versus `by: trigger` versus
neither, guard inputs and the `entity` refusal, triggers and `when_closed`,
final states, stranded states, `entered_at`, protected notices, relation to the
lifecycle); the session rules in "Checks when the file loads"; the `session`
block and notices in "HTTP API" and "Agents and other processes"; session
behaviour in "Reloading". `GUIDE-mcp-server.md`: the `read_channel` and
`post_message` rows. Code: godoc on the new types and on the `Append` contract
and the decider rules.

## Design Review

- [x] Run `/design-review` before starting implementation
- [x] All critical/significant findings addressed in plan

**Design Review Findings:** see the RRs linked to TKT-Y6NJ6U. The transition is
now decided under the store lock by a pure decider that sees the committed
lifecycle and session state. This removes the retry and 409 path, keeps
`when_closed: post` triggers working, and lets the store record the same `from`
in the notice and the audit. Final states are marked per state, trigger-only
transitions use `by: trigger`, and the plan now covers the frontend mirror,
OpenAPI, CLI, protected notices, `entered_at` semantics and the missing
edge-case tests.

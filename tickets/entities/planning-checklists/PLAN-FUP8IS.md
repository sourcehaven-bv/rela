---
id: PLAN-FUP8IS
type: planning-checklist
title: 'Planning: Session composer: offer only message types that apply in the current state'
started: "2026-10-28"
completed: "2026-10-28"
status: done
---

<!-- @managed: claude-workflow v1 -->

## Understanding

- [x] Problem/requirements clearly understood
- [x] Scope defined (what's in/out documented below)
- [x] Acceptance criteria documented with specific test scenarios

**Problem.** In a session channel, `_actions.post` lists every message type
whose `post:` roles admit the caller, in every state. The composer offers all of
them. In the agent-runner PoC (`rela-mission/project/realtime.yaml`) the owner
in `plan-review` sees abandon, accept, allow, answer, approve, deny, go, retry,
revise, rework and text. Only approve and revise move the session there, and
text is free talk. A message type's `when:` reads only the message, so the
operator cannot say "approve belongs to plan-review".

Today a post that matches no transition is stored and moves nothing (docs: "a
transition is part of what a message means, not a separate permission"). That
stays the default. The ticket adds an opt-in per message type.

**Scope:**

In scope:

- A new optional key `channel_types.<ct>.message_types.<mt>.in:`, a list of
session states in which the type may be posted. Without it, the type may be
posted in every state, as today.
- Load checks for the key.
- `_actions.post` (head, channel list, `read_channel` `can_post`,
`read_session` and inbox `transitions`) follows `in:`.
- Server enforcement on post: a type posted outside its states is refused
with 409 `session_wrong_state`. The check runs inside the store's append step,
so it is judged against the same state the session decider sees.
- Trigger posts outside `in:` are skipped, like a post into a closed channel.
- The SPA maps `session_wrong_state` (refetch, en + nl message). The
composer itself needs no new logic: it already offers only `_actions.post`.
- Docs (`docs/realtime.md`, `docs/mcp-server.md`, the two docs-project
guides), OpenAPI text and MCP tool descriptions.

Out of scope:

- One-click buttons for transitions (TKT-T9G1AE). They read
`_actions.transitions`, which stays a subset of `_actions.post`, so this ticket
does not block them.
- Branching transitions, several `on:` entries per type with different
guards (TKT-FPXTQJ). The `in:` check is per state and type, so it holds for any
number of entries per state.
- Deriving `in:` from `on:` automatically, including an opt-in
`session.offer: transitions` shorthand. That becomes a follow-up ticket (see
Open questions, Q1b).
- Filtering typing by session state (documented instead; see Q4).
- Restricting edits, removals, actions or typing by state. An edit never
moves a session; `in:` is about new posts only.
- Changing the PoC's `realtime.yaml`. The PoC adopts `in:` itself after the
merge.
- Adding `in` to the `GET /_schema/channels` descriptor (see Q3).

**Acceptance Criteria:**

1. **`in:` compiles and is opt-in.** A message type may declare
`in: [state, ...]`. A type without `in:` behaves exactly as before. Test:
`realtimecfg` table test compiles the PoC session shape with `approve: { in:
[plan-review] }` and reads `MessageType.In`. The existing `realtimecfg` and
`realtime` suites pass unchanged, because no existing fixture (including
`testdata/valid.yaml`) gains `in:` (see AC 10).
2. **Load checks.** The file is refused, with a problem at the key, for:
`in:` on a type of a channel type without `session`; `in:` with no value or `in:
~` ("name the states, or drop in to allow every state"); an empty list; an
unknown state; a state listed twice; a state whose `on:` has an entry keyed by
the type while `in:` leaves that state out ("the transition could never fire");
a reminder whose `post.type` has `in:` that does not cover every state in the
reminder's `in:`; an `in:` that lists only final states while `session.on_final:
close` ("the channel closes when the session enters these states, so the type
would be offered at most until the close runs"). Test: one table-driven case per
problem in `internal/realtimecfg/message_in_test.go` asserting the key and
reason; `rela validate` prints the `✗` line
(`internal/cli/validate_channels_test.go` case).
3. **Affordances follow the state.** `_actions.post` lists a type with `in:`
only while the session is in one of its states. `read_channel` `can_post`,
`_actions.transitions`, `read_session.transitions` and the inbox `transitions`
follow, because they all derive from `call.postable()`. Fixture: the PoC run
channel type with `in:` on every transition-keyed type, each listing exactly the
states whose `on:` keys it (go `[new]`; claim, resume `[queued]`; plan
`[planning]`; ask `[planning, working]`; answer `[plan-asking, asking]`;
approve, revise `[plan-review]`; permission, finished `[working]`; fail
`[planning, working]`; allow, deny `[approving]`; accept, rework `[review]`;
retry, abandon `[blocked]`), progress `in: [planning, working]`, and text
without `in:`. Test: `internal/realtime/session_affordance_test.go`: owner in
`plan-review` gets post `[approve, revise, text]` and transitions `[approve,
revise]`; owner in `working` gets `[text]`; runner in `working` gets `[ask,
fail, finished, permission, progress, text]`.
4. **Enforced on post.** A post of a type outside its states answers 409
`session_wrong_state`, stores nothing and moves nothing. A post refused by the
pre-check spends no rate token. A post refused by the store after a concurrent
move spends one, like any failed append. `detail` names
`channel_types.<ct>.message_types.<mt>.in`. Both paths use one title, built from
the state the refusal saw: "<mt> may be posted only while the session is in
<states>; it is in <state>" (stranded: "...; the session is stranded in
<state>"). Test: `internal/realtime/session_test.go` (exact title, nothing
appended, limiter untouched); `internal/dataentry/channels_handler_test.go`
(HTTP 409 with problem type); `internal/mcp/tools_session_test.go`
(`post_message` returns `session_wrong_state: ...` with the states);
`internal/cli/channel_test.go` (`rela channel post` prints the same text and
exits non-zero).
5. **Judged under the store lock.** The session step's new `Admit`
callback runs in `channels.DecideSession`, under the store lock on every
backend. It refuses with `*channels.SessionStateError{State}`, which unwraps to
`channels.ErrSessionState` and carries the state actually seen. Test: a
`channelstest` conformance case run by filechannels, pgchannels and
sqlitechannels: an append whose step's `Admit` refuses stores nothing (LastSeq
unchanged, no notice, no mention rows) and returns the typed error unchanged;
`SetSession` refuses a step that sets `Admit` with `ErrInvalidMessage`. A
`realtime` concurrency test posts `approve` and `revise` at once in
`plan-review`: exactly one moves the session, and the other is refused 409 with
the title naming `working` (the real state, not the stale head). The test does
not assert the loser's limiter bucket.
6. **Trigger posts.** A trigger whose message type has `in:` and the session
is elsewhere is skipped: nothing is stored, `Skipped` counts it, a debug line
names the rule. Test: `internal/realtime/trigger_test.go` with the PoC `ready`
trigger posting `go` (`in: [new]`) into a session in `working`. A `when_closed:
post` trigger into a closed channel whose final state is outside the type's
`in:` is skipped the same way.
7. **Stranded, final, closed.** A stranded session admits only types
without `in:`. A final state admits the types that list it. A closed or archived
channel answers `channel_not_open` first, as today, and `_actions.post` stays
empty. Test: cases in `session_affordance_test.go` and `session_test.go` for
each.
8. **Channel without a session is unchanged.** Its affordances and posts do
not change; `in:` there is a load error (AC 2). Test: existing no-session tests
pass; one new case asserts identical `_actions.post`.
9. **SPA.** In the PoC shape, the owner's composer in `plan-review` offers
approve and revise (marked) and text. A `session_wrong_state` refusal shows a
toast and refetches the channel, so the stale type disappears. The toast text is
translated (en, nl). Test: `channelErrors.test.ts` (rule present, enforced by
its errors.go scan; nl message); `e2e/tests/channels.spec.ts` session scenario
extended with a type that has `in:`: it is absent from the type picker before
the move and present after.
10. **Docs and API text.** `docs/realtime.md` and
`docs-project/entities/guides/GUIDE-realtime.md` document `in:` in sync: the
load checks, the 409, the composer rule, the trigger and `when_closed` notes,
typing, and "every process that loads the project needs this release or later".
The breach session example and `testdata/valid.yaml` stay unchanged.
`TestDocsExample_Realtime` pins the guide's session block to `valid.yaml`, which
`load_test.go`, `internal/realtime/reminders_test.go`, `timers_test.go` and
`internal/appbuild/realtime_handoff_test.go` load. `in:` gets its own short
snippet instead (a PoC-style plan-review). Test: `docs_example_test.go` passes
untouched; a new case in `message_in_test.go` compiles the docs snippet, copied
verbatim; OpenAPI golden (if any) and MCP `tools_list` golden updated.

## Research

- [x] ~~For larger features: run `/research` to create a structured research doc~~ (N/A: medium change; the options are few and are settled under Open questions)
- [x] ~~Searched for existing libraries that solve this problem~~ (N/A: configuration and state rule inside rela; no library applies)
- [x] Checked codebase for similar patterns or reusable code
- [x] ~~Looked for reference implementations in other projects~~ (N/A: the in-repo precedents below are closer than any outside example)
- [x] Reviewed relevant rela concepts for prior art

**Research Doc:** N/A

**Existing Solutions:**

- `session.reminders.<r>.in:` (`internal/realtimecfg/reminders.go:147-163`)
is a per-item list of states, with checks for unknown, final and duplicate
states. `in:` on a message type copies its name, shape and messages.
- `lifecycle_wrong_state` (`internal/realtime/lifecycle.go:132`) is a 409
for "the channel is not in a state this operation starts from". The new
`session_wrong_state` mirrors it.
- `SessionStep.Decide` + `channels.DecideSession`
(`internal/channels/session.go:272-387`) is a pure callback that every backend
runs under its lock on every session-channel append (`filechannels/ops.go:56`,
`sqlchannels/sql.go:463`), also for a `one` channel the append creates. Every
post in a session channel already carries a step (`newSessionPost`,
`internal/realtime/session.go:353`), trigger posts included (`system.go`). The
store-side check is one more callback on that step, so there is one copy of the
session rules.
- `call.postable()` (`internal/realtime/service.go`) is the single source of
`_actions.post`, of `read_channel` `can_post` (`Read` calls `postable()` at
`read.go:127`; `messageTypes` sets `CanPost` at `read.go:222`), and of the input
to `call.fireable()` (`session.go:297`), which the head, `read_session`
(`waiting.go:200`) and the inbox (`inbox.go:869`) use. Filtering there updates
every surface at once.
- `currentSession` (`session.go:126`) reads an unset session
(`Session.IsZero()`) as `initial` and flags stranded. Both the pre-check and the
store-side callback use it.
- The trigger engine already skips `ErrNotOpen` (`trigger.go:1235`).
- Frontend: `RlChannelComposer` offers `props.postable` only
(`RlChannelComposer.vue:137`), and `OpenChannel.vue` passes `_actions.post`.
`channelErrors.ts` maps every realtime problem type, and its test scans
`internal/realtime/errors.go`. `rate_limited` shows a translated message
(`classifyChannelError`); the new type follows that pattern.
- `.claude/rules/acl-ceiling.md` forbids a subtractive runtime deny in the
ACL evaluator. `in:` is not ACL. It is an allowlist of the states where a type
is allowed, so the rule does not apply. `.claude/rules/predicate.md`: no
condition is added; `in:` is a plain list, not a predicate.

## Approach

- [x] Technical approach chosen and documented
- [x] Approach builds on existing patterns (not reinventing)
- [x] Alternatives considered (document why rejected)
- [x] Dependencies identified (packages, APIs, types)

### Open questions (decided)

**Q1. Declarative shape.** Options: (a) per-type `in: [states]`; (b) a per-state
list of allowed types; (c) derive "types that move the session now" from `on:`,
plus an always-on flag. **Decision: (a), named `in:` like reminders.** Reasons:

- It is opt-in and keeps today's documented meaning for every existing
file. Option (c) as an unconditional rule would change what existing configs
allow. In the breach example a lead could no longer post `dismiss` as a plain
message in `contained`.
- It expresses free types that are limited to some states, such as the
PoC's `progress` (planning and working only). Option (c) alone can only say
"moves now" or "always".
- Option (b) makes free types like `text` repeat in every state.
- It mirrors `session.reminders.<r>.in`, so operators learn one idea once.
(Lifecycle `from:` names channel states, not session states, so it is not the
right word.)
- The overlap with `on:` is guarded in the harmful direction: a state with
an `on:` entry for the type must be in `in:` (load error otherwise). The other
direction is allowed on purpose: a type may be a plain message in a state where
it moves nothing.

**Q1b. An opt-in default for transition types (review finding 7).** In the PoC,
17 of 19 types would repeat their `on:` states in `in:`. A session-level switch
such as `session.offer: transitions` ("a type keyed by a transition may be
posted only where it is keyed, unless it declares `in:`") would remove that
copying without changing existing files. **Decision: follow-up ticket, not this
one.** The switch is a pure derivation at load time: it fills `MessageType.In`
for types that do not declare it. So it touches only `realtimecfg` and builds on
this ticket's runtime unchanged. This ticket ships the mechanism; the follow-up
ships the shorthand. The PoC can use explicit `in:` lists until then.

**Q2. UI filter only, or also enforced?** **Decision: enforced on post, 409
`session_wrong_state`.** Reasons:

- `_actions.post` is documented as what the caller may post. A UI-only
filter would make it disagree with what the server accepts, and agents over MCP
could still post `approve` in `working`.
- It is a session rule, like `lifecycle_wrong_state`, not ACL. The
acl-ceiling rule targets subtractive denials in the ACL evaluator; `in:` is a
positive list on the channel's own state.
- The store-side check runs in the same callback chain as the session
decider, on the same channel snapshot, so the two can never disagree about the
current state. A pre-check alone, on the head the call read, would race a
concurrent move.
- Status 409, not 403: the refusal depends on the channel's state, and
reading again changes the answer. The SPA refetches.

**Q3. Where it surfaces.** **Decision:** through the existing per-channel
affordances only.

- HTTP head and list: `_actions.post` is filtered; `_actions.transitions`
keeps its meaning (still a subset).
- MCP: `read_channel` `can_post` and `actions.post`, `read_session`
`transitions` and `list_inbox` transitions, all through `postable()`.
`post_message` refusals render as `session_wrong_state: <title> (rule: ...)`
through `realtime.UserText`, so agents get the allowed states. The tool
descriptions mention it.
- CLI: `rela channel read` shows the filtered post set; `rela channel post`
prints the refusal. No new flags.
- `GET /_schema/channels`: unchanged. The descriptor is state-free by design
("the transitions out of a channel's current state come with the channel"). The
config is not secret, so this is a design choice, not concealment. A later
ticket may add `in` if a client needs it.
- A channel type without a session: unchanged; `in:` there is a load error.

**Q4. Closed/archived, stranded, final, stdio agents, triggers, reminders,
typing, versions.**

- Closed or archived: `postable()` is already empty, and `Post` answers
`channel_not_open` before the new check. Unchanged.
- Stranded: the stored state is in no `in:` list, so only types without
`in:` may be posted. This matches "posts are accepted and move nothing" for free
types, and keeps restricted types from landing in an unknown state. Declaring
the state again restores them.
- Final: a type may list a final state (for example a closing remark). If
every listed state is final and `on_final: close` is set, the load check refuses
it, because the type would be offered at most until the close runs.
- Stdio MCP and `rela channel`: same `Service.Post`. The check sits in the
store step, which is the cross-process lock for filechannels and the transaction
for SQL backends. Nothing is transport-specific.
- Triggers: the store refuses with `ErrSessionState`, and the engine skips
the post like `ErrNotOpen`. A trigger that fires in the wrong state leaves no
noise message. This includes a `when_closed: post` trigger into a closed channel
whose final state is outside the type's `in:`.
- Reminders: they post through `SetSession`'s `Remind`, which does not run
`Admit`. The load check (AC 2) guarantees that the reminder's states lie inside
the type's `in:`.
- Typing (review finding 10): left as is and documented. "Typing follows
the post roles, not the session state." It is a hint, and the SPA sends typing
only from a mounted composer, which renders nothing when no type is offered.
- Degraded config: the last good snapshot stays in force, with its `in:`.
- Mixed versions: an older rela binary refuses a `realtime.yaml` with `in:`
as an unknown key, and runs realtime degraded or off. Every process that loads
the project needs this release or later: rela-server, stdio `rela mcp`, `rela
channel` and rela-desktop. The docs and the release note say so.

### Technical Approach

1. **Raw and compiled config** (`internal/realtimecfg`).
   - `raw.go`: add `In []string \`yaml:"in"\``to`rawMessageType`.
Presence is read from the YAML node, not from the decoded value, because yaml.v3
leaves the field nil both for an absent key and for `in:`or`in: ~`, and
`keyChecker`skips null scalars. Add a helper next to`mappingKeys`
(`session.go:657`) that reports, for `channel_types.<ct>.message_types.<mt>`,
whether the `in` key exists and whether its value is null.
   - `compiled.go`: add `In []string` to `MessageType` (nil means every
state; file order kept), and `func (mt *MessageType) PostableIn(state string)
bool`, true when `mt.In == nil` or the state is listed.
   - `clone.go`: clone `In`.
   - `validate.go` `channelType`: when `raw.Session == nil`, add a problem at
`<mt key>.in` for every type whose node has an `in` key: "in names session
states; this channel type has no session".
   - `keys.go` `refusedKeys`: add
`channel_types.*.message_types.*.states` and
`channel_types.*.message_types.*.from`, each with "use in: the session states in
which the type may be posted".
   - New `message_in.go` with `func (v *validator) messageStates(key string,
raw *rawChannelType, ct *ChannelType, s *Session, rawStates
map[string]rawSessionState)`, called at the end of `session()`, after states,
transitions and reminders compile. For each raw type, sorted, whose node has an
`in` key:
     - skip the type when `ct.MessageTypes[mt]` is nil (it failed to
compile and already has a problem);
     - null value: "name the states, or drop in to allow every state";
     - empty list: "in names no state, so the type could never be posted;
drop in to allow every state";
     - per entry `in[i]`: unknown state (`unknownState`), or listed twice;
     - for each state `X` where `rawStates[X].On[mt]` exists and `X` is not
in `in`: a problem at `...states.X.on.<mt>`: "<mt> may be posted only in <list>,
so this transition could never fire; add X to message_types.<mt>.in";
     - every listed state final and `s.OnFinal() == OnFinalClose`: "the
channel closes when the session enters these states, so the type would be
offered at most until the close runs";
     - otherwise set `ct.MessageTypes[mt].In`.
   - `reminders.go` `post`: after the type is accepted, if that type has
`in:`, each reminder state outside it is a problem at `<reminder>.in[i]`: "the
reminder posts <type>, which may be posted only in <list>". (The check needs the
parsed `in:`; run it from `messageStates`, which runs after reminders compile,
using the compiled reminders.)
2. **Store: one more callback on the session step** (`internal/channels`).
   - `session.go`: add to `SessionStep` a field
`Admit func(cur Channel) error`. Doc: pure and fast, like `Decide`; called by
`Store.Append` under the lock before `Decide`; a non-nil error refuses the
append, which stores nothing. Nil means admit.
   - `DecideSession`: call `step.Admit(cur)` first when it is set, and
return its error unchanged.
   - `PrepareTimerStep` (the `SetSession` path): refuse a step with
`Admit` set, with `ErrInvalidMessage` ("a session move cannot admit a post"), as
`PrepareAppendStep` refuses `Timer` and `Remind`.
   - `channels.go`: add the sentinel `ErrSessionState` and
`type SessionStateError struct { State string; Stranded bool }` with `Error()`
and `Unwrap() error { return ErrSessionState }`.
   - `sqlchannels/sql.go:375`: add `ErrSessionState` to the sentinel list
that is returned unchanged. It is matched with `errors.Is`, so the
`*SessionStateError` value passes through. Check that the filechannels append
returns the callback's error unwrapped as well.
   - No change to `CheckAppendState`, `NewMessage` or the backends' create
branches. `DecideSession` already runs on the committed channel, including one
the append creates.
   - `channelstest`: new conformance case (AC 5).
3. **Service** (`internal/realtime`).
   - `session.go`: add `func admitIn(s *realtimecfg.Session, mt
*realtimecfg.MessageType) func(channels.Channel) error`. It returns nil when
there is no session or `mt.In == nil`. Otherwise it returns a closure: `cur :=
currentSession(s, ch)`; if `cur.stranded` or `!mt.PostableIn(cur.State)`, return
`&channels.SessionStateError{State: cur.State, Stranded: cur.stranded}`. This is
the one place that decides; both the pre-check and the store use it.
   - `newSessionPost`: set `sp.step.Admit = admitIn(s, mt)`. Trigger posts
(`system.go`) go through `newSessionPost`, so they get it with no extra code.
   - Add `func (c *call) sessionAdmits(mt) error`: run `admitIn` on the
channel as `fireable` reads it (the head, or `{CreatedAt: c.now}`).
   - `service.go` `postable()`: add `&& c.sessionAdmits(mt) == nil`.
   - `service.go` `Post`: after `requireOpen`, before the limiter, return
`sessionWrongState(c.ct, mt, err)` when `c.sessionAdmits(mt)` fails. This gives
the early answer without spending a token, and it covers ephemeral signals,
which are never appended. After `store.Append`, a `*channels.SessionStateError`
(from `errors.AsType`) becomes `sessionWrongState(c.ct, mt, err)` before
`storeError`.
   - `sessionWrongState(ct, mt, e *channels.SessionStateError)`: 409,
`TypeSessionWrongState = "session_wrong_state"`, title "<mt> may be posted only
while the session is in <in, comma-separated>; it is in <e.State>", or "...; the
session is stranded in <e.State>"; `Rule`
`channel_types.<ct>.message_types.<mt>.in`. One title for both paths, always
from the state the check saw.
   - Update the check-order comment on `Service.Post` (`service.go:359-363`)
to add "the session state (`in:`)" after "an open channel".
   - `trigger.go` `postOne`: treat `errors.Is(err, channels.ErrSessionState)`
like `ErrNotOpen`: count it as Skipped, with the debug line "trigger skipped;
the session is not in a state its message type allows".
   - Typing: unchanged (Q4).
4. **HTTP and OpenAPI** (`internal/dataentry`, `internal/openapi`).
   - `dataentry/channels_errors.go`: `realtime.TypeSessionWrongState:
http.StatusConflict`. `TestChannelErrorStatus_CoversEveryType` enforces it.
   - `openapi/channels.go`: extend `postMessageText` and the `409` text with
`session_wrong_state`.
5. **MCP** (`internal/mcp/tools_channels.go`): add one sentence to the
`read_channel` and `post_message` descriptions: "A message type may be limited
to some session states; outside them it is not in actions.post, and a post is
refused with session_wrong_state, which names the allowed states." Update the
tools-list golden.
6. **Frontend.**
   - `channelErrors.ts`: add `session_wrong_state: REFETCH` under "not in
the state the request needs". In `classifyChannelError`, return `{ ...REFETCH,
message: t('channels.sessionWrongState') }` for this type, like `rate_limited`.
The SPA meets this only with a stale view, so a generic line is enough; MCP and
CLI keep the server title with the states.
   - `i18n/locales/en.json` and `nl.json`: add `channels.sessionWrongState`
(see i18n).
   - No composer change: it already offers `_actions.post`, and
`OpenChannel.vue` keys its cache on `_actions.post`.
7. **Docs** (see Documentation Planning).

**Alternatives rejected:** derive from `on:` unconditionally (Q1); a per-state
allow list (Q1); the `session.offer` shorthand in this ticket (Q1b, follow-up);
a UI-only filter (Q2); a pre-check without a store-side check (races a
concurrent move); a separate store gate with its own `Initial`/`States` data in
`CheckAppendState` (a second copy of the session rules that could drift from
`currentSession`); a 403 (the refusal depends on state, not on who you are).

**Dependencies:** none new. Packages touched: `realtimecfg`, `channels` (+
`sqlchannels`, `channelstest`), `realtime`, `dataentry`, `openapi`, `mcp`, the
frontend channels panel and i18n.

**Files to modify:**

- `internal/realtimecfg/raw.go`, `compiled.go`, `clone.go`, `validate.go`,
`keys.go`, `session.go` (call site and node helper), new `message_in.go` and
`message_in_test.go`.
- Unchanged on purpose: `internal/realtimecfg/testdata/valid.yaml`,
`docs_example_test.go`, and the `valid.yaml` consumers (`load_test.go`,
`internal/realtime/reminders_test.go`, `timers_test.go`,
`internal/appbuild/realtime_handoff_test.go`). The breach session block in
`GUIDE-realtime.md` and `docs/realtime.md` is not edited.
- `internal/channels/session.go`, `channels.go`, `sqlchannels/sql.go`
(sentinel list), `channelstest/channelstest.go`; filechannels only if its append
wraps the callback's error.
- `internal/realtime/service.go`, `session.go`, `errors.go`, `trigger.go`;
tests in `session_affordance_test.go`, `session_test.go`, `concurrent_test.go`,
`trigger_test.go`.
- `internal/dataentry/channels_errors.go`, `channels_handler_test.go`.
- `internal/openapi/channels.go` (+ golden if present).
- `internal/mcp/tools_channels.go`, `tools_session_test.go`,
`testdata/tools_list_channels.golden.json` (and any other tools golden).
- `internal/cli/channel_test.go`, `validate_channels_test.go`.
- `frontend/src/components/entity/channels/channelErrors.ts`,
`channelErrors.test.ts`; `frontend/src/i18n/locales/en.json`, `nl.json`.
- `e2e/tests/channels.spec.ts` and its room project writer (session
fixture gets one type with `in:`).
- `docs/realtime.md`, `docs/mcp-server.md`,
`docs-project/entities/guides/GUIDE-realtime.md`,
`docs-project/entities/guides/GUIDE-mcp-server.md`.

## Security Considerations

- [x] Input sources identified (user input, config, external APIs)
- [x] Input validation approach defined (allowlist preferred over blocklist)
- [x] Security-sensitive operations identified (file access, auth, crypto)
- [x] Error handling doesn't leak sensitive information

**Input Sources & Validation:**

- `realtime.yaml` `in:` (operator config): validated at load as an
allowlist of declared state names. A null, empty, unknown, duplicate or
unreachable list refuses the whole file, as every other realtime check does. A
bare `in:` or `in: ~` is refused rather than read as "every state", so a mistake
fails toward refusing. The key checker reports a wrong shape (not a list).
- Post request `type` (user or agent): unchanged validation; the new check
only narrows what is accepted.
- Session state: read by the store under its lock, never from the client.

**Security-Sensitive Operations:**

- The session step gains one pure callback (`Admit`). It narrows; a bug fails toward
refusing a post, not toward admitting one the roles forbid. `post:` roles are
still checked first, so `in:` never widens access.
- The error names the type, the current state and the allowed states. The
caller already passed the channel read and post gates, and the head they can
read shows the session state; `realtime.yaml` is configuration, which is not
secret (CLAUDE.md). No entity data is in the message.
- A refusal by the pre-check runs before the rate limiter and costs one
head read, as a role denial does; the call reads the head anyway, so there is no
amplification. A refusal by the store after a concurrent move has spent one
token, like any failed append.
- No entity or graph read added; no new path to hidden data.

## Test Plan

- [x] Test scenarios documented for each acceptance criterion
- [x] Edge cases identified and documented
- [x] Negative test cases defined (invalid input, error conditions)
- [x] Integration test approach defined (not just unit tests)

**Test Scenarios:**

| AC | Test |
| --- | --- |
| 1 | `realtimecfg/message_in_test.go`: compile PoC-shaped session, assert `In` and `PostableIn`; no-`in` type returns true for every state. |
| 2 | Same file, one table row per load problem, asserting key and reason substring; `cli/validate_channels_test.go` prints `✗`. |
| 3 | `realtime/session_affordance_test.go`: post and transitions per role and state (plan-review, working, queued, done); `read_channel` `can_post` via `read_test.go`; `read_session` via `waiting_test.go`; inbox transitions via `inbox_test.go`. |
| 4 | `realtime/session_test.go`: 409 with the exact title, no append (LastSeq unchanged), limiter bucket unchanged on the pre-check path; `dataentry/channels_handler_test.go`: HTTP status, problem type, detail; `mcp/tools_session_test.go`; `cli/channel_test.go`. |
| 5 | `channelstest` conformance case (fs, pg, sqlite): refusing `Admit` stores nothing and returns the typed error; `SetSession` refuses a step with `Admit`. `realtime/concurrent_test.go`: approve and revise posted at once from plan-review, one moves, one 409 whose title names `working`, one notice; the loser's limiter is not asserted. |
| 6 | `realtime/trigger_test.go`: `go` with `in: [new]` into `working`, report `Skipped == 1`, no message; a `when_closed: post` trigger into a closed channel in a final state outside `in:` is skipped too. |
| 7 | Cases for stranded (reload removes the state), final with `in: [done]`, closed channel (`channel_not_open`). |
| 8 | Channel type without session: affordances equal to before; load error for `in:` there. |
| 9 | `channelErrors.test.ts` (rule exists; nl message under nl locale); e2e session scenario. |
| 10 | `docs_example_test.go` passes untouched (breach block and `valid.yaml` unchanged); a `message_in_test.go` case compiles the new docs snippet verbatim; goldens regenerated and reviewed. |

**Edge Cases:**

- Session never moved (`Session.IsZero()`): reads as `initial` in both the
pre-check and the store callback, because both call `currentSession`.
- A `one` channel not stored yet: the first post creates it; `DecideSession`
runs on the created channel, so `Admit` sees the initial state.
- Reload changes `initial`: an unset session reads as the new initial in
both places, as `currentSession` already does.
- Reload adds `in:` while a browser holds old affordances: the post gets
409, the panel refetches, the type disappears.
- Reload drops the state a channel is in (stranded): only types without
`in:` remain.
- Reply of a restricted type: same check as a post.
- Edit of a stored message whose type is now outside its states: allowed
(edits never move a session and are outside `in:`).
- Ephemeral type with `in:`: checked only by the pre-check against the head
the call read (never appended, so no store callback); documented.
- Typing while no type is offered: typing follows post roles, not states
(Q4); documented.
- A type listed in `in:` for a state where it moves nothing: allowed; it is
a plain message there.
- Concurrent posts: the store decides; see AC 5.
- Degraded config: last good `in:` stays in force.
- Unicode or odd state names: refused by the existing state-name grammar
before `in:` is checked (`unknownState`).

**Negative Tests:**

- `in:` (no value), `in: ~`, `in: []`, `in: plan-review` (scalar, shape
error), `in: [nope]`, `in: [a, a]`, `in:` on a no-session type, transition
outside `in:`, reminder state outside the type's `in:`, only-final-with-close:
each refuses the file with its key. A message type with `states:` or `from:`
gets the refused-key pointer to `in:`.
- A message type that fails to compile for another reason and also has
`in:`: one problem for that reason, no panic in `messageStates`.
- Post of `approve` in `working`: 409 `session_wrong_state`, nothing stored.
- Post of `approve` in a stranded session: 409.
- Post of `approve` in a closed channel: 409 `channel_not_open` (unchanged
order).
- Trigger post in the wrong state: skipped, not failed, not retried.
- Append whose `Admit` refuses: `*SessionStateError` (matches
`ErrSessionState`), LastSeq unchanged, no notice, no mention rows.
- `SetSession` with a step that sets `Admit`: `ErrInvalidMessage`.

**Integration:** HTTP handler tests drive `Service` over the real store;
`channelstest` runs on every backend (pg and sqlite under their tags in CI); MCP
and CLI tests go through the real service; e2e covers the SPA path.

## Risk Assessment

- [x] Technical risks assessed with mitigations
- [x] Security risks assessed (see Security Considerations)
- [x] Effort estimated (xs/s/m/l/xl)

**Effort:** m

**Risks:**

- **A new `SessionStep` callback** must reach every backend. Mitigation: it
runs inside the shared `channels.DecideSession`, so no backend code changes
beyond the SQL sentinel list; a conformance test runs on all backends.
- **Older binaries refuse the file** once `in:` is added (unknown key).
Mitigation: docs and release note say every process needs this release or later;
the failure is closed (degraded or off), not open.
- **Operators duplicate `on:` into `in:`** and may forget one. Mitigation:
the transition-outside-`in:` load error catches the harmful case; the follow-up
`session.offer: transitions` (Q1b) removes the copying.
- **Triggers silently skipped** when the operator's `in:` is too narrow.
Mitigation: debug log names the rule; `Skipped` count in the report; docs say so
next to triggers.
- **Golden drift** in MCP/OpenAPI. Mitigation: regenerate and review the
diff.
- **Behaviour change for agents**: an agent that posted a type in any state
now gets 409 once the operator adds `in:`. Opt-in only; the error text names the
allowed states and `read_session.transitions` already tells the agent what to
post.

## Documentation Planning

- [x] User-facing docs identified (skip if internal refactor)
- [x] Docs-checklist will be created when entering implementation

**Documentation Impact:**

- [x] `docs/realtime.md`: Message types table (new `in` row); Sessions:
a short paragraph "Where a message type may be posted" after "How a message
moves a session", with its own short PoC-style snippet (the breach example is
not edited; see AC 10), plus stranded, trigger and `when_closed: post` notes,
"typing follows post roles, not states", and "every process that loads the
project needs this release or later"; Checks when the file loads (new bullet,
including the null `in:` refusal); In the web app (composer row: "and the
session is in one of the type's `in:` states"); HTTP API (`_actions.post` note;
Errors table 409 `session_wrong_state`); Agents and other processes (sessions
note).
- [x] `docs/mcp-server.md`: `post_message` refusal and `read_channel`
`can_post` note.
- [x] `docs-project/entities/guides/GUIDE-realtime.md` and
`GUIDE-mcp-server.md`: mirror the above, in sync with `docs/realtime.md`.
- [x] Release note: `in:` needs every process (rela-server, stdio
`rela mcp`, `rela channel`, rela-desktop) on this release or later.
- [x] ~~docs/metamodel.md~~ (N/A: `in:` is a realtime.yaml key, not schema.yaml)
- [x] ~~docs/cli-reference.md~~ (N/A: no new command or flag; output rides on existing commands)
- [x] ~~docs/data-entry.md~~ (N/A: the composer rule lives in docs/realtime.md "In the web app")
- [x] ~~CLAUDE.md~~ (N/A: no new convention)
- [x] ~~README.md~~ (N/A: no project-level change)

**i18n:** develop routes SPA app strings through vue-i18n. One new app string,
key `channels.sessionWrongState`:

- en: "The session has moved on. This message type does not apply now;
choose another."
- nl: "De sessie is intussen verdergegaan. Dit berichttype past nu niet;
kies een ander."

The server's problem title stays English, as for every realtime refusal; MCP and
CLI show it as is, with the allowed states. No rela-components string is added:
the composer already renders nothing when no type is offered, and the session
header already says whom the session waits for.

## Design Review

- [x] Run `/design-review` before starting implementation
- [x] All critical/significant findings addressed in plan

**Design Review Findings:** 17 findings (3 significant, 6 minor, 8 nit) in
`.ignored/dr-qyflrh.tsv`; resolutions in `.ignored/dr-qyflrh-res.txt`. All three
significant findings are addressed: the store check is a `SessionStep.Admit`
callback; the store returns `*SessionStateError` with the real state; the docs
example and `valid.yaml` are left unchanged. Review responses are to be recorded
by the coordinator.

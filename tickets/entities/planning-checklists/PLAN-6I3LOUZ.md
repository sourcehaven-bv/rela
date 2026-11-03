---
id: PLAN-6I3LOUZ
type: planning-checklist
title: 'Planning: Inbox: answer waiting sessions from the inbox row'
started: "2026-11-03"
completed: "2026-11-03"
status: done
---

<!-- @managed: claude-workflow v1 -->

Plan file: `.ignored/plan-TKT-V05706X.md` (same content).

## Understanding

- [x] Problem/requirements clearly understood
- [x] Scope defined (what's in/out documented below)
- [x] Acceptance criteria documented with specific test scenarios

**Problem.** A waiting row in the inbox shows the anchor title, the entity type,
the channel and the state. It shows no question and no buttons. To answer, the
person opens the entity page, finds the channel, scrolls to the message and acts
there. In the agent runner POC every agent question and every plan approval
waits on the owner this way.

On the entity page, the channel already shows decision buttons (TKT-T9G1AE,
TKT-TXZZZ2) and a question's choices (TKT-03FQPD). Both are built in the SPA
from three inputs:

1. the channel head: channel state, session state, generation, session
transitions (edges with `by`, `guarded`, `to` and preset `button`);
2. the caller's `_actions` at that generation: `post`, `transitions`,
`entries`;
3. the held messages, from which `decisionCause` finds the message that moved
the session into its current state (the "waiting message"): the `cause` prop of
the session notice of the current generation.

The inbox has (1) in part and `transitions` of (2). It has neither `post`,
`entries`, the session edges, nor any message. The server does not store the
waiting message's seq: `channels.Session` holds state, generation and entry time
only. Finding it means reading the channel's message tail, which is a per-row
lookup on a list path (forbidden by `collection-reads.md`).

Terms used below:

- **Waiting message**: the message whose post moved the session into its
current state. Its seq is the `cause` of that move. A timer move, a session that
never moved, and a session moved before this change have none.
- **Decision inputs**: the session edges, `post`, `transitions` and `entries`
for the caller, computed in the same pass as the entry.
- **One-click decision / choice**: a `ChannelDecisionView` without
`opensForm`.
- **Form decision**: a `ChannelDecisionView` with `opensForm` (a type with a
required property such as Revise plan, a guarded or multi-target decision, and
the "Other answer" choice).

**Scope.**

In:

- Store: `channels.Session.Cause` (seq of the waiting message), written by
`ApplySession` on every move made by a post, 0 for a timer move. Stored by every
`channels.Store` implementation (file, postgres, sqlite), with a postgres
migration and a sqlite column migration.
- Store: `channels.Store.GetMessages`, a batch read of messages by
(channel key, seq), at most `MaxGetMessages` (200) refs per call.
- Every waiting entry carries its decision inputs (`actions: {post,
transitions, entries}` and the session edges), computed by the same function as
the channel head's `_actions`, and the channel's `last_seq`.
- A second, separate step, `realtime.WaitingMessages`, renders the waiting
messages of the served entries of an inbox answer (one batch read, one ref
batch). Only `GET /api/v1/_inbox` (and its live shape) runs it. The summary
(sidebar badge) does not, so the badge costs what it costs today.
- `session.cause` on the channel head (`ChannelView.Session`), so the entity
page and the drawer read the stored cause and fall back to the notice scan only
when it is 0.
- Wire types, OpenAPI, wire golden, TypeScript types.
- SPA: a waiting row shows the waiting message (the channel's message
component, folded when long), its choices and its decisions. One-click decisions
and choices post from the row through `useSessionDecisions`. A form decision or
"Other answer" opens the channel in a drawer over the inbox and starts that
decision there (OpenChannel, composer included).
- rela-components: `RlInbox` gets a per-row slot for the waiting detail;
stories for a question with choices and a plan approval.
- Docs: `docs/realtime.md` (inbox section), `docs/data-entry/api-reference.md`
(inbox entry fields), `docs/data-entry.md` (inbox page).

Out:

- MCP `list_inbox` and `rela channel inbox` are unchanged: no waiting
message and no new decision fields (`read_session` already serves the
affordances to agents). Whether agents get the waiting message is its own
decision: TKT-WVIPAKQ (backlog, depends on this ticket).
- No backfill of `Cause` for sessions that moved before this change. Their
rows show the decisions without a message until the next move.
- Mention, reply and unread rows get no buttons.
- Showing each decision button once on the entity page (FINDINGS gap 5) and
compact session display (gap 3) are separate tickets.
- No new message or session semantics: the server gates every post as today.

**Example.** The runner posts `ask` with `question: "Use Postgres or SQLite?"`
and `options: [Postgres, SQLite]` in the `run` channel of TKT-002. The session
moves to `asking` (generation 7, cause seq 41). Within one inbox grid tick, the
owner's inbox row for TKT-002 shows "Question for the owner", the question
rendered as markdown, and buttons "Postgres", "SQLite" and "Other answer". She
clicks "SQLite". The SPA posts `answer` with `body: SQLite`, `reply_to: 41`,
`expect_generation: 7`. The session moves to `working`; at the next replay the
row leaves the inbox. Had the runner moved the session first, the post is
refused with `session_moved` and the row says the question has moved on. Had
someone posted after the question, the row also says "2 newer messages" with a
button that opens the channel in the drawer. "Other answer" opens the `run`
channel in a drawer with the Answer form open as a reply to seq 41.

**Acceptance Criteria:**

1. **Cause is stored.** After a post moves a session, `Channel.Session.Cause`
equals the post's seq and equals the `cause` prop of the session notice, on
every backend. A timer move sets 0. A post that does not move the session, a
reminder post, close, reopen, rekey, tombstone and untombstone (with a resume
schedule) keep it. A row written before the migration reads 0. Test:
`channelstest.RunSessionCause` (file, postgres, sqlite);
`TestMigration_SessionCause` (pg) and the sqlite migration test.
2. **Batch message read.** `GetMessages` returns, keyed by ref, every
requested message that exists in a live channel, removed messages as their
removal stub; a ref to a missing channel or seq is absent from the answer, not
an error; more than `MaxGetMessages` refs or an invalid key is `ErrInvalidKey`;
duplicate refs are read once. A damaged message or channel row is left out and
reported through the backend's `DamageWarner`, as listings do; only an I/O or
connection error fails the call. Test: `channelstest.RunGetMessages` on all
backends, plus a damaged-segment case in `filechannels` and a damaged-row case
in `sqlchannels`.
3. **Entries carry the waiting message.** On `GET /api/v1/_inbox`, a waiting
entry whose session has a Cause carries `message` equal (JSON) to that message
in `GET .../messages` for the same caller, except `actions`, `edit_until` and
`release_after`, which the inbox leaves out. Every entry carries `last_seq`
equal to the head's `channel.last_seq`. A timer move, Cause 0 and a missing
message give no `message`. A removed waiting message is its removal stub. Test:
`TestInbox_WaitingMessage` (table: ask, plan, timer move, legacy 0, removed,
edited).
4. **Decision inputs match the head.** For every waiting entry,
`actions.post`, `actions.transitions`, `actions.entries` and
`session.transitions` equal `_actions.post`, `_actions.transitions`,
`_actions.entries` and `channel.session.transitions` of `GET` channel head for
the same caller, under the real declarative ACL, for a caller with one role, two
roles, and a role whose post set excludes a transition type. Test:
`TestInbox_DecisionInputsMatchHead` (`internal/dataentry`, real ACL fixture).
5. **Nothing hidden leaks.** Adding hidden channels, entries and messages
(rows the caller cannot read, refs to entities the caller cannot read) changes
the inbox answer byte for byte nowhere; a ref the caller cannot read renders
`restricted`. Test: extend `TestInbox_HiddenRowsChangeNothingRealACL`;
`TestInbox_WaitingMessageRefs`.
6. **Cost does not grow with rows.** The inbox at 10 and 50 waiting rows makes
the same store calls, with exactly one `GetMessages` and at most one ref resolve
batch, and zero `Get`, `Head` and `Messages`. The summary route, MCP
`list_inbox` and the CLI make zero `GetMessages` and zero ref resolves. Test:
extend `TestInbox_Budget`; `TestInboxSummaryBudget_10vs50` unchanged.
7. **The live round stays within its bound.** `BenchmarkLive_InboxRound`
(TKT-9YRRM5 AC 6) still meets 4 s on the file store (its grid) and 500 ms on
postgres, with each waiting channel seeded so its cause sits in a well-filled
segment (700 messages of about 1 KB before it, a 4 KB cause). A round still
makes one `Service.Inbox` call per principal, and at most one `WaitingMessages`
step per principal (two page tabs share it). Test: `TestLive_InboxRoundMemo`
unchanged; benchmark numbers recorded in the implementation checklist.
8. **One-click from the row.** In the SPA, a row whose session waits in
`plan-review` shows the plan, "Approve plan" and "Revise plan". Clicking Approve
posts `approve` with `expect_generation` equal to the entry's generation and
`reply_to` the waiting message when the type may reply, then refetches the
inbox. A decision that the descriptor marks `confirm` asks first. After the
post, focus moves to the next waiting row, or to the section heading when none
is left. Test: `InboxWaitingItem.test.ts`.
9. **Choices from the row.** A row whose waiting message is a question with
options shows one button per option and "Other answer". A click posts the answer
type with the option in `into`, as a reply to the question, with the generation.
Choices show only when the waiting message is top-level, not removed, has
`reply_count` 0, and the caller may post the answer type. The option list
renders once, as buttons, not also as a bulleted list (the row passes the
choices to `RlChannelMessage`, as the page does). Test:
`InboxWaitingItem.test.ts`, `channelView.test.ts` (`choiceViews` with an
explicit cause).
10. **Form decisions open the channel.** "Revise plan" and "Other answer" open
a drawer with the channel. The drawer starts that decision once the head and the
first message page have arrived and any reveal has finished, and the head's
session generation equals the entry's: the composer shows the type (for Other,
as a reply to the question). For Other, when the session generation matches but
the question's choices are not built (question not held, newer pages), it opens
the answer type as a reply to the cause directly, without the stale toast. If
the generation differs, the drawer shows the channel and the "moved on" toast,
and starts nothing. After the started decision is sent, the drawer closes and
focus goes as in AC 8. A `gone` from OpenChannel closes the drawer and refetches
the inbox. Test: `OpenChannel.start.test.ts` (messages arrive after the head;
cause older than the newest page; generation moved), `InboxView.test.ts` (close
after send, `gone`).

11a. **Newer messages are visible.** When `last_seq` exceeds `cause` by more
than one (messages after the notice), the row says "N newer messages" (N =
last_seq minus cause minus 1, at most "99+") with a button that opens the
channel in the drawer. The decisions stay on the row. Test:
`InboxWaitingItem.test.ts`.
11. **Refusals.** A `session_moved` or `session_wrong_state` refusal of a row
click shows the "moved on" toast and refetches the inbox; a 403 or 404 shows the
mapped channel error and refetches. A click while another click of the same row
is in flight does nothing. Test: `InboxWaitingItem.test.ts`.
12. **No buttons without inputs.** A row whose `actions.post` is empty (a
waiting role holder without a post role) shows the message and no buttons. A row
whose state or message type the held descriptor lacks (config reload) shows no
buttons until `channelSchema.checkNames` has reloaded it. A closed channel or a
degraded config never reaches the row from the server; the client cases are
stale-entry tests only. Test: `InboxWaitingItem.test.ts`.
13. **End to end.** In `e2e/tests/channels.spec.ts`: a runner-like principal
posts `ask` with options; the owner opens `/inbox`, sees the question, clicks an
option, and the row leaves the inbox; a second question is answered through
"Other answer" in the drawer; a plan is approved from the row. A second user who
cannot read the ticket sees an empty inbox.

## Research

- [x] For larger features: run `/research` to create a structured research doc
- [x] Searched for existing libraries that solve this problem
- [x] Checked codebase for similar patterns or reusable code
- [x] Looked for reference implementations in other projects
- [x] Reviewed relevant rela concepts for prior art

**Research Doc:** RES-U5ZE6UW (options below, recorded there).

**Existing Solutions:**

- No library: the behaviour is rela's own channel and session model.
- Decision and choice logic: `frontend/src/components/entity/channels/
channelView.ts` (`decisionViews` l.395, `decisionCause` l.463, `choiceViews`
l.509, `rowDecisions` l.562, `decisionReplyTo` l.577) and
`useSessionDecisions.ts` (lock, generation, confirm, stale handling, pending
keys, form decisions). Both are pure over a head-like value plus messages, so
they run on an inbox entry once it carries the same inputs.
- Presentation: `RlChannelDecisions.vue`, `RlChannelMessage.vue` in
rela-components; `messageView()` in `channelView.ts`.
- Affordances: `call.affordancesWith`, `call.postable`,
`call.sessionAffordances` (`internal/realtime/service.go`, `session.go`). The
inbox already calls `sessionAffordances(postable())` per entry (`inbox.go`
`item`).
- Message rendering: `renderer.message`, `rendererWith`, `renderer.refIDs`
(`internal/realtime/render.go`, `service.go`).
- Batch reads with a budget test: `TestInbox_Budget`, `Store.Reads`,
`Store.ListAddressed`; migration precedent for a new session column:
`0033_channel_session_reminders.sql` and the sqlite reminders migration.
- Drawer: `RlDrawer.vue` in rela-components.
- Prior art elsewhere: GitHub and Slack notifications show the message text and
allow a reply or approval inline; Slack's interactive messages post a button
value as a reply. Same shape as here.

**Options considered** (the RES entity holds the full comparison):

- **A. Server enriches the entry; SPA reuses the decision builders (chosen).**
Store the cause seq on the session, read the waiting messages of a page in one
batch, compute the decision inputs in the existing pass. The SPA builds a
head-like value from the entry and runs the same `decisionViews`, `choiceViews`
and `useSessionDecisions`. Form decisions go to the real OpenChannel in a
drawer. Cost: one store column and one batch read method.
- **B. SPA loads each channel.** Each row fetches the head and the newest
page. Reuses the page as is, but costs two requests per row (50 rows: 100
requests, per replay), which `collection-reads.md` forbids. Rejected.
- **C. Lazy expand.** A row shows nothing new; expanding it mounts OpenChannel
for that channel. Cheap and full reuse, but the question is not visible on the
row, which is the point of the ticket. Kept only for form decisions (drawer),
where the composer is needed anyway.
- **D. Find the cause without storing it.** The notice is at cause seq + 1,
but its own seq is not stored either; finding it needs a tail read per channel.
Rejected for the same reason as B.
- **E. Build an inline composer on the row.** Reuses `RlChannelComposer`
but needs a second copy of OpenChannel's composer glue (conversion, errors, ref
picker, compose presets). Rejected: the drawer reuses all of it.

## Approach

- [x] Technical approach chosen and documented
- [x] Approach builds on existing patterns (not reinventing)
- [x] Alternatives considered (document why rejected)
- [x] Dependencies identified (packages, APIs, types)

**Technical Approach:**

1. **Session cause (store).**
   - `channels.Session` gains `Cause int64 \`json:"cause,omitempty"\``:
the seq of the message whose post moved the session into its current state; 0
for the initial state, a timer move, or a row stored before.
   - `ApplySession(c, move, at)` becomes `ApplySession(c, move, at, cause
int64)`. The post path passes the causing message's seq; `SetSession` (timer)
passes 0. Every caller is in `internal/channels/**`; the compiler finds them.
   - filechannels: a flat `session_cause` key in `meta.yaml`, read in
`meta.channel()` and written in `meta.setSession()`. `setSession` runs on every
schedule write (posts that do not move, reminder posts, close, reopen), so both
must carry it or Cause is silently zeroed (AC 1). No migration: an absent key
reads 0.
   - pgchannels: migration `0038_channel_session_cause.sql`:
`ALTER TABLE channels ADD COLUMN IF NOT EXISTS session_cause BIGINT NOT NULL
DEFAULT 0`. sqlchannels: `chanCols`, `chanColCount`, `scanChannelWith`,
`bumpSession` and the channel insert.
   - sqlite: the same column through `internal/sqlitedb/migrate.go`, following
the reminders migration.
   - Rekey copies the whole session; tombstone/untombstone keep the row. Pinned
by AC 1. Existing tests that compare whole `Session` values after a post-driven
move (`channelstest/session.go` l.82, l.365) are updated.
2. **Batch message read (store).**
   - `channels.MessageRef{Key Key; Seq int64}`; `MaxGetMessages = 200`
(= `realtime.MaxInboxLimit`, with a compile-time assertion in realtime).
   - `Store.GetMessages(ctx, refs []MessageRef) (map[MessageRef]Message,
error)`: live channels only (tombstoned ones are absent), missing refs absent,
removed messages as stored (removal stub), duplicates folded. Validation: each
key valid, seq > 0, count ≤ max, else `ErrInvalidKey`.
   - Damaged rows: left out and reported through `DamageWarner` (AC 2).
   - sqlchannels: two queries. First the channel ids of the distinct keys
(text columns, as `reads.go` does), then the messages by `(channel_id, seq)`
with the seq parameter cast (`CAST(? AS BIGINT)`), so postgres does not infer
text. Both use existing indexes (channel key unique index, messages primary
key); an `EXPLAIN` test on sqlite like `reads_explain_test.go`.
   - filechannels: groups refs by channel, reads each channel's meta once and
each needed segment once. A segment holds up to 1000 messages and `loadMessage`
parses all of it, so AC 7 measures this with filled segments. If it misses the 4
s grid, the fallback is a seek to the one message line inside the segment
instead of a full parse (decided at implementation with the benchmark numbers,
recorded in the checklist).
   - Plimsoll: `GetMessages` raises both stores' `max-exported-methods`
directive from 34 to 35 under the documented "required interface" exception;
helpers go in package functions so `max-methods` does not move.
   - Conformance: `channelstest.RunGetMessages`, wired into the three backend
test files.
3. **Inbox entry (realtime).**
   - `InboxEntry` gains `Actions InboxActions{Post, Transitions, Entries}`,
`LastSeq int64` and `Message *MessageView`; `InboxSession` gains `Transitions
[]SessionTransitionView` (from the `sessionView` the entry already computes) and
`Cause int64`. The top-level `Transitions` (types) stays for compatibility.
   - `call` is at the plimsoll `max-methods` cap (40, `authz.go` l.35) and
`Service` at its directive (42). So: a package function `decisionAffordances(c
*call, post map[string]bool) (post, types []string, entries []EntryRef)`.
`affordancesWith` calls it for `Post`, `Transitions` and `Entries`;
`inboxRun.item` calls it instead of `sessionAffordances`. One function, so head
and inbox cannot drift (AC 4 pins it).
   - `inboxItem` records `cause`, the channel type name and `moderator`
(`c.moderator()` while `c` is set for the row). `InboxView` keeps the served
entries' `waitingRef{key, cause, ct, moderator}` in an unexported field, like
`WakeKeys`; it is never on a wire.
   - Second step, package function `WaitingMessages(ctx, s *Service, v
InboxView) (InboxView, error)`. It begins its own call (identity, snapshot, refs
resolver of this request), refuses when the call's reader differs from
`v.Reader` (`ErrIdentityMismatch`-style refusal; a programming error), skips
refs whose channel type the snapshot no longer declares, makes one
`GetMessages`, collects ref ids with `renderer.refIDs`, resolves them in one
`s.refs.Resolve` (gated, per caller), and renders each message with a new
package function `messageRenderer(ct, moderator, userType, stored, refs)` that
leaves `call` nil (no per-message `actions`); stored refs via `local.storedRefs`
per type, cached per configuration. It works only on entries step 1 already
gated for this principal, so it gates nothing new and reads nothing for any
other row. A store error fails the inbox page (not the badge, which never runs
this step).
   - `SessionView` (the head's `channel.session`) gains `Cause` (omitempty),
read from the stored session.
   - The waiting message is rendered only for entries that already passed the
row gate, the channel read role and `WaitingOnMe`; nothing new is read for any
other row.
   - `noteInbox`, `recordInbox`, wake sets: unchanged. An edit or removal of
the waiting message is a durable change in a channel of a waiting type, so the
inbox already replays on it.
4. **HTTP and wire.**
   - `serveInbox` runs step 1 through the round memo as today, then
`WaitingMessages` through a second memo entry (same principal and request key
plus `\nmessages`), so two page tabs of one principal share it.
`serveInboxSummary` is unchanged.
   - `v1.InboxEntry` gains `Actions InboxActions \`json:"actions"\``
(`post`, `transitions`, `entries`, mirroring `_actions`), `LastSeq int64
\`json:"last_seq"\``, `Message *Message \`json:"message,omitempty"\``;
`v1.InboxSession`gains`Transitions []ChannelSessionTransition
\`json:"transitions,omitempty"\``and`Cause int64 \`json:"cause,omitempty"\``.
`v1.ChannelSession`gains`Cause`. Converted with the existing `wireMessage`and
the session transition converter used by`wireChannel`.
   - OpenAPI (`internal/openapi/channels.go`), wire golden, generated
TypeScript types / `frontend/src/types/channels.ts`, `wireFields.test.ts`.
   - MCP `list_inbox` and CLI: unchanged. The MCP tool renders its own
projection of the entry, so the new realtime fields do not reach it (checked by
its existing golden).
5. **SPA: decision builders over an entry.**
   - `choiceViews(ct, head, actionsGeneration, hasNewer, messages, cause?)`:
when `cause` is given it is used instead of `decisionCause(messages, cs)`.
Existing callers pass nothing.
   - `decisionCause(messages, session)` returns `session.cause` when it is
set and the message is held and not removed, and scans for the notice only when
it is 0 (legacy). The page, the drawer and the row then read the same stored
value.
   - `DecisionChannel` gains optional `cause: Ref<number | undefined>`;
`useSessionDecisions` uses it for `decisionSeq` and passes it to `choiceViews`
when present (the row, which holds only the waiting message).
   - `useSessionDecisions` gains `onAnswer(messageType, seq)`: opens the
answer type as a reply to seq with the head generation recorded as a form
decision; it is the drawer's fallback for "Other" (AC 10).
   - New `frontend/src/components/inbox/inboxDecisionHead.ts`:
`inboxHead(entry): ChannelHead-like` (channel state, session state, generation,
transitions, `_actions.post/transitions/entries`) and `inboxMessages(entry):
Message[]` (the waiting message or empty). Pure, unit-tested.
   - Inbox-only rule for choices: the row offers choices only when the waiting
message has `reply_count === 0`. The page hides choices once an answer of the
answer type is held and ignores a removed answer; `reply_count` counts replies
of every type and still counts a removed reply. So the row is stricter, never
looser. With a reply, the row shows the message, the decisions, and "Open
channel".
6. **SPA: the row.**
   - New app component `frontend/src/components/inbox/InboxWaitingItem.vue`
per waiting entry. It renders the waiting message with `RlChannelMessage` (via
`messageView` and the channel schema store), passing `rowDecisions(...)` as
`decisions` and the choices as `choices`, exactly as the page does, so the
option list is not shown twice. A long message is folded with the existing fold
component. Without a message it renders a bare `RlChannelDecisions` with all
`decisions`.
   - Below it, "N newer messages" when `last_seq > cause + 1` (AC 11a).
   - It calls `channelSchema.checkNames(type, inboxHead(entry),
inboxMessages(entry))` when its entry changes and shows no buttons until the
descriptor is loaded (AC 12).
   - It calls `useSessionDecisions` with:
`ch` = a `DecisionChannel` adapter over the entry (`head` from `inboxHead`,
`messages` from `inboxMessages`, `hasNewer` false, `actionsGeneration` the
entry's generation, `cause` the waiting message's seq, `refreshHead` and
`refetch` both the inbox `refetch`); `post` = `postMessage(target, …)`;
`setPending` from the row's own `usePendingKeys` (pending keys such as `approve`
repeat across rows); `ui.confirm` = `useConfirm`, `ui.toast` = `ui.error`;
`focusRow` and `focusHeader` emit `focusNext`, and the inbox focuses the next
waiting row or the section heading (AC 8).
   - Shared with OpenChannel, extracted into
`frontend/src/components/entity/channels/decisionGlue.ts`: the `shownPending`
rule (adds `DECISIONS_BLOCKED_KEY` while a decision or a send runs) and the
refusal mapping `handleChannelRefusal(err, hasForm, {refetch, gone,
reloadSchema})`. OpenChannel passes its own callbacks; the row maps `refetch`
and `gone` to an inbox refetch and `schema` to `channelSchema.reload()`. One
copy, so `npm run dupes` stays clean.
   - The row calls `resetDecisions()` in `onScopeDispose`, so a refetch that
unmounts it while a confirm is open releases the lock and the click finishes
without touching anything.
   - Click routing: a view with `opensForm` emits `answer` with
`{entry, kind: 'decide' | 'choose', key, generation}`; any other view calls
`onDecide(key, 'row')` or `onChoose(key)`. The composer deps are a stub that is
never reached (select returns false), because form views never enter
`useSessionDecisions` on the row.
   - Links inside the message use `useChannelLinks` (`onLogClick`,
`entityHref`).
7. **SPA: the drawer.**
   - `InboxView.vue` holds one `answering` state and renders
`InboxAnswerDrawer.vue`: `RlDrawer` with `OpenChannel` for the entry's channel
and `revealSeq` = cause.
   - `OpenChannel` gains prop `start?: { kind: 'decide' | 'choose'; key:
string; generation: number; cause?: number; answerType?: string }` and emits
`started` and `startSent`. It runs the start once per `start` object, when the
head and the first message page have arrived, any `revealSeq` jump has finished,
and `actionsGeneration` equals the head's session generation. If that generation
differs from `start.generation` it toasts `channels.decisionStale` and starts
nothing. Otherwise it calls `onDecide(key, 'row')` or `onChoose(key)`; for a
choice whose key is not built, `onAnswer(answerType, cause)` (AC 10).
`startSent` fires when the composer's send of that started decision succeeds. A
new `start` object (another row) runs again; a channel switch already resets
decisions.
   - The drawer: `startSent` closes it and the inbox moves focus (AC 8);
`gone` closes it, drops what it held and refetches the inbox; `changed`
refetches the inbox. Closing returns focus to the row that opened it, or to the
next row if that row has left. OpenChannel marks the channel read while it is
open (`useMarkRead`), so its unread and mention rows clear behind the drawer;
that is intended and documented.
8. **rela-components.**
   - `RlInbox` waiting rows: the link stays the row's head; a named slot
`waiting-detail` (`{ row }`) renders below it, outside the link, so the buttons
and markdown links are never inside an anchor.
   - The `InboxWaitingRow` type is unchanged; the app fills the slot.
   - Stories: a question with choices, a plan approval, a row without a
message, a pending click; Messages keys in en and nl for "Open channel" and the
drawer title.

**Files to modify:**

Go:
- `internal/channels/session.go`, `store.go` (doc + interface), `channels.go`
(MessageRef, MaxGetMessages)
- `internal/channels/filechannels/ops.go`, `file.go`
- `internal/channels/sqlchannels/sql.go`
- `internal/store/pgstore/migrations/0038_channel_session_cause.sql`
- `internal/sqlitedb/migrate.go`, `sqlitedb.go`
- `internal/channels/channelstest/session.go`, new `messages.go`; backend
test files
- `internal/realtime/inbox.go`, `service.go`, `session.go`, new
`inbox_messages.go` (`WaitingMessages`, `messageRenderer`)
- plimsoll directives on `filechannels.Store` and `sqlchannels.Store`
(`max-exported-methods` 34 to 35, required interface)
- `internal/realtime/inbox_test.go` (+ new `inbox_message_test.go`)
- `internal/dataentry/inbox_handler.go`, `live_inbox.go`
(`inboxRequestKey`), new `inbox_decisions_test.go`
- `internal/apiwire/v1/channels.go`, `internal/openapi/channels.go`, wire
golden
- every other `channels.Store` fake in tests (compiler finds them)

Frontend:
- `frontend/src/types/channels.ts`, `frontend/src/types/wireFields.test.ts`
- `frontend/src/components/entity/channels/channelView.ts` (+ test)
- `frontend/src/components/entity/channels/useSessionDecisions.ts` (+ test)
- `frontend/src/components/entity/channels/OpenChannel.vue` (+ new
`OpenChannel.start.test.ts`), new `decisionGlue.ts` (+ test)
- new `frontend/src/components/inbox/inboxDecisionHead.ts` (+ test),
`InboxWaitingItem.vue` (+ test), `InboxAnswerDrawer.vue`
- `frontend/src/views/InboxView.vue` (+ test)
- `frontend/packages/rela-components/src/components/channel/RlInbox.vue`,
`RlInbox.stories.ts`, `types.ts`, messages (en, nl)
- `frontend/src/i18n/locales/en.json`, `nl.json`
- `e2e/tests/channels.spec.ts`

Docs: `docs/realtime.md`, `docs/data-entry/api-reference.md`,
`docs/data-entry.md`.

## Security Considerations

- [x] Input sources identified (user input, config, external APIs)
- [x] Input validation approach defined (allowlist preferred over blocklist)
- [x] Security-sensitive operations identified (file access, auth, crypto)
- [x] Error handling doesn't leak sensitive information

**Input Sources & Validation:**

- The inbox request: unchanged query keys (`limit`, `kinds`, `cursor`), still
validated by `CheckInboxLimit`, `CheckInboxKinds`, `decodeCursor`; the live path
keeps its allowlist. Whether the message step runs is decided by the route, not
by the client.
- `GetMessages` refs come from stored sessions only (never from a request).
They are validated anyway (key, seq > 0, count ≤ 200).
- Message content is untrusted (any poster writes it). The SPA renders it
with the channel components, which use `renderMessageMarkdown`. The inbox answer
keeps `notice: UntrustedNotice`.
- Clicks post through the existing `POST .../messages` route, which
re-authorizes the post (role, `in:`, `when:`), checks `expect_generation` under
the store lock, and validates props. The buttons are a UI hint only, like
`_actions`.

**Security-Sensitive Operations:**

- Disclosure of message content: a message is read only for an entry that
passed the anchor row gate, redaction, the channel read role and `WaitingOnMe`
for this caller; the same reader may already read it in the channel. Refs in it
resolve through the gated `RefResolver` per caller. Removal By/Reason only for a
moderator, as in the channel read. AC 3, 5.
- Existence oracle: hidden rows add nothing (AC 5); the batch read happens
after the gate and only for served entries, so its cost is not an oracle on
hidden rows either (AC 6 counts calls, not rows).
- Affordance parity: decision inputs come from the same function as the head
(AC 4). The server still gates the post.
- Stale click: `expect_generation` is the entry's generation; a moved session
refuses with `session_moved`. A choice also sends `reply_to`, so the answer
attaches to the question it was shown with.
- Nothing is cached across principals (round memo is per principal and per
round, unchanged).
- Errors: refusals pass through `writeChannelError`. A damaged message is
skipped and warned; an I/O error in `GetMessages` fails the inbox page (never
the badge), logged with ids only.

## Test Plan

- [x] Test scenarios documented for each acceptance criterion
- [x] Edge cases identified and documented
- [x] Negative test cases defined (invalid input, error conditions)
- [x] Integration test approach defined (not just unit tests)

**Test Scenarios:** see the test named in each acceptance criterion. Layers:

- Store conformance (`channelstest`) on file, postgres (`-tags postgres` with
`RELA_TEST_DATABASE_URL`, schema-pinned) and sqlite: AC 1, 2.
- realtime unit tests over memstore/file store with the declarative ACL:
AC 3, 5, 6.
- dataentry handler tests with the real ACL fixture: AC 4; live round tests:
AC 7.
- Vitest: AC 8 to 12, with `postMessage` mocked at the API module.
- Playwright e2e: AC 13.

**Edge Cases:**

- Session at generation 0 (never moved), waiting in its initial state: no
message; decisions only.
- Timer move into a waiting state: no message; decisions only.
- Waiting message removed: removal stub, no choices, decisions stay.
- Waiting message edited: the current revision; options edited away: no
choices.
- Waiting message is a reply (`reply_to` set): no choices (as on the page).
- Question already replied to (`reply_count > 0`): no choices on the row.
- Options list empty or not a list: no choices.
- Answer type not postable for the caller: no choices.
- Choice label with markdown, unicode, 100 characters: rendered as text,
posted verbatim (same as page).
- Channel closed while the row is shown: entry leaves the inbox at the next
replay; a click before that is refused and mapped.
- Two rows clicked in quick succession: separate locks, separate channels;
both posts go.
- The same row clicked twice: the lock drops the second.
- Drawer opened for row A, then row B: the drawer switches channel; the
pending start of A is dropped by `resetDecisions`.
- Drawer opened after the session moved: stale toast, nothing started.
- Cause points to a seq beyond `last_seq` or a purged message: absent from
`GetMessages`, no message.
- 200 waiting entries on a page (max limit): one `GetMessages` of 200 refs.
- Degraded configuration: inbox empty (unchanged).

**Negative Tests:**

- `GetMessages` with 201 refs, an invalid key, seq 0: `ErrInvalidKey`.
- `GetMessages` on a tombstoned channel: absent.
- Summary route, MCP `list_inbox`, CLI: no `message`, no `GetMessages` call.
- `WaitingMessages` with a view of another reader: refused.
- Row click refused with 403, 404, `session_moved`, `session_wrong_state`,
`invalid_message`: mapped message, inbox refetched, lock released.
- `start` prop with a generation the head does not have: nothing posted.

## Risk Assessment

- [x] Technical risks assessed with mitigations
- [x] Security risks assessed (see Security Considerations)
- [x] Effort estimated (xs/s/m/l/xl)

**Risks:**

- **Store interface change across three backends.** A new method and a new
column. Mitigation: conformance suites; migration tests on pg and sqlite; the
new column has a default, so old rows read 0.
- **Answer size and replay cost.** 50 entries with up to 8 KB markdown each is
about 400 KB per inbox page answer, rendered and hashed on each replay of an
open inbox page. Mitigation: messages of the served page only, and only for the
page route (never the badge); AC 7 benchmark; the SPA folds long messages. If
the benchmark misses, lower the SPA's page limit for the inbox (the API default
stays).
- **File-store segment parses.** One full segment parse per waiting channel
per page replay. Measured in AC 7 with filled segments; fallback is a
single-line seek.
- **Legacy sessions show no message.** Accepted and documented; the next move
fills Cause.
- **Drift between row and page rules.** Mitigated by reusing the builders and
the engine; the only inbox-only rule (choices need `reply_count` 0) is stricter,
never looser.
- **Front-end lint caps** (`max-lines` 500 on Vue files, bare strings). The
row logic lives in a new component, not in `InboxView.vue`.

Effort: l.

## Documentation Planning

- [x] User-facing docs identified (skip if internal refactor)
- [x] Docs-checklist will be created when entering implementation

**Documentation Impact:**

- [x] `docs/realtime.md`: the inbox entry's waiting message and decision
inputs; `Session.Cause`; legacy sessions; what MCP and CLI do not carry.
- [x] `docs/data-entry/api-reference.md`: new inbox entry fields.
- [x] `docs/data-entry.md`: answering from the inbox page; the drawer.
- [x] ~~`docs/metamodel.md`~~ (N/A: no configuration change)
- [x] ~~`CLAUDE.md`~~ (N/A: no new convention)

## Design Review

- [x] Run `/design-review` before starting implementation
- [x] All critical/significant findings addressed in plan

**Design Review Findings:** significant RR-R9F1XS9, RR-13XDZX7, RR-0OHEH7Z,
RR-1KJPR7L, RR-EHBB1UG, RR-MKLKIT0; minor RR-VZZVKMD, RR-GNLG1TQ, RR-0QLLXJG,
RR-Q096KDG, RR-P2OA2ZA, RR-E1XRC0V, RR-Y3J0L2D, RR-G7IYZRS, RR-VPYEBAB; nits
RR-HL8VH24, RR-OYNWWSI, RR-G3MILWS, RR-E3QSX8W. All addressed. No critical
findings. Significant: badge cost (split the message step off the summary),
plimsoll caps (package functions, directive bump), drawer start before messages
load (wait for page and reveal, `onAnswer` fallback, stored cause on the head),
damaged messages (skip and warn), file segment cost (filled-segment benchmark),
newer messages hidden (`last_seq`, AC 11a). Minor and nits addressed in the plan
as well.

## Open questions

1. Form decisions in a drawer (chosen) or an inline expansion of the row.
2. MCP `list_inbox` with the waiting message: TKT-WVIPAKQ (backlog).
3. Choices only when the question has no replies: stricter than the page.
4. With newer messages after the question, the row keeps its one-click
buttons and shows "N newer messages". The alternative is to hide the one-click
buttons then and send the person to the drawer.
5. Sessions that moved before the upgrade show no message until their next
move (no backfill).

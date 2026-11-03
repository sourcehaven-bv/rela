---
id: PLAN-GPDE8G
type: planning-checklist
title: 'Planning: Session decisions as one-click buttons'
started: "2026-10-28"
completed: "2026-10-28"
status: done
---

<!-- @managed: claude-workflow v1 -->

## Understanding

- [x] Problem/requirements clearly understood
- [x] Scope defined (what's in/out documented below)
- [x] Acceptance criteria documented with specific test scenarios

**Problem.** A session that waits for a person shows that person a stepper and a
composer. To approve a plan, the owner must pick the `approve` type in the
composer's segmented control and press Send. The agent-runner PoC
(`rela-mission/project/realtime.yaml`) has this in every waiting state:
`plan-review` (approve, revise), `approving` (allow, deny), `review` (accept,
rework), `blocked` (retry, abandon). The decision is the whole point of the
state, so it should be one click, next to what is being decided.

A one-click button also makes an old race easy to hit. `approving` is entered
again for every tool request. If device A allows request N and the runner at
once asks request N+1, device B's Allow (still showing request N) lands in the
new `approving` state and allows a tool call the owner never saw. No client lock
and no `in:` can stop this, because the state admits `allow` every time. So the
ticket adds one small server precondition: a post may name the session
generation it was decided against.

**Scope (in):**

- **Server: generation precondition on post.** An optional
`expect_generation` on `POST .../messages`, the MCP `post_message` tool and
`rela channel post`. The store checks it under its lock, in the existing
`SessionStep.Admit`, on every backend. A mismatch is refused with 409
`session_moved` and nothing is stored.
- **SPA: decision buttons.** Built from data the server already sends: the
head's `_actions.transitions` and `_actions.post`, the head's
`session.transitions`, `state` and `generation`, the session notices in the held
message page, and the message type schema from `GET /api/v1/_schema/channels`.
- Buttons in the session header, always. The same buttons on the message
that moved the session into its current state, when the page holds it.
- A one-click post sends empty props, `expect_generation`, and `reply_to`
the deciding message when there is one and the type may reply.
- A type with a required property, or with a guarded edge, opens the
composer with that type selected.
- A confirm dialog for a move into a final state, with danger tone for a
final state whose status is `cancelled` or `failed`.
- One decision in flight per channel, locked before any await.
- A button model with an opaque `key` and optional preset `props`, so the
follow-ups add buttons without an API break.
- Refusal handling through `classifyChannelError`, with a rule for
`session_moved`.
- New library component `RlChannelDecisions` with stories; slot and props
on `RlChannelSession`, `RlChannelMessage`, `RlChannel`; `select(type)` and a
`blocked` prop on `RlChannelComposer`.
- i18n in en and nl. Unit, component, story, Go and e2e tests. Docs.

**Scope (out):**

- Authored per-transition `confirm:`, `label:` or `style:` for a button.
Proposed follow-up: "Authored confirm and style for session decisions" (see Open
question 6).
- `session.offer: transitions` (TKT-S5OJ7S). It fills `in:` from `on:` at
load, so a decision type is postable only where it moves the session. It is the
server-side guard against a late click stored as a message that moves nothing.
This plan names it as a mitigation and recommends doing it next; it does not
depend on it.
- Guarded branching transitions per type (TKT-FPXTQJ) and message options
as reply buttons (TKT-03FQPD). The model below leaves room for both.
- Buttons in the inbox page. The inbox row links to the channel; a
decision is made with the content in view.
- Optimistic UI. The session moves when the server says so.
- Changing the PoC `realtime.yaml` (outside this repo). It should give
every decision type a `label:` and an `in:` (or `session.offer:` once TKT-S5OJ7S
lands).

**Acceptance Criteria:**

1. **Generation precondition (server).** A post with `expect_generation:
n` is stored only when the session's generation, read under the store lock, is
`n`. Otherwise it is refused with 409 `session_moved`, nothing is stored, no
rate-limit token is spent by the pre-check, and the title names the expected and
the current generation and state. Without the field, behaviour is unchanged.
`expect_generation` on a channel type without a session, or on an `ephemeral`
type, is a 400 `invalid_message`. *Test:* `internal/realtime` table test
`TestPost_ExpectGeneration` (match stores and moves; mismatch 409 with nothing
stored; absent unchanged; 0 on a never-moved session matches; no session 400;
ephemeral 400). Conformance `channelstest` case `SessionAdmitGeneration` on
every backend: an Admit returning `*channels.SessionGenerationError` comes back
unchanged, and two concurrent appends that both expect generation `g` and both
move give exactly one stored message. `internal/dataentry` handler test for the
body field and the 409 problem body.
2. **MCP, CLI and OpenAPI carry it.** `post_message` takes an optional
integer `expect_generation` (at least 0). `rela channel post` takes
`--expect-generation N`. The OpenAPI request body of each message type lists
`expect_generation`. *Test:* `internal/mcp/tools_channels_test.go` case for the
param and the refusal; golden `tools_list_channels.golden.json` updated;
`internal/cli/channel_test.go` case for the flag (set to 0 is sent; absent is
not); `internal/openapi` golden or test asserts the property.
3. **One-click decision.** In a state where the caller's
`_actions.transitions` holds a type with no required property and no guarded
edge, the header shows a button with the type's `label`. Clicking it posts that
type with `props: {}`, `expect_generation` set to the head's generation, and
`reply_to` set as in AC 5. The session moves. *Test:* `ChannelsPanel.test.ts`
"posts approve from the header button": head in `plan-review`, transitions
`["approve","revise"]`, generation 4; clicking "Approve plan" calls the POST
once with `{type: "approve", props: {}, expect_generation: 4, reply_to: 12}`;
after the follow-up head (generation 5, state `working`, transitions `[]`) no
decision button remains. e2e (AC 14).
4. **Form decision.** A type with a required property, or with a guarded
edge, shows a button that sends nothing. It selects that type in the composer,
ends a reply the type may not carry, and focuses the first field. During an edit
it first asks to discard the edit. *Test:* `ChannelsPanel.test.ts` "Revise plan
opens the composer": no POST; composer type `revise`; focus on `feedback`. "asks
before discarding an edit": cancel keeps the edit and sends nothing.
`RlChannelComposer.stories.ts` play "SelectByMethod": `select('revise')` returns
true; `select('unknown')` and `select` during an edit return false and change
nothing.
5. **Buttons on the deciding message, bound to it.** When the held page
has the session notice whose `generation` equals the head's and that notice has
a numeric `cause`, the message with that `seq` shows the same buttons, if held
and not removed. No other message shows them. A decision (from either place)
carries `reply_to: cause` when the cause is known, is itself top-level, and the
type has `reply: true`. *Test:* `channelView.test.ts` table for `decisionCause`:
found; older generation; generation 0; timer notice (no `cause`); cause not
held; cause removed; cause is a reply (no `reply_to` sent); two notices (the
current generation wins). `RlChannel.stories.ts` play "DecisionsOnMessage": only
the `plan` row has the group. `ChannelsPanel.test.ts` "the row buttons go when
the cause is removed, the header stays".
6. **Only what the caller may do now.** A decision needs: channel `open`,
session not `final` or `stranded`, the head state known to the descriptor, the
type in `_actions.transitions` and `_actions.post`, an edge of the current state
for the type, a `log` type in the descriptor, and `_actions` fetched at the
head's current generation (see Approach 9). *Test:* `channelView.test.ts` table
for `decisionViews`: owner in `plan-review` gets approve (post) and revise
(form); runner none; `channel.state: 'closed'` with stale `_actions.transitions`
none; final none; stranded none; unknown state none; guarded edge gives a form;
required boolean gives a form; `ephemeral` type dropped; type missing from the
descriptor dropped; two edges of one type with different targets gives one
button, form, no `toLabel`.
7. **No double post.** A click takes the lock before anything is awaited
(confirm dialog, `nextTick`, typing settle). While it is held, every other
decision button (header and row) is disabled, and the composer is blocked
without showing "Sending". The clicked button shows the library's pending
treatment and keeps focus. *Test:* `ChannelsPanel.test.ts` "a double click posts
once" (POST held open; clicks on header and row; one POST), "two clicks on
Abandon, confirm once, one POST", "the composer cannot send while a decision is
in flight and shows no Sending label".
8. **The lock cannot stick.** The in-flight lock is released when the POST
and the head refresh after it settle, or when either fails. Decisions show again
only when the held `_actions` were fetched at the head's current generation. A
201 that moved nothing (a reload removed the edge, the caller lost the role)
shows the still-valid decisions again once the fresh head lands. *Test:*
`ChannelsPanel.test.ts` "201 that moves nothing brings the decisions back"
(follow-up head: same generation, transitions `["revise"]`: Revise shows); "a
live frame lands before the POST answer" (generation already + 1: no flicker of
old decisions); "head refresh fails after the 201" (lock released, toast); "the
channel closes during the request" (no decisions after).
9. **Refusals.** 409 `session_moved` and 409 `session_wrong_state` show
"The session moved on. Choose again." and read the channel again. A 403 such as
`session_output_denied` follows its `CHANNEL_ERROR_RULES` entry. A 400
`invalid_message` on a one-click post opens the composer with the type and shows
the server's message there, or as a toast during an edit. A 429 or network error
is a toast. The lock is released in every case. *Test:* `ChannelsPanel.test.ts`
table "decision refusals map by problem type". `channelErrors.test.ts` passes
with `session_moved` in `CHANNEL_ERROR_RULES` (it reads
`internal/realtime/errors.go`).
10. **Confirm a move into a final state.** A decision whose every edge
leads to a final state asks first, naming that state. Danger tone when the final
state's status is `cancelled` or `failed`. Cancel sends nothing and releases the
lock. If the decision is gone when the dialog closes, nothing is sent and the
stale toast shows. *Test:* `ChannelsPanel.test.ts` "abandon asks first" (cancel:
no POST; confirm: one POST), "accept asks without danger tone", "the session
moved while the dialog was open".
11. **Accessibility and keyboard.** Native `<button>`s in a group named
"Decisions". Each has visually hidden text with its effect. A row group is
described by its message's type label and author line. The pending button is
`aria-disabled`, not `disabled`, and keeps focus. After a one-click success,
focus moves to the row the click came from (`preventScroll`), or to the session
header for a header click; the header gets a visible focus style. The log notice
stays the one announcement of the move. *Test:* `RlChannelDecisions.stories.ts`
play: group name, button names with the hint, pending button
`aria-disabled="true"` and focused, the others `disabled`.
`ChannelsPanel.test.ts` "focus returns to the row after a row decision" and "to
the header after a header decision".
12. **Narrow panel.** Below 32rem the header folds its states, but the
decision buttons stay visible on their own row and wrap. *Test:*
`RlChannelSession.stories.ts` "NarrowWithDecisions" at a 300px container:
buttons visible, steps hidden.
13. **Languages and stories.** Every new string exists in en and nl.
`RlChannelDecisions` has stories for post and form buttons, pending, blocked,
danger, many buttons wrapping, and Dutch. *Test:* `src/i18n/guards.test.ts`,
`vue-tsc` on the typed `messages.nl.ts`, and the play functions under `npm run
test` in `frontend/packages/rela-components`.
14. **End to end.** See Test Plan: the lead decides with one click and
with a form, both readers see the moves live, the buttons show on the deciding
message, and a member sees none.

## Research

- [x] ~~For larger features: run `/research` to create a structured research doc~~ (N/A: options are few and recorded under Open questions)
- [x] Searched for existing libraries that solve this problem
- [x] Checked codebase for similar patterns or reusable code
- [x] Looked for reference implementations in other projects
- [x] Reviewed relevant rela concepts for prior art

**Research Doc:** N/A (see Open questions below).

**Existing Solutions:**

- No library needed. `RlButton` (`loading`, `pendingLabel`, `tone`, and
`aria-disabled` while loading) and `RlButtonGroup` cover the buttons.
- Server reuse:
  - `channels.SessionStep.Admit` (`internal/channels/session.go:276`)
already runs under the store lock on every backend, before `Decide`, and every
backend returns an error wrapping `ErrSessionState` unchanged. `channelstest`
case `SessionAdmit` pins that. The precondition is one more check in the same
hook.
  - `admitIn` and `sessionRefusal` (`internal/realtime/session.go:340`)
and their pre-check in `Service.Post` (`service.go:389`) are the pattern for the
cheap pre-check plus the authoritative check under the lock.
- SPA reuse:
  - `composerTransitions` in `channelView.ts` (the composer marks) shares
its filter with the new `decisionViews`.
  - Message actions (`RlChannelMessage` `actionButtons`, `OpenChannel`
`onAction` with `runPending`) are the pattern for buttons on a row.
  - `OpenChannel` `onLifecycle` is the pattern for `useConfirm` with
`danger`.
  - `classifyChannelError` and `CHANNEL_ERROR_RULES`, with the special
case for `session_wrong_state`.
  - `onPost`'s `typing.settle()` and `typing.reset()`.
- Reference implementations:
  - HTTP `If-Match` and optimistic concurrency: a write names the version
it was decided against. `expect_generation` is that, for a session.
  - Slack Block Kit: buttons in a message, `style: danger`, an authored
`confirm`. The authored form is the proposed follow-up.
  - GitHub pull request review: Approve is one click, Request changes
needs a comment.
  - Microsoft Teams Adaptive Cards `Action.Submit` on the card that asks.
- rela concepts: realtime sessions, `in:` (TKT-QYFLRH), "the
configuration is not a secret" (CLAUDE.md).

## Approach

- [x] Technical approach chosen and documented
- [x] Approach builds on existing patterns (not reinventing)
- [x] Alternatives considered (document why rejected)
- [x] Dependencies identified (packages, APIs, types)

**Technical Approach:**

*Server*

1. **`internal/channels/channels.go`**: add

   ```go
   // SessionGenerationError is the refusal of a post that expected the
   // session at generation Want; under the lock it was at Got, in State.
   // It unwraps to [ErrSessionState], so every backend returns it unchanged.
   type SessionGenerationError struct {
       Want, Got int64
       State     string
   }
   ```

No store change: backends already return any `ErrSessionState` error unchanged.

2. **`internal/realtime/service.go`**: `PostInput.ExpectGeneration
*int64` (nil: no precondition). In `Post`, after the `in:` pre-check and before
the rate limit: if set and the type has no session or is `ephemeral`, return 400
`invalid_message` ("expect_generation applies only to a log message type of a
channel type with a session"). Else compare against `currentSession(s,
sessionChannel(c.head, c.now))`; on mismatch return `sessionMoved(...)`. After
`Append`, map `*channels.SessionGenerationError` to `sessionMoved` (before the
existing `SessionStateError` mapping).

3. **`internal/realtime/session.go`**: replace `admitIn(s, mt)` with
`admitPost(s, mt, expect *int64)`. It returns nil when there is no session, or
neither `in:` nor `expect` applies. Otherwise it runs the `in:` refusal first,
then compares `currentSession(s, ch).Generation` with `*expect` and returns a
`*channels.SessionGenerationError`. `newSessionPost` takes `expect`; triggers
and reminders pass nil.

4. **`internal/realtime/errors.go`**: `TypeSessionMoved = "session_moved"`.
`sessionMoved(ct, e)` gives 409 with title "The session moved on: it is at
generation 7 (approving); this post expected generation 5." No `detail` rule: it
is not a config rule.

5. **HTTP** (`internal/apiwire/v1/channels.go`): `PostMessageRequest`
gains `ExpectGeneration *int64 \`json:"expect_generation,omitempty"\``; the
handler passes it through. `internal/dataentry/testdata/
channel_wire_fields.json` updated.

6. **OpenAPI** (`internal/openapi/channels.go`, next to `reply_to` at
line 538): `expect_generation` integer, minimum 0, "The session generation this
post was decided against. A post at another generation is refused with 409
session_moved."

7. **MCP** (`internal/mcp/tools_channels.go`): `withNumber(
"expect_generation", ...)` on `post_message`, read with `wholeArg(args,
"expect_generation", 0)`. Agents answering a `permission` use it too. Golden
file updated.

8. **CLI** (`internal/cli/channel.go`): `ExpectGeneration *int64
\`name:"expect-generation"\``on`ChannelPostCmd`; a negative value is refused
like `--reply-to`.

*Library*

9. **Type** (`types.ts`):

   ```ts
   /** A button that posts a message to move the session, or opens its form. */
   export interface ChannelDecisionView {
     /** Unique in the list; what `decide` emits and the pending key uses. */
     key: string
     messageType: string
     /** Props the post carries, for a button that stands for one choice. */
     props?: Record<string, ChannelPropValue>
     label: string
     /** The target state's label; absent when the edges lead to several. */
     toLabel?: string
     /** The button opens the composer instead of posting. */
     opensForm?: boolean
     /** Ask before posting: the move ends the session. */
     confirm?: boolean
     tone?: 'default' | 'danger'
   }
   ```

This ticket sets `key = messageType` and no `props`. TKT-03FQPD and TKT-FPXTQJ
add keys like `approve:yes` with preset props, without an API break.
`channelFields.ts`: `decisionPendingKey(key)` and `DECISIONS_BLOCKED_KEY =
'decision:*'`.

10. **`RlChannelDecisions.vue`** (new). Props `decisions`, `pending?`,
`describedBy?`. Emits `decide: [key]`. Empty list: renders nothing. Otherwise a
`role="group"` named by `channelText({key: 'decisions'})`. Per decision an
`RlButton` (`size="sm"`, `variant="secondary"`, `tone`, `icon="workflow"`). The
pending one gets `loading` and `pendingLabel` from the existing `sending` text,
and never `disabled`, so it keeps focus as `aria-disabled`. The others get
`disabled` when any `decision:` key is pending. Button text is the label, then a
`rl-visually-hidden` hint from `channelDecisionHint({to, form})`. It wraps and
holds no state.

11. **`RlChannelSession.vue`**: a `decisions` slot in a full-width row
after the waiting line, also inside the narrow container query. The section gets
`tabindex="-1"`, a `:focus-visible` outline and `defineExpose({ focus })`. Doc
comment: "only a message moves a session; the `decisions` slot holds buttons
that post one".

12. **`RlChannelMessage.vue`**: prop `decisions?`; emit `decide`. Renders
`RlChannelDecisions` in the controls row before Reply, `describedBy` the ids of
the card head (type label) and the author line, both from new `useId()`s.
`hasControls` counts decisions. Never on a notice or removed message. Exposes
`focus()` (the row, `preventScroll`).

13. **`RlChannel.vue`**: props `decisions?` and `decisionSeq?`; emit
`decide: [key, from: 'row']`. It passes `decisions` only to the row with `seq
=== decisionSeq` and exposes `focusRow(seq)` using the existing `jumpTo` focus
with `preventScroll`. `pending` already flows to rows; no extra disabled prop.

14. **`RlChannelComposer.vue`**: prop `blocked?: boolean` stops submit
(button `disabled`, no spinner, no "Sending"). Expose `select(type): boolean`:
false, with no change, during an edit or when `type` is not offered; else it
selects the type, keeps its draft, waits a tick and focuses the first field.

15. **Library strings** (`useMessages.ts`, `messages.nl.ts`):
`ChannelTextKey` `decisions` ("Decisions" / "Beslissingen").
`channelDecisionHint({to, form})`: " (moves the session to ‹to›)", " (moves the
session)" without `to`, " (opens the form)"; nl " (zet de sessie op ‹to›)", "
(zet de sessie verder)", " (opent het formulier)". The wording matches
`channelSessionOptionHint` and `channelSessionHint`. The pending label reuses
`sending`.

*App*

16. **`useChannel.ts`**: hold `actionsGeneration`, the session generation
of the head whose `_actions` are held, set in `takeHead` only (not in
`setChannel`, which moves the session ahead of the actions). Add `refreshHead():
Promise<void>`, a direct head fetch that is not folded into a running follow, so
a caller can await a fresh head.

17. **`channelView.ts`** (pure, unit-tested):
    - `decisionViews(ct, head, actionsGeneration)`. Returns `[]` unless
`head.channel.state === 'open'`, the session is neither `final` nor `stranded`,
the state is in the descriptor, and `actionsGeneration === session.generation`.
Groups `session.transitions` by `message_type`. Keeps a type that is in
`_actions.transitions` and `_actions.post`, and found in `ct.message_types` as a
`log` type. `opensForm` when any edge is guarded or any property is required.
`toLabel` when all edges share one target. `confirm` when every target is final;
`tone: 'danger'` when every target is final with status `cancelled` or `failed`.
Order: the server's (by name). This function is the one place a later offer,
choice or style plugs in.
    - `decisionCause(messages, session)`: undefined for no session or
generation 0; else scans held messages from the newest for the session notice
with `props.generation === generation`, takes a numeric `props.cause`, and
returns it when that message is held and not removed. One pass over at most 500
held messages, no fetch.
    - `decisionReplyTo(messages, cause, mt)`: `cause` when the cause has no
`reply_to` and `mt.reply`; else undefined.

18. **`OpenChannel.vue`**:
    - `decisions` computed with the `Keyed` memo, key adding
`actionsGeneration` and `channel.state`. `decisionSeq` from `decisionCause`.
    - Pending: decision keys through `setPending`; while the composer
submits, `DECISIONS_BLOCKED_KEY` is held. Composer gets `:blocked="deciding"`;
`onPost` returns early while `deciding`.
    - `deciding = ref(false)`. `postTyped(type, props, opts)` is factored
out of `onPost`: `await typing.settle()`, `ch.post(...)`, `finally
typing.reset()`. Both paths use it.
    - `onDecide(key, from: 'header' | 'row')`:
      1. If `deciding` or `submitting`, return. Set `deciding = true` and
`setPending(decisionPendingKey(key), true)` **before any await**. Everything
below runs in `try { } finally { deciding = false; clear the key }`.
      2. Find the view; if gone, stale toast, return.
      3. `opensForm`: if editing, `confirm` discard (return on no), then
`stopEditing()`. Clear `replyTo` when the type has `reply: false`. `await
nextTick()`; `composer.select(type)`; false gives the stale toast. Return.
      4. `confirm`: `await confirm({...})`; return on no. Find the view
again in the current `decisions`; if gone, stale toast, return.
      5. `gen = session.generation`. `await postTyped(type, view.props ??
{}, { replyTo: decisionReplyTo(...), expectGeneration: gen })`.
      6. `await ch.refreshHead()`. Then focus: `channel.focusRow(seq)` for
a row click, `sessionRef.focus()` for a header click.
      7. On error: `invalid_message` (or any `place: 'form'` rule) outside
an edit: `composer.select(type)` and `formError`; during an edit, toast.
Otherwise `handle(err, false)`. A failed `refreshHead` after a 201 gives a toast
and releases the lock too.
    - **Why it cannot stick.** The lock lives only for one `onDecide` call
and is released in `finally`. Visibility after it uses no memory of the click:
decisions show when the held `_actions` match the head's generation, which every
head fetch re-establishes. **Why a stale click is safe.** `expect_generation` is
checked under the store lock; the client gating is UX only.

19. **`channelErrors.ts`**: `session_moved: REFETCH`, and the
`session_wrong_state` special case also covers `session_moved` with
`channels.decisionStale`.

20. **App strings** (`en.json`, `nl.json`, under `channels`):
    - `decisionConfirmBody`: "This ends the session in {state}. You cannot
undo it." / "Hiermee eindigt de sessie in {state}. Dit kun je niet ongedaan
maken."
    - `decisionStale`: "The session moved on. Choose again." / "De sessie
is verder gegaan. Kies opnieuw."
    - `decisionDiscardEditTitle`: "Discard your edit?" / "Je bewerking
weggooien?"
    - `decisionDiscardEditBody`: "This decision opens a new message. The
changes to the message you are editing are lost." / "Deze beslissing opent een
nieuw bericht. Je wijzigingen in het bericht dat je bewerkt gaan verloren."
    - `decisionDiscardEditConfirm`: "Discard" / "Weggooien".

**Alternatives considered:**

- *SPA only, no precondition.* Rejected after review: no client lock can
tell request N from request N+1 in a re-entered state, and the PoC's `approving`
re-enters on every tool call.
- *`reply_to` only, no precondition.* Kept as an addition, not the fix: it
binds the decision to its request in the log and for the agent, but a reply is
still stored and still moves the session.
- *Store the cause seq on the session record.* Rejected: a field on
`channels.Session` across four backends to save one scan of held messages.
- *Button data in `_actions`.* Rejected: it is config the descriptor
already serves; repeating it per head adds bytes to every frame.
- *A generation lock that waits for the generation to change.* Rejected:
a 201 that moves nothing never changes it, and the lock sticks.
- *Header only, or message only.* Rejected: the message is where the
reader is; the header covers timer moves, generation 0 and an unheld cause.
- *Optimistic move.* Rejected: the server decides under its lock.

**Dependencies:** no new packages. Existing: `channels.SessionStep.Admit`,
`channelstest`, `RlButton`, `RlButtonGroup`, `useConfirm`, `useChannel`,
`classifyChannelError`, `composerTransitions`, `isSessionNotice`.

**Files to modify:**

- `internal/channels/channels.go`
- `internal/channels/channelstest/channelstest.go`, `internal/channels/channelstest/session.go`
- `internal/realtime/service.go`, `session.go`, `errors.go` (+ `session_test.go` or a new `expectgen_test.go`)
- `internal/apiwire/v1/channels.go`
- `internal/dataentry` channel post handler, its test, `testdata/channel_wire_fields.json`
- `internal/openapi/channels.go` (+ golden)
- `internal/mcp/tools_channels.go`, `tools_channels_test.go`, `testdata/tools_list_channels.golden.json`
- `internal/cli/channel.go`, `channel_test.go`
- `frontend/packages/rela-components/src/components/channel/types.ts`, `channelFields.ts`
- `frontend/packages/rela-components/src/components/channel/RlChannelDecisions.vue` and `.stories.ts` (new)
- `RlChannelSession.vue`, `RlChannelMessage.vue`, `RlChannel.vue`, `RlChannelComposer.vue` and their `.stories.ts`
- `frontend/packages/rela-components/src/fixtures/channels.ts`
- `frontend/packages/rela-components/src/composables/useMessages.ts`, `messages.nl.ts`
- `frontend/src/composables/useChannel.ts` (+ test)
- `frontend/src/api` channel post call (adds `expect_generation`)
- `frontend/src/types/channels.ts` (request type)
- `frontend/src/components/entity/channels/channelView.ts`, `channelView.test.ts`
- `frontend/src/components/entity/channels/channelErrors.ts`, `channelErrors.test.ts`
- `frontend/src/components/entity/channels/OpenChannel.vue`
- `frontend/src/components/entity/ChannelsPanel.test.ts`
- `frontend/src/i18n/locales/en.json`, `nl.json`
- `e2e/tests/realtime-project.ts`, `e2e/pages/channels.page.ts`, `e2e/tests/channels.spec.ts`
- `docs/realtime.md`, `docs-project/entities/guides/GUIDE-realtime.md` (through the docs flow)

## Open questions

1. **SPA only, or a server change?** Decision: a small server change. An
optional `expect_generation` on post, checked in `Admit` under the lock, refused
with 409 `session_moved`. A sibling code, not `session_wrong_state`, because the
type is valid in the state; only the request it answers is gone. Everything else
is SPA and comes from data already sent. Labels use the message type `label:`;
order is the server's (by name).
2. **Where do buttons appear?** Decision: the header always; the deciding
message too, found as the `cause` of the held notice of the current generation.
One scan of held messages, no per-row lookup, no fetch.
3. **What does a one-click post send?** Decision: `props: {}` (or the
view's preset), `expect_generation`, and `reply_to: cause` when the cause is
known, top-level, and the type may reply.
4. **Double click and concurrency?** Decision: an in-flight lock taken
before any await and released in `finally`; decisions show only when `_actions`
match the head's generation; the server precondition is the safety net.
TKT-S5OJ7S closes the remaining case (a late click stored as a message that
moves nothing, when no `expect_generation` is sent, for example from the
composer) and is recommended next.
5. **Refusals?** Decision: through `classifyChannelError`, with
`session_moved` and `session_wrong_state` both giving the stale toast and a
refetch.
6. **Confirm, by config or not?** Decision: derived for now. Confirm when
every edge leads to a final state; danger tone for `cancelled` or `failed`. This
is a proxy for "the session ends", not for the size of the effect: `allow`,
`approve`, `rework` and `retry` get no confirm, and the form path never
confirms. The docs say exactly that. TKT-S5OJ7S does not own an override.
Proposed follow-up: **"Authored confirm and style for session decisions"**: an
optional `confirm:` and `style:` on a transition (`on.<type>`), served in the
schema descriptor, that override the derived rule for its button.
7. **Types with required props?** Decision: the button opens the composer
with the type selected; guarded edges do the same. During an edit the app asks
first.
8. **Accessibility, mobile, i18n, stories?** Decision: AC 11 to 13.
9. **MCP and CLI?** Decision: both get `expect_generation`, so an agent
answering a `permission` can bind its answer the same way. Nothing else changes
for them.
10. **Follow-ups.** TKT-FPXTQJ: several edges per type are grouped; a
guarded edge means "opens form"; `toLabel` only for a single target; a later
per-choice button is a new `key` with preset `props`. TKT-03FQPD: options become
views with `key` and `props` on the asking row, through `RlChannelDecisions`.
TKT-S5OJ7S: complements this plan (see 4); no conflict.

## Security Considerations

- [x] Input sources identified (user input, config, external APIs)
- [x] Input validation approach defined (allowlist preferred over blocklist)
- [x] Security-sensitive operations identified (file access, auth, crypto)
- [x] Error handling doesn't leak sensitive information

**Input Sources & Validation:**

- `expect_generation` (HTTP body, MCP arg, CLI flag): an integer of at
least 0; anything else is a 400 from the existing JSON and `wholeArg` checks. It
only narrows when a post is admitted; it grants nothing.
- The head and message page: per-caller server data, already row-gated
and redacted. The client uses them only to decide what to show.
- The schema descriptor: config, not a secret. Labels render by
interpolation, never `v-html`.
- Notice props `generation` and `cause`: `typeof === 'number'` checks.
- The click: the posted type and props come from the decision views, an
allowlist built from `_actions.transitions`. The server checks `post`, `in:`,
`roles`, the outputs ACL, rate limits and now the generation on every POST.

**Security-Sensitive Operations:**

- Posting a message that moves a session and may write graph outputs as
the poster. Unchanged path, plus a precondition checked under the store lock on
every backend.
- The 409 `session_moved` title names the generation and state. A caller
who reaches it may already read the channel and its session, so it discloses
nothing new. The pre-check runs after the read and post checks, so a caller who
may not post learns nothing from it.

## Test Plan

- [x] Test scenarios documented for each acceptance criterion
- [x] Edge cases identified and documented
- [x] Negative test cases defined (invalid input, error conditions)
- [x] Integration test approach defined (not just unit tests)

**Test Scenarios:**

- *Go:* `internal/realtime` (AC 1), `channelstest`
`SessionAdmitGeneration` on every backend (AC 1), `internal/dataentry` handler
(AC 1), `internal/mcp`, `internal/cli`, `internal/openapi` (AC 2).
- *Unit:* `channelView.test.ts` (`decisionViews`, `decisionCause`,
`decisionReplyTo`; AC 5, 6, 10), `useChannel.test.ts` (`actionsGeneration` set
by `takeHead`, not by `setChannel`; `refreshHead` not folded),
`channelErrors.test.ts` (AC 9).
- *Component:* `ChannelsPanel.test.ts` (AC 3, 4, 5, 7 to 11).
- *Stories with play functions:* `RlChannelDecisions`, `RlChannelSession`
"WithDecisions" and "NarrowWithDecisions", `RlChannelMessage` "WithDecisions",
`RlChannel` "DecisionsOnMessage", `RlChannelComposer` "SelectByMethod" and
"Blocked" (AC 4, 5, 11 to 13).
- *e2e:* `RoomOptions.decisions` in `realtime-project.ts`, mutually
exclusive with `session` (the function throws when both are set). It emits
`contained` (as today), `approve` (label "Approve", optional `note`, `post:
[member, lead]`, `in: [review]`) and `revise` (label "Revise", required
`feedback`, `in: [review]`), and a session `drafting` → `contained` → `review`
(`waiting_for: [lead]`) with `approve: { to: approved, roles: [lead] }`,
`revise: { to: drafting, roles: [lead] }`, `approved` final `completed`. A third
server fixture `decisionTest` writes it, like `sessionTest`. Spec "the lead
decides with one click":
  1. Lead and member open the feature; the lead posts Contained
("Ready for review"); both see Review.
  2. The lead sees Approve and Revise in the header and on the "Ready for
review" message (`expectDecisionOnMessage`). The member sees no decision in the
header or on that message.
  3. The lead clicks Revise, fills Feedback, sends; both see Drafting.
  4. The lead posts Contained again, clicks Approve on the message,
confirms the dialog; both see "Session moved from Review to Approved". Page
object methods: `expectDecision(label, shown)`, `decide(label)`,
`expectDecisionOnMessage(text, label, shown)`, `decideOnMessage(text, label)`,
`confirmDialog()`.

**Edge Cases:**

- Generation 0 or an unstored `one` channel: header only;
`expect_generation: 0` matches.
- Timer move (no `cause`), removed or unheld cause: header only.
- Cause is a reply: buttons show; no `reply_to` is sent.
- Re-entry: the notice of the current generation wins.
- Descriptor reloading, unknown type, `ephemeral` type: no decision.
- Two edges of one type with different targets: one form button, no
`toLabel`, confirm only if both are final.
- Required boolean, or a required field hidden by `data-entry.yaml`:
form (the composer's existing behaviour for hidden fields applies).
- Six or more decisions, long or non-Latin labels: the group wraps.
- Unlabelled types show their name (labels are authored); the docs tell
authors to set `label:` on decision types.
- Reply open: a one-click post sends its own `reply_to` (or none) and
leaves the composer's reply alone.
- Edit open: a one-click post leaves it; a form decision asks first; a
form refusal becomes a toast.
- A live frame lands before the POST answer: `actionsGeneration` lags, so
nothing shows until the fresh head.
- Channel switch while in flight: `useChannel` drops the late answer;
`finally` releases the lock.
- The channel closes during the request: 409 `channel_not_open`, toast,
refetch; `channel.state` hides the buttons.
- A half-written draft: `postTyped` settles and resets typing, so no
ghost "is typing" line remains.

**Negative Tests:**

- Server: mismatched `expect_generation` (409, nothing stored, audit
unchanged); on a type without session (400); on an `ephemeral` type (400);
negative value via CLI and MCP (refused); concurrent equal expectations (one
stored).
- SPA: no buttons for a caller without the role, a closed, final or
stranded session, stale `_actions`; double click and double confirm give one
POST; each refusal type maps as in AC 9; confirm cancelled sends nothing; stale
after confirm sends nothing; `composer.select` of a type not offered returns
false.

## Risk Assessment

- [x] Technical risks assessed with mitigations
- [x] Security risks assessed (see Security Considerations)
- [x] Effort estimated (xs/s/m/l/xl)

**Risks:**

- *A stale decision approves the wrong request in a re-entered state.*
Mitigation: `expect_generation` under the store lock, plus `reply_to` the
request. Residual: a post without the field (composer, old client) keeps today's
behaviour.
- *A late click stored as a message that moves nothing, for a type with
no `in:`.* Mitigation: the precondition refuses it for buttons; TKT-S5OJ7S
(`session.offer: transitions`) refuses it for every post and is recommended
next. Docs advise `in:` meanwhile.
- *An older process loads a client that sends `expect_generation`.* The
SPA is served by the same server, so it never meets an older one. An older
server would refuse the unknown field with 400 (the body decoder rejects unknown
fields), which fails loudly, not silently. Docs note that every process needs
this release, as for `in:`.
- *Two sets of the same buttons.* Mitigation: one pending set and lock;
the row group is described by its message; the header stays.
- *Focus loss when buttons unmount.* Mitigation: focus moves to the row
or the header; tested.
- *Confirm covers only moves into a final state.* Accepted for this
ticket; documented; follow-up proposed.
- *Library API churn for the follow-ups.* Mitigation: `key` and `props`
in the model now.

**Effort:** m (server part s, SPA part m).

## Documentation Planning

For enhancements: identify what documentation needs updating.

- [x] User-facing docs identified (skip if internal refactor)
- [x] Docs-checklist will be created when entering implementation

**Documentation Impact:**

- [x] ~~docs/metamodel.md - New metamodel features~~ (N/A: no schema change)
- [x] docs/cli-reference.md - New/changed commands: `rela channel post --expect-generation`, if the reference lists channel flags.
- [x] docs/data-entry.md - UI changes: covered in `docs/realtime.md` "In the web app"; check data-entry.md for a channel summary that needs one line.
- [x] ~~CLAUDE.md - New patterns or conventions~~ (N/A: no new convention)
- [x] ~~README.md - Project-level changes~~ (N/A)
- `docs/realtime.md`:
  - HTTP API: `expect_generation` in the post body; 409 `session_moved` in
the error table; the post check order in "How a message moves a session".
  - "Agents and other processes": use `expect_generation` with the
generation from `read_session` when answering a request.
  - "In the web app": a "Decision button" row in the control table; a
paragraph on where decisions show, one-click versus form, `reply_to`, the lock,
and exactly what the confirm covers (moves into a final state only).
  - "Where a message type may be posted": give decision types a
`label:` and an `in:` (or `session.offer:` once TKT-S5OJ7S lands).
  - The "only a message moves a session" wording stays; buttons post one.
  - The "every process needs this release" note for `expect_generation`.
- `docs-project/entities/guides/GUIDE-realtime.md`: the same, through the
docs flow.

## Design Review

- [x] Run `/design-review` before starting implementation: cranky-code-reviewer, 18 findings
- [x] All critical/significant findings addressed in plan: all 5 significant folded in

**Design Review Findings:** <!-- List review-response IDs, e.g., RR-xxxx -->

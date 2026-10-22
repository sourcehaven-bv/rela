---
id: PLAN-94ZZ7L
type: planning-checklist
title: 'Planning: Sessions 4/2: session UI'
started: "2026-10-22"
completed: "2026-10-22"
status: done
---

<!-- @managed: claude-workflow v1 -->

## Understanding

- [x] Problem/requirements clearly understood
- [x] Scope defined (what's in/out documented below)
- [x] Acceptance criteria documented with specific test scenarios

**Problem:** Slice 1 (TKT-Y6NJ6U, merged as cdec8db67) gave channels a session:
a flat state machine on the channel type, moved by typed messages, stored on the
channel row, with a notice per move. The web app does not show any of it. The
channel header has no state, the composer does not say which message moves the
session, and a notice renders as an unknown message type (type `session`, raw
props). The channel type descriptor (`GET /api/v1/_schema/channels`) does not
describe the session, so the app cannot draw the states. The research
(RES-O8I8S7, section (g)) fixes the minimum UI: a stepper in the channel header,
the transition message types first in the existing composer (no new button
type), and notices as system messages. This is slice 2 of 7.

Terms used below:
- **Descriptor**: the answer of `GET /api/v1/_schema/channels`. It is the
same for every principal.
- **Head**: the answer of `GET …/_channels/{type}[/{name}]`, with the
channel, the caller's `_actions` and live data.
- **Stepper**: the ordered list of a session's states in the channel header,
with the current state marked.
- **Next state**: a state that a transition out of the current state leads
to.
- **Fireable transition**: a transition out of the current state that this
caller's post would fire, provided its guard (`when:`) holds.
- **Notice**: the system message a move appends (type `session`, author
`system:realtime`, `triggered_by: session:<type>`, props `from`, `to`,
`generation`, `cause`).
- **Causing message**: the message whose append moved the session
(`MovedSession` in the store).
- **Stranded**: a stored state the configuration no longer declares
(`stranded: true` on the wire).

**Scope:**
- In: a `session` object on each channel type of the descriptor: label,
initial state, and the states in declaration order with label and `final`. No
transitions in the descriptor.
- In: `guarded` (true when the transition has a `when:`) on the head's
`channel.session.transitions`, the single source for the transitions out of the
current state.
- In: `_actions.transitions` on the head (and on the MCP `read_channel`
actions, which share the `Affordances` type): the message types whose post by
this caller moves the session out of its current state, guard aside.
- In: rela-components: `RlChannelSession` (the stepper), a notice row in
`RlChannelMessage`, transition hints and ordering in `RlChannelComposer`, a
`session` slot in `RlChannel`, new captions in `useMessages`, fixtures and
stories ported from the session parts of the mockups.
- In: the app: mapping in `channelView.ts`, wiring in `OpenChannel.vue`, a
head refetch in `useChannel` when the session moves, notices exempt from the
descriptor's unknown-type reload in `stores/channelSchema.ts`, wire types in
`types/channels.ts`.
- In: OpenAPI (descriptor `session`, `_actions.transitions`), the wire
golden, docs (`GUIDE-realtime.md`, `GUIDE-mcp-server.md`).
- Out (later slices): timers and their chips (slices 3 and 4; the stepper
gets an `aside` slot for them now), `outputs:` (5), `status:` and `waiting_for:`
(6), `count()` guards and the `session_state:` menu filter (7).
- Out: a `label` key on transitions in `realtime.yaml` (the UI names a
transition by its message type label and target state label); a
`data-entry.yaml` override for state labels; a session history view (the notices
in the log are the history).
- Out: clickable steps. A state is never a button; the composer is the only
way to move a session (research option d3 rejected).

**Example (the `breach-response` type of
`internal/realtimecfg/testdata/valid.yaml` and `docs/realtime.md`).** Descriptor
entry, abridged:

```json
{
  "name": "breach-response", "label": "Breach response",
  "session": {
    "label": "GDPR breach response",
    "initial": "detected",
    "states": [
      { "name": "detected", "label": "Detected" },
      { "name": "contained", "label": "Contained" },
      { "name": "assessed", "label": "Risk assessed" },
      { "name": "notified", "label": "AP notified", "final": true },
      { "name": "dismissed", "label": "Dismissed", "final": true }
    ]
  }
}
```

The descriptor holds the states only. The transitions out of the current state
come from one source, the head's `channel.session.transitions`, which gains
`guarded`. A head in `contained`:

```json
"session": { "state": "contained", "label": "Contained", "generation": 1,
  "entered_at": "2026-10-21T09:12:03Z",
  "transitions": [ { "message_type": "assessed", "to": "assessed", "guarded": true } ] }
```

The lead's head in `detected`: `"_actions": { "post": ["contained", "dismiss",
"escalated", "text"], …, "transitions": ["contained", "dismiss"] }`. A member's
head in `detected`: `"transitions": []` (`contained` names `roles: [lead]`;
`dismiss` is posted by `lead` only).

**Acceptance Criteria:**
1. `Describe` returns the session of a session type: label, initial, and
the states in declaration order with labels (name when unset) and `final`. It
holds no transitions. A type without a session has none. Test
`TestDescribe_Session` in `internal/realtime/describe_test.go` over the
`breach-response` example, plus a row for a type without `session:`.
2. The descriptor wire carries `session` exactly as in the example, omits it
for a type without one, names no role, permission, guard or transition, and is
the same for every principal. Tests `TestChannelSchema_Session` in
`internal/dataentry/channels_schema_test.go` (JSON compared to the example) and
the existing `TestChannelSchema_SameForEveryPrincipal` extended with the session
type.
3. `_actions.transitions` lists the fireable transitions for the caller.
Table-driven `TestAffordances_SessionTransitions` in
`internal/realtime/session_test.go`, one row each: lead in `detected` gets
`[contained, dismiss]` (the `by: trigger` entry `escalated` is left out); member
in `detected` gets `[]`; dpo in `contained` gets `[assessed]` (a guarded
transition is listed); a closed channel gives `[]`; a final state gives `[]`; a
stranded state gives `[]`; a type without a session gives `[]`; an unstored
cardinality-one channel reads as `initial` and lists the lead's `[contained,
dismiss]`. Role narrowing that survives the post check: the fixture gains a
transition with `roles: [dpo]` on a type that `member` may post, and a client
ceiling with `deny_permissions: [privacy.officer]`; for a caller holding both
roles under that ceiling the type is in `post` but not in `transitions`.
4. `_actions.transitions` is always a subset of `_actions.post` and always a
JSON array, never null. Test `TestAffordances_TransitionsSubsetOfPost` (every
principal of the fixture, every state of the example, on a channel stored in
that state), plus a row for a caller without identity: the head's JSON has
`"transitions": []`.
5. Affordance and behaviour agree. Test
`TestAffordances_TransitionsAgreeWithPost` in
`internal/realtime/session_test.go`: for every fixture principal, every state,
and every type in that principal's `_actions.post`, post a message whose props
satisfy the guard to a channel stored in that state, then assert that the
session moved exactly when the type was in `_actions.transitions`.
6. The head's cost does not grow with the session. In
`internal/dataentry/channels_budget_test.go` the session fixture
(`sessionThreads`) gains transitions whose roles come from a permission and from
an entity property, both shared with a post role. Pinned: the head's
entity-store and channel-store calls are the same for a session with 2 states
and 2 transitions and one with 10 states and 20 transitions, and each distinct
role is asked once (calls counted through the `acc` fake).
7. MCP `read_channel` carries `actions.transitions`. Test
`TestReadChannel_IncludesTransitions` in `internal/mcp/tools_channels_test.go`
(lead: `[contained, dismiss]`; member: `[]`).
8. Wire mirror: `TestChannelWireGolden` regenerated
(`channel_wire_fields.json` gains `ChannelSessionSchema`,
`ChannelSessionStateSchema`, `ChannelTypeSchema.session`,
`ChannelSessionTransition.guarded` and `ChannelActions.transitions`;
`channel_wire.json` gains the example descriptor entry and a head with a guarded
transition), and `frontend/src/types/wireFields.test.ts` passes with the
mirrored fields.
9. OpenAPI: `TestOpenAPI_ChannelSessionDescriptor` in
`internal/openapi/channels_test.go` checks the descriptor's
`channel_types[].session` schema (properties of `v1.ChannelSessionSchema` and
the state items), `guarded` on the `ChannelSession` transition items, and the
head's `_actions.transitions` (array of string, required).
10. Stepper (`RlChannelSession`), story play tests in
`RlChannelSession.stories.ts`:
    - `Linear`: five states in order; exactly one item has
`aria-current="step"` (`Risk assessed`); `AP notified` carries the visually
hidden "next" text; no item is a button or link.
    - `Branching`: in `detected`, both `Contained` and `Dismissed` are
marked next; `Contained` is marked once although two transitions lead there.
    - `Initial`: generation 0 shows the initial state as current with no
extra text (an unset session is not special in the UI).
    - `Final`: current state `AP notified` shows the check icon, the
visually hidden "end state" text, the caption "Ended", and no next marks.
    - `Stranded`: no item has `aria-current`; a warning tag "Stranded" and
the text naming the stored state are shown; no next marks.
    - `Closed`: the current state keeps `aria-current`, the caption "Paused
while the channel is closed" shows, no next marks.
    - `FinalClosed`: a final session on a closed channel shows "Ended" and
not "Paused".
    - `StrandedClosed`: shows the stranded tag and text and not "Paused".
    - `Narrow` (container 320px): only the current state and a "Show all
states" button with `aria-expanded="false"` are visible; clicking it sets
`aria-expanded="true"` and shows the full list.
    - `NarrowStranded` (container 320px): the summary reads "Stranded ·
‹stored name›".
    - Storybook a11y addon reports no violation on every story.
11. Notice row, story play tests in `RlChannelMessage.stories.ts`:
    - `SessionNotice`: renders "Session moved from Detected to Contained"
and the time; no reply, edit, delete, remove or jump control, even when
`canReply` is set.
    - `SessionNoticeReentry`: `from` equals `to` renders "Session entered
Contained again".
    - `SessionNoticeRemovedState`: a state the descriptor no longer has
renders by its name.
12. Composer, story play tests in `RlChannelComposer.stories.ts`:
    - `TransitionTypesFirst`: with `postable` `[contained, dismiss, text]`
and transitions `[contained, dismiss]`, the type options read `Contained`,
`Dismiss`, `Text` in that order; each transition option has the `workflow` icon
and the visually hidden "moves the session"; the initial selection is `Text`.
    - `TransitionHint`: selecting `Contained` shows "Sending this moves the
session to Contained." in the foot, referenced by the submit button's
`aria-describedby`; selecting `Text` removes it.
    - `GuardedTransitionHint`: a guarded transition reads "Sending this
moves the session to Risk assessed if its condition holds."
    - `OnlyTransitionTypes`: with every offered type a transition, the
first one is selected.
    - `OneTransitionType`: with a single offered type that is a transition
(no type picker rendered), the hint shows and the submit button references it
with `aria-describedby`.
    - `ReorderKeepsFocus`: with focus on the `Text` option, `transitions`
changes so the options reorder; the selected value is still `Text` and
`document.activeElement` is still the `Text` option.
    - `EditHasNoHint`: in edit mode of a `contained` message, no hint shows
(an edit never moves a session).
13. `RlChannel` renders the `session` slot between the header and the log,
also when the channel is closed or archived. Story `SessionRoom` in
`RlChannel.stories.ts` (the breach room from the mockup fixtures): the stepper,
a notice in the log, and the composer with transition types first; a play test
checks the slot order in the DOM.
14. Live announcement, play tests in `RlChannel.stories.ts`:
    - `AnnouncesNotice`: appending a lone notice puts "Session moved from
Detected to Contained." in the hidden polite region.
    - `AnnouncesMessageAndNotice`: appending a message and its notice in
one update puts "1 new message from Ann. Session moved from Detected to
Contained." in the region.
15. App mapping, `frontend/src/components/entity/channels/channelView.test.ts`:
    - `sessionView` builds the stepper view from the descriptor states, the
head session, the channel state and `_actions.transitions`: current index, the
next set (from the head's `session.transitions[].to`), and the caption by
precedence (stranded, then final, then paused).
    - A head state missing from a stale descriptor (and not `stranded`)
maps to the stranded look and does not throw.
    - `noticeView` maps a message for which `isSessionNotice` holds to the
notice view with labels; a message of type `session` without the `session:`
marker is not a notice.
    - `composerTransitions` keeps the types in `_actions.transitions` ∩
`head.channel.session.transitions[].message_type` ∩ `_actions.post`, with `to`
and `guarded` from the head transition and the target label from the descriptor.
    - Moved ahead: a head whose `channel.session` already moved to
`contained` while `_actions.transitions` still lists `[contained, dismiss]`
gives no composer transitions (neither type leaves `contained`).
16. Live refresh, `frontend/src/composables/useChannel.test.ts`:
    - A frame whose page brings a notice and a channel with a higher
`session.generation` updates `head.channel.session` and refetches the head once,
so `_actions.transitions` follows.
    - A frame whose page brings a plain message and an unchanged session
does not refetch the head (existing behaviour kept).
17. Descriptor reload, `frontend/src/stores/channelSchema.test.ts`:
    - A page holding notices does not trigger a descriptor reload.
    - A head whose `session.state`, or a `session.transitions[].to`, is
missing from the held descriptor's states (not stranded) triggers one reload,
and only once.
    - A plain message of type `session` on a channel type without a session
(where that name is not reserved) and unknown to the descriptor still triggers
the unknown-type reload.
18. Panel, `frontend/src/components/entity/ChannelsPanel.test.ts`:
"shows the session stepper, notices and transition types first on a session
channel"; "shows no stepper on a channel type without a session"; "offers no
edit or delete on the causing message" (a message whose `_actions` lacks them,
as the server sends it).
19. End to end, `e2e/tests/channels.spec.ts` with a session type added to
`e2e/tests/realtime-project.ts`: the lead sees `Detected` current and posts the
transition type; the stepper moves to the next state without a reload, a notice
appears, and the lead's message offers no Edit; the member, in another context,
sees the stepper move live and no transition types in the composer.
20. Docs: `just docs` regenerates `docs/realtime.md` and
`docs/mcp-server.md` without diff after the guide edits.

## Research

- [x] For larger features: run `/research` to create a structured research doc
- [x] ~~Searched for existing libraries that solve this problem~~ (N/A: a stepper is a styled ordered list; the library has every primitive, and the research doc covers the design)
- [x] Checked codebase for similar patterns or reusable code
- [x] Looked for reference implementations in other projects
- [x] Reviewed relevant rela concepts for prior art

**Research Doc:** RES-O8I8S7 (`.ignored/res-sessions-final.md`), section (g) "UI
surface, minimum", slice 2 in (h), and the decisions of 2026-10-21 (flat states;
the session runs only while the channel is open; sessions allowed on
`cardinality: many`).

**Existing Solutions:**
- `realtime.Describe` and `serveChannelSchema`: the descriptor. Its rule
"carries no gate, role or condition; the affordances on each channel say what a
caller may do" decides the split between descriptor and `_actions`.
- `call.affordancesWith` and `call.transitionRoles` (slice 1): the per-caller
role checks with the client ceiling. `fireable` and `transitionRoles` will share
one memoized helper over `c.can(role, CapabilityUpdate)`, so each distinct role
is asked once and the two cannot drift. The parity pattern of
`TestDetailAction_AffordanceAndGateAgree` (`internal/dataentry/CLAUDE.md`) is
the model for AC 5.
- `messageActions` already offers nothing on a notice and no edit or delete
on a causing message (`editCheck` and `deleteOwn` return `sessionMessage()`),
and `channelErrors.ts` already maps `session_notice` and `session_message` to a
refetch. The UI only has to keep following `_actions`.
- `useChannel.follow` already copies `page.channel` into the head
(`setChannel`), so the stepper's state follows every frame; only the per-caller
list needs the head refetch.
- `RlSegmentedControl` (radio group with icons), `RlTag`, `RlIcon`
(`workflow`, `check`, `warning`), the `rl-visually-hidden` utility, and
container queries as used in `components/layout`.
- `useMessages` / `provideMessages`: fixed captions by key, so a localised
app overrides one table. Operator labels come from `realtime.yaml`, with the
name as fallback, like schema and data-entry labels.
- Mockups (`.ignored/mockups/RlRealtimeSession.vue`, `realtime.ts`
`warRoom.session`, `Realtime.stories.ts`): the visual starting point for the
stepper and the story fixtures. Their "done" check marks by index and timer
chips are not carried over (see Approach, decision 4).
- Prior art: the WAI-ARIA `aria-current="step"` pattern for step indicators
(GOV.UK and USWDS step indicators), and the GitHub/Jira habit of showing a
workflow's available transitions next to the current status rather than the
whole graph.

## Approach

- [x] Technical approach chosen and documented
- [x] Approach builds on existing patterns (not reinventing)
- [x] Alternatives considered (document why rejected)
- [x] Dependencies identified (packages, APIs, types)

**Technical Approach:**

1. **Descriptor (`internal/realtime/describe.go`).** `ChannelTypeView`
gains `Session *SessionDescription`:

   ```go
   type SessionDescription struct {
       Label, Initial string
       States []SessionStateDescription // declaration order
   }
   type SessionStateDescription struct {
       Name, Label string
       Final       bool
   }
   ```

Built from `ct.Session().States()`; nil without a session. Labels are already
defaulted by `realtimecfg` (session label to the channel type label, state label
to the name), so the "never empty" rule of the descriptor holds. The descriptor
carries no transitions, roles or guard source. This is not concealment (the
configuration is not a secret): the descriptor's contract is that it carries no
gate, role or condition, and the transitions out of the current state already
come with the head. Keeping one source for them means a stale descriptor after a
reload cannot disagree with the head about where the session can go.

Wire (`internal/apiwire/v1/channels.go`): `ChannelTypeSchema.Session
*ChannelSessionSchema` (`json:"session,omitempty"`),
`ChannelSessionSchema{Label, Initial string; States
[]ChannelSessionStateSchema}`, `ChannelSessionStateSchema{Name, Label string;
Final bool (omitempty)}`. Mapping in `dataentry/channels_schema.go`
(`wireSessionSchema`). Later slices add per-state `after`, `status`,
`waiting_for` and a session-level `timers` list to these objects without
changing existing fields.

2. **Head transitions gain `guarded`.** `SessionTransitionView.Guarded bool`
(`json:"guarded,omitempty"`, set when `t.When != nil`) in
`internal/realtime/session.go` `sessionView`;
`v1.ChannelSessionTransition.Guarded` mapped in `dataentry/channels_wire.go`
`wireChannelSession`; the TS type and
`CHANNEL_WIRE_FIELDS.ChannelSessionTransition` in `types/channels.ts`. `guarded`
is the same for every reader; the composer needs it to word its hint honestly.
Every UI piece reads one source:

   | UI piece | Reads |
   | --- | --- |
   | Stepper states, order, labels, `final`, initial | descriptor `session` |
   | Current state, label, `final`, `stranded`, next marks (`to`) | head `channel.session` |
   | Paused | head `channel.state` |
   | Composer transitions (`message_type`, `to`, `guarded`) | head `channel.session.transitions` ∩ `_actions.transitions` ∩ `_actions.post` |
   | Target state label in hints and notices | descriptor states (name as fallback) |

3. **Per-caller affordance (`internal/realtime/session.go`,
`service.go`).** `Affordances.Transitions []string` (`json:"transitions"`,
sorted). `call.fireable(post map[string]bool) []string` returns the message
types `mt` of the current state's transitions where: the type has a session, the
channel is open, the current state is declared and not final, `post[mt]` holds,
and `poster{roles: …}.admits(t)` holds for a person or agent (so `by: trigger`
entries are never listed, `roles` entries need a held role, entries with neither
need only the post right). The role answers are asked once per distinct role
with `c.can(role, CapabilityUpdate)`, so the client ceiling applies. To keep the
affordance and the decider from drifting, `transitionRoles` (the post path) and
`fireable` share one helper, `c.heldRoles(names)`, which memoizes per call; AC 5
checks the two agree by posting. The current state comes from `currentSession(s,
head)` with the call's clock for an unstored channel, the same reading as
`sessionView`. `Head` and `Read` (MCP) share it. The guard is not evaluated: it
reads the message, which does not exist yet; the head's `guarded` tells the UI
so. Like every `_actions` entry it is a UI hint; the post is judged again under
the store lock.

Always `[]`, never null, on every return path: `affordancesWith` initializes
`Transitions: []string{}` next to `Post` and `Lifecycle` before its early return
for a caller without identity (`internal/realtime/service.go`), and `wireHead`
(`internal/dataentry/channels_wire.go`) nil-guards it as it does `Post` and
`Lifecycle`. AC 4 has a no-identity row.

Wire: `ChannelActions.Transitions []string` (`json:"transitions"`), mapped in
`wireHead`. Doc comment: "the message types whose post by you would move the
session out of its current state, if the transition's guard holds; a subset of
post; empty, never null".

Cost: one role check per distinct transition role of the current state, on a
single-channel read (head, MCP read), memoized with the post checks. Not on the
list path, so `collection-reads.md` is unaffected; AC 6 pins that the cost does
not grow with states and transitions and that each role is asked once.

4. **OpenAPI (`internal/openapi/channels.go`).** In
`addChannelDescriptorSchema`: `session` on `channelType` (object with `label`,
`initial`, `states` items `{name, label, final}`), with a description that the
states come in declaration order and that the transitions out of the current
state are on the channel's `session`. In `channelSessionSchema`: `guarded`
(boolean) on the transition items. In `addHeadSchemas`: `_actions.transitions`
(array of string, required), description as in decision 3.

5. **Stepper (`rela-components/src/components/channel/RlChannelSession.vue`).**
Props (a presentational view the app builds):

   ```ts
   interface ChannelSessionView {
     label: string                 // session label
     states: { name: string; label: string; final?: boolean }[] // declaration order
     current?: string              // absent when stranded
     currentLabel: string          // the stored name when stranded
     next: string[]                // state names a transition leads to
     final?: boolean
     stranded?: boolean
     paused?: 'closed' | 'archived' // channel not open
   }
   ```

Markup: a `<section aria-labelledby>` with a kicker "Session" and the label as
its heading text (not a heading element, to keep the page outline: the channel
title is the heading), then an `<ol>` of states. The current item has
`aria-current="step"`. Each non-current item that is a next state carries a
dotted outline and the visually hidden text "next"; a final state carries "end
state". Items are plain text; nothing is focusable except the narrow-layout
toggle.

Flow rules:
- **Non-linear flows.** Order is declaration order, nothing more. The
stepper does not mark earlier states as done: the server keeps only the current
state, and in a branching flow (`detected` to `dismissed`) an index-based check
mark would claim steps that never happened. The next marks show where the
session can go from here, which covers branches. The mockup's check marks are
dropped for this reason.
- **Re-entry.** The stepper does not change; the notice says "entered
… again".
- **Unset (generation 0).** The head already reads it as `initial`; the
stepper shows it as current with no special text.
- **Many states.** Up to 64 states wrap across lines (`flex-wrap`).

Status precedence: exactly one status applies, in this order: **stranded > final
> paused > normal**. (Stranded and final cannot both hold, since a stranded
> state is undeclared; the order still fixes the rendering.) Next marks show only
> in normal.
- **Stranded.** No item is current. A warning `RlTag` "Stranded" and the
text "‹name› is no longer a state of this session. Messages do not move it."
show after the list. The list still shows the declared states so readers see the
configured flow. No "Paused" caption, even on a closed channel.
- **Final.** The current item shows the `check` icon and the caption
"Ended" next to the label. No "Paused" caption on a closed channel: a session
that ended is not paused.
- **Paused (closed or archived channel).** The current state keeps
`aria-current`; the caption "Paused while the channel is closed" (or "archived")
shows, because the session runs only while the channel is open (decision 2 of
2026-10-21).
- **Normal.** Current and next marks.

Layout and mobile: a container query on the stepper. Below 32rem of container
width it collapses to one summary line plus a "Show all states" button
(`aria-expanded`, `aria-controls` on the list); the expanded list is vertical.
The summary follows the same precedence: "Stranded · ‹stored name›", "Ended ·
‹label›", "‹label› · Paused", or "Session · ‹label›". A container query, not a
viewport query, because the channel panel is also narrow in side panels on
desktop. Colour is never the only signal: current has weight, background and
`aria-current`; next has an outline and hidden text; final has an icon. Tokens
only (`check:theme`, `check:contrast` gate it); no animation, so `check:motion`
stays clean.

Live updates: the stepper is not a live region. The notice that comes with every
move is announced by `RlChannel`'s polite region (decision 7), so a move is read
out once, not twice.

Room for timers: an `aside` slot after the list (empty now). Slices 3 and 4 put
timer chips there, as in the mockup's `rt-session__timers`.

6. **Composer (`RlChannelComposer.vue`).** New prop `transitions?:
ChannelComposerTransition[]` with `{ messageType: string; toLabel: string;
guarded?: boolean }`. The app builds it from `_actions.transitions` ∩
`head.channel.session.transitions[].message_type` ∩ `_actions.post`, taking `to`
and `guarded` from the head transition and the label from the descriptor. The
head session refreshes with every frame (`setChannel`), so between a frame and
the head refetch the composer never claims a transition out of a state the
session already left.
   - Order: offered types whose name is in `transitions` come first, in the
order of `transitions` (sorted by message type, as the server sends it), then
the other offered types in their existing order.
   - Marking: a transition option gets the `workflow` icon and a visually
hidden " (moves the session)". `SegmentOption` gains an optional `srHint`
rendered in a `rl-visually-hidden` span inside the option. With a single offered
type no picker renders; the foot hint is then the only marker, and the submit
button references it.
   - Default selection: the first offered type that is not a transition,
when there is one; otherwise the first. Listing transition types first makes
them easy to find, but a plain chat message must not move the session by
default.
   - Live reorder: when `transitions` changes (another user moved the
session), the selected value stays on the same type, and when focus is in the
picker it stays on that type's option. The options are keyed by message type, so
Vue moves the focused element instead of recreating it. Only when the selected
type is no longer offered does the existing fallback pick another.
   - Hint: while a transition type is selected, the foot shows "Sending
this moves the session to ‹label›." or, for a guarded one, "… to ‹label› if its
condition holds.". The submit button references it with `aria-describedby`.
   - Replies: a reply of a transition type moves the session (slice 1), so
the hint shows in reply mode too. Edit mode shows no hint: an edit never moves a
session.
   - With no `transitions` the composer behaves exactly as today.

7. **Notices (`RlChannelMessage.vue`, `RlChannel.vue`, `types.ts`).**
`ChannelMessageView` gains `notice?: { fromLabel: string; toLabel: string;
reentry: boolean }`. A message with `notice` renders as one compact system line
instead of a card: the `workflow` icon, "Session moved from ‹from› to ‹to›" (or
"Session entered ‹to› again") and the `<time>`. There is no link to the causing
message: both are written in one append, so the causing message is always the
row directly above. The row shows no controls: the server sends no `_actions` on
a notice, and the row does not render reply either.

Announcement: `RlChannel.announce` splits the added messages into notices and
others. It reports the others as today (count, and type and author for a single
one), then appends the text of each added notice. A message and its notice
arriving in one frame read "1 new message from Ann. Session moved from Detected
to Contained."; a lone notice reads "Session moved from Detected to Contained.".

Edit and delete on notices and causing messages: the server already leaves them
out of the message's `_actions`, and the components render only what `_actions`
names. A stale UI that still tries gets 409 `session_notice` or
`session_message`, which `channelErrors.ts` already maps to a toast and a
refetch. No client-side rule is added, so the client cannot drift from the
server.

8. **`RlChannel.vue`.** A `session` slot rendered between the header and
the log, in every channel state (a closed channel still shows where its session
stopped). The app mounts `RlChannelSession` there. The library stays free of
wire types.

9. **Captions (`useMessages.ts`).** New `ChannelTextKey`s: `session`,
`sessionNext`, `sessionEndState`, `sessionEnded`, `sessionStranded`,
`sessionPausedClosed`, `sessionPausedArchived`, `sessionShowAll`,
`sessionHideAll`, `sessionMovesSession`. New functions:
`channelSessionMoved({from, to})`, `channelSessionReentered({state})`,
`channelSessionStrandedText({state})`, `channelSessionHint({to, guarded})`, and
`channelNewMessages` gains an optional `notices: string[]` it appends. English
defaults; a localised app overrides them through `provideMessages`. Operator
labels (session, state, message type) are shown as the descriptor sends them,
like every other descriptor and schema label: `realtime.yaml` is written in the
project's language, and rela has no per-locale label tables.

10. **App wiring.**
    - `types/channels.ts`: `ChannelSessionSchema`,
`ChannelSessionStateSchema`, `ChannelTypeSchema.session?`,
`ChannelSessionTransition.guarded?`, `ChannelActions.transitions`, and their
keys in `CHANNEL_WIRE_FIELDS`. A helper `isSessionNotice(m)` (type `session` and
`triggered_by` starting with `session:`), mirroring `realtime.isNotice`.
    - `channelView.ts`: pure `sessionView(ct, channel, actions)`,
`noticeView(m, ct)` (only when `isSessionNotice(m)`; labels from the
descriptor's states, the name when a state is gone, `reentry` when `from ===
to`), and `composerTransitions(ct, head)` per decision 6. `messageView` sets
`notice` through `noticeView`.
    - `OpenChannel.vue`: mounts `RlChannelSession` in the `session` slot
when `channelType.session` and `head.channel.session` are both set, and passes
`composerTransitions` to the composer.
    - `useChannel.ts` `follow`: `sessionMoved` (the page's
`channel.session?.generation` or `?.state` differs from the held head's) is
computed before `setChannel`, as `stateMoved` is, and joins the condition that
refetches the head, so `_actions.transitions` follows a move made by someone
else. A move by this user's own post arrives the same way, on the frame of the
notice's rev.
    - `stores/channelSchema.ts` `checkNames`: skip a message only when
`isSessionNotice(m)` holds (a type without a session may declare a message type
named `session`, which stays subject to the check); add the head's
`session.state` and each `session.transitions[].to`, when not `stranded`, to the
known-name checks against the descriptor's states, so a reloaded `realtime.yaml`
with new states reloads the descriptor once. `_actions.transitions` is not
checked separately: it is a subset of `post`, which is already checked.

11. **Stories and fixtures (`rela-components/src/fixtures/channels.ts`).**
Port the breach room from the mockup's `warRoom`: the five-state session (with
`dismissed` added as a second final state to show a branch), message types
`contained`, `assessed`, `notified`, `dismiss`, `text`, and messages including
two notices. Stories: `RlChannelSession.stories.ts` (AC 10), notice stories in
`RlChannelMessage.stories.ts` (AC 11), composer stories (AC 12), and
`SessionRoom`, `AnnouncesNotice` and `AnnouncesMessageAndNotice` in
`RlChannel.stories.ts` (AC 13, 14). The mockup's timer chips are not ported; the
`aside` slot is shown empty.

**Alternatives rejected:**
- Compute fireable transitions in the browser from the descriptor and
`_actions.post`. The browser cannot know whether the caller holds a transition's
role, and the descriptor deliberately has no roles. It would show a member a
transition that does nothing.
- Put roles in the descriptor. Breaks the descriptor's documented
contract, and the browser still could not evaluate a role (roles come from ACL
and entity properties).
- Evaluate guards for the affordance. A guard reads the message, which
does not exist before the post.
- Mark earlier states as done by declaration index (the mockup). Wrong in
any branching flow. Deriving visited states from notices in the held log is
incomplete under paging and retention.
- Show the transition graph (arrows between states). Heavy on a narrow
panel and poor for screen readers; next marks and the composer hint say what
matters now.
- Select the first transition type by default. A chat message would move
the session by accident.
- Separate transition buttons. Rejected in the research (d3): a transition
without a message has no payload and no record.
- Re-read the head on every frame. Doubles the requests for every message
in a busy channel; refetching only when the session moved is enough.
- A `label` key on transitions. Not needed for the minimum UI; it can be
added later without breaking the wire.

**Files to modify:**
- `internal/realtime`: `describe.go` (+ `describe_test.go`), `session.go`
(`fireable`), `service.go` (`Affordances.Transitions`, `affordancesWith`) (+
`session_test.go`).
- `internal/apiwire/v1/channels.go`.
- `internal/dataentry`: `channels_schema.go`, `channels_wire.go`
(+ `channels_schema_test.go`, `channels_budget_test.go`,
`channels_wire_golden_test.go`, `testdata/channel_wire*.json`).
- `internal/openapi/channels.go` (+ `channels_test.go`).
- `internal/mcp/tools_channels.go` (description of `read_channel`
actions) (+ `tools_channels_test.go`, golden tool list if the description
changes).
- `frontend/packages/rela-components/src`: new
`components/channel/RlChannelSession.vue` and `RlChannelSession.stories.ts`;
`RlChannel.vue`, `RlChannelMessage.vue`, `RlChannelComposer.vue`, `types.ts`,
their stories; `components/data/RlSegmentedControl.vue` (`srHint`);
`composables/useMessages.ts`; `fixtures/channels.ts`.
- `frontend/src`: `types/channels.ts`,
`components/entity/channels/channelView.ts` (+ test),
`components/entity/channels/OpenChannel.vue`, `composables/useChannel.ts` (+
test), `stores/channelSchema.ts` (+ test),
`components/entity/ChannelsPanel.test.ts`, `types/wireFields.test.ts` (fixture
only if needed).
- `e2e/tests/realtime-project.ts`, `e2e/tests/channels.spec.ts`.
- `docs-project/entities/guides/GUIDE-realtime.md`,
`docs-project/entities/guides/GUIDE-mcp-server.md`; `docs/realtime.md` and
`docs/mcp-server.md` regenerated by `just docs`.

## Security Considerations

- [x] Input sources identified (user input, config, external APIs)
- [x] Input validation approach defined (allowlist preferred over blocklist)
- [x] Security-sensitive operations identified (file access, auth, crypto)
- [x] Error handling doesn't leak sensitive information

**Input Sources & Validation:**
- `realtime.yaml` (operator config, not secret): state names, labels and
transitions, validated at load by slice 1. The descriptor serves them to every
principal; that is the settled rule for config surfaces.
- Notice props (`from`, `to`, `generation`, `cause`): written only by the
store's session step; the `session:` marker cannot be forged by a poster (slice
1). The app treats them as untrusted anyway: labels are looked up by name in the
descriptor, unknown names render as text, `cause` is used only when it is a
positive integer.
- All labels and names render through interpolation. No `v-html` is added;
the library's `check:purity` gate covers it.

**Security-Sensitive Operations:**
- No new write path. Moving a session stays a post, authorized under the
store lock as in slice 1. `_actions.transitions` is a UI hint; a stale or forged
client list changes nothing on the server.
- `_actions.transitions` tells the caller whether they hold a transition
role. That is the caller's own membership, the same one bit `post` and the
lifecycle affordances already show. It is computed per caller and never cached
across principals.
- The role checks use `CapabilityUpdate`, so the client ceiling applies
and the affordance agrees with the decider (AC 3 ceiling row). Admit-only roles,
no runtime deny (`acl-ceiling.md`).
- Row-level visibility is unchanged: the head is served only to a reader
of the channel, and the stepper adds no entity data. The descriptor holds no
entity data.
- Errors: no new error type; `session_notice` and `session_message` name no
data.

## Test Plan

- [x] Test scenarios documented for each acceptance criterion
- [x] Edge cases identified and documented
- [x] Negative test cases defined (invalid input, error conditions)
- [x] Integration test approach defined (not just unit tests)

**Test Scenarios:** one test, table row or story per acceptance criterion, named
there. Layers: realtime unit tests over the file store on MemFS with the test
clock (AC 1, 3, 4, 5); HTTP handler, budget and golden wire tests (AC 2, 6, 8);
MCP (AC 7); OpenAPI (AC 9); library story play tests run by `npm test` in
`packages/rela-components` with the a11y addon (AC 10 to 14); app unit tests
with Vitest (AC 15 to 18); Playwright end to end against a real server with two
principals (AC 19); docs (AC 20). Race detector on for Go.

**Edge Cases (each has a test):**
- Unset session on an unstored cardinality-one channel (AC 3 row, AC 10
`Initial`).
- Branching flow and two transitions to one state (AC 10 `Branching`).
- Final state, also on a closed channel (AC 3, AC 10 `Final`,
`FinalClosed`).
- Stranded state, also on a closed channel and in the narrow layout, and a
head state missing from a stale descriptor (AC 3, AC 10 `Stranded`,
`StrandedClosed`, `NarrowStranded`, AC 15, AC 17).
- Closed and archived channel (AC 3, AC 10 `Closed`, AC 13).
- Guarded transition (AC 3, AC 12 `GuardedTransitionHint`).
- `by: trigger` transition never offered (AC 3).
- A post role that survives a ceiling while the transition role does not
(AC 3).
- Caller without identity (AC 4).
- Re-entry notice (AC 11).
- Notice naming a removed state (AC 11, AC 15).
- A message and its notice in one frame (AC 14
`AnnouncesMessageAndNotice`).
- Every offered type is a transition, and a single offered type (AC 12
`OnlyTransitionTypes`, `OneTransitionType`).
- Options reorder while focused (AC 12 `ReorderKeepsFocus`).
- Head session moved ahead of `_actions` (AC 15 "moved ahead").
- Reply and edit modes (AC 12).
- Narrow container (AC 10 `Narrow`).
- A move by another user while the panel is open (AC 16, AC 19).
- A plain message type named `session` on a type without a session (AC 17).
- Not covered by a test: 64 states wrapping on very narrow screens beyond
the collapsed layout (visual only; covered by the `Narrow` story's collapse).

**Negative Tests:** AC 3 (member, permission-axis ceiling, closed, final,
stranded, no session, trigger-only), AC 5 (an unlisted postable type does not
move the session), AC 11 (no controls on a notice), AC 15 (type `session`
without the marker is not a notice; no stale composer transitions), AC 16 (no
head refetch when the session did not move), AC 17 (no reload for notices), AC
18 (no edit or delete on the causing message).

## Risk Assessment

- [x] Technical risks assessed with mitigations
- [x] Security risks assessed (see Security Considerations)
- [x] Effort estimated (xs/s/m/l/xl)

**Risks:**
- The affordance and the decider disagree (UI offers a transition that does
not fire, or hides one that does). Mitigation: both use `poster.admits`,
`currentSession` and the shared `heldRoles` helper; AC 5 posts every postable
type in every state and compares the outcome with the affordance; the hint for
guarded transitions says "if its condition holds".
- Stale composer hints between a frame and the head refetch. Mitigation:
composer transitions intersect with the head session, which updates on every
frame (AC 15 "moved ahead").
- Extra head requests in busy channels. Mitigation: refetch only when
`session.generation` or `state` moved (AC 16).
- A notice reaching the descriptor's unknown-type reload on every channel.
Mitigation: notices are skipped through `isSessionNotice` (AC 17).
- Other readers miss a move because the notice arrives with its message.
Mitigation: the announcement appends notice text (AC 14).
- Wire drift between Go and the frontend mirror. Mitigation: golden and
`wireFields.test.ts` (AC 8).
- Plimsoll: `call` gains two methods (`fireable`, `heldRoles`); check
`just plimsoll`. `ChannelTypeView` gains one field (well under 20).
- Later slices crowd the header (timers, waiting for). Mitigation: the
`aside` slot and the extensible descriptor objects.

**Effort:** m

## Documentation Planning

- [x] User-facing docs identified (skip if internal refactor)
- [x] Docs-checklist will be created when entering implementation

**Documentation Impact:** Enhancement. Edits go into
`docs-project/entities/guides/GUIDE-realtime.md` (rendered to `docs/realtime.md`
by `just docs`):
- "In the web app": a paragraph on the session header (states in
declaration order, current state, next states, end state, stranded, paused while
closed, which status wins, the collapsed layout on narrow panels), how the
composer lists the message types that move the session first and words its hint,
and how notices read and are announced. Two table rows: "Session header" and "A
message type marked as moving the session".
- "Sessions": one sentence pointing to the web app section; a note that
the stepper does not show history, the notices do.
- "HTTP API": the descriptor's `session` object (label, initial, states in
declaration order with label and `final`; no transitions, roles or guard
source), `guarded` on the channel's `session.transitions`, and
`_actions.transitions` next to `_actions.lifecycle`.
- `GUIDE-mcp-server.md`: the `read_channel` row mentions
`actions.transitions`.
- Code: godoc on `SessionDescription`, `SessionTransitionView.Guarded`,
`Affordances.Transitions`, `fireable`, `heldRoles`, and the component header
comments in the library.

## Design Review

- [x] Run `/design-review` before starting implementation
- [x] All critical/significant findings addressed in plan

**Design Review Findings:** the findings of `/design-review` are recorded as
review-response entities linked to this ticket. All 11 were adopted in this
plan: the notice text is announced when it arrives with its message; a parity
test posts every postable type and compares the outcome with
`_actions.transitions`; the ceiling and budget criteria can now fail; the
descriptor holds only the ordered states, and the head's transitions gain
`guarded`, so each UI piece has one source; composer transitions intersect with
the head session; the stepper has a status precedence (stranded > final > paused
> normal) and a stranded summary in the collapsed layout; the "caused by" link
> is gone; `checkNames` uses `isSessionNotice` and checks the head's state and
> transition targets; the composer has single-type and focus-keeping tests; and
> `transitions` is `[]` on every return path.

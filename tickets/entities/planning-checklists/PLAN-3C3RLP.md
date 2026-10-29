---
id: PLAN-3C3RLP
type: planning-checklist
title: 'Planning: Message options rendered as reply buttons'
started: "2026-10-29"
completed: "2026-10-29"
status: done
---

<!-- @managed: claude-workflow v1 -->

## Understanding

- [x] Problem/requirements clearly understood
- [x] Scope defined (what's in/out documented below)
- [x] Acceptance criteria documented with specific test scenarios

**Problem.** An agent asks "(A) ... (B) ... which?". The person who answers has
to type the choice into the composer. The PoC
(`rela-agent-runner-poc/rela-mission/project/realtime.yaml`) posts `ask` with
one markdown `question`, and the owner answers with `answer.body`, a free-text
string. Two things are missing:

- A message property cannot hold a list. Every property type in
`realtimecfg.Property` is a scalar (`internal/realtimecfg/validate.go`
`property`, `internal/realtime/validate.go` `normalizeProp`). So the poster has
no place to put the options.
- A message type cannot say "the options in this property are answers,
and a click posts message type X with the choice in property Y".

Enums do exist, but their values come from `schema.yaml`. They are fixed per
type, not per message. So they cannot carry an agent's ad-hoc options.

**Terms used below.**

- *List property*: a message property declared with `list: true`. Its value
is a JSON array of strings. This mirrors `list: true` on a `schema.yaml`
property (`metamodel.PropertyDef.List`).
- *Question*: a stored message whose type declares `choices:`.
- *Option*: one string in the question's list property.
- *Answer type*: the message type a click posts, named by `choices.answer`.
- *Into property*: the `string` property of the answer type that receives
the option text, named by `choices.into`.
- *Choice button*: one button per option, drawn on the question.
- *Other button*: an extra button that opens the composer with the answer
type selected, for a free-text answer.
- *Live question*: a question whose buttons may show (AC 8).

**Scope (in):**

- `list: true` on a `string` message property, with `max` (characters per
item) and `max_items` (number of items). Server validation, storage as a JSON
array, the predicate `message` record, JSON Schema, wire descriptors and
OpenAPI.
- `message_types.<mt>.choices: { from, answer, into, other }` in
`realtime.yaml`, with load checks.
- Read side: `choices` in the schema descriptor (`ChannelMessageTypeSchema`)
and in `read_channel`'s `MessageTypeView`.
- SPA: choice buttons and an Other button on a live, top-level question. A
click posts the answer type with `{<into>: option}`, `reply_to` the question,
and `expect_generation` (the head's generation) in a channel with a session.
Other opens the composer with the answer type and the reply set.
- SPA: a list property renders as a list in a message. An edit carries a
list value through unchanged.
- Agents: MCP `post_message` takes a JSON array for a list property. The
CLI takes `-P key=value` repeated for a list property, and `--props-json` as
today. The CLI prints a list one item per line.
- Docs (`docs/realtime.md`, `docs/cli-reference.md`), MCP descriptions,
OpenAPI, i18n (en + nl).

**Scope (out):**

- Entering a list in the composer. The composer does not render a list
property; agents author options, which is the PoC need. A follow-up picks the
widget.
- Lists of other item types (`enum`, `ref`, `integer` and so on).
- Options as objects (`{label, value}`). Plain strings only.
- A poster-chosen answer type. See Q2.
- Server-side enforcement that a question is answered once, or that the
answer equals one of its options. See Q3.
- Choice buttons on a question that is itself a reply. Such a question
shows its options as a plain list. See Q3.
- Choice buttons on a question in a session channel when that question did
not move the session. See Q3.
- `into` of type `markdown`. See Q2.
- Marking which option was chosen on an answered question.
- An authored label or confirm for the buttons (TKT-TXZZZ2), presets per
transition entry (TKT-S9TLJJ), `session.offer: transitions` (TKT-S5OJ7S).
- Trigger and reminder literals, `promote_to.map` and
`{{message.<list>}}` for a list property. All are refused at load.
- Changes to the channel store or any storage backend.

**Acceptance Criteria:**

1. **A list property loads and validates.** `options: { type: string,
list: true, max: 80, max_items: 4 }` loads. A post with `options: ["Yes", " No
"]` stores `["Yes", "No"]`: each item is trimmed and NFC-normalized. Refused
with 400 `invalid_message`, naming the property: a value that is not an array;
an item that is not a string; an item empty after trimming; an item over 80
characters; more than 4 items; two equal items after normalization (`"é"` as
U+00E9 and as `e` + U+0301); an item with a display hazard (see step 4: Cc, Cf
other than U+200D, U+2028, U+2029, U+034F, U+061C, variation selectors, the
Hangul fillers). An empty array counts as absent, so a required list refuses it
with "property is required". Test: `internal/realtime/listprop_test.go`
`TestValidateProps_StringList` (table through `validateProps`, with negative
cases for U+2028, U+200B, U+E0041 and U+202E, and a positive ZWJ emoji case),
and `TestPost_ListProp` through `Service.Post`.
2. **List declaration checks at load.** Refused with keyed problems:
`list: true` on a type other than `string` (at `.list`); `max_items` on a
property without `list: true`; `max_items` outside 1 to 20; `max` outside 1 to
500 on a list. Without `max`, an item may hold 200 characters. Without
`max_items`, a list may hold 20 items. A trigger or reminder literal, a
`promote_to.map` entry or a `{{message.<p>}}` reference that names a list
property is refused. Test: `internal/realtimecfg/load_test.go`
`TestParse_ListProperty`; cases in `template_test.go` and the existing trigger
and promote tests.
3. **A list property reads as a list everywhere.** `PropsSchema` gives
`{type: array, items: {type: string, maxLength: 80}, maxItems: 4, uniqueItems:
true}`. The schema descriptor and `PropertyDescription` carry `list: true` and
`max_items: 4`. A `when:` such as `len(message.options) <= 3` compiles and
holds. `rela channel read` describes the property as `list of string, at most
4`. Test: `jsonschema_test.go` case; `describe_test.go` case; a `when` case in
`listprop_test.go`; `channel_render_test.go` case; OpenAPI goldens.
4. **`choices:` loads and is checked.** For the PoC shape (`ask` with
`choices: { from: options, answer: answer, into: body, other: true }`) the
config loads, and `MessageType.Choices` holds the four values. Refused, by key
under `message_types.<mt>.choices`: `from` that names no property, or a property
that is not a list (`.from`); `answer` that names no message type, an ephemeral
type, or the question type itself (`.answer`); `choices` on an ephemeral type;
`into` that names no property of the answer type, a list property, or a type
other than `string` (`.into`); an `into` whose `max` is lower than the list's
item `max` (`.into`, "an option of up to N characters does not fit into X"); an
answer type with another required property (`.answer`). An unknown key under
`choices` is refused by the key checker. A load warning at `.from` when the list
allows more than 10 items or more than 100 characters per item ("too many or too
long for buttons"). Test: `internal/realtimecfg/choices_test.go`
`TestChoices_Load` and `TestChoices_Problems` (table with exact keys and
reasons).
5. **A session warning when buttons can never show.** In a channel type
with a session, buttons show only on the question that moved the session (AC 8).
The config warns at `.choices.answer` when the question type has no transition
into a state where the answer type is postable. This covers a question type with
no transition at all. No warning for the PoC shape (`ask` moves to `asking`,
where `answer` is postable). Test: `TestChoices_SessionWarning` in
`choices_test.go`: a warning when `answer.in: [review]` and `ask` leads to
`asking`; a warning when `ask` has no transition; none for the PoC.
6. **Descriptors and agent reads carry `choices`.** `/api/v1/_schema`
message type `ask` has `choices: {from: options, answer: answer, into: body,
other: true}`. `read_channel` lists the same under `message_types[].choices`.
Types without `choices` omit the key. Test: `describe_test.go` and
`read_test.go` cases; `channels_wire_golden_test.go` (goldens
`channel_wire_fields.json` and `channel_wire.json` regenerated); OpenAPI
goldens.
7. **Agents can post options.** MCP `post_message` with
`props: {question: "...", options: ["A", "B"]}` stores the list. The CLI `rela
channel post T-1 run ask -P question=Which -P options=A -P options=B` stores
`["A", "B"]`; `-P` given twice for a scalar property is still refused. `rela
channel read` prints a list one item per line, each line indented and made
printable. Test: `internal/mcp` channel tool test (post with an array);
`internal/cli/channel_test.go` cases for repeated `-P` on a list and the scalar
refusal; `channel_render_test.go` list case.
8. **Choice buttons show only on a live question.** For a held question
with options `["A", "B"]` and `other: true`, the caller sees buttons "A", "B"
and "Other answer…" on that message when all hold:
   - the message is not removed and is top-level (no `reply_to`);
   - the channel is open and the answer type is in `_actions.post`;
   - `hasNewer` is false;
   - no held message of the answer type has `reply_to` equal to the
question's seq;
   - in a channel with a session: `decisionCause` returns the question's
seq, and `actionsGeneration` equals the head's generation. A caller who may not
post the answer type sees the options as a plain list. Test:
`channelView.test.ts` `choiceViews` cases: live; removed; reply (no buttons); no
post right; answered; `hasNewer`; session, question caused the current
generation (buttons); session, a later notice moved it (none); session, head
ahead of the held notice (none); session, question did not move it (none);
`_actions` stale; no session; `other: false`.
9. **A click posts the answer.** Clicking "B" posts
`{type: answer, props: {body: "B"}, reply_to: <question seq>}` and, in a channel
with a session, `expect_generation: <head generation>` read at the click, as
`onDecide` does. While the post is in flight, every decision and choice button
is disabled and the clicked one shows pending. A 409 `session_moved` shows "The
session moved on. Choose again." A 400 from the answer type (for example its
`when:`) goes through the existing `refused` path: the composer opens with the
answer type and shows the error. Test: `useSessionDecisions.test.ts` `onChoose`
cases: posts the right request; no session (no `expect_generation`); double
click posts once; `session_moved`; `invalid_message` opens the form.
10. **Other opens the composer.** Clicking "Other answer…" selects the
answer type and sets the reply to the question. In a session channel it records
a form decision with the head generation, so the send carries it (TKT-FPXTQJ AC
13). Without a session it records none, and the send carries no
`expect_generation`. During an edit the user is asked first. Test:
`useSessionDecisions.test.ts` cases "other with session" and "other without
session sends no generation"; `ChannelsPanel.test.ts` mount case: click Other,
type, send, assert the request.
11. **One row, no duplicate answer buttons.** When the question is also the
session decision row (`decisionSeq`), the row drops the session decision of the
answer type. The stepper header keeps it. Test: `channelView.test.ts`
`rowDecisions` case; a mount case in `ChannelsPanel.test.ts` with one "Answer"
button in the header and none on the row.
12. **A list property renders and survives an edit.** A message shows a
list property as a bulleted list, each item as text. A stored value that is not
an array for a list property shows as text. The choice group renders after the
fields whenever there are choice views, also when the presentation hides the
`from` field; while the group shows, the `from` field is not rendered a second
time. Editing a message whose list property the composer does not render sends
the list back unchanged. Test: `channelValues.test.ts` case "list prop carried
through an edit"; `RlChannelMessage` test or story: list renders as items, not
as a ref chip; group shows with `from` hidden; `channelView.test.ts` case for a
non-array stored value and for an excerpt with a list prop.
13. **A second answer outside a session is stored.** Two posts of the
answer type replying to one question in a channel without a session are both
stored. This pins the documented behaviour. Test:
`TestChoices_SecondAnswerWithoutSession` in
`internal/realtime/listprop_test.go`, through `Service.Post`.
14. **Docs and descriptions.** `docs/realtime.md` has a "Choices" example
that loads (pinned by a case in `docs_example_test.go`). The MCP `post_message`
and `read_channel` descriptions mention list properties and `choices`.
`docs/cli-reference.md` says `-P` repeats for a list.

## Research

- [x] ~~For larger features: run `/research` to create a structured research doc~~ (N/A: the shape follows existing patterns: `list: true` from schema.yaml, decision buttons from TKT-T9G1AE; the open choices are recorded below)
- [x] Searched for existing libraries that solve this problem
- [x] Checked codebase for similar patterns or reusable code
- [x] Looked for reference implementations in other projects
- [x] Reviewed relevant rela concepts for prior art

**Research Doc:** N/A

**Existing Solutions:**

- Libraries: `golang.org/x/text/unicode/norm` (already a dependency, used
by `internal/realtime/mentions.go`) for NFC. The predicate engine already has
`predicate.ListType` and `predicate.NewList`.
- Codebase patterns reused:
  - `metamodel.PropertyDef.List`: a multi-value property is `type` plus
`list: true`. `max_items` matches `internal/subscription/config.go`.
  - `promote_to` and `actions` in `realtime.yaml`: behaviour declared on
the message type, names checked at load. `choices` sits next to them.
  - `ChannelDecisionView.props` and `presetProps`: a button that posts a
type with preset props already exists.
  - `useSessionDecisions`: the click lock, `live()`, `refused`, `openForm`
and `formDecision` (TKT-T9G1AE, TKT-FPXTQJ). Choices add `onChoose` here, so a
choice and a decision share one lock.
  - `decisionCause` in `channelView.ts`: already proves which held message
caused the current generation, and returns undefined when the proof is not held.
The session rule reuses it.
  - Display hazard sets in `internal/cli/printable.go`,
`internal/comments/comments.go` and `internal/lease/lease.go`.
- Reference implementations: Slack Block Kit buttons, Teams Adaptive Card
`Action.Submit`, Telegram inline keyboards. A click is an ordinary user message,
and quick replies disappear once answered. The Claude Agent SDK
`AskUserQuestion` takes 2 to 4 options plus "Other", which supports the warning
for long lists.
- rela prior art: TKT-T9G1AE, TKT-FPXTQJ, TKT-QYFLRH, and the
"configuration is not a secret" rule in CLAUDE.md.

## Approach

- [x] Technical approach chosen and documented
- [x] Approach builds on existing patterns (not reinventing)
- [x] Alternatives considered (document why rejected)
- [x] Dependencies identified (packages, APIs, types)

**Technical Approach:**

1. **Limits (`internal/realtimelimits/limits.go`).** `MaxListItems = 20`
(cap and default for `max_items`), `MaxListItemRunes = 500` (cap for `max`),
`DefaultListItemRunes = 200`. The 16 KiB props cap (`MaxMessageBytes`) still
applies to the whole message, so 20 long non-ASCII items can hit it first. The
docs say so.
2. **Raw config (`internal/realtimecfg/raw.go`).** `rawProperty` gains
`List bool` and `MaxItems *int` (yaml `list`, `max_items`). `rawMessageType`
gains `Choices *rawChoices` with `from`, `answer`, `into`, `other`. The key
checker refuses unknown keys by reflection.
3. **Compiled config (`compiled.go`, `validate.go`, new `choices.go`).**
   - `Property` gains `List bool` and `MaxItems int`. For a list, `Max` is
the item limit, always set. `MessageType` gains `Choices *Choices`; `clone.go`
copies it.
   - `validator.property`: the list checks of AC 2.
   - Sites that would treat a list as a scalar string get a `p.List` test:
`literalProps`, `promotable`, `template.go` `refType` and `textRef`,
`programs.go` `messageRecordType` (a list is `predicate.ListType{Elem:
predicate.StringType}`; session guards use the same function), `jsonschema.go`.
   - `validator.choices` runs after all message types of a channel type
are compiled. It makes the checks of AC 4, including the warning for more than
10 items or 100 characters.
   - Session warning (AC 5): `choicesReachable(sess, question, answer)`.
For each state `s` where `question.PostableIn(s)`, collect the `to` of every
entry in `Branches(s, question)`. Warn when there are none, or when
`answer.PostableIn` holds for none of them. The godoc says this
over-approximates: it treats every guarded entry as reachable and ignores entry
order, so an entry after an unguarded one counts although it is dead.
Over-approximation can only hide a warning, never refuse a config.
4. **Post-time validation (`internal/realtime/validate.go`).**
   - `normalizeProp` gets a list branch first: `normalizeList(p, raw)`
accepts `[]any` and `[]string`. Each item must be a valid UTF-8 string. It is
trimmed, then NFC-normalized, then checked: non-empty, at most `p.Max` code
points, no display hazard. Duplicates are compared after normalization. At most
`p.MaxItems`. The stored form is `[]any` of strings. Items are trimmed although
scalar strings are not; the docs say so.
   - Display hazards live in one helper, `texthazard.IsDisplayHazard(r)`
in a new leaf package `internal/texthazard`: Cc; Cf except U+200D (keeps ZWJ
emoji); U+2028, U+2029; U+034F; U+061C (Cf already, listed for clarity);
variation selectors; the Hangul fillers U+115F, U+1160, U+3164, U+FFA0. Making
`printable.go`, `comments.go` and `lease.go` use it is a follow-up. Today
`printable` replaces hazards instead of refusing them, and `lease` checks a
narrower set. Variation selectors are refused here, as in `comments.go`, because
two options that differ only by one would render the same.
   - In `validateProps`, a normalized empty list is absent.
   - `predicateValue`: a list becomes `predicate.NewList` of strings.
   - `mentions.go` scans markdown only, so list items never mention.
5. **JSON Schema, descriptors and wire.**
   - `PropSchema` gains `Items`, `MaxItems` and `UniqueItems`.
   - `PropertyDescription` gains `List`, `MaxItems`.
`MessageTypeDescription` and `MessageTypeView` gain `Choices
*ChoicesDescription`.
   - `apiwire/v1/channels.go`: `ChannelPropertySchema` gains `list`,
`max_items`; `ChannelMessageTypeSchema` gains `choices`.
`internal/dataentry/channels_schema.go` maps them. Regenerate the wire goldens
and the OpenAPI goldens.
   - Message wire is unchanged: a list travels as a JSON array.
6. **MCP and CLI.**
   - `post_message` description: a list property takes a JSON array of
strings; a type with choices shows them as buttons, and a click posts the answer
type as a reply to the question. `read_channel`: mention
`message_types[].choices`.
   - `channelProps.collect`: when the schema type is `array`, a repeated
`-P` appends; other types still refuse a repeat. Update the help.
   - `channel_render.go`: a `[]any` value prints one `- item` per line
through `printable`. The property list prints `list of string, at most N`
instead of `array`.
7. **SPA: pure views (`channelView.ts`).**
   - `choiceViews(ct, head, actionsGeneration, hasNewer, messages)`
returns `Map<number, ChannelDecisionView[]>` keyed by question seq. It returns
an empty map when `hasNewer`, the channel is not open, or the answer type is not
in `_actions.post`. In a session channel it considers only the message
`decisionCause(messages, cs)` returns, and only when `actionsGeneration ===
cs.generation`. A candidate must be held, not removed, top-level, of a type with
`choices`, with a non-empty array in `props[from]`, and with no held answer-type
message whose `reply_to` is its seq. One view per option (`key:
"choice:<seq>:<i>"`, `messageType: answer`, `label: option`, `props: {[into]:
option}`), plus with `other` a view `key: "choice:<seq>:other"`, `label:
t('channels.choiceOther')`, `opensForm: true`.
   - `rowDecisions(decisions, choices, decisionSeq)` (AC 11).
   - Array guards: `isReadableRef` (line 107) and the excerpt loop
(line 524) test `Array.isArray` before `typeof v === 'object'`. An excerpt
prints a list as its items joined by ", ".
8. **SPA: click handling (`useSessionDecisions.ts`).**
   - A `choices` computed with its own memo. The key is the decisions key
plus `hasNewer` plus the identity of `d.ch.messages.value`. That ref is a
`shallowRef` replaced on every merge (`useChannel.ts`), so a held answer, an
edit or a removal rebuilds the views. Without a choice type in the channel type,
the memo returns an empty map at once.
   - `onChoose(key)` takes the `deciding` lock and reads, before the first
await, the head generation (`undefined` without a session) and the view, looked
up again from `choices` (a missing view is the stale toast). It posts
`d.post(answer, presetProps(view.props), seq, generation)` and then refreshes
the head and focuses the question row, like a row decision. A refusal goes
through `refused`.
   - Other calls `openForm` and then `d.composer.reply(seq)`. `openForm`
changes: when the channel has no session, it skips the generation check and does
not set `formDecision`. Its `generation` parameter becomes `number | undefined`.
   - `DecisionComposer` gains `reply(seq)`, wired in `OpenChannel.vue` to
the existing `onReply`.
   - `presetProps` and `channelValues.ts` `carryProp` get an
`Array.isArray` branch first, so an array passes through as is.
`isRestrictedRef` gets the same guard.
9. **SPA: components (`rela-components`).**
   - `types.ts`: `ChannelPropValue` adds `string[]`;
`ChannelPropertyDef` adds `list?`, `maxItems?`; `ChannelMessageTypeDef` adds
`choices?`.
   - `RlChannelMessage.vue`: `isRef` tests `Array.isArray` first. A list
renders as `<ul>` with each item as text. New prop `choices`: when non-empty, an
`RlChannelDecisions` group renders after the fields, and the `from` field is
skipped. Button labels clamp to two lines with CSS; the full text stays in the
accessible name.
   - `RlChannel.vue`: the excerpt loop (line 183) tests `Array.isArray`
first. New prop `choices` passed to each row; new emit `choose`.
   - `RlChannelDecisions.vue`: new optional props `label` and `icon`, and
a `hint` kind. The choice group passes the "Choices" label, no `workflow` icon,
and a hint "Posts this answer as a reply".
   - Library messages (`useMessages.ts`, `messages.nl.ts`): `choices`
group label ("Choices" / "Keuzes") and `channelChoiceHint` ("Posts this answer
as a reply" / "Plaatst dit antwoord als reactie").
   - Fixtures and stories: a question with live choices, an answered one,
and a list message.
10. **SPA: app.**
    - `frontend/src/types/channels.ts`: `MessagePropValue` and
`MessagePropInput` add `string[]`; schema types add `list?`, `max_items?`,
`choices?`.
    - App locales `src/i18n/locales/{en,nl}.json`: `channels.choiceOther`
("Other answer…" / "Ander antwoord…"). The view is built in the app, next to
`channels.decisionStale`.
    - The composer skips list properties when it builds fields. A human
cannot post a type with a required list from the composer; the server refuses
with 400 and the docs say so.
    - `OpenChannel.vue`: pass `choices` to `RlChannel`, use
`rowDecisions` for the row, handle `@choose`, supply `reply`.

**Alternatives considered:**

- *A new property type such as `type: list` or `type: choices`.* Rejected:
schema.yaml uses `type` plus `list: true`.
- *A new internal `PropertyStringList` type.* Rejected: it splits one YAML
key into two internal shapes; the switch sites are few and tested.
- *The poster chooses the answer type per message.* Rejected (Q2).
- *Put `choices` in `data-entry.yaml`.* Rejected: agents and the server
must see it.
- *Compute a question generation from the held notices.* Rejected: the
held window is not always contiguous to the live end (`hasNewer`), and a head
frame can be ahead of the held notices. A wrong value would send the current
generation for an old question, and the server would accept it. `decisionCause`
refuses whenever the proof is not held.
- *Choices on threaded questions, replying to the thread root.* Rejected:
a reply row goes to the author of the parent, so the answer would address the
thread root's author, not the asker.
- *`into` of type `markdown`.* Rejected: the clicker is the author, so a
markdown option could post a disguised link or a mention in the clicker's name.
- *Server-side single answer.* Rejected for this slice: it changes the
channel store on four backends.
- *Separate composable `useMessageChoices`.* Rejected: a choice and a
decision must share one lock.

**Dependencies:** `internal/predicate` (no change), `internal/realtimelimits`,
new `internal/texthazard`, `golang.org/x/text/unicode/norm` (present),
`internal/apiwire/v1`, `internal/openapi`, `internal/mcp`, `internal/cli`.
Frontend: no new packages.

**Files to modify:**

- `internal/realtimelimits/limits.go`; new `internal/texthazard`;
`.go-arch-lint.yml` (component for `texthazard`).
- `internal/realtimecfg/raw.go`, `compiled.go`, `validate.go`,
`programs.go`, `template.go`, `jsonschema.go`, `clone.go`; new `choices.go`.
- `internal/realtime/validate.go`, `describe.go`, `read.go`.
- `internal/apiwire/v1/channels.go`, `internal/dataentry/channels_schema.go`;
`internal/dataentry/testdata/channel_wire_fields.json`, `channel_wire.json`.
- `internal/openapi/channels.go`; goldens under
`internal/dataentry/testdata/golden/`.
- `internal/mcp/tools_channels.go`.
- `internal/cli/channel.go`, `channel_render.go`.
- `frontend/src/components/entity/channels/channelView.ts`,
`useSessionDecisions.ts`, `OpenChannel.vue`; `frontend/src/types/channels.ts`;
`frontend/src/utils/channelValues.ts`; `frontend/src/i18n/locales/en.json`,
`nl.json`.
- `frontend/packages/rela-components/src/components/channel/types.ts`,
`RlChannel.vue`, `RlChannelMessage.vue`, `RlChannelDecisions.vue`,
`RlChannelDecisions.stories.ts`, `RlChannelMessage.stories.ts`,
`composables/useMessages.ts`, `composables/messages.nl.ts`,
`fixtures/channels.ts`.
- Tests: new `internal/realtimecfg/choices_test.go`,
`internal/realtime/listprop_test.go`, `internal/texthazard` test; cases in
`internal/realtimecfg/{load,template,jsonschema,docs_example}_test.go`,
`internal/realtime/{describe,read,session}_test.go`,
`internal/dataentry/channels_wire_golden_test.go`, the `internal/mcp` channel
tool test, `internal/cli/{channel,channel_render}_test.go`,
`frontend/src/components/entity/channels/{channelView,useSessionDecisions}.test.ts`,
`frontend/src/components/entity/ChannelsPanel.test.ts`,
`frontend/src/utils/channelValues.test.ts`.
- Docs: `docs/realtime.md`, `docs/cli-reference.md`.

## Security Considerations

- [x] Input sources identified (user input, config, external APIs)
- [x] Input validation approach defined (allowlist preferred over blocklist)
- [x] Security-sensitive operations identified (file access, auth, crypto)
- [x] Error handling doesn't leak sensitive information

**Input Sources & Validation:**

- *`realtime.yaml` (operator config).* The new keys go through the key
checker and keyed `validator` problems. Names in `choices` must resolve. A
failed reload keeps the last good config.
- *Options (poster input over HTTP, MCP, CLI).* `normalizeList` checks
type, UTF-8, trim, NFC, length, count, duplicates and display hazards. The props
byte cap (413) and `channels.PrepareProps` still apply. A refusal names the
property and the rule, never stored data.
- *The click (person input).* An ordinary post of the answer type.
`Service.Post` runs every check it runs today: read access, post role, open
channel, `in:`, `expect_generation`, rate limit, props validation and the answer
type's `when:`. The server does not require the text to be an option, because
Other sends free text into the same property.

**Security-Sensitive Operations:**

- *Attribution.* The clicker is the author of the answer, but the question
author chose its text. `into` must be `string`, which renders as plain text and
never mentions, so an option cannot become a link or a mention in the clicker's
name. The button shows exactly what is posted.
- *Lookalike options.* NFC and the hazard list stop two options that
render the same but post different values, and stop tag characters that hide
ASCII from a person while an agent reads it.
- *Who sees and who may click.* Buttons render only when the answer type
is in the caller's `_actions.post`. The buttons are a UX filter; `Post` is the
gate. `choices` is config and not a secret.
- *Stale clicks.* In a session channel, buttons show only on the question
that caused the current generation, proven from held messages, and the click
sends the head generation. A click after the session moved gets 409
`session_moved`. When the proof is not held (`hasNewer`, head ahead of the held
notice), no buttons show.
- *Addressing.* Only top-level questions get buttons, and the answer
replies to the question itself, so the reply row addresses the asker.
- *Rendering.* Option text renders through Vue interpolation, never
`v-html`. The CLI prints items through `printable`, one per line.
- *Agents.* Option text is message content behind the `UntrustedNotice`.
- *Cost.* At most 20 items inside the 16 KiB cap. `choiceViews` is one
pass over the held messages when the head key or the messages array changes.
Only one candidate is checked in a session channel.

## Test Plan

- [x] Test scenarios documented for each acceptance criterion
- [x] Edge cases identified and documented
- [x] Negative test cases defined (invalid input, error conditions)
- [x] Integration test approach defined (not just unit tests)

**Test Scenarios:**

| AC | Test |
| --- | --- |
| 1 | `TestValidateProps_StringList` (table), `TestPost_ListProp`, `texthazard` table |
| 2 | `TestParse_ListProperty`, template, trigger and promote cases |
| 3 | `jsonschema_test.go`, `describe_test.go`, `listprop_test.go` `when` case, CLI render case, OpenAPI goldens |
| 4 | `TestChoices_Load`, `TestChoices_Problems` |
| 5 | `TestChoices_SessionWarning` |
| 6 | `describe_test.go`, `read_test.go`, wire goldens, OpenAPI goldens |
| 7 | MCP post with an array, CLI repeated `-P` and render cases |
| 8 | `channelView.test.ts` `choiceViews` cases |
| 9 | `useSessionDecisions.test.ts` `onChoose` cases |
| 10 | `useSessionDecisions.test.ts` Other cases, `ChannelsPanel.test.ts` mount case |
| 11 | `channelView.test.ts` `rowDecisions`, `ChannelsPanel.test.ts` mount case |
| 12 | `channelValues.test.ts` edit carry, `RlChannelMessage` render cases, `channelView.test.ts` |
| 13 | `TestChoices_SecondAnswerWithoutSession` |
| 14 | `docs_example_test.go` |

A memo test in `useSessionDecisions.test.ts`: only `messages` changes (an answer
arrives), and the buttons hide.

Integration: `TestChoices_PoCFlow` in `internal/realtime/session_test.go` runs
through `Service.Post` with the in-memory store and a PoC-shaped config. The
runner posts `ask` with options (session to `asking`, generation 2). The owner
posts `answer` with `body: "B"`, `reply_to` the question and `expect_generation:
2` (session to `working`). The reply row for the runner exists. A second
identical post gets 409 `session_moved`. On the SPA side,
`ChannelsPanel.test.ts` drives a click end to end.

**Edge cases:**

- Question without options or with `[]`: a normal message, no buttons.
- One option: one button (plus Other).
- ZWJ emoji, CJK and combining marks are allowed; length counts code
points after NFC.
- An option with a newline, U+2028 or a zero-width space: refused.
- Question that is a reply: options as a plain list, no buttons.
- Question removed: no buttons.
- Question edited while shown: the memo rebuilds; a click posts the text
the user saw. Accepted.
- Session channel, question did not move the session: no buttons; the
config warning flags a type that can never move it.
- Session channel, timer move after the question: `decisionCause` is
undefined; no buttons.
- Older messages loaded (`hasNewer`): no buttons until the live end is
back.
- Two clicks at once in a session channel: one wins; the other gets
`session_moved`.
- Two clicks at once without a session: both stored (AC 13). The buttons
hide once an answer is held.
- Config reload removes `choices` or `list: true`: old arrays render as
items; no buttons.
- CLI `-P options=a,b`: one item `a,b`.

**Negative Tests:**

- `list: true` on `markdown`, `integer` or an enum: refused at `.list`.
- `max_items: 0`, `max_items: 21`, `max: 501` on a list; `max_items`
without `list`: refused.
- A list property in a trigger literal, `promote_to.map` or an output
template: refused.
- `choices.from` scalar; `choices.answer` ephemeral, missing or the
question type; `choices` on an ephemeral type; `choices.into` a list,
`markdown`, `integer` or missing; an answer type with another required property;
unknown key `choices.label`: refused at the keys of AC 4.
- Post: not an array, `[1]`, `[""]`, U+202E, U+2028, U+200B, U+E0041,
21 items, an item over `max`, NFC duplicates: 400 naming the property.
- Click after the session moved: 409, nothing stored.
- Click after losing the post role: 403; the panel shows the refusal.

## Risk Assessment

- [x] Technical risks assessed with mitigations
- [x] Security risks assessed (see Security Considerations)
- [x] Effort estimated (xs/s/m/l/xl)

**Risks:**

- *A Go site keeps treating a list as a scalar string.* The sites are
named in step 3, each with a test. Review greps for `PropertyString`.
- *An SPA site treats an array as a ref.* `vue-tsc` does not catch guards
that test only `typeof v === 'object'`. The sites are named in steps 7 to 9
(`isRef`, `isReadableRef`, both excerpt loops, `carryProp`, `isRestrictedRef`,
`presetProps`), each with an `Array.isArray` branch first and a test. Review
greps for `typeof v === 'object'` and `typeof value === 'object'` in channel
code.
- *Buttons on a stale question.* Removed by design: the session rule
reuses `decisionCause`, and `hasNewer` hides all buttons.
- *Scope.* Size l. The cuts are fixed: no composer entry for lists, no
threaded questions, no markdown `into`, no prefill after a 400, no enum or ref
lists, no object options.
- *Mixed-version processes.* An older process refuses `list:` and
`choices:` as unknown keys and fails the load. An older SPA shows a list value
as text. Documented in the version note.

**Effort:** l.

## Documentation Planning

- [x] User-facing docs identified (skip if internal refactor)
- [x] Docs-checklist will be created when entering implementation

**Documentation Impact:**

- [x] ~~docs/metamodel.md - New metamodel features~~ (N/A: realtime config is documented in docs/realtime.md)
- [x] docs/cli-reference.md, `rela channel post`: `-P` repeats for a list
property; `--props-json` takes an array.
- [x] ~~docs/data-entry.md - UI changes~~ (N/A: the realtime web app is documented in docs/realtime.md)
- [x] docs/realtime.md, "Message types": `list: true` (string only), `max`
per item, `max_items`, items trimmed unlike scalar strings; a `choices` row.
- [x] docs/realtime.md, new section "Choices": the PoC example; what a
click posts; Other; when buttons show (top-level, live, in a session only on the
question that moved it); that the server does not check the answer against the
options and stores a second answer outside a session; that `into` is a string;
that the composer cannot enter a list.
- [x] docs/realtime.md, "Checks when the file loads": list and choices
checks, and both warnings.
- [x] docs/realtime.md, "Limits": 20 items, 500 characters per item, 200 by
default, within the 16 KiB message cap.
- [x] docs/realtime.md, "In the web app" and "Agents and other processes":
choice buttons, Other, posting a list over MCP and the CLI.
- [x] docs/realtime.md, "HTTP API": `list`, `max_items`, `choices`; the
version note.
- [x] ~~CLAUDE.md - New patterns or conventions~~ (N/A: no new convention)
- [x] ~~README.md - Project-level changes~~ (N/A)

## Open questions (decided)

- **Q1. How is a list property declared and validated?** `type: string`
plus `list: true`. `max` limits each item (default 200, at most 500);
`max_items` limits the count (default and cap 20). Items are trimmed,
NFC-normalized, non-empty, unique and free of display hazards. An empty array is
absent.
- **Q2. How does a type declare reply options?** `choices: { from,
answer, into, other }` on the question type, with the answer type and a `string`
into property fixed in config, so the load can check them. A poster-chosen
answer type is rejected: no load checks, and an agent could aim a person's click
at any type. Markdown `into` is rejected because the clicker would be credited
with markup the asker chose.
- **Q3. How does a click link to the question; what about double answers
and stale options?** Only a top-level question gets buttons. The click posts
with `reply_to` the question, so the reply row addresses the asker. A threaded
question would address the thread root's author, so it shows a plain list. In a
session channel, buttons show only on the question `decisionCause` names, while
the held `_actions` match the head; the click sends the head generation, and a
click after a move gets
  409. No buttons while `hasNewer`. The server does not record "answered"
and does not check the text against the options; outside a session a second
answer is stored (pinned by AC 13).
- **Q4. How does Other work?** `other: true` adds "Other answer…" /
"Ander antwoord…", a fixed label. It opens the composer with the answer type and
the reply set, and in a session channel a form decision with the head
generation. An authored label belongs to TKT-TXZZZ2.
- **Q5. ACL and session interaction.** Buttons show only when the answer
type is in `_actions.post`. The click is a normal post. No new write path, no
change to transitions.
- **Q6. Agents.** MCP takes a JSON array (`props_schema` type `array`,
`uniqueItems`). The CLI appends repeated `-P` for a list. `read_channel` lists
`choices`.

**Cut in this revision:** composer entry of lists (textarea, hint,
`toPropValue`/`fromPropValue`/`initialValues` changes, the `RlChannelField` mode
and its story); `DecisionComposer.prefill`; the `questionGeneration` function;
threaded-question handling; markdown `into`; repeated pattern and reference
prose.

## Design Review

- [x] Run `/design-review` before starting implementation
- [x] All critical/significant findings addressed in plan

**Design Review Findings:** <!-- List review-response IDs, e.g., RR-xxxx --> 23
findings (1 critical, 6 significant, 10 minor, 6 nit). All are addressed in this
plan, except finding 2: the split into two PRs is declined at the owner's
request; the plan stays one ticket and one PR, is trimmed, and the effort is l.

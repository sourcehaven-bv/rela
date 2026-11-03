---
id: PLAN-M2PQKG
type: planning-checklist
title: 'Planning: Preset buttons that open the composer prefilled'
started: "2026-10-30"
completed: "2026-10-30"
status: done
---

<!-- @managed: claude-workflow v1 -->

## Understanding

- [x] Problem/requirements clearly understood
- [x] Scope defined (what's in/out documented below)
- [x] Acceptance criteria documented with specific test scenarios

**Problem.** TKT-S9TLJJ added `on.<type>.button: { label, props }`: a one-click
button that posts the entry's message type with a fixed preset. The load refuses
a preset that leaves out a required property. `buttonChecker.preset` passes the
preset to `MessageType.NormalizeProps`, which returns `property "feedback" is
required` (`internal/realtimecfg/buttons.go`, `props.go`). So the button is
refused at load. It is not loaded and then hidden. A decision that needs text,
such as Revise with required feedback, cannot have a button. It falls back to
the type button, which opens an empty composer. `docs/realtime.md` says this and
names this ticket as the follow-up.

This ticket lets a button open the composer with the preset fixed. The person
then writes the rest and presses Send. The post is a normal post, so the server
still checks every required property.

The load-time guarantee of TKT-S9TLJJ stays: when a button is shown, the entry
its click takes is known when the file loads. For a compose button this needs
two rules. The preset's values are locked in the composer, so the person cannot
change where the Send goes. And no guard that decides the route may read a
property the person fills in.

**Terms used below.**

- *Compose button*: a button with `compose: true`. A click opens the
composer instead of posting.
- *One-click button*: a button without `compose: true`, as in TKT-S9TLJJ.
- *Given property*: a property the preset sets. In compose mode it is
locked: it is shown read-only, not as a field, and the app adds it to the Send.
- *Left-out property*: a declared property of the message type that the
preset does not set. The person may fill it in.
- *Compose mode*: the composer's state from a compose click until the
compose decision ends. It shows the given properties read-only and renders
fields only for the left-out properties.

**Scope (in):**

- `realtime.yaml`: `on.<type>.button.compose`, a strict boolean, read
with `yamlscalar.PlainYAML11Bool` as `confirm:` and `unread:` are. Checked at
load, on reload and by `rela validate`.
- Load checks for a compose button: the given properties follow the rules
of a post, but a required property may be left out. A type `when:` that reads a
left-out property is not checked at load, because it refuses a post rather than
routing it. The entry's own guard, and an earlier entry's guard that admits a
role that could click, may not read a left-out property.
- A key hint for `decision_button.compose`.
- `confirm` and `style` on a compose button (TKT-TXZZZ2).
- Served config: `compose` on the served `button` of a transition row
(REST head, MCP `read_channel` and `read_session`, CLI JSON), OpenAPI and the
MCP tool descriptions. The godoc that calls every button one-click.
- rela-components: a `compose` prop and a `cancelCompose` event on
`RlChannelComposer`, with a read-only strip that has a cancel control. Storybook
stories show it.
- SPA: a compose button opens the composer in compose mode. Its Send
carries the given properties and the session generation.
- E2E: one test in `e2e/tests/channels.spec.ts`, with its own fixture.
- Docs: `GUIDE-realtime.md`, then `just docs` regenerates
`docs/realtime.md`.

**Scope (out):**

- Any change to the post path, the decider, `GuardHolds` or the store. A
compose post is a normal post.
- Any change to the shape of `_actions.entries`. See Approach 6.
- Inferring compose from a missing required property. See Approach 1.
- Partial evaluation of a guard at load, which would decide some guards
that read a left-out property. It needs access tracking in `internal/predicate`.
See Approach 4.
- Allowing `ref` or `markdown` in a preset, also for a compose button.
See Open questions Q1.
- Setting the reply to the decision's cause. A compose button opens the
form as the type button does, without a reply. See Q3.
- Folding the PoC's `approve` and `revise` types into one type. This is
now possible, but it changes the PoC and its runner. See Q2.
- Compose buttons in the inbox. `InboxEntry` is unchanged.
- MCP tool behaviour. Only the descriptions and the golden file change.

**Acceptance Criteria:**

1. **The key loads, and only as a boolean.** `button: { label: Revise,
props: { decision: revise }, compose: true }` loads on a mapping entry and on a
list entry, though the message type requires `feedback`. The compiled
`TransitionButton` has `Compose bool`. `compose: false`, `compose: null` and no
`compose` key all give false. `compose: yes`, `compose: "yes"`, `compose: on`
and `compose: 1` are refused at `...button.compose` with "is not a boolean; use
true or false" or "must be true or false". A compose button may set every
required property, or no property at all. `Props()` returns only the given
properties, in stored form. Test: `internal/realtimecfg/buttons_test.go`, new
`TestButtons_Compose` table: required left out on a mapping entry; the same on a
list entry; every property set; empty props; `compose: null`; `compose: false`
with a required property left out is refused (see AC 2); props in stored form;
and the four refused spellings.
2. **A partial preset still type-checks what it gives.** Every preset
goes through one call, `normalize(..., requireAll false)`. Each given property
follows the rules of a post. Refused, each with its key path: an undeclared
property; a value of the wrong type; an enum value outside the enum; a string
over its `max`; a bad list item; a list over `max_items`; more than
`realtimelimits.MaxMessageProperties` properties; an encoding over
`message_bytes`; a `ref` or `markdown` value; an integer beyond 2^53. The
left-out set is computed from the normalized result, so `feedback: ~` and `tags:
[]` count as left out, as the post path counts them. A compose button that
leaves out a required list property is refused, because the composer has no
field for a list. A one-click button that leaves out a required property is
refused, as today, with "property "feedback" is required; set it in props, or
add compose: true so the person fills it in". The prop rules are reported before
the missing required property. Test: new `TestButtons_ComposeProblems` table,
one case per refusal, plus the hint case with the property absent, with `~` and,
for a required list, with `[]`. `TestButtons_PropsProblems` stays green, with
its required-property case updated to the new message.
3. **The type's `when:` is judged on what the preset gives.** For a
compose button, a type `when:` that reads a left-out property is not checked at
load. It does not route the post. A Send that fails it is refused with
`invalid_message`, and the composer keeps the text. A type `when:` that reads
only given properties must hold for the preset. A type `when:` that calls
`today()` is refused, as for every preset. Test: `TestButtons_ComposeProblems`
cases: "when reads a left-out property loads", "when on given properties fails
is refused", "when calls today() is refused".
4. **No routing guard reads a left-out property.** For a compose button:
   - The entry's own guard may read only given properties and
`session.state`. It must hold for the preset, as today. A guard that reads a
left-out property, or the whole `message`, is refused at `...button`, with a
message that names the property and says to set it in props or to move the check
to the type's `when:`.
   - An earlier entry that admits a role that could click, and whose
guard reads a left-out property or the whole `message`, is refused at
`...button`, with a message that names entry i and the property.
   - An earlier entry that admits no role that could click, and whose
guard reads a left-out property, joins the button's hidden set, as an
undecidable `session` guard does today. Only a caller who also holds one of its
roles reaches it, and that caller does not see the button.
   - Every other rule applies as today: shadowing, guard errors, the
undecidable `session` and `today()` refusal, and the reachability check.
   - A one-click button is judged exactly as today. Its left-out set is
empty.

The rule is coarse on purpose. It refuses some buttons whose route is in fact
decided, such as one below `message.decision == 'approve' and message.note !=
''` when the preset sets `decision: revise`. The workaround is to move the
compose entry above that entry. It also rules out an own guard that checks the
person's text, such as `len(message.feedback) >= 20`; that check belongs in the
type's `when:` or `max`, which refuse a post instead of sending it elsewhere.
Test: new `TestButtons_ComposeGuards` table: own guard on a given property holds
and loads; own guard on a given property fails and is refused; own guard reads a
left-out property and is refused; own guard `message.decision == 'approve' and
message.feedback != ''` with preset `decision: revise` is refused (no dead
button); own guard reads `session.reminders` and is refused; earlier guard on a
given property that does not hold (the docs example) loads with an empty hidden
set; earlier guard on a left-out property that admits a clicker's role is
refused; earlier `message.decision == 'approve' and message.note != ''` with
preset `decision: revise` is refused (the documented over-refusal); earlier
guard on a left-out property that admits only another role loads.
`TestButtons_Offers` gains the case "earlier guard on a left-out property for
another role hides a compose button from a caller with both roles".
5. **`confirm` has no effect on a compose button; `style` does.** A
compose button's own `confirm:` loads with a warning at `...button.confirm`:
"has no effect: a button with compose: true opens the composer, and Send is the
confirmation". It is dropped from the compiled options. A compose button takes
`style` from its type's `decision_button`, but not `confirm`. When no type
button shows and every preset either sets its own `confirm` or is a compose
button, `decision_button.confirm` warns with "confirm has no effect: every entry
of decide has a preset button that sets its own confirm or opens the composer".
When a required property keeps the type button away, a compose preset never
counts as a one-click button that uses the type's `confirm`. Test:
`internal/realtimecfg/buttonoptions_test.go`, new `TestButtonOptions_Compose`
table: own confirm warns and is dropped; own style kept; type style inherited;
type confirm not inherited; type confirm with only compose presets and no type
button warns with the text above, asserted in full; type confirm with one
one-click preset beside a compose preset does not warn.
6. **The served row says compose.** A compose button's served `button`
carries `compose: true`, its given props in stored form, and its `style`. It
never carries `confirm` or `confirm_text`. A one-click button omits `compose`.
`_actions.entries` keeps its shape: a compose entry is listed by the same
`Offers` rule. The REST head, `read_channel`, `read_session` and `rela channel
read -o json` carry the field. The MCP descriptions no longer call every button
one-click. Test: `internal/realtime/session_test.go` `TestSessionView_Button`,
new case "compose"; `internal/dataentry/transitions_wire_test.go`, new case
"compose button"; `internal/dataentry/testdata/channel_wire_fields.json` gains
`compose`; `internal/openapi/channels_test.go` asserts the property;
`internal/mcp/tools_session_test.go`, new case "session carries a compose
button"; the golden `internal/mcp/testdata/tools_list_channels.golden.json` is
regenerated, and the diff shows the rewritten sentence.
7. **A compose post is a normal post, and its route is the button's.**
Posting only the given props is refused with `invalid_message` (the required
property is missing). Posting the given props plus the left-out value, with
`expect_generation`, moves the session along the button's entry and runs its
outputs. A post at an old generation is refused with `session_moved`. Test:
`internal/realtime/expectgen_test.go`, new `TestPost_ComposePreset`.
`TestButtons_DeciderAgrees` gains a compose entry whose type requires
`feedback`. For each principal it posts the preset with two different feedback
values, "ok" and a 200-character text, and checks that both take the listed
entry, or the entry that hides it. Two values with the same outcome pin that no
routing guard reads the left-out property.
8. **The composer has a compose mode.** In rela-components,
`RlChannelComposer` gains the prop `compose?: ChannelComposeView` (`{ key, type,
label, toLabel?, given }`, where `given` maps a property name to a
`ChannelFieldValue` or a string list) and the event `cancelCompose`. While
`compose` is set, the composer is not editing, and `compose.type` is the
selected type:
- A field whose name is in `given` is not rendered and is not in the
post event's `values` or `rendered`.
- A strip above the fields shows `compose.label` and, for each given
property the presentation does not hide, "Label: value". An enum shows its
option label, a boolean the yes or no text, and a list its items joined by ", ".
The strip is a polite status region.
- The strip has a cancel icon button, "Cancel decision" (nl:
"Besluit annuleren"), that emits `cancelCompose`.
- The session hint names `compose.toLabel` as a definite move, through
`channelSessionHint({ to, guarded: false })`. The send button's
`aria-describedby` names the strip and the hint.
- The drafts are not touched. A given value never enters
`drafts[type]`, so it cannot outlive compose mode. Fields for left-out
properties keep their draft text and the existing seeding, so a required boolean
starts as false.
- A new `compose.key` moves the focus, after the next render, to the
first rendered field that is required and empty, else to the first rendered
field. Each field wrapper carries `data-field="<name>"`, and the focus goes to
the first focusable element inside it. The field is scrolled into view with
`block: 'nearest'`.
- During an edit, `compose` is ignored and the edit renders as today.
It applies again when the edit ends, if the prop is still set.
- When the user selects another type, the composer emits `select` as
today and stops applying `compose`. Ending compose mode is the app's job.

Test: `RlChannelComposer.stories.ts`, new stories, each run with the Storybook
test project. `ComposeMode`: the strip reads "Request changes" and "Topic:
Rollback plan"; Topic is not a field; the focus is on the empty required
Feedback field; the hint names Drafting; the cancel button emits
`cancelCompose`; Send emits only Feedback. `ComposeModeFocus`: the first
left-out field is an enum and the second a ref; the focus lands in the enum
field; a required boolean left out starts unchecked and is skipped.
`ComposeModeTypeSwitch`: the story's `select` handler clears `compose`, as the
app does; after a switch and back, Topic is an empty field and the typed
Feedback is kept. `ComposeModeDuringEdit`: `compose` set during an edit leaves
the edit alone.
9. **The SPA builds a form view for a compose button.** In
`presetViews`, a listed entry whose button has `compose: true` gives a view with
the key `preset:<type>:<entry>`, the button's label, its props, the target's
`toLabel`, `opensForm: true` and `compose: true`. Its look comes from
`applyOptions(view, button, false, {})`, so only `style` applies: no `confirm`,
and no derived `danger`, as for the type button's form. The entry is not left
for the type button. Test:
`frontend/src/components/entity/channels/channelView.test.ts`,
`describe('decisionViews presets')`, new cases: "compose preset opens the form
with its props"; "compose preset into a final state does not ask"; "compose
preset keeps style danger"; "compose preset is not left for the type button".
10. **A click opens compose mode, and the Send carries the preset.**
`onDecide` on a compose view goes through `openForm`. It asks before it discards
an edit and ends a reply the type cannot carry. It records `formDecision = {
messageType, generation, compose }` before it calls `select`, so the composer
renders compose mode before it moves the focus. When `select` returns false, it
clears the record. `compose` holds the view key and a click counter as `key`,
the label, the `toLabel`, the preset in stored form, and `given` through
`fromPropValue`. A computed `composeView` exposes it for the composer.
`sendComposer` posts `{ ...values, ...preset }` with the generation when the
record is of the posted type. The record ends by the existing rules (a type
switch, a landed Send, a send of another type, `session_moved`, a channel
switch, or a head frame with another generation), and also by `cancelCompose`.
Only the head frame tells the user the decision is stale. A second click on the
same button gives a new key and reads the generation again. A click on another
compose button of the same type replaces the record whole. Test:
`frontend/src/components/entity/channels/useSessionDecisions.test.ts`,
`describe('useSessionDecisions form decisions')`, new cases: "a compose preset
records its compose view before it selects"; "its Send carries the preset,
including a list, and the generation"; "cancel ends it and the next Send of the
type carries no preset"; "a type switch ends it"; "button A then button B of one
type carries only B's preset"; "a session move ends it with the stale toast"; "a
compose preset during an edit asks first"; "a refused Send keeps it".
11. **The panel wires it end to end.** `OpenChannel.vue` binds
`:compose="decisions.composeView.value"` and
`@cancel-compose="decisions.cancelCompose"` on the composer. Test:
`frontend/src/components/entity/ChannelsPanel.test.ts`, `describe('ChannelsPanel
preset buttons')`, new case "a compose preset opens compose mode and posts the
preset with the feedback".
12. **E2E.** A lead clicks a compose button and sees the strip with the
given topic, and the focus in the empty Feedback field, which is in the
viewport. The lead writes the feedback and sends. Both users then see the
session back in Drafting. Test: `e2e/tests/channels.spec.ts`, a new fixture
`composeDecisionTest` that calls `writeRoomProject(testProject, { decisions:
true, composePreset: true })`, with a new `describe("compose decisions")` and
the test "a preset that needs text opens the composer". See Test Plan for the
fixture.
13. **Docs.** `GUIDE-realtime.md` documents `compose` in the session key
table, replaces the "A preset cannot leave a required property" bullet with a
"Buttons that open the composer" part, updates the `confirm` bullet under
"Button options", gives Revise `compose: true` in the branching example, and
documents the served `compose`. `just docs` regenerates `docs/realtime.md`.
Test: `internal/realtimecfg/docs_example_test.go` `TestDocsExample_Branching`
asserts that Revise has `Compose` true and that the file loads with no problems
and no warnings; `just docs-check` passes; the prose is reviewed.
14. **The key checker points the way.** `compose:` under
`decision_button:` is refused as an unknown key with the hint "the type's
decision button already opens the composer when a property is required; for a
preset that opens it, add a button with compose: true to the entry". Test:
`buttonoptions_test.go`, the `shapes` table that holds the `label` and `props`
hint cases gains a `compose` case.

## Research

- [x] ~~For larger features: run `/research` to create a structured research doc~~ (N/A: the choices are few and recorded under Approach and Open questions)
- [x] Searched for existing libraries that solve this problem
- [x] Checked codebase for similar patterns or reusable code
- [x] Looked for reference implementations in other projects
- [x] Reviewed relevant rela concepts for prior art

**Research Doc:** N/A

**Existing Solutions:**

- Libraries: none needed. No new Go or npm package.
- Codebase patterns reused:
  - `buttonChecker.preset`, `guards`, `undecidable` and
`loadGuardHolds` (`internal/realtimecfg/buttons.go`): the checks a compose
button reuses, with one extra input, the left-out set.
  - `MessageType.NormalizeProps` (`internal/realtimecfg/props.go`): the
prop rules of a post. Its body gains an internal flag for "do not require"; the
exported function keeps its signature.
  - `predicate.Program.Attributes` and `UsesWhole`: tell which `message`
fields a condition reads, as they already tell which `session` fields.
  - `rawUnread`, `unreadOf` and `confirmOf` with
`yamlscalar.PlainYAML11Bool` (`internal/realtimecfg/raw.go`, `validate.go`,
`buttonoptions.go`): the strict boolean reader.
  - The hints for `decision_button.label` and `decision_button.props` in
`internal/realtimecfg/keys.go`.
  - `ButtonOptions.inherit` and `typeButtonChecker.effect`
(`internal/realtimecfg/buttonoptions.go`): where a compose button drops
`confirm`.
  - `applyOptions(view, options, oneClick, derived)`
(`frontend/src/components/entity/channels/presetViews.ts`): already applies only
`style` when `oneClick` is false.
  - `openForm` and `formDecision` (`useSessionDecisions.ts`): the form
path of a type button and of a choice's Other.
  - The composer's `replyTo` prop and `cancelReply` event: the model for
a state the app owns, shown in a strip with a cancel control.
  - `fromPropValue` (`frontend/src/utils/channelValues.ts`): converts
stored values to field values, here for the strip.
  - The `decisions` fixture in `e2e/tests/realtime-project.ts` and
`decisionTest` in `channels.spec.ts`.
- Reference implementations: data-entry create forms take `prop.<name>`
prefill from the URL (`docs/data-entry.md`, "Pre-filling a create form").
GitHub's "Request changes" opens a review box with the choice made and fixed,
and the comment empty. To change the choice, the person starts over.
- rela prior art: TKT-S9TLJJ (presets, `Offers`, `_actions.entries`, the
known-outcome rule, Q6 names this ticket), TKT-TXZZZ2 (Approach 10: `confirm`
has no effect on any button that opens the composer; the load warns when it
knows), TKT-T9G1AE (type buttons and the form path), TKT-FPXTQJ (form decisions
carry the generation), TKT-03FQPD (Other opens the composer with the answer
type), DEC-6C1NAA (labels are authored).
- TKT-S5OJ7S (`session.default_in: keyed`) fills `MessageType.In` at
load. A compose entry is listed only when the caller may post the type now, so
it follows `in:` with no extra work.

## Approach

- [x] Technical approach chosen and documented
- [x] Approach builds on existing patterns (not reinventing)
- [x] Alternatives considered (document why rejected)
- [x] Dependencies identified (packages, APIs, types)

**Technical Approach:**

1. **What marks a compose button: an explicit key.** `button.compose:
true`. A load-time refusal exists today, and it catches real mistakes: a
forgotten required value on a button meant to be one-click. If compose were
inferred, that mistake would load silently and turn a one-click button into a
form. The data-entry prefill is also explicit (`prop.<name>`), and realtime keys
such as `choices.other: true` name their behaviour. The refusal stays, and its
message says how to opt in. With `compose: true`, a button may still set every
required property: an "Approve with a note" button is a valid use.
2. **Raw config (`internal/realtimecfg/raw.go`).** `rawButton` gains
`Compose rawCompose yaml:"compose"`. `rawCompose` keeps the YAML node, as
`rawUnread` does. A new `composeOf(n)` takes only a YAML boolean: it refuses a
plain YAML 1.1 spelling (`yes`, `no`, `on`, `off`) with
`yamlscalar.PlainYAML11Bool`, and refuses a quoted value or a number as "must be
true or false". A plain `bool` field would not do this: yaml.v3 decodes `yes`
and `"yes"` into a typed bool. The key checker reads keys by reflection, so it
allows `compose` with no change to the allowed-key list.
3. **Partial normalization (`internal/realtimecfg/props.go`).** The body
of `NormalizeProps` moves to `func (mt *MessageType) normalize(in
map[string]any, maxBytes int, requireAll bool)`. `NormalizeProps` calls it with
true, so the post path is unchanged. The load calls it with false for every
preset. Every other rule still runs on the given properties: declared, typed,
enum, max, list items, the property cap and the byte limit.
4. **Compose checks (`internal/realtimecfg/buttons.go`).**
   - `compile` passes the compose flag to `preset` and `guards`, and sets
`TransitionButton.Compose`.
   - `preset(key, raw, compose, sc)` returns the props and the left-out
set. After the `ref`/`markdown` refusal, it calls `normalize(..., false)`. The
left-out set is every declared property of the message type that is not in the
normalized result, sorted. A nil value and an empty list are absent there, as on
the post path. Without compose, the first required left-out property is refused
with the opt-in hint, and the left-out set is then empty, because a click posts
exactly the preset. With compose, a required list in the left-out set is refused
("the composer has no field for a list; set it in props"). `exactInBrowser` runs
for both kinds.
   - A new helper, `readsLeftOut(prog, leftOut) string`, returns the
first left-out name that `prog.Attributes("message")` names, `"message"` when
`prog.UsesWhole("message")` and the set is not empty, or "".
   - Type `when:`: the `today()` refusal stays. When `readsLeftOut`
names a property, the hold check is skipped, because the type's `when:` refuses
a post and does not route it (`whenHolds` in `internal/realtime/service.go`).
Otherwise it must hold, as today.
   - Own guard: `undecidable(t.When)` refuses as today. When
`readsLeftOut` names a property, the button is refused. Otherwise it must hold,
as today.
   - Earlier entries: an entry whose guard is `undecidable` joins the
undecided list, as today. An entry whose guard reads a left-out property is
refused when it admits a role in `classes`, and joins the undecided list when it
does not. This follows the existing rule for a guard error (`err != nil &&
admitsClass`). The rest is unchanged: shadowing, guard errors and the
reachability check.
   - Why this keeps the outcome known. In compose mode the given
properties cannot change (Approach 8), so every guard that reads only given
properties and `session.state` evaluates at Send as it did at load. The decider
tries the entries in order (`sessionDecider`, `internal/realtime/session.go`).
Every earlier entry that could take the post for the clicker is either decided
false, or it hides the button. The own guard holds. So a Send that the server
accepts takes the button's entry. A Send that fails the prop rules or the type's
`when:` is refused and moves nothing.
   - Why not decide more guards. A guard such as `message.decision ==
'approve' and message.note != ''` is decided by the given `decision` alone,
because `and` short-circuits. But the evaluator binds an absent field to nil
without an error (`evalAttr` in `internal/predicate/eval.go`), so the load
cannot tell whether evaluation read a left-out field. Deciding these cases needs
an access-tracking hook in `internal/predicate`. That is not cheap, and
`GuardHolds` maps `ErrNilOperand` to false, which would need a separate path at
load. The plan takes the simple refusal and documents the over-refusal and its
workaround (AC 4).
   - The godoc of `TransitionButton` describes both kinds and the
left-out rule.
5. **Options (`internal/realtimecfg/buttonoptions.go`).** In `compile`,
when the button is a compose button and `raw.Confirm.set()`, warn at
`key+".confirm"` and drop the confirm from `opts`. In `typeButtonChecker.check`,
a compose button calls `inherit` with the type's options minus `Confirm` and
`ConfirmText`. In `effect`'s no-type-button branch, a compose preset counts as
one that the type's `confirm` cannot reach. When any preset is a compose button,
the warning text is "confirm has no effect: every entry of X has a preset button
that sets its own confirm or opens the composer". When none is, the text stays
as today. In the required-property branch, a compose preset is skipped, so it
never counts as a one-click button that uses the type's `confirm`. This is
TKT-TXZZZ2 Approach 10 applied to a compose button. It warns rather than
refuses, as `decision_button` does for a `confirm` with no effect.
6. **Server contract.** `realtime.SessionButtonView` gains `Compose bool
json:"compose,omitempty"`, filled from `b.Compose` in `sessionView`.
`apiwire.ChannelSessionButton` gains the same field, mapped in
`internal/dataentry/channels_wire.go`. The godoc of both types, and of
`TransitionView.Button`, says a button is one-click unless it has `compose`.
`internal/openapi/channels.go` adds `compose` (boolean) to the button schema
with this description: "The button opens the composer with props fixed; the post
needs the message type's other required properties." `_actions.entries` does not
change. It stays a list of `{message_type, entry}` refs. The props and the flag
are config, the same for every reader, and they are already on the served row
the ref points to. Repeating them in a per-caller list would add bytes to every
head and list row for no gain. `Offers` and `GuardHolds` do not change. The
load's hidden set already encodes the compose rule.
7. **MCP and the PoC.** MCP serializes `SessionView` as it is, so
`compose` arrives. In `internal/mcp/tools_channels.go`, the existing sentence in
the `read_channel` and `read_session` descriptions is rewritten, not appended
to: "A row of session.transitions with button is a preset. Without compose it is
one-click and posts its props. With compose: true, post its props unchanged and
add the message type's other required properties." The golden file is
regenerated. The PoC runner reads sessions and posts through MCP, but only the
owner posts `approve` and `revise`, through the web app. So the runner needs no
change. Folding the PoC's two types into one is Q2.
8. **Composer compose mode (rela-components).**
`packages/rela-components/src/components/channel/RlChannelComposer.vue` gains
the `compose` prop and the `cancelCompose` event of AC 8. `types.ts` gains
`ChannelComposeView`. `useMessages.ts` and `messages.nl.ts` gain `cancelCompose`
("Cancel decision", "Besluit annuleren"); the yes and no texts exist. The header
comment describes compose mode, the focus order and the cancel control.
   - The given properties are locked. They are not rendered as fields
and never enter `drafts`. The app adds them to the Send. So the person cannot
change the route, and nothing of the preset stays in a draft after compose mode
ends.
   - This follows `replyTo`: the app owns the state, the composer shows
it in a strip with a cancel control, and the composer emits an event for the
cancel.
   - The lock is not a security boundary. A hand-made post can set any
value, and the server judges it as any post. The lock makes the button keep the
promise of its label, and keeps the load's routing analysis true for the web
app.
   - The focus uses `data-field` on each `.rl-channel-composer__field`
wrapper. `select(type)` uses the same helper with the plain rule (first field),
so there is one focus path.
   - On iOS Safari, `focus()` after an await outside the tap handler
does not open the keyboard. The field is focused and scrolled into view, and one
tap opens the keyboard. This is the same for the type button's form today; the
header comment notes it.
9. **SPA types.** `frontend/src/types/channels.ts`: `ChannelSessionButton`
gains `compose?: boolean`, and its `wireKeys` list gains `'compose'`.
`AppDecisionView` gains `compose?: boolean`.
10. **SPA view (`presetViews.ts`).** In `presetViews`, for `tr.button`
with `compose`, set `opensForm: true`, `compose: true` and call
`applyOptions(view, tr.button, false, {})`. One-click buttons keep
`applyOptions(view, tr.button, true, endTone([tr.to], stateOf))`. The comment "A
preset is one-click" is rewritten to name both kinds.
11. **SPA click (`useSessionDecisions.ts`).**
    - `formDecision` gains `compose?: { key, label, toLabel, preset,
given }`. `composeView` is a computed over it.
    - `openForm` for a view with `compose` records the form decision
(generation read at the click) before it calls `select`, and clears it when
`select` returns false. Other views keep the current order.
    - `sendComposer(type, values, replyTo)` posts `{ ...values,
...f.compose.preset }` when the form decision is of `type` and has `compose`.
The values hold only rendered fields, so the preset overrides nothing the person
typed.
    - `cancelCompose()` clears `formDecision`.
    - `DecisionComposer` does not change. The choice Other path behaves
as today.
12. **Panel (`OpenChannel.vue`).** Binds `:compose` and
`@cancel-compose` (AC 11).

**Alternatives considered:**

- *Infer compose when a required property is missing.* Rejected (see
Approach 1). It removes a useful load error and makes the button's behaviour
depend on the message type's `required:` flags, which can change in another part
of the file.
- *Put the flag in `_actions.entries`.* Rejected (see Approach 6). The
flag is config, not a per-caller fact.
- *Keep the given properties editable.* Rejected after the design review.
An edited `decision` could send a Revise click to Approved, past the Approve
button's confirm, and the load's routing analysis would no longer hold. A person
who clicked the wrong button cancels and clicks the right one.
- *Prefill the draft through `select(type, values)`.* Rejected after the
design review. The values outlived the decision in the draft, so a later plain
post of the type could carry a preset value the person never chose, and a type
switch and back could post half a preset.
- *Skip the hold check of a guard that reads a left-out property, and
hide the button behind such an earlier guard.* Rejected after the design review.
A Send could fall through to a later entry, such as one with outputs, and the
button's outcome would no longer be known.
- *Partial evaluation of guards at load.* Deferred (see Approach 4).
- *A post-answer toast when the session moved elsewhere.* Not needed.
The load rule makes the target known for every accepted Send.
- *Refuse `confirm:` on a compose button.* Rejected for a warning,
matching TKT-TXZZZ2. There, a `confirm` that can have no effect warns.
- *A plain `bool` for `compose`.* Rejected. yaml.v3 reads `yes` and
`"yes"` as true; the repo's boolean keys refuse them.
- *A new `kind:` key (`kind: compose | post`).* Rejected. It is a second
name for a yes/no choice, and `false` is already the default.

**Dependencies:** `internal/realtimecfg`, `internal/realtime` (field and godoc
only), `internal/apiwire/v1`, `internal/dataentry`, `internal/openapi`,
`internal/mcp` (descriptions and golden only). `internal/realtimecfg` already
imports `internal/yamlscalar`, so `.go-arch-lint.yml` does not change. No change
to the stores, `entitymanager`, `dataentryconfig`, `internal/predicate` or the
post path. Frontend: rela-components `RlChannelComposer.vue`, `types.ts` and the
messages; no new packages.

**Files to modify:**

- `internal/realtimecfg/raw.go`, `props.go`, `buttons.go`,
`buttonoptions.go`, `keys.go`.
- `internal/realtime/session.go` (field and godoc).
- `internal/apiwire/v1/channels.go`, `internal/dataentry/channels_wire.go`,
`internal/openapi/channels.go`, `internal/mcp/tools_channels.go`.
- `frontend/packages/rela-components/src/components/channel/RlChannelComposer.vue`,
`types.ts`, `RlChannelComposer.stories.ts`;
`frontend/packages/rela-components/src/composables/useMessages.ts`,
`messages.nl.ts`.
- `frontend/src/types/channels.ts`,
`frontend/src/components/entity/channels/presetViews.ts`,
`useSessionDecisions.ts`, `OpenChannel.vue`.
- `e2e/tests/realtime-project.ts`, `e2e/tests/channels.spec.ts`,
`e2e/pages/channels.page.ts`.
- Tests: `internal/realtimecfg/buttons_test.go`, `buttonoptions_test.go`,
`docs_example_test.go`; `internal/realtime/session_test.go`,
`expectgen_test.go`; `internal/dataentry/transitions_wire_test.go`,
`testdata/channel_wire_fields.json`, `testdata/channel_wire.json`;
`internal/openapi/channels_test.go`; `internal/mcp/tools_session_test.go`,
`testdata/tools_list_channels.golden.json`;
`frontend/src/components/entity/channels/channelView.test.ts`,
`useSessionDecisions.test.ts`;
`frontend/src/components/entity/ChannelsPanel.test.ts`.
- Docs: `docs-project/entities/guides/GUIDE-realtime.md`, and
`docs/realtime.md` regenerated by `just docs`.

## Security Considerations

- [x] Input sources identified (user input, config, external APIs)
- [x] Input validation approach defined (allowlist preferred over blocklist)
- [x] Security-sensitive operations identified (file access, auth, crypto)
- [x] Error handling doesn't leak sensitive information

**Input Sources & Validation:**

- *`compose` and the preset in `realtime.yaml` (operator).* Checked at
load, on reload and by `rela validate`. `compose` takes only a YAML boolean.
Each given value passes the same rules as a post. The allowlist of preset types
is unchanged: string, enum, integer, boolean, date, datetime and string lists. A
failed reload keeps the last good config.
- *The composer's Send (SPA).* A normal post. The server checks the post
gate, `in:`, every required property, the prop rules, the type's `when:`,
mentions, the rate limit, `expect_generation`, the decider and the outputs gate.
A preset cannot skip a required property, because the post path runs
`NormalizeProps` with every property required. A client that ignores the button,
or changes a given value, gets what a hand-made post gets. The lock in the
composer is a UX promise, not a gate.
- *Given values in the browser.* They render as text in the strip
through Vue bindings, never `v-html`. They are config, already served.

**Security-Sensitive Operations:**

- ACL: no new path and no new gate. The served `compose` is config, the
same for every reader; the configuration is not a secret (CLAUDE.md).
`_actions.entries` keeps its rule and shape, so it reveals nothing new.
- A preset still cannot set `markdown` or `ref`. So a compose button
cannot plant a mention or an entity id in the person's message.
- No new write path. The post path, the decider and `GuardHolds` do not
change.
- Outputs: a compose Send cannot reach an entry with outputs that its
button does not name, because no routing guard may read what the person types
(AC 4).
- Errors: load errors and warnings name config paths and values. All of
it is config.

## Test Plan

- [x] Test scenarios documented for each acceptance criterion
- [x] Edge cases identified and documented
- [x] Negative test cases defined (invalid input, error conditions)
- [x] Integration test approach defined (not just unit tests)

**Test Scenarios:**

| AC | Test |
| --- | --- |
| 1 | `TestButtons_Compose`, including the refused spellings |
| 2 | `TestButtons_ComposeProblems`, updated `TestButtons_PropsProblems` |
| 3 | `TestButtons_ComposeProblems` when cases |
| 4 | `TestButtons_ComposeGuards`, `TestButtons_Offers` new case |
| 5 | `TestButtonOptions_Compose` |
| 6 | `TestSessionView_Button` "compose", `transitions_wire_test.go` "compose button", wire fields golden, OpenAPI test, `tools_session_test.go` case, MCP golden |
| 7 | `TestPost_ComposePreset`, `TestButtons_DeciderAgrees` compose entry with two feedback values |
| 8 | Stories `ComposeMode`, `ComposeModeFocus`, `ComposeModeTypeSwitch`, `ComposeModeDuringEdit` |
| 9 | `decisionViews presets` compose cases |
| 10 | `useSessionDecisions form decisions` compose cases |
| 11 | `ChannelsPanel preset buttons` compose case |
| 12 | E2E "a preset that needs text opens the composer" |
| 13 | `TestDocsExample_Branching`, `just docs-check`, reviewed |
| 14 | `buttonoptions_test.go` `shapes` table, `compose` case |

Integration:

- `TestButtons_DeciderAgrees` runs the real service on the memory channel
store. It gains a compose entry and posts the preset with two left-out values
for each principal. This pins the hidden set and the left-out rule against the
decider.
- `ChannelsPanel preset buttons` mounts the panel with mocked reads and
posts. It covers the path through `OpenChannel`, `useSessionDecisions`,
`presetViews` and the real `RlChannelComposer`.
- The E2E test runs the built server and the SPA in a browser. It adds a
`RoomOptions.composePreset` flag to `writeRoomProject`, used only with
`decisions`. `roomRealtimeYaml` throws when `composePreset` is set without
`decisions`, as it does for the other combinations. The flag gives `revise` an
optional `topic: { type: string, label: Topic }` and its review entry `button: {
label: Request changes, props: { topic: Rollback plan }, compose: true }`.
`decisionTest` and its fixture do not change. A new `composeDecisionTest =
base.extend<ChannelFixtures>` writes the project with both flags. New
page-object methods in `e2e/pages/channels.page.ts`: `expectComposeStrip(text)`
and `expectFieldFocused(label)`, which also asserts `toBeInViewport()`. The spec
uses only page-object methods, as `e2e/tests/AGENTS.md` requires. Steps: the
lead posts Contained; both users see Review; the lead sees "Request changes" and
the member does not; the lead clicks it; the strip reads "Topic: Rollback plan"
and Feedback has the focus; the lead posts "Add the rollback plan" in Feedback;
both users see Drafting.

**Edge cases:**

- A compose button that sets every required property: the strip shows
them, and the focus goes to the first rendered field.
- A compose button with no props: the composer opens on the type with
the button's label in the strip and no given values.
- Every entry of a type has a button, some compose: the type button goes,
as today.
- A required boolean left out: the draft seeding sets it to false, so it
is not empty, and the focus skips it.
- An integer or date preset: shown in the strip through `fromPropValue`,
posted in stored form from `formDecision.compose.preset`.
- A list preset on a compose button: shown in the strip, posted from the
preset.
- A preset for an optional field that `data-entry.yaml` hides: not in the
strip, posted from the preset, as a one-click button posts it. A required field
cannot be hidden (`dataentryconfig/channels.go`).
- The person typed into a field that the preset gives, before the click:
that draft value is not shown in compose mode, and the preset wins at Send.
After compose mode ends, the person's own value shows again.
- The person had typed feedback earlier in that type's draft: the
feedback stays.
- Compose button A, then compose button B of the same type: B's record
replaces A's whole. Nothing of A is posted.
- The person cancels, then picks the type by hand: a plain form, with no
given values and no generation.
- The person switches type and back: compose mode has ended, so the form
is plain and no half preset can be posted.
- The person clicks the compose button twice: a new key, the focus rule
runs again, and the generation is read again. Typed text stays.
- An edit starts during compose mode: the edit renders as today. When it
ends, compose mode shows again if the record is still open.
- A Send is refused (`invalid_message`, the type's `when:`): the record
stays, and the composer shows the error with the text kept.
- The session moves while the composer is open: the existing watch ends
the form decision with the stale toast, and the strip goes.
- A reload drops `compose` while the composer is open: the open record
keeps its preset until it ends. Its Send carries the generation, so a changed
route after a session move is refused with `session_moved`.
- An older server that does not know `compose`: it refuses the key as
unknown. Documented as for other new keys.
- An MCP agent that reads a compose button and posts only its props: it
gets `invalid_message`, which names the missing property.

**Negative Tests:**

- Every refusal in AC 2 to AC 4, with its key path.
- `compose: yes`, `compose: "yes"`, `compose: on` and `compose: 1`:
refused at `...button.compose`.
- `compose: true` on an entry with `by: trigger`: refused as today, "no
person could click its button".
- A compose preset that sets a `markdown` property: refused as today.
- `compose:` under `decision_button:`: refused with the hint.
- `ComposeModeDuringEdit`: `compose` does not touch an edit.
- `TestPost_ComposePreset`: the preset alone is refused with
`invalid_message`.

## Risk Assessment

- [x] Technical risks assessed with mitigations
- [x] Security risks assessed (see Security Considerations)
- [x] Effort estimated (xs/s/m/l/xl)

**Risks:**

- *The left-out rule refuses buttons whose route is in fact decided.* An
earlier guard such as `message.decision == 'approve' and message.note != ''`
refuses a compose button with `decision: revise`. The refusal message names the
entry, the guide gives the workaround (move the compose entry up), and the case
is pinned in `TestButtons_ComposeGuards`. Partial evaluation can relax the rule
later without a breaking change, because it only admits configs that are refused
now.
- *Operators lose a guard on the person's text.* An own guard such as
`len(message.feedback) >= 20` is refused. The type's `when:` or `max` covers the
need, and it refuses the post rather than rerouting it. The refusal message says
so.
- *The post path changes by accident.* Mitigated by keeping
`NormalizeProps` as a wrapper with `requireAll` true. The existing
`internal/realtime` value tests stay green.
- *Two sources of compose state.* There is one: `formDecision` in the
app. The composer reads the prop and emits events, as for `replyTo`.
- *Focus fights in the composer.* The record is set before `select`, so
one focus move runs after the render. `openForm` does not call `composer.reply`
or `focus` afterwards. Pinned by the stories' play functions and the E2E focus
check.
- *E2E flakiness.* The new test reuses the decisions project with one
flag and the existing page-object waits. It adds one browser test.

**Effort:** l. The Go side is a strict-boolean key, a partial normalize, the
left-out rule in the guard checks, the options warning and one served field on
four surfaces with their goldens. The frontend side is a new composer mode in
the shared library, with a strip, a cancel control, locked fields and a
field-to-element focus, plus three SPA modules. Tests span five layers, and the
E2E needs its own fixture. The work lands in two commits on one branch: the
rela-components compose mode with its stories first, then the server and the
SPA.

## Documentation Planning

- [x] User-facing docs identified (skip if internal refactor)
- [x] Docs-checklist will be created when entering implementation

**Documentation Impact:**

- [x] `docs-project/entities/guides/GUIDE-realtime.md`, "Sessions":
  - The key table row for `on.<type>.button` names `compose`, a boolean.
  - The branching example gives Revise `compose: true`, so the owner
writes the note in the composer.
  - The bullet "A preset cannot leave a required property" is replaced by
a short part, "Buttons that open the composer". It covers what a click does,
which properties may be left out, the list-property refusal, that the given
properties are fixed, the left-out rule for `when:` and guards with its
workaround, and that the Send is a normal post with `expect_generation`.
  - "Button options": `confirm` on a compose button warns and has no
effect; `style` applies.
  - "In the web app": the strip, the cancel control and the focus.
  - "HTTP API": the served `compose`.
  - `just docs` regenerates `docs/realtime.md`. Direct edits there are
reverted.
- [x] Storybook: the four `ComposeMode` stories in
`RlChannelComposer.stories.ts`.
- [x] ~~docs/metamodel.md - New metamodel features~~ (N/A: no schema.yaml change)
- [x] ~~docs/cli-reference.md - New/changed commands~~ (N/A: no CLI change; the JSON output gains a field)
- [x] ~~docs/data-entry.md - UI changes~~ (N/A: channel UI is documented in GUIDE-realtime.md, "In the web app")
- [x] ~~CLAUDE.md - New patterns or conventions~~ (N/A: follows existing patterns)
- [x] ~~README.md - Project-level changes~~ (N/A)
- [x] ~~N/A - Internal change, no user-facing docs needed~~ (N/A: the change is user-facing; docs listed above)

## Open questions

- **Q1. May a compose preset set a `markdown` property?** TKT-S9TLJJ
refused it because "a click posts text the clicker never sees". That reason does
not hold for a compose button, because the person sees the text before Send.
Decision: keep the refusal in this PR, so all presets follow one rule. Lift it
in a follow-up if an operator asks for a feedback template.
- **Q2. Fold the PoC's `approve` and `revise` into one type?** This is now
possible: one `plan-decision` type with an Approve one-click button and a Revise
compose button. It changes the PoC config and the runner's type names. Decision:
a follow-up ticket in the PoC, not this PR.
- **Q3. Should a compose post reply to the decision's cause?** A one-click
preset replies to the cause when the type may reply. The type button's form does
not. Decision: no reply, as for the type button. This keeps the focus on the
first missing field and the form path the same for both form buttons.

## Design Review

- [x] Run `/design-review` before starting implementation
- [x] All critical/significant findings addressed in plan

**Design Review Findings:** 17 findings (6 significant, 8 minor, 3 nit),
numbered as in `.ignored/dr-53fn0l.tsv`. Resolutions are in
`.ignored/dr-53fn0l-res.txt`. Finding 17 is declined; the rest are addressed.

**What changed after the review:**

- The given properties are locked in compose mode. The composer shows
them read-only in a strip with a cancel control, never writes them to a draft,
and the app adds them at Send (findings 2, 5, 6, 9). The `select(type, values)`
prefill is gone. A `compose` prop and a `cancelCompose` event replace it,
modelled on `replyTo`.
- No routing guard may read a left-out property. The own guard, and an
earlier guard that admits a clicker's role, are refused at load when they do. An
earlier guard for other roles hides the button (findings 1 and 3). The finer
partial evaluation is out of scope, and the over-refusal is documented and
pinned by tests.
- `compose:` is a strict boolean through `yamlscalar.PlainYAML11Bool`
(finding 4).
- One `normalize(..., false)` call for every preset; the hint is derived
from its result (finding 7).
- The `decision_button.confirm` warning has its own text for compose
presets (finding 8).
- The focus uses `data-field` wrappers and scrolls into view; the strip
is a status region named by the send button; the iOS keyboard limit is noted
(findings 10, 14).
- The MCP sentence is rewritten, and the godoc and comments that say
one-click are listed (finding 11).
- A separate E2E fixture, `composeDecisionTest` (finding 12).
- `TestButtons_DeciderAgrees` posts two left-out values and expects one
route (finding 13).
- Effort is l, in two commits (finding 15).
- A key hint for `decision_button.compose` (finding 16, new AC 14).

---
id: PLAN-VIQD6R
type: planning-checklist
title: 'Planning: Authored confirm and style for session decisions'
started: "2026-10-30"
completed: "2026-10-30"
status: done
---

<!-- @managed: claude-workflow v1 -->

## Understanding

- [x] Problem/requirements clearly understood
- [x] Scope defined (what's in/out documented below)
- [x] Acceptance criteria documented with specific test scenarios

**Problem.** The web app derives how a one-click decision looks. A button asks
first when every target is final, and it is drawn in the danger tone when every
target ends cancelled or failed (`endTone` in
`frontend/src/components/entity/channels/presetViews.ts`, from TKT-T9G1AE and
TKT-S9TLJJ). An operator cannot change this. Two examples from the ticket: an
Allow that runs a tool should ask first, although its target `working` is not
final. An Abandon should look dangerous, although the operator did not give its
state the status `cancelled`.

The ticket description says to put `confirm:` and `style:` on the entry
(`on.<type>`). That is outdated. The TKT-S9TLJJ design review (its Q5) decided
otherwise:

- A preset button takes them inside its `button:`.
- The type's decision buttons get a type-level key. The type button stands
for every listed entry of a type without a button, so a per-entry key would
force those entries to agree, and would give no single answer when they do not.

**Terms used below.**

- *Preset button*: the web app button of an entry with `button:`
(TKT-S9TLJJ).
- *Type button*: the per-type decision button of TKT-T9G1AE. It stands for
the caller's listed entries of the type that have no `button:`.
- *Decision buttons of a type*: the type button and the type's preset
buttons. Choice buttons (TKT-03FQPD), message actions and the composer are not
decision buttons.
- *Opens the composer*: a decision button that opens the composer instead
of posting. Today a type button does so when it has several targets, a guard, an
earlier guarded entry, or a required property. TKT-53FN0L may add other reasons.
- *One-click*: a decision button that posts at once, so does not open the
composer. A preset button is one-click today.
- *Derived rule*: the current `endTone` rule above. It applies only to a
one-click button.
- *Button options*: the two new keys, `confirm` and `style`.

**Scope (in):**

- `realtime.yaml`: `on.<type>.button.confirm` and `on.<type>.button.style`
on a preset button, and `message_types.<type>.decision_button` with only
`confirm` and `style` for all decision buttons of the type. Checked at load, on
reload and by `rela validate`.
- Inheritance per key, resolved at load: a preset button's option, else the
type's `decision_button` option, else the derived rule.
- Load checks for both keys, including texthazard on a confirm text.
- A small leaf package, `internal/yamlscalar`, with the YAML 1.1 boolean
check. `realtimecfg` and both call sites in `dataentryconfig` use it.
- A named limit, `realtimelimits.MaxConfirmTextRunes`.
- Served config: the row's `button` on `session.transitions` gains the
resolved options. The message type gains `decision_button` in the channel schema
descriptor (`GET /api/v1/_schema/channels`) and in `read_channel`'s message
types. OpenAPI and MCP tool descriptions follow.
- SPA: the derived rule stays in `endTone`, unchanged. A new
`applyOptions` applies the options over it for both kinds of button. The confirm
dialog shows the authored text, or a translated body that fits a non-final
target.
- rela-components: `ChannelDecisionView` gains `primary`, and
`RlChannelDecisions` draws it as the primary variant. No other library field is
added.
- Docs in `GUIDE-realtime.md`, regenerated into `docs/realtime.md`.

**Scope (out):**

- Options on choice buttons (TKT-03FQPD), lifecycle buttons or message
actions. The docs say `decision_button` does not affect them.
- A per-state type-level key. See Alternatives.
- Preset buttons that open the composer prefilled (TKT-53FN0L). The rule
for `confirm` is stated for any button that opens the composer, so it covers
that ticket in whatever shape it takes; see "Open to TKT-53FN0L".
- Any server-side confirmation. The options are presentation. A post from
MCP, the CLI or the REST API is never asked anything.
- Any change to the post path, the decider, `_actions` or the store.

**Acceptance Criteria:**

1. **The keys load.** On a preset button, `button: { label: Allow, props:
{}, confirm: true, style: primary }` loads. `confirm` also takes `false` or a
text, such as `confirm: "This runs the tool on your machine."`. On a message
type, `decision_button: { confirm: true, style: danger }` loads. A null
`decision_button:` loads as absent. An empty mapping `decision_button: {}` loads
as absent with a warning ("sets neither confirm nor style"). A null `confirm` or
`style` loads as absent. The compiled `TransitionButton` has `Options
ButtonOptions`, and the compiled `MessageType` has `DecisionButton
*ButtonOptions`, nil when absent. `ButtonOptions` has `Confirm` of type
`ConfirmMode`, whose zero value is `ConfirmDerived` (then `ConfirmAsk`,
`ConfirmNever`), `ConfirmText` and `Style` (empty, `default`, `primary` or
`danger`). A text sets `ConfirmAsk` and the folded text. Test:
`internal/realtimecfg/buttonoptions_test.go` (new) `TestButtonOptions_Load`
(preset with true, false and a text; type with both keys; null type key; empty
type key warns; null confirm; null style; text folded with NFC and trimming; a
`|` block text keeps no trailing newline; absent keys give `ConfirmDerived` and
an empty style).
2. **Presets inherit per key.** At load, each preset button of a type takes
every option it does not set from the type's `decision_button`. A preset's own
value always wins, including `style: default` and `confirm: false`.
`TransitionButton.Options` holds the resolved values, so the served row is the
only source the SPA reads for a preset. Test: `TestButtonOptions_Inherit` table:
"a preset inherits each unset type option"; "a preset's style default overrides
the type's danger"; "a preset's confirm false overrides the type's text"; "a
preset's text replaces the type's text"; "a type without decision_button leaves
presets as authored"; "a refused decision_button gives nothing to inherit".
3. **The values are checked.** Refused, each with its key path, such as
`...on.allow.button.confirm` or
`...message_types.abandon.decision_button.style`:
   - a `style` other than `default`, `primary` or `danger` (case-sensitive);
   - a `confirm` that is a mapping or a list;
   - a `confirm` that is a number;
   - a `confirm` that is not single- or double-quoted and reads, after
trimming, as `yes`, `no`, `on`, `off`, `y` or `n` in any case
(`yamlscalar.PlainYAML11Bool`, the rule of an action's `confirm:` in
`data-entry.yaml`; a quoted one is text);
   - an empty text, or one that is empty after trimming ("write false to
turn the question off, or the text to show");
   - a text over `realtimelimits.MaxConfirmTextRunes` (300) runes after
`foldDisplay`;
   - a text with a line break, checked before texthazard so the operator
gets the specific message ("must be one line");
   - a text with another display hazard (`texthazard.Find`);
   - `label` or `props` under `decision_button`, by the key checker;
   - a scalar `decision_button: true`: "must be a mapping".

Test: `TestButtonOptions_Problems` table, one case per bullet, for both places
where it applies.
4. **The type-level key is placed where it can apply.** All checks live in
one `typeButtonChecker`, called beside `choiceChecker`.
- Refused on a channel type without a session: "only on a channel type
with a session".
- Refused on an `ephemeral` type: "only for log messages".
- Warning when no state has an entry of the type that a person could
click, by the same predicate as `personClasses` (not `by: trigger`, and some
role may post it): "no person moves a session with X, so decision_button has no
effect".
- Warning when `confirm` is set, the type has a required property, and
no preset button of the type inherits it: "confirm has no effect: the type's
button opens the composer because P is required, and no preset button of X uses
it". This is the general rule of Approach 8 applied to what the load can know.

Test: `TestTypeButton_Checks` table (one case per bullet, plus "type with an
unbuttoned entry loads clean" and "confirm with a required property and a preset
button loads clean").
5. **The options are served.** A row of `session.transitions` with a
button carries `confirm` (true or false, absent when neither the preset nor the
type sets it), `confirm_text` (absent without a text) and `style` (absent when
neither sets it) inside `button`. A message type with `decision_button:` carries
`decision_button: { confirm, confirm_text, style }` with the same rules, in the
channel schema descriptor and in `read_channel`'s `message_types`. Invariant:
`confirm_text` present implies `confirm` is true, and `confirm: false` never
carries a text. All are config, the same for every reader. Test:
`internal/realtime/session_test.go` `TestSessionView_ButtonOptions` (includes
the invariant and an inherited value); `internal/realtime/describe_test.go`
`TestDescribe_DecisionButton` (Describe and the read's `MessageTypeView`);
`internal/dataentry/transitions_wire_test.go` new case "button options";
`internal/dataentry/channels_schema_test.go` `TestChannelSchema_DecisionButton`;
`internal/dataentry/testdata/channel_wire_fields.json` gains the new fields and
`ChannelDecisionButtonSchema`; `internal/openapi/channels_test.go`
`TestOpenAPI_ChannelButtonOptions` (maxLength equals
`realtimelimits.MaxConfirmTextRunes`); `internal/mcp/tools_session_test.go`
`TestReadSession` case "button carries confirm and style"; the regenerated
`internal/mcp/testdata/tools_list_channels.golden.json`.
6. **Without the keys, nothing changes.** A file without options gives the
same views as before: confirm only on a one-click button into a final target,
danger only on a one-click button into cancelled or failed targets, no
`primary`, and a button that opens the composer stays plain. This holds because
`endTone` is unchanged and `applyOptions` adds no key when no option is set. It
never writes a key with the value `false` or `undefined`, so the existing
`toEqual` cases still match. Test: the existing `decisionViews` and
`decisionViews presets` cases in `channelView.test.ts` stay green without edits;
new cases "no options keep the derived rule" and "form button into a cancelled
state stays plain without options" in `describe('decisionViews options')`.
7. **The SPA applies the options.** `endTone(targets, stateOf)` stays as it
is. A new `applyOptions(view, options, derived)` in `presetViews.ts` applies the
options:
- On a one-click button, it starts from the derived result and each
option overrides its own part: `confirm: true` or a text asks into any target,
final or not; `confirm: false` never asks; `style: danger` sets `danger`;
`style: primary` sets `primary` and drops a derived `danger`; `style: default`
drops a derived `danger`. An absent option keeps its derived part.
- On a button that opens the composer, only an authored `style`
applies. The derived rule contributes nothing, as today, and `confirm` is
ignored.

A preset button reads the options from its row's `button`, where the type's
options are already merged. The type button reads them from its message type's
`decision_button` in the descriptor. Test:
`frontend/src/components/entity/channels/channelView.test.ts`, new
`describe('decisionViews options')`: "confirm true asks into a non-final state";
"confirm false does not ask into a final state"; "confirm text is carried";
"style danger without a cancelled target"; "style default drops the derived
danger"; "style primary"; "type button reads the message type's
decision_button"; "type button that opens the form keeps style and drops
confirm"; "preset options do not reach the type button"; "a preset reads the
merged row options".
8. **The dialog says the right thing.** With an authored text, the dialog
body is that text, as written. Without one, a final target keeps
`channels.decisionConfirmBody` ("This ends the session in {state}. You cannot
undo it."). A non-final target uses a new key,
`channels.decisionConfirmMoveBody`: "This moves the session to {state}." in
`en.json` and "Hiermee gaat de sessie naar {state}." in `nl.json`. The dialog's
confirm button follows the outcome, not the button's style: it is red when the
target ends cancelled or failed, or when the button has `style: danger`. So
`style: primary` or `style: default` into a cancelled final state gives a plain
button and a red confirm in the dialog that says the session ends. Test:
`frontend/src/components/entity/channels/useSessionDecisions.test.ts` cases
"authored confirm text is the dialog body", "confirm into a non-final state says
it moves the session", "confirm into a final state keeps the end text",
"declined authored confirm posts nothing", "style primary into a cancelled state
keeps the dialog red"; the locale parity test covers the new key.
9. **The library draws `primary`.** `ChannelDecisionView` gains `primary?:
boolean`. `RlChannelDecisions` passes `variant="primary"` when it is set, and
`secondary` otherwise. `tone` still follows `danger`. The docs of `confirm` and
`danger` in `types.ts` stop saying why the app sets them ("the move ends the
session"), because authored options now set them too. Test:
`frontend/packages/rela-components/src/components/channel/RlChannelDecisions.stories.ts`
new story `Primary` whose play asserts the class `rl-button--primary`; the
`Danger` story stays green.
10. **Docs.** `GUIDE-realtime.md` documents both keys, their values, the
inheritance, the load checks and the web app behavior (see Documentation
Impact). The branching example gains `style: primary` on Approve, and a new
Allow and Abandon example shows `decision_button`. Test:
`internal/realtimecfg/docs_example_test.go` `TestDocsExample_Branching` also
asserts that Approve's `Options.Style` is `primary` and Revise's is empty. New
`TestDocsExample_DecisionButton` loads the Allow and Abandon example with no
problems and no warnings, and asserts `MessageType.DecisionButton` for both
(Allow: `ConfirmAsk` and the folded text; Abandon: style `danger`). The prose
review is an item on the docs checklist.

## Research

- [x] ~~For larger features: run `/research` to create a structured research doc~~ (N/A: small change; the choices are recorded under Approach and Open questions)
- [x] Searched for existing libraries that solve this problem
- [x] Checked codebase for similar patterns or reusable code
- [x] Looked for reference implementations in other projects
- [x] Reviewed relevant rela concepts for prior art

**Research Doc:** N/A

**Existing Solutions:**

- Libraries: none needed. No new Go or npm package.
- Codebase patterns reused:
  - `dataentryconfig.ActionConfirm` (`internal/dataentryconfig/config.go`):
an action's `confirm:` is a boolean or a text and refuses an unquoted YAML 1.1
boolean spelling. Operators already know this shape. The union is not shared,
because the meaning differs: here `false` turns off a derived question and an
empty text is refused, while in `data-entry.yaml` an empty text means no
confirm. Only the scalar rule is shared, through `internal/yamlscalar`.
  - `isYAML11Bool` and its quoted-style test, today in
`dataentryconfig/config.go` and `dataentryconfig/dashboard.go`. Both move to the
leaf.
  - The SPA's action confirm (`EntityList.vue`, `useToolbarActions.ts`):
the authored text replaces the translated body; the title and confirm label
stay.
  - `RlButton` (`packages/rela-components/src/components/common/RlButton.vue`):
`variant` is the visual weight (`primary`, `secondary`, `ghost`, `subtle`) and
`tone` the meaning (`default`, `danger`). Decision buttons use `secondary` with
the tone from `danger`.
  - `realtimecfg.Choices` on `MessageType`, served through
`describeChoices` into `MessageTypeView`, `MessageTypeDescription`,
`v1.ChannelChoicesSchema` and `choicesSchema()` in OpenAPI. The
`decision_button` options follow the same path.
  - `choiceChecker` (`choices.go`): one checker for a message-type key that
needs the compiled session, called once in `validate.go` after the session is
compiled. It refuses an ephemeral type and warns when the buttons cannot show.
`typeButtonChecker` follows it.
  - `rawPresetValue` and `keyChecker.presetValue` (`raw.go`, `keys.go`): a
raw type that keeps its YAML node and has its own key check. `rawConfirm`
follows it.
  - `buttonChecker.compile` (`buttons.go`, called from `branches.go`) and
`personClasses` (`buttons.go`): where a preset button is compiled and who could
click it.
  - `foldDisplay` (`props.go`) and `texthazard.Find`: the label rules of
TKT-S9TLJJ. `texthazard.Find` already treats a newline as a hazard.
  - `realtimelimits`: the realtime caps that `internal/openapi` already
quotes.
  - `endTone` (`presetViews.ts`): the one place both buttons get their
derived look.
- Reference implementations: GitHub's merge button styles and Slack Block
Kit buttons both use a small fixed style set (`primary`, `danger`, plain) and an
optional confirm dialog with authored text. Slack's `confirm` object is the
closest match; its text is capped at 300 characters.
- rela prior art: TKT-T9G1AE (decision buttons and the derived rule),
TKT-S9TLJJ (preset buttons; Q5 decided the placement), TKT-03FQPD (choices),
DEC-6C1NAA (labels are authored, never derived). Authored config text is shown
as written and not translated (`GUIDE-data-entry.md`, "Text you write in the
configuration").

## Approach

- [x] Technical approach chosen and documented
- [x] Approach builds on existing patterns (not reinventing)
- [x] Alternatives considered (document why rejected)
- [x] Dependencies identified (packages, APIs, types)

**Technical Approach:**

1. **Shape.**

   ```yaml
   message_types:
     allow:
       label: Allow
       decision_button: { confirm: "This runs the tool on your machine." }
     abandon:
       label: Abandon
       decision_button: { style: danger }
     decide:
       properties: { decision: { type: plan_decision, required: true } }
   session:
     states:
       review:
         on:
           allow: { to: working }
           abandon: { to: stopped }
           decide:
             - when: message.decision == "approve"
               to: approved
               button: { label: Approve, props: { decision: approve }, style: primary }
             - to: drafting
               button: { label: Revise, props: { decision: revise } }
   ```

2. **The type-level key is `message_types.<type>.decision_button`.**
Reasons:
   - On a message type, `actions:` and `choices:` describe buttons drawn
on a message of that type. A key named `button` would read the same way.
`decision_button` says that it styles the type's decision buttons in the session
header and rows, and nothing else.
   - It sits next to the type's `label:`, which is the type button's text.
   - It is independent of the entries, so entries without a button no
longer have to agree on anything. A new or reordered entry cannot change it.
   - The key checker allows only `confirm` and `style`, because the label
is the type's label and the type button posts no preset.
   - `decision:` was rejected as a name, because the docs example already
has a property named `decision` on the same type. `decision_button` does not
collide with it.

3. **Inheritance.** Each preset button of a type takes each option it does
not set from the type's `decision_button`; the derived rule is the last
fallback. So an operator who marks Abandon `style: danger` also gets a red
"Abandon (no reason)" preset, and an "Allow once" preset still asks. A preset
opts out with `style: default` or `confirm: false`. The merge runs at load in
`typeButtonChecker`, after the session is compiled, so
`TransitionButton.Options` is already resolved and the SPA reads one source per
button.

4. **Values.**
   - `confirm`: `true`, `false` or a text. Absent keeps the inherited or
derived value. A text implies `true`. This is the shape of an action's
`confirm:`, with one difference: `false` is meaningful here, because it turns
off the derived question. A text is 1 to `realtimelimits.MaxConfirmTextRunes`
runes after `foldDisplay`, as the label is counted. The limit is 300, the cap
Slack Block Kit puts on its confirm text; it fits a short dialog. OpenAPI quotes
the same constant as `maxLength`, which also counts code points, so the two
agree. A text has no line break and no display hazard. The line-break refusal
matches the label rules and gives a clearer message than the texthazard refusal
that would catch it anyway; `ConfirmModal` would show a break (`white-space:
pre-line`), so the reason is consistency, not display. `foldDisplay` trims, so
the trailing newline of a `|` block text is not a break.
   - `style`: `default`, `primary` or `danger`. Absent keeps the inherited
or derived value. The values map onto `RlButton`: `default` is `secondary` with
tone `default`; `primary` is `primary` with tone `default`; `danger` is
`secondary` with tone `danger`, as the derived danger draws now. `ghost` and
`subtle` are left out: a decision is an action the session waits for, and those
variants read as secondary chrome. Separate authored `variant` and `tone` keys
are left out too: three values cover the two examples and Approve, and a fourth
combination (`primary` plus `danger`) can be added later without a break.

5. **i18n.** A confirm text is config text and is shown as written, like
every label in `realtime.yaml`. The docs say so. Without a text, the dialog body
is translated: the existing `decisionConfirmBody` for a final target, and a new
`decisionConfirmMoveBody` for a non-final one, because the existing text ("This
ends the session") would be false there. The dialog title and confirm label stay
the button's label.

6. **Config (`internal/realtimecfg`).**
   - `raw.go`: `rawButton` gains `Confirm rawConfirm yaml:"confirm"` and
`Style string yaml:"style"`. `rawMessageType` gains `DecisionButton
*rawDecisionButton yaml:"decision_button"` with the same two fields.
`rawConfirm` keeps its YAML node, as `rawPresetValue` does.
   - `keys.go`: `walk` special-cases `rawConfirm`: a scalar passes;
anything else is "must be true, false or the text to show".
   - `buttonoptions.go` (new): `ButtonOptions`, `ConfirmMode`
(`ConfirmDerived` as the zero value, `ConfirmAsk`, `ConfirmNever`),
`ButtonStyle` and its three constants, `ButtonOptions.inherit(from)`,
`optionsOf(v, key, confirm, style) (ButtonOptions, bool)` for AC 3, and
`typeButtonChecker{*validator}.check(raw, ct)` for AC 1, 2 and 4. A new file,
because `buttons.go` already has 410 lines.
   - `typeButtonChecker.check` does, per message type with
`decision_button`: refuse without a session or on an ephemeral type; parse the
options; set `MessageType.DecisionButton`; warn as in AC 4; then merge the
options into each compiled preset button of the type.
   - `buttons.go`: `compile` calls `optionsOf` and sets
`TransitionButton.Options`. A bad option refuses the button, as a bad label
does.
   - `validate.go`: one call, `typeButtonChecker{v}.check(raw, ct)`, next
to `choiceChecker{v}.choices(raw, ct)`, after the session is compiled. No second
hook in `messageInCheck.withoutSession`.
   - `compiled.go`: `MessageType` gains `DecisionButton *ButtonOptions`,
cloned by `clone.go`.

7. **YAML 1.1 booleans (`internal/yamlscalar`, new leaf).** One function,
`PlainYAML11Bool(n *yaml.Node) bool`: true when the node is a scalar, not
single- or double-quoted, whose value trimmed of whitespace is `yes`, `no`,
`on`, `off`, `y` or `n` in any case. About 15 lines and a table test.
`.go-arch-lint.yml` gains a component entry beside `texthazard`. It needs no
`deps` entry, because it imports only `yaml`, which is in `commonVendors`.
`realtimecfg` and `dataentryconfig` add it to `mayDependOn`. `dataentryconfig`'s
two call sites (`ActionConfirm.UnmarshalYAML`, `DashboardRef.UnmarshalYAML`)
switch to it and `isYAML11Bool` is deleted. This is cheap and safe: the existing
check is the same bitmask test on the same spellings. The one change is that a
`|` block scalar holding `yes` is now refused too; today only `|-` is, because
the trailing newline hides the spelling. Their existing tests stay green, and
one case is added for the block scalar.

8. **Served config.**
   - `internal/realtime`: `ButtonOptionsView` with `Confirm *bool
json:"confirm,omitempty"`, `ConfirmText string json:"confirm_text,omitempty"`
and `Style string json:"style,omitempty"`, built by one `buttonOptionsView(o)`
function. `SessionButtonView` embeds `ButtonOptionsView`, so the three fields
sit flat in `button`. `MessageTypeView` (`read.go`) and `MessageTypeDescription`
(`describe.go`) gain `DecisionButton *ButtonOptionsView
json:"decision_button,omitempty"`, nil when absent.
   - The wire splits the YAML union into `confirm` and `confirm_text`. A
union served as "boolean or text" would also work in JSON, since absent, false,
true and a text are four distinct states. The split is chosen because a client
then reads one boolean and an optional string, and OpenAPI needs no `oneOf`. It
differs knowingly from an action's served `confirm`, which is a union. The
invariant is pinned by tests and stated in the OpenAPI description:
`confirm_text` present implies `confirm: true`, and `confirm: false` never
carries a text.
   - `internal/apiwire/v1/channels.go`: `ChannelSessionButton` gains the
three fields; new `ChannelDecisionButtonSchema` with the same three;
`ChannelMessageTypeSchema` gains `DecisionButton *ChannelDecisionButtonSchema
json:"decision_button,omitempty"`. The fields are written out, not embedded, so
the wire-field golden lists them plainly.
   - `internal/dataentry/channels_wire.go` and `channels_schema.go` map
them.
   - `internal/openapi/channels.go`: one `buttonOptionsProps()` helper adds
`confirm` (boolean), `confirm_text` (string, `maxLength`
`realtimelimits.MaxConfirmTextRunes`) and `style` (enum) to the session button
schema and to a new decision button schema on the message type.
   - `internal/realtimelimits/limits.go`: `MaxConfirmTextRunes = 300`.
   - `internal/mcp/tools_channels.go`: the `read_channel` and
`read_session` descriptions gain one sentence: "button.confirm and button.style
on a transition row, and decision_button on a message type, are how the web app
draws that decision; they do not change what a post does." The golden is
regenerated.

9. **SPA.**
   - `frontend/src/types/channels.ts`: `ChannelButtonOptions` with
`confirm?: boolean`, `confirm_text?: string` and `style?: 'default' | 'primary'
     | 'danger'`. `ChannelSessionButton` extends it. `ChannelMessageTypeSchema` gains
`decision_button?: ChannelButtonOptions`. The `wireKeys` lists follow
(`ChannelSessionButton`, `ChannelDecisionButtonSchema`).
   - `presetViews.ts`: `endTone` is unchanged. New `export interface
AppDecisionView extends ChannelDecisionView` with the app-only dialog fields
`confirmText?: string`, `movesOnly?: true` (an authored confirm into a non-final
target) and `outcomeDanger?: true` (an authored style removed a derived danger;
the dialog stays red). Each is set only when an authored option causes it. New
`applyOptions(view, options, oneClick, derived)` implements AC 7. A preset view
passes `tr.button`.
   - `channelView.ts`: the type button calls `applyOptions` with
`mt.decision_button`. When it opens the form, `derived` is empty, as today, and
only `style` applies.
   - `useSessionDecisions.ts` `onDecide`: the dialog message is
`view.confirmText`, else `decisionConfirmMoveBody` when `view.movesOnly`, else
`decisionConfirmBody`. The dialog's `danger` is `view.danger ||
view.outcomeDanger`.
   - `frontend/src/i18n/locales/en.json` and `nl.json`:
`channels.decisionConfirmMoveBody`.
   - rela-components `types.ts`: `ChannelDecisionView` gains only
`primary?: boolean`; the docs of `confirm` and `danger` are reworded (AC 9).
`RlChannelDecisions.vue` binds `variant`.

10. **Open to TKT-53FN0L.** The rule is general: `confirm` has no effect on
any decision button that opens the composer, whatever makes it open, because the
composer's Send is the confirmation. `style` applies to such a button. The load
warns when it can know that a `confirm` never reaches a one-click button (AC 4).
TKT-53FN0L lets a preset leave out a required prop and open the composer
prefilled. It may do so with a new key or by inference from the preset. In
either shape the same rule applies, and `applyOptions` already handles a button
that opens the composer.

**Alternatives considered:**

- *Options on the entry (the ticket's first idea).* Rejected by the
TKT-S9TLJJ design review: the type button aggregates entries, which would have
to agree.
- *`message_types.<type>.button` as the name.* Rejected. On a message type
it reads as a button drawn on the message, as `actions:` and `choices:` are. See
Approach 2.
- *A session-level map, `session.decisions.<type>: { confirm, style }`.*
Rejected. It repeats the type name in a second place, needs a cross reference
check, and splits a type's presentation over two blocks.
- *A per-state key, `states.<state>.buttons.<type>`.* Rejected. It is
verbose and brings back agreement, now between states. An operator who needs a
different look per state can use preset buttons, which are per entry.
- *No inheritance: each button has one source.* Rejected. A type marked
`style: danger` would give a plain preset of the same type, which is a surprise,
and every preset would repeat the type's options. The per-key fallback is short
and predictable, and the opt-outs already exist.
- *`confirm` as a boolean only.* Rejected. The Allow example needs to say
why it asks ("This runs the tool on your machine."), and the derived final-state
body is wrong for a non-final target anyway. An action's `confirm:` already
takes a text.
- *Translatable confirm texts (a key per locale).* Rejected. No other text
in `realtime.yaml` or `data-entry.yaml` is translated.
- *`style` as the full `RlButton` matrix (`variant` and `tone`).* Rejected;
see Approach 4.
- *Serve `confirm` as one field, boolean or text, like an action.*
Rejected; see Approach 8.
- *A third copy of the YAML 1.1 boolean check in `realtimecfg`.* Rejected.
The leaf costs about 15 lines and an arch-lint entry, and removes two copies.
- *App-only dialog fields on the library's `ChannelDecisionView`.*
Rejected. The library never reads them; they live on `AppDecisionView`.
- *Let the server enforce `confirm` (a two-step post).* Rejected. A
confirmation is a UI step. The server already guards a post by roles, guards and
`expect_generation`.

**Dependencies:** `internal/realtimecfg`, `internal/realtimelimits`,
`internal/yamlscalar` (new), `internal/dataentryconfig` (two call sites),
`internal/realtime`, `internal/apiwire/v1`, `internal/dataentry`,
`internal/openapi`, `internal/mcp` (descriptions and golden). New import edges:
`realtimecfg` and `dataentryconfig` to `yamlscalar`, listed in
`.go-arch-lint.yml`. `openapi` already imports `realtimelimits`. No change to
stores or `entitymanager`. Frontend: no new packages; one small library change.

**Files to modify:**

- `internal/yamlscalar/yamlscalar.go` and `yamlscalar_test.go` (new),
`.go-arch-lint.yml`.
- `internal/dataentryconfig/config.go`, `internal/dataentryconfig/dashboard.go`.
- `internal/realtimelimits/limits.go`.
- `internal/realtimecfg/buttonoptions.go` (new), `buttons.go`, `raw.go`,
`keys.go`, `validate.go`, `compiled.go`, `clone.go`.
- `internal/realtime/session.go`, `read.go`, `describe.go`.
- `internal/apiwire/v1/channels.go`, `internal/dataentry/channels_wire.go`,
`internal/dataentry/channels_schema.go`, `internal/openapi/channels.go`,
`internal/mcp/tools_channels.go`.
- `frontend/src/types/channels.ts`,
`frontend/src/components/entity/channels/presetViews.ts`,
`frontend/src/components/entity/channels/channelView.ts`,
`frontend/src/components/entity/channels/useSessionDecisions.ts`,
`frontend/src/i18n/locales/en.json`, `frontend/src/i18n/locales/nl.json`.
- `frontend/packages/rela-components/src/components/channel/types.ts`,
`RlChannelDecisions.vue`, `RlChannelDecisions.stories.ts`.
- Tests: `internal/realtimecfg/buttonoptions_test.go` (new),
`internal/realtimecfg/docs_example_test.go`,
`internal/dataentryconfig/config_test.go` (block scalar case),
`internal/realtime/session_test.go`, `internal/realtime/describe_test.go`,
`internal/dataentry/transitions_wire_test.go`,
`internal/dataentry/channels_schema_test.go`,
`internal/dataentry/testdata/channel_wire_fields.json`,
`internal/dataentry/testdata/channel_wire.json`,
`internal/openapi/channels_test.go`, `internal/mcp/tools_session_test.go`,
`internal/mcp/testdata/tools_list_channels.golden.json`,
`frontend/src/components/entity/channels/channelView.test.ts`,
`frontend/src/components/entity/channels/useSessionDecisions.test.ts`,
`frontend/src/components/entity/ChannelsPanel.test.ts`.
- Docs: `docs-project/entities/guides/GUIDE-realtime.md`, then `just docs`
to regenerate `docs/realtime.md`.

## Security Considerations

- [x] Input sources identified (user input, config, external APIs)
- [x] Input validation approach defined (allowlist preferred over blocklist)
- [x] Security-sensitive operations identified (file access, auth, crypto)
- [x] Error handling doesn't leak sensitive information

**Input Sources & Validation:**

- *`confirm` and `style` in `realtime.yaml` (operator).* Checked at load,
on reload and by `rela validate`. `style` is an allowlist of three values.
`confirm` is a boolean or a bounded text: folded, 1 to
`realtimelimits.MaxConfirmTextRunes` runes, one line, no display hazard
(`texthazard.Find`), so the dialog cannot show text that reads as something
else. Unknown keys under either `button:` or `decision_button:` are refused by
the key checker. A failed reload keeps the last good config.
- *Served values.* The confirm text reaches the SPA as `confirm_text` and
is shown through `ConfirmModal`'s `description` prop, which renders plain text,
never `v-html`. `style` only selects one of three fixed classes.

**Security-Sensitive Operations:**

- None new. The options are presentation. No new write path, gate or
affordance. The post path is unchanged, so a client that skips the dialog gets
what any post gets.
- The options are config, the same for every reader. The configuration is
not a secret (CLAUDE.md), so serving them in the schema descriptor and on every
row needs no gating. They name no entity and no person.
- `confirm: false` on a final transition removes a UI safety net. That is
the operator's choice; the docs say what it does.
- Errors: load errors name config paths and values. All are config.

## Test Plan

- [x] Test scenarios documented for each acceptance criterion
- [x] Edge cases identified and documented
- [x] Negative test cases defined (invalid input, error conditions)
- [x] Integration test approach defined (not just unit tests)

**Test Scenarios:**

| AC | Test |
| --- | --- |
| 1 | `TestButtonOptions_Load` |
| 2 | `TestButtonOptions_Inherit` |
| 3 | `TestButtonOptions_Problems`, `yamlscalar` table test, `dataentryconfig` block scalar case |
| 4 | `TestTypeButton_Checks` |
| 5 | `TestSessionView_ButtonOptions`, `TestDescribe_DecisionButton`, `transitions_wire_test.go` "button options", `TestChannelSchema_DecisionButton`, wire fields golden, `TestOpenAPI_ChannelButtonOptions`, `TestReadSession` "button carries confirm and style", MCP golden |
| 6 | Existing `decisionViews` cases unchanged, "no options keep the derived rule", "form button into a cancelled state stays plain without options" |
| 7 | `decisionViews options` cases |
| 8 | `useSessionDecisions.test.ts` confirm cases, locale parity |
| 9 | `RlChannelDecisions` story `Primary` |
| 10 | `TestDocsExample_Branching` (asserts `Options.Style`), `TestDocsExample_DecisionButton` |

Integration: `internal/dataentry/channels_schema_test.go`
`TestChannelSchema_DecisionButton` loads a project with a `decision_button:` and
requests `GET /api/v1/_schema/channels` through the real handler. In the SPA, a
new case in `frontend/src/components/entity/ChannelsPanel.test.ts`
`describe('ChannelsPanel preset buttons')`, "authored confirm asks before Allow
posts", mounts the panel with a fixture whose Allow type has `decision_button: {
confirm: "..." }` and a non-final target, clicks Allow, checks the dialog text,
confirms, and checks the post.

**Edge cases:**

- A type button that opens the composer (required property) with `style:
danger`: drawn danger, no dialog.
- A type button that opens the composer into a cancelled state, without
options: plain, as today.
- `confirm: true` on a type whose entries lead to several targets: the
type button opens the composer, so no dialog. No warning, because the targets
depend on the caller's listed entries. Its presets inherit the confirm.
- `confirm: false` and `style: default` on a transition into a `cancelled`
final state: one click, not red.
- `style: primary` into a `cancelled` state with the derived confirm: the
button is primary; the dialog's confirm button is red, because it follows the
outcome (AC 8).
- A type with `style: danger` and a preset with `style: default`: the
preset is plain, the type button is red.
- A text with an emoji style selector or composed and decomposed forms:
folded to NFC, served folded.
- A quoted `"yes"`: the dialog text "yes". Unquoted `yes`, or a `|` block
holding `yes`: refused.
- A reload that changes the options while a head is held: the next
descriptor or head carries the new values; an open dialog keeps the text it
showed.
- Mixed versions: an older process refuses `confirm`, `style` and
`decision_button:` as unknown keys. Documented as for `button:`.
- An older server sends no `_actions.entries`, so `presetViews` sends every
row to the type button. The type button then uses `decision_button`, as it would
for any unbuttoned entry.

**Negative Tests:**

- Every refusal in AC 3 and AC 4, with its key path.
- `style: Danger` (wrong case): refused; the values are case-sensitive, as
other enum-like keys in `realtime.yaml` are.
- A bad option on a preset refuses that button only: the entry and the
session still load their other problems, and no second problem is reported for
the button.
- A refused `decision_button` is not merged into any preset.

## Risk Assessment

- [x] Technical risks assessed with mitigations
- [x] Security risks assessed (see Security Considerations)
- [x] Effort estimated (xs/s/m/l/xl)

**Risks:**

- *The derived rule changes by accident.* Mitigated by leaving `endTone`
as it is, by `applyOptions` adding no key without an option, and by leaving the
existing `decisionViews` tests unedited (AC 6).
- *The type-level key applies in every state.* A type that is final in one
state and not in another gets the same authored look in both. Documented; a
preset button with its own options covers the rare case that needs a difference.
- *Inheritance surprises.* A preset may now ask or look red because of its
type. Mitigated by the docs, by the explicit opt-outs, and by serving the
resolved values on the row, so `read_session` shows what the web app draws.
- *`dataentryconfig` behavior change.* A `|` block scalar holding a
YAML 1.1 boolean spelling is now refused. No sensible file does this; a test
pins it.
- *Many primary buttons in one row.* Nothing stops an operator from
marking every button primary. Accepted; the docs advise one primary per state.
- *Library change.* `primary` is additive and optional. Existing stories
stay green.

**Effort:** m. The review added the leaf package, the inheritance merge and the
app-side view type. Each is small, so the work stays m and fits one PR.

## Documentation Planning

- [x] User-facing docs identified (skip if internal refactor)
- [x] Docs-checklist will be created when entering implementation

**Documentation Impact:**

- [x] `GUIDE-realtime.md`, regenerated into `docs/realtime.md` by `just
docs`:
  - "Message types" key table: a row for `decision_button` ("How the
type's decision buttons look: `confirm` and `style`. Applies to the type button
and, for each key a preset does not set, to the type's preset buttons. Does not
affect actions, choice buttons or the composer. Only on a log type in a channel
type with a session. See 'Button options' under Sessions.").
  - "Sessions" key table: the `on.<type>.button` row names `confirm` and
`style`.
  - "One-click buttons": a new paragraph, "Button options", with the
values, the order (preset, then `decision_button`, then the derived rule), that
each key overrides only its own part, that a text is shown as written and not
translated, that `confirm` does nothing on a button that opens the composer, how
the dialog tone follows the outcome, and that the options do not change what a
post does. The branching example gains `style: primary` on Approve, and a short
Allow and Abandon example shows `decision_button`.
  - "Checks when the file loads": the AC 3 and AC 4 checks and warnings.
  - "In the web app": the two bullets that describe when a decision asks
and when it is red say that the options override this. The sentence "Choosing
whether a button asks and how it looks is planned (TKT-TXZZZ2)" is replaced.
  - "HTTP API": the served `confirm`, `confirm_text` and `style` on a
transition row's `button`, their invariant, and the message type's
`decision_button` in the channel schema.
- [x] ~~docs/metamodel.md - New metamodel features~~ (N/A: no schema.yaml change)
- [x] ~~docs/cli-reference.md - New/changed commands~~ (N/A: no CLI change; JSON output gains fields)
- [x] ~~docs/data-entry.md - UI changes~~ (N/A: channel UI is documented in the realtime guide, "In the web app")
- [x] ~~CLAUDE.md - New patterns or conventions~~ (N/A: follows existing patterns)
- [x] ~~README.md - Project-level changes~~ (N/A)

## Open questions

- **Q1. Where does the type-level key go?** Decided:
`message_types.<type>.decision_button: { confirm, style }`. See Approach 2.
- **Q2. Boolean or text?** Decided: both, as an action's `confirm:`, with
`false` meaning "never ask". The text is not translated. Served as `confirm`
plus `confirm_text`.
- **Q3. Does the type-level key reach the type's preset buttons?** Decided:
yes, per key. A preset's own value wins, then the type's, then the derived rule.
Resolved at load in `realtimecfg`. See Approach 3.
- **Q4. Which styles?** Decided: `default`, `primary`, `danger`. `primary`
stays, with its one additive library field, because the branching example's
Approve needs it.
- **Q5. Refuse or warn on a type-level key that has no effect?** Decided,
following `choices:`: refuse structural misuse (no session, ephemeral type);
warn when the graph gives the key no effect (no entry a person could click, or a
`confirm` that reaches no one-click button). A warning keeps a reload working
when an edit elsewhere leaves the key unused.
- **Q6. Does the dialog's red confirm follow the target or the style?**
Decided: the outcome. It is red when the target ends cancelled or failed, or
when the button has `style: danger`. See AC 8.

## Design Review

- [x] Run `/design-review` before starting implementation
- [x] All critical/significant findings addressed in plan

**Design Review Findings:** <!-- List review-response IDs, e.g., RR-xxxx -->

Eighteen findings (five significant, seven minor, six nits). All are addressed
in this plan; the resolutions are in the review responses.

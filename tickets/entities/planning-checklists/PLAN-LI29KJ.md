---
id: PLAN-LI29KJ
type: planning-checklist
title: 'Planning: Authored one-click presets for session transition entries'
started: "2026-10-29"
completed: "2026-10-29"
status: done
---

<!-- @managed: claude-workflow v1 -->

## Understanding

- [x] Problem/requirements clearly understood
- [x] Scope defined (what's in/out documented below)
- [x] Acceptance criteria documented with specific test scenarios

**Problem.** Since TKT-FPXTQJ, `on.<type>` may hold a list of guarded entries,
so one enum-typed message type can lead to several states. The docs example is
`decide` with `decision: approve | revise`. The web app (TKT-T9G1AE) shows one
decision button per message type. For a branching type, that button opens the
composer, because the button cannot tell which entry a post will take. So an
operator who wants one-click buttons still has to declare two message types,
such as `approve` and `revise` in the agent-runner PoC
(`rela-mission/project/realtime.yaml`). `docs/realtime.md` says so: "two message
types remain the better choice for now".

The ticket lets an entry declare `button: { label, props }`. The web app then
shows one one-click button per such entry. A click posts the message type with
the preset props and `expect_generation`, through the same path as a choice
button (TKT-03FQPD, `presetProps`).

**Terms used below.**

- *Entry*: one mapping under `on.<type>`, as a single mapping or as an item
of a list (`realtimecfg.Transition`).
- *Button entry*: an entry with `button:`.
- *Preset*: the `props:` of a button. It is the exact set of properties a
click posts.
- *Preset button*: the web app button of one button entry.
- *Type button*: the existing per-type decision button of TKT-T9G1AE.
- *Person class*: one role that a load check asks about. The person classes
of an entry are its `roles:`, or the message type's `post:` roles when the entry
has no roles. A click is always a person's post, so a trigger is never a person
class.
- *Decidable guard*: a `when:` whose result the preset and the owning state
decide. It does not call `today()`, and it reads no field of `session` other
than `session.state`, which is the name of the state that owns the entry.
- *Shadowing entry*: an earlier entry of the same list that has no guard,
or whose decidable guard holds for the preset. The decider tries it first, so a
caller it admits takes it instead.
- *Undecidable entry*: an earlier entry of the same list whose guard is not
decidable. Whether it takes a click is unknown at load.

**Scope (in):**

- `realtime.yaml`: `on.<type>.button` with `label` and `props`, on a
mapping entry and on a list entry. Checked at load, on reload and by `rela
validate`.
- Load checks: the shape, the preset against the message type with the same
rules as a post, the type's own `when:`, the entry's guard, the earlier entries,
and the labels of a state together.
- The pure prop rules of a post move from `internal/realtime` to
`internal/realtimecfg` as `MessageType.NormalizeProps`, `Property.Normalize` and
`Property.PredicateValue`, so the load and the post run the same code.
- Served config: each row of `session.transitions` carries `entry`, and a
button entry's row carries `button` with `label` and `props` in stored form.
This covers the REST channel head, MCP `read_channel` and `read_session`, and
the CLI's JSON output.
- A per-caller affordance, `_actions.entries`: the entries this caller's
post could take. Also on `read_session` as `entries`.
- OpenAPI and MCP tool descriptions.
- SPA app: `decisionViews` builds one preset view per listed button entry,
and the type button from the listed entries without a button.
- Docs: `docs/realtime.md` and its mirror `GUIDE-realtime.md`.

**Scope (out):**

- `confirm:` and `style:` (TKT-TXZZZ2). Q5 says where they go.
- A preset that leaves a required property unset and opens the composer
prefilled. The composer has no prefill API. Follow-up: "Preset buttons that open
the composer prefilled".
- Folding the PoC's `approve` and `revise` into one type. Follow-up; see
"The PoC fold".
- One-click buttons in the inbox. `InboxEntry` is unchanged.
- More than one button per entry. One entry is one outcome; see Risks for
the limit this sets.
- Any change to the post path, the decider or the store. See Q3.
- A `button:` key in `data-entry.yaml`. See Q2.
- The rela-components library. `ChannelDecisionView` already carries
`props`, and `RlChannelDecisions` already draws any list of views.

**Acceptance Criteria:**

1. **The key loads.** `button: { label: Approve, props: { decision: approve
} }` loads on a mapping entry and on a list entry. `button:` with a null value
loads as absent, as other null keys do in `realtime.yaml`. `props:` may be
absent or empty: the click then posts no properties. A null preset value is an
absent property, as a JSON null is in a post. The compiled `Transition` has
`Index() int` and `Button() (TransitionButton, bool)`, and
`TransitionButton.Props()` returns a deep copy in stored form. A transition
without `button:` reports false. Test: `internal/realtimecfg/buttons_test.go`
(new) `TestButtons_Load` (mapping entry, list entry, null button, no props, null
value, props in stored form, copy does not alias, `Index`).
2. **The shape is checked.** Refused, each with its key path such as
`...on.decide[0].button.label`:
   - no `label`, or a label that is empty after trimming;
   - a label over 100 characters (the `buttonRunes` limit of choices);
   - a label with a display hazard (`texthazard.Find`);
   - an unknown key under `button:`, by the key checker;
   - a mapping as a preset value ("must be a single value or a list");
   - `button:` on an entry with `by: trigger` (no person can click it);
   - `button:` on an entry with no person class, because the message
type's `post:` is empty and the entry has no roles.

Test: `TestButtons_Problems` table with one case per bullet.
3. **The preset follows the rules of a post.** The preset goes through
`MessageType.NormalizeProps`, the code a post runs. Refused: an undeclared
property; a value of the wrong type (a string for an integer, `"true"` for a
boolean); an enum value outside the enum; a string over its `max`; a list item
that is empty, too long, repeated or holds a hazard; a list over `max_items`; a
required property the preset leaves out, sets to null, or sets to an empty list;
more than `realtimelimits.MaxMessageProperties` properties; an encoded preset
over `message_bytes`; a `ref` or `markdown` property (Q1); a preset that fails
the message type's own `when:`; and a message type whose `when:` calls
`today()`. Test: `TestButtons_PropsProblems` table, one case each. The moved
value tests (`internal/realtime/validate_test.go`, `listprop_test.go`) stay
green against the new home.
4. **The guards are checked.** All guard checks use the person classes of
the entry, never the trigger class.
- The entry's own guard must be decidable and must hold for the preset.
- An earlier entry that admits a person class of the entry is a
shadowing entry, an undecidable entry, or neither. The button records the
shadowing and the undecidable entries.
- A guard that returns an evaluation error for the preset, other than
`ErrNilOperand`, refuses the button. This applies to the own guard and to every
earlier guard that admits a person class of the entry. At runtime such an error
stops the decision, so the button would never move anything. `ErrNilOperand`
does not hold, as in `guardHolds`.
- The button is refused as unreachable when every person class of the
entry is admitted by a shadowing or an undecidable entry, because no caller
would ever see it.
- An entry that `unreachable()` already refused, or whose guard failed
to compile, gets no second problem from these checks.

Test: `TestButtons_Guards` table: own guard holds; own guard does not hold; own
guard reads `session.reminders`; own guard calls `today()`; own guard reads
`session.state` and holds; own guard errors for the preset; earlier guard errors
for the preset; earlier guard with `session.reminders` for a shared role makes
it unreachable; earlier guard with `session.reminders` for a disjoint role loads
and records the entry; earlier `by: trigger` entry with a `session` guard loads;
earlier unguarded entry for the same role makes it unreachable; earlier guarded
entry that holds for the preset makes it unreachable; two button entries with
the same preset (the second is unreachable); shadowed for a lead but reachable
for others loads; entry already refused by `unreachable()` reports once; broken
guard reports once.
5. **The labels of a state are checked together.** Labels compare after
`texthazard.DropStyleSelectors`, NFC, trimming and case folding, as
`normalizeListItem` does. Two equal preset labels in one state are refused at
the second one, unless no single caller can see both. That holds only when both
are entries of the same type and the earlier one is a shadowing entry of the
later one: every caller the earlier entry admits then loses the later button.
Entries of different types, or entries without that shadow, could both be shown
to a caller who holds a role for each. A state with more than 10 preset buttons
loads with a warning, as long choice lists do. A preset label equal to the label
of a message type with a transition out of the same state loads with a warning,
because a type button may show beside it. Test: `TestButtons_StateChecks` table
(duplicate across types, case folded duplicate, duplicate that differs by a
style selector, same label on a lead entry that shadows a later entry loads,
same label on disjoint role entries refused, eleven buttons warn, label equals a
type label warn, no problems for the docs example).
6. **The entry and the button are served.** Each row of
`session.transitions` carries `entry`, its place in its list from 0. A button
entry's row also carries `button: { label, props }` in stored form: an integer
as a number, a list as an array, a date as `2006-01-02`. Both are config, the
same for every reader. The REST head, `read_channel`, `read_session` and `rela
channel read -o json` carry them. Test: `internal/realtime/session_test.go`
`TestSessionView_Button`; `internal/dataentry/transitions_wire_test.go` new case
"button"; `internal/dataentry/testdata/channel_wire_fields.json` gains
`ChannelSessionButton` and the new fields; `internal/openapi/ channels_test.go`
asserts the schema; `internal/mcp/tools_session_test.go` case "session carries a
button".
7. **`_actions.entries` names what the caller's post could take.** It lists
`{ message_type, entry }` for each person-fired entry of the current state where
the caller may post the type now and `SessionState.Offers(t, holds)` is true.
`Offers` is true when the entry admits the caller and no earlier unguarded entry
admits the caller. For a button entry it also requires that no recorded
shadowing or undecidable entry admits the caller. The caller's roles are looked
up once, so a caller with two roles is judged on both. The list is sorted by
message type and entry, and empty, never null. It is empty on a closed channel,
in a final or stranded state, for a type without a session, and for a caller
without identity. `read_session` carries the same list as `entries`. Test:
`internal/realtime/session_affordance_test.go` `TestAffordances_Entries` table:
owner sees both; a member who may not post sees none; a lead is shadowed by an
earlier unguarded lead entry while a member sees the button; a lead+member
caller loses a button whose earlier `session.reminders` entry admits lead; a
member does not get a lead-only entry without a button; type outside its `in:`;
closed channel; final state; no session (empty list); caller without identity.
`internal/realtime/waiting_test.go` case "session read carries entries".
8. **A preset post is a normal post.** Posting the preset as the caller,
with `expect_generation`, moves the session along the button's entry and runs
its outputs. A post at an old generation is refused with `session_moved`.
Nothing in the post path knows about buttons. Test:
`internal/realtime/expectgen_test.go` `TestPost_ButtonPreset`;
`internal/realtime/session_test.go` `TestButtons_DeciderAgrees` (see Test Plan).
9. **The SPA builds views from `_actions.entries`.** For each type in
`_actions.transitions`, `decisionViews` takes the type's listed entries and
matches them to rows by the served `entry`. In entry order, it emits one preset
view per listed entry whose row has a button. A view has the key
`preset:<type>:<entry>`, the button's label, its props, and the `toLabel` of the
entry's target. It never opens the form. It asks first when the target is final,
and is `danger` when that state ends cancelled or failed, the rule of the type
button applied to one target. After the preset views comes the type button, but
only when a listed entry of the type has no button. Its `toLabel`, `confirm` and
`danger` come from those entries only. It sets `opensForm` when one of those
entries is guarded, or when a person-fired entry of the type before one of them
is guarded, a button entry included, because that entry could take a one-click
post. A listed entry without a served row shows nothing. Test:
`frontend/src/components/entity/channels/channelView.test.ts`, new
`describe('decisionViews presets')`: two presets and no type button; one preset
and a type button computed from the rest; a lead-only entry not listed for a
member gives no type button target; earlier guarded preset entry makes the type
button open the form; trigger entries ignored; preset not in `_actions.entries`
hidden; final target asks; cancelled target is danger; order across types; stale
`actionsGeneration` shows none; list prop preset carried as an array.
10. **A click posts the preset.** A preset view goes through `onDecide`: it
takes the lock, reads the generation, posts the type with
`presetProps(view.props)` and `expect_generation`, and replies to the decision
cause when the type may reply. A confirm view asks first, and after the dialog
posts the view the user confirmed; it does not look the view up again by key. A
`session_moved` or `session_wrong_state` refusal says the decision is stale. An
`invalid_message` refusal opens the composer with the type and shows the
server's message. A `session_output_denied` refusal goes through `ui.handle`, as
for a type button. The decision memo is rebuilt when `_actions.entries` changes.
Test: `frontend/src/components/entity/channels/useSessionDecisions.test.ts`
cases "preset posts its props with the generation", "preset replies to the
cause", "preset to a final state asks", "confirmed preset posts what was
confirmed after a reload reorders entries", "stale preset", "refused preset
opens the composer", "output denied preset", "memo follows actions.entries".
11. **Presets, choices and quiet types coexist.** On the row of a question
with choices, `rowDecisions` drops preset views of the answer type, as it drops
the type button now. The session header keeps them. A preset and a choice share
the lock and the pending keys. Presets follow the decision row, which
`RlChannel` keeps when quiet types fold, so a decision type with `display:
hidden` or `collapsed` (TKT-03DMXH) still shows its presets. The preset message
itself may then be hidden in the log. Test: `channelView.test.ts` `rowDecisions`
case "drops presets of the answer type";
`frontend/src/components/entity/ChannelsPanel.test.ts`, new
`describe('ChannelsPanel preset buttons')`, cases "plan review shows Approve and
the form button", "Approve posts the preset", "choice click disables the
presets", "hidden decision type keeps its presets".
12. **Docs.** `docs/realtime.md` documents `on.<type>.button` in the session
key table, gives the branching example buttons, replaces the "two message types
remain the better choice" bullet, adds the load checks, describes preset buttons
under "In the web app", and documents `_actions.entries`, the served `entry` and
the served `button`. Test: `TestDocsExample_Branching` loads the example with
its buttons, with no problems and no warnings; the prose is reviewed.

## Research

- [x] ~~For larger features: run `/research` to create a structured research doc~~ (N/A: the choices and their tradeoffs are recorded under Approach and Open questions)
- [x] Searched for existing libraries that solve this problem
- [x] Checked codebase for similar patterns or reusable code
- [x] Looked for reference implementations in other projects
- [x] Reviewed relevant rela concepts for prior art

**Research Doc:** N/A

**Existing Solutions:**

- Libraries: none needed. No new Go or npm package.
- Codebase patterns reused:
  - `Transition.Admits`, `entryAdmits`, `shadowed`, `firesFor` and
`unreachable` (`internal/realtimecfg/branches.go`): the poster rule and the
shadow reasoning. `Offers` joins `Admits` there.
  - `choiceChecker` (`internal/realtimecfg/choices.go`): a checker type
beside the validator, which is at its method budget, and the `buttonRunes` and
`buttonItems` limits.
  - `literalProps` and `literalMessage` (`internal/realtimecfg/validate.go`):
the load check of a trigger's or reminder's literal message, including the
type's `when:` and the refusal of `today()`.
  - `validateProps`, `normalizeProp` and its helpers, and `predicateValue`
(`internal/realtime/validate.go`): the prop rules of a post. They move to
`realtimecfg`.
  - `guardHolds` and `sessionDecider` (`internal/realtime/session.go`): how
a guard is bound and evaluated. The load evaluation binds the same variables.
  - `predicate.Program.Attributes`, `UsesWhole` and `Functions`: tell which
`session` fields a guard reads and whether it calls `today()`.
  - `rawBranches` and `keyChecker.branches` (`internal/realtimecfg/raw.go`,
`keys.go`): a raw type with its own `UnmarshalYAML` and its own key check, the
pattern for the preset values.
  - `call.fireable` (`internal/realtime/session.go`): the per-caller
affordance whose role lookup the entries share.
  - `decisionViews`, `choiceViews`, `rowDecisions` (`channelView.ts`) and
`onDecide`, `postView`, `presetProps` (`useSessionDecisions.ts`): the SPA path.
- Reference implementations: Slack message buttons, GitHub review actions
and Telegram inline keyboards all keep a fixed payload with the button.
- rela prior art: TKT-FPXTQJ (entry lists), TKT-T9G1AE (decision buttons),
TKT-03FQPD (choices and preset props), TKT-03DMXH (presentation and quiet types
in `data-entry.yaml`), DEC-6C1NAA (labels are authored, never derived). Authored
config text is not translated (`docs/data-entry.md`, "Text you write in the
configuration").

## Approach

- [x] Technical approach chosen and documented
- [x] Approach builds on existing patterns (not reinventing)
- [x] Alternatives considered (document why rejected)
- [x] Dependencies identified (packages, APIs, types)

**Technical Approach:**

1. **Move the prop rules (`internal/realtimecfg/props.go`, new).** The core
of `validateProps` becomes `func (mt *MessageType) NormalizeProps(in
map[string]any, maxBytes int) (NormalizedProps, error)`, returning the stored
map, the encoded size and the sorted refs. It keeps every rule: a null or an
empty list is absent, required is checked after that, the property cap, and the
size against `maxBytes`. It returns sentinel errors `ErrTooManyProps` and
`ErrTooLarge`, and other refusals as plain errors. `realtime.validateProps`
calls it and maps the two sentinels to its `TypeTooManyProps` (400) and
`TypeTooLarge` (413) denials and the rest to `invalid`, so the wire errors do
not change. `normalizeProp` and its helpers become `Property.Normalize`, and
`predicateValue` becomes `Property.PredicateValue`, beside `messageRecordType`.
`.go-arch-lint.yml`: `realtimecfg` gains `texthazard` under `mayDependOn` and
`xnorm` under `canUse`. `entity` is already a common component. `realtimecfg`
may not import `channels`, so the cap is `realtimelimits.MaxMessageProperties`,
which equals `channels.MaxProps` (32) and is already a dependency.
2. **Raw config (`internal/realtimecfg/raw.go`).** `rawTransition` gains
`Button *rawButton yaml:"button"`. `rawButton` has `Label string` and `Props
rawPreset`. `rawPreset` is a `map[string]rawPresetValue`, where `rawPresetValue`
keeps the YAML node of a value. `keyChecker.walk` special-cases `rawPresetValue`
as it does `rawBranches`: null, a scalar or a sequence of scalars passes,
anything else is "must be a single value or a list".
3. **Typed preset values.** A preset value is read by the declared type of
its property, not by the YAML tag guess. A boolean needs a YAML bool, an integer
a YAML int, a list a sequence of scalars, and every other type takes the
scalar's text, so an unquoted date stays its text. A null value is passed as
nil, which `NormalizeProps` treats as absent. The docs say that a preset boolean
must be written `true` or `false`, as in a post, even though other booleans in
`realtime.yaml` also accept `yes` and `on`.
4. **Button checks (`internal/realtimecfg/buttons.go`, new).** A
`buttonChecker{*validator}`, like `choiceChecker`.
   - `compile(key, raw, list, j, ct, mt, limits)` runs from
`branchCompiler.compile` for each entry with `button:`, for the mapping form
too, after `unreachable()`. It skips an entry that `unreachable()` refused. It
checks the shape (AC 2), the preset (AC 3) and the guards (AC 4), and returns a
`*TransitionButton` or nil.
   - The person classes come from the entry's roles, or from `mt.Post`
when it has none. They never include `triggerPoster`.
   - Guard evaluation binds `message` from the preset through
`PredicateValue`, `session` with `state` set to the owning state and the other
fields zero, and the standard library at the zero time. It runs only on a
decidable guard. Any error other than `ErrNilOperand` refuses the button.
   - `TransitionButton` holds `Label`, the unexported stored props, and an
unexported `hiddenBy []int`: the indices of the shadowing and the undecidable
entries that admit a person class of the entry.
   - `states(s)` runs from `validator.session` after every `on:` of the
session is compiled, for the label checks of AC 5.
   - `Transition` gains `button *TransitionButton`, `Index() int` and
`Button() (TransitionButton, bool)`. `TransitionButton.Props()` deep copies the
map and its list values. The pointer is shared by copies of the transition; it
is never written after load.
5. **The offer rule (`internal/realtimecfg/branches.go`).**
`func (st SessionState) Offers(t Transition, holds func(role string) bool)
bool`, beside `Admits`. It is true when `t.Admits(holds)`, no earlier unguarded
entry admits `holds`, and, for a button entry, no entry in `hiddenBy` admits
`holds`. The load's reachability check asks the same rule for each person class,
so the load and the affordance share one function, and `hiddenBy` stays
unexported.
6. **Served config (`internal/realtime/session.go`).**
`SessionTransitionView` gains `Entry int json:"entry"` and `Button
*SessionButtonView json:"button,omitempty"`, with `Label string` and `Props
map[string]any`. `sessionView` fills them from `tr.Index()` and `tr.Button()`.
`apiwire/v1/channels.go`: `ChannelSessionTransition` gains `Entry` and `Button
*ChannelSessionButton`. `dataentry/channels_wire.go` maps them.
`openapi/channels.go` adds both to the transition schema, with `props` as an
object of free values.
7. **Per-caller affordance (`internal/realtime/session.go`, `service.go`).**
`fireable` becomes `sessionAffordances(post) (types []string, entries
[]EntryRef)`, so the held roles are looked up once. An entry `t` is listed when
`post[t.MessageType]` and `st.Offers(t, p.roles)`. `EntryRef` is `{MessageType
string json:"message_type"; Entry int json:"entry"}`. `Affordances` gains
`Entries []EntryRef json:"entries"`, never nil. `apiwire.ChannelActions` gains
`Entries []ChannelEntryRef`, mapped and defaulted to `[]` in `channels_wire.go`.
`SessionRead` gains `Entries` from the same call. The inbox keeps calling the
types half only.
8. **MCP and CLI.** MCP serializes `realtime.SessionView`,
`realtime.ReadView` and `SessionRead` as they are, so the new fields arrive. The
`read_channel` and `read_session` descriptions gain one sentence: "A row of
session.transitions with button is a one-click preset that posts its props;
actions.entries lists the rows your post could take." The golden
`tools_list_channels.golden.json` is regenerated. The CLI prints transitions
only in JSON, which carries the fields.
9. **SPA types (`frontend/src/types/channels.ts`).**
`ChannelSessionTransition` gains `entry?: number` and `button?: { label: string;
props?: Record<string, MessagePropInput> }`, and `ChannelActions` gains
`entries?: { message_type: string; entry: number }[]`, plus the `wireKeys` lists
(`ChannelSessionButton`, `ChannelEntryRef`).
10. **SPA views (`frontend/src/components/entity/channels/presetViews.ts`,
new).** `presetViews(name, rows, entries, s, stateOf)` returns the preset views
of one type and the listed rows left for the type button. `decisionViews` calls
it inside its `flatMap` and computes the type button from the remaining rows, or
skips it when none remain. The rule for `confirm` and `danger` moves into one
helper, `endTone(targets, stateOf)`, used by both. A new file, because
`channelView.ts` already has 779 lines against a warn-only limit of 500.
11. **SPA click (`useSessionDecisions.ts`).** The decision memo key gains
`h?._actions.entries`. After the confirm dialog, `onDecide` keeps the confirmed
view and checks only that the panel is live; the server's `expect_generation`
check refuses a stale post. `postView` already posts `presetProps(view.props)`,
and `decisionReplyTo` and `refused` apply as they are.

**The PoC fold (follow-up, not in this PR).** The PoC could fold `approve` and
`revise` into one `plan-decision` type with an enum `decision`, an Approve
preset on the approve entry, and a revise entry without a button that the type
button opens in the composer. The fold must keep the PoC's rules: a type-level
`when:` that requires `feedback` when `decision` is `revise`, and the optional
`note` on approve. The AC 3 check confirms that the Approve preset passes that
`when:`. The runner keys on the type names `approve` and `revise`, so the fold
also changes the runner, which is why it is a follow-up. `allow` and `deny` stay
two types: both lead to `working` with outputs, which the existing duplicate
check refuses as one type.

**Alternatives considered:**

- *`button:` in `data-entry.yaml` (Q2).* Rejected. The preset decides what
a click posts and which entry fires. The load checks need the entry list, the
guards, the roles and the message types, which only `realtimecfg` has. The
per-caller list needs the service, which does not read `data-entry.yaml`. MCP
and CLI clients would never see it. `choices:` set the precedent: a button that
posts lives in `realtime.yaml`.
- *Let the SPA decide which buttons a caller sees.* Rejected. The SPA does
not know which roles the caller holds, so a lead-only button, or a type button
aimed at a lead-only target, would show to everyone who may post the type.
- *Judge earlier entries one role at a time.* Rejected. A caller with two
roles is admitted by the entries of both, so a per-role check would serve a
button whose click an earlier entry takes.
- *Evaluate guards that read `session` at runtime, per head.* Rejected for
now: it costs a guard evaluation per button per head and per list row, and a
button would come and go with a reminder count. Hiding the button from callers
an undecidable entry admits keeps the outcome of a click known. Q1 records this.
- *Allow `markdown` and `ref` presets.* Rejected (Q1). A markdown preset
can mention people, which notifies them in the clicker's name with text the
clicker never saw. A ref preset names an entity whose existence and visibility
change at runtime, behind a label.
- *`buttons:` as a list per entry.* Rejected. One entry is one outcome; two
buttons on one entry would be two labels for the same move.
- *Hide the type button whenever any entry has a button.* Rejected: the
entries without a button would then be reachable only by picking the type in the
composer.
- *Match entries to rows by row order.* Rejected: the SPA drops trigger
rows before it groups, so it would have to count before it filters. The served
`entry` removes that trap for the SPA and for MCP agents.
- *Keep the prop rules in `realtime` and duplicate a subset in
`realtimecfg`.* Rejected: the load would accept a preset the post refuses, or
the reverse, as soon as one copy changes.
- *Split into three PRs.* Rejected. The ticket stays one ticket and one PR
at effort l; the move is reviewed as its own commit.

**Dependencies:** `internal/realtimecfg`, `internal/realtime`,
`internal/apiwire/v1`, `internal/dataentry`, `internal/openapi`, `internal/mcp`
(descriptions and golden only), `.go-arch-lint.yml`. No change to the stores,
`entitymanager` or `dataentryconfig`. Frontend: no new packages, no library
change.

**Files to modify:**

- `.go-arch-lint.yml`.
- `internal/realtimecfg/props.go` (new), `buttons.go` (new), `raw.go`,
`keys.go`, `branches.go`, `session.go`.
- `internal/realtime/validate.go`, `session.go`, `service.go`,
`waiting.go`.
- `internal/apiwire/v1/channels.go`, `internal/dataentry/channels_wire.go`,
`internal/openapi/channels.go`, `internal/mcp/tools_channels.go`.
- `frontend/src/types/channels.ts`,
`frontend/src/components/entity/channels/presetViews.ts` (new),
`frontend/src/components/entity/channels/channelView.ts`,
`frontend/src/components/entity/channels/useSessionDecisions.ts`.
- Tests: `internal/realtimecfg/buttons_test.go` (new),
`internal/realtimecfg/docs_example_test.go`,
`internal/realtime/validate_test.go` and `listprop_test.go` (imports),
`internal/realtime/session_test.go`,
`internal/realtime/session_affordance_test.go`,
`internal/realtime/expectgen_test.go`, `internal/realtime/waiting_test.go`,
`internal/dataentry/transitions_wire_test.go`,
`internal/dataentry/testdata/channel_wire_fields.json`,
`internal/dataentry/testdata/channel_wire.json`,
`internal/openapi/channels_test.go`, `internal/mcp/tools_session_test.go`,
`internal/mcp/testdata/tools_list_channels.golden.json`,
`frontend/src/components/entity/channels/channelView.test.ts`,
`frontend/src/components/entity/channels/useSessionDecisions.test.ts`,
`frontend/src/components/entity/ChannelsPanel.test.ts`.
- Docs: `docs/realtime.md`,
`docs-project/entities/guides/GUIDE-realtime.md`.

## Security Considerations

- [x] Input sources identified (user input, config, external APIs)
- [x] Input validation approach defined (allowlist preferred over blocklist)
- [x] Security-sensitive operations identified (file access, auth, crypto)
- [x] Error handling doesn't leak sensitive information

**Input Sources & Validation:**

- *`button:` in `realtime.yaml` (operator).* Checked at load, on reload and
by `rela validate`. The key checker allows only `label` and `props`. The preset
passes the same `NormalizeProps` as a post. Property types are an allowlist:
string, enum, integer, boolean, date, datetime and string lists. The label is
length-limited and free of display hazards. A failed reload keeps the last good
config.
- *A click (SPA).* The SPA sends a normal post. The server checks it as any
post: post gate, `in:`, prop rules, the type's `when:`, mentions, rate limit,
`expect_generation`, the decider and the outputs gate. A client that sends other
props, or skips the button, gets exactly what a hand-made post gets. The button
grants nothing.
- *Served values.* Labels and preset strings render through Vue
interpolation, never `v-html`.

**Security-Sensitive Operations:**

- No new write path and no new gate. The post path is unchanged (Q3).
- `_actions.entries` is per caller. It reveals which entries the caller's
roles admit, which `_actions.transitions` and `waiting_on_me` already reveal per
type. It is computed from the roles the caller holds, through `heldRoles` and
the client ceiling, as `fireable` is now. It names no other person and no entity
data.
- The served `entry` and `button` are config. The configuration is not a
secret (CLAUDE.md), so they are the same for every reader. Refusing `ref`
presets keeps entity ids out of them.
- A preset never posts markdown, so a click cannot mention someone in the
clicker's name with text the clicker did not see.
- Errors: load errors name config paths, labels and values. All are config.

## Test Plan

- [x] Test scenarios documented for each acceptance criterion
- [x] Edge cases identified and documented
- [x] Negative test cases defined (invalid input, error conditions)
- [x] Integration test approach defined (not just unit tests)

**Test Scenarios:**

| AC | Test |
| --- | --- |
| 1 | `TestButtons_Load` |
| 2 | `TestButtons_Problems` |
| 3 | `TestButtons_PropsProblems`, moved value tests in `internal/realtime` |
| 4 | `TestButtons_Guards` |
| 5 | `TestButtons_StateChecks` |
| 6 | `TestSessionView_Button`, `transitions_wire_test.go` "button", wire fields golden, OpenAPI test, `tools_session_test.go` case |
| 7 | `TestAffordances_Entries`, "session read carries entries" |
| 8 | `TestPost_ButtonPreset`, `TestButtons_DeciderAgrees` |
| 9 | `decisionViews presets` cases |
| 10 | `useSessionDecisions.test.ts` preset cases |
| 11 | `rowDecisions` "drops presets of the answer type", `ChannelsPanel preset buttons` |
| 12 | `TestDocsExample_Branching`, reviewed |

Integration: `TestButtons_DeciderAgrees` runs the real service on the memory
channel store. Its fixture has a type with four entries: a lead-only unguarded
entry, a lead-only entry with a `session.reminders` guard, a guarded button
entry and a catch-all button entry. Each entry has an output whose mark names
it. The principals follow `examplePrincipals` in `session_affordance_test.go`: a
lead, a member, and a caller holding both. For each principal and each button
entry, on a fresh channel, the test reads `_actions.entries` and posts the
preset with `expect_generation`. When the entry is listed, the session reaches
the entry's `to` and the entry's mark is written. When a shadowing entry hides
it, the session reaches the shadowing entry's `to` and that entry's mark is
written. This pins both directions of the load-time sets against the decider. In
the SPA, `ChannelsPanel preset buttons` mounts the channel panel with a
branching fixture and drives it with mocked reads and posts, through
`OpenChannel`, `useSessionDecisions`, `decisionViews` and `RlChannelDecisions`.

**Edge cases:**

- A mapping entry (`on.approve: { to: working, button: ... }`): entry 0,
no earlier entries.
- A button with no props on a type whose properties are all optional: a
relabeled one-click. With all entries buttoned, the type button goes.
- A list preset value: normalized like a posted list (trimmed, NFC), served
as an array, posted as an array by `presetProps`.
- A date preset written unquoted (`2026-11-01`): read as text, stored as
`2026-11-01`.
- A boolean property with preset `"true"` (quoted) or `yes`: refused, as a
post with a string would be.
- A button entry that is shadowed for a lead but not for a member: the lead
does not see it; the member does; a lead+member caller does not.
- Two button entries leading to the same state without outputs: both load
and both show, with different labels.
- A type in `_actions.transitions` with no entry in `_actions.entries` for
this caller: no preset and no type button for that type.
- A reload that reorders entries while the confirm dialog is open: the
confirmed view is posted; if the session moved, the server refuses with
`session_moved`.
- A reload that drops a button while a head is held: the next head has no
`button` on the row; the view goes.
- The question row of TKT-03FQPD whose answer type has presets: presets
show in the session header only.
- Mixed versions: an older process refuses `button:` as an unknown key.
Documented as for entry lists.

**Negative Tests:**

- Every refusal in AC 2 to AC 5, with its key path.
- `button: Approve` (a scalar): "must be a mapping".
- `props: { decision: [approve] }` on an enum property: refused as not a
string.
- A preset on an ephemeral message type: the transition is already refused
("an ephemeral message is never stored"); no second problem.
- An `_actions.entries` ref without a served row (a client bug or a race):
the SPA shows nothing for it.
- A forged post with props other than the preset: handled as any post; the
decider takes whichever entry its props reach.

## Risk Assessment

- [x] Technical risks assessed with mitigations
- [x] Security risks assessed (see Security Considerations)
- [x] Effort estimated (xs/s/m/l/xl)

**Risks:**

- *Load-time sets drift from the decider.* Mitigated by one shared
`Offers` rule, by hiding buttons from callers an undecidable entry admits, by
judging callers on all their roles, and pinned by `TestButtons_DeciderAgrees` in
both directions.
- *Moving the prop rules changes post behavior.* Mitigated by moving the
bodies unchanged, mapping the sentinels to the same denials, and keeping every
existing value test green. The move is its own commit.
- *Cost of `_actions.entries`.* One extra pass over the current state's
entries, with the held roles already looked up for `fireable`. No entity read,
no store read. Affordances on list rows pay the same small cost.
- *Hiding buttons for undecidable entries hides too much.* A caller who
holds a role of an earlier `session` guarded entry loses the button even when
that guard would not hold. Recorded as Q1; runtime evaluation can lift it later
without a breaking change.
- *One button per entry.* Two presets that lead to the same state with
outputs cannot share a type, because of the existing duplicate check. Documented
with `allow` and `deny` as the example.
- *Type button label beside presets.* "Plan decision" next to "Approve" is
clear only if the type's label says what the form is for. The warning in AC 5
and the docs example guide the operator.
- *Size of the change.* One PR across config, service, wire, MCP, SPA and
docs. Mitigated by commits per layer: the move, the config, the service and
wire, the SPA, the docs.

**Effort:** l.

## Documentation Planning

- [x] User-facing docs identified (skip if internal refactor)
- [x] Docs-checklist will be created when entering implementation

**Documentation Impact:**

- [x] docs/realtime.md, "Sessions": a row for `on.<type>.button` in the key
table, with the note that a preset boolean is `true` or `false`; the branching
example gains `button:` on both entries; the bullet "two message types remain
the better choice" is replaced by how presets and the type button show,
including that with every entry buttoned the type button goes and the optional
`note` can only be written by picking the type in the composer; "Checks when the
file loads" lists the button checks; "In the web app" describes preset buttons,
their order, the remaining type button and quiet types; "HTTP API" documents
`_actions.entries`, the served `entry` and the served `button`. Mirrored in
`GUIDE-realtime.md`.
- [x] ~~docs/metamodel.md - New metamodel features~~ (N/A: no schema.yaml change)
- [x] ~~docs/cli-reference.md - New/changed commands~~ (N/A: no CLI change; JSON output gains fields)
- [x] ~~docs/data-entry.md - UI changes~~ (N/A: channel UI is documented in docs/realtime.md, "In the web app")
- [x] ~~CLAUDE.md - New patterns or conventions~~ (N/A: follows existing patterns)
- [x] ~~README.md - Project-level changes~~ (N/A)

## Open questions

- **Q1. Key shape and load checks.** Decided:
`on.<type>.button: { label, props }` on any entry. The preset passes the same
prop rules as a post and the type's own `when:`. It may not set `ref` or
`markdown` properties. Its own guard must be decidable and hold. Earlier entries
that a person class of the entry admits are recorded as shadowing or
undecidable, a guard error refuses the button, and a button no caller could see
is refused. Callers an undecidable entry admits do not see the button. Equal
labels are refused unless no single caller can see both; more than 10 buttons in
a state warns. Alternatives: evaluate `session` guards at runtime (later, if a
config needs it); allow markdown and ref presets (rejected, see Approach).
- **Q2. `realtime.yaml` or `data-entry.yaml`?** Decided: `realtime.yaml`.
The preset decides what a click posts and which entry fires; the server checks
it and computes who may click. The label stays with it, as `choices:` and
message type labels do.
- **Q3. What is new on the server?** Decided: nothing on the post path. A
preset post is a normal post. New are the load checks, the served `entry` and
`button`, and the per-caller `_actions.entries`, because only the server knows
which roles the caller holds and so which entry a post would take.
- **Q4. How do the buttons coexist in the SPA?** Decided: per type, the
preset buttons of the caller's listed entries in entry order, then the type
button only for the caller's listed entries without a button. Choice buttons are
unchanged; on the question's row, presets of the answer type are dropped as the
type button is now. All share the lock and pending keys.
- **Q5. Where do TKT-TXZZZ2's `confirm:` and `style:` go?** Decided:
inside `button:`, as `button.confirm` and `button.style`, served in the row's
`button`. They apply to that entry's preset button. The type button aggregates
every listed entry without a button, so a per-entry key would have no single
answer for it. It gets a type-level key instead, whose exact place TKT-TXZZZ2
decides; without it the type button keeps the derived rule. In the SPA both
replace the derived rule in the one `endTone` helper. TKT-TXZZZ2's note is
updated to say this.
- **Q6. Presets that need text.** Decided: out of scope. Revise needs
feedback, so it cannot be one-click. A preset that leaves a required property
out could open the composer with the preset filled in. Follow-up: "Preset
buttons that open the composer prefilled".

## Design Review

- [x] Run `/design-review` before starting implementation
- [x] All critical/significant findings addressed in plan

**Design Review Findings:** <!-- List review-response IDs, e.g., RR-xxxx --> The
21 findings of the design review are resolved in this plan: the critical one
(two-role callers) through the undecidable set, `Offers` and the two-role
agreement test; the significant ones through person classes, guard errors,
`NormalizeProps`, `_actions.entries` and effort l.

---
id: PLAN-N0AM2O
type: planning-checklist
title: 'Planning: Guarded branching transitions per message type'
started: "2026-10-29"
completed: "2026-10-29"
status: done
---

---
id: PLAN-N0AM2O type: planning-checklist title: 'Planning: Guarded branching
transitions per message type' started: "2026-10-29" status: in-progress
---

<!-- @managed: claude-workflow v1 -->

## Understanding

- [x] Problem/requirements clearly understood
- [x] Scope defined (what's in/out documented below)
- [x] Acceptance criteria documented with specific test scenarios

**Problem.** `states.<s>.on` is a map keyed by message type, and each key holds
one entry. So one message type has one target state per state. A "plan decision"
type with an enum `decision: approve | revise`, where `approve` goes to
`working` and `revise` goes back to `planning`, cannot be written. The PoC
(`rela-agent-runner-poc/rela-mission/project/realtime.yaml`) works around it
with two message types, `approve` and `revise`. A guard (`when:`) on a
transition already exists (`Transition.When`, compiled by `validator.guard` in
`internal/realtimecfg/session.go`), but only one guard per type and state.

**Terms used below.**

- *Entry*: one `{to, roles, by, when, outputs}` under `on.<type>`.
- *Branch list*: `on.<type>` written as a YAML list of entries.
- *Applies*: an entry applies to a post when the poster passes the entry's
poster rule (`roles` / `by`) and its `when` holds.
- *Edge*: one entry as the client sees it: `(message_type, to)`.

**Scope (in):**

- `on.<type>` accepts today's single mapping, or a list of 1 to 8 entries.
The single mapping stays valid and means a list of one.
- Entries are tried in file order. The first entry that applies moves the
session. Later entries are not tried.
- When no entry applies, the message is stored and the session does not
move. This is today's rule for a guard that does not hold. A list with no
catch-all entry loads with a warning, and the MCP `post_message` description and
the docs tell agents to compare `session.generation`.
- Load checks for the list form (see Technical Approach, step 3).
- Every graph check that walks transitions sees the entries *per role*:
an entry counts for a role (or for a trigger) only when no earlier entry without
`when` already admits that role. This covers waiting answers, deadline reset,
job loops (`jobEdges`), `transitionCause`, `run_as` job outputs, ACL grant
warnings and handoff warnings. `in:` consistency stays per type and state.
- The outputs path (pre-check, decider gate, mark, mark resolution, retry)
uses the entry that fired, identified by `(from, message type, to)`. Two entries
may share a `to` unless either has outputs.
- SPA form decisions carry `expect_generation`: the generation is captured
when a decision button opens the form and sent with that form's Send.
- Read side: `session.transitions` (`read_channel`, `read_session`, HTTP
head) lists one row per entry. `_actions.transitions`, the inbox and the waiting
list keep listing message types, once each.
- Triggers follow the same decider, so a trigger's post can branch too.
- SPA: the composer hint for a branching type says it "may move" the
session to any of its targets. The decision buttons already open the form for a
type whose edges lead to several states (TKT-T9G1AE).
- Docs (`docs/realtime.md`), MCP tool descriptions (`read_channel`,
`read_session`, inbox, `post_message`), OpenAPI description of `transitions`,
i18n en + nl for the new composer hint.

**Scope (out):**

- One-click buttons per enum value (Approve and Revise as two buttons with
preset props from one type). Out of scope; see Open questions, Q4.
- TKT-03FQPD (message options as reply buttons), TKT-S5OJ7S
(`session.offer:` transitions), TKT-TXZZZ2 (authored confirm/style).
- A typed refusal (4xx) for a post that matches no entry. See Q1.
- Guards that read the entity. They stay refused, as today.
- Two entries of one type and one state that lead to the same state when
either has outputs. See Q3.
- Per-caller filtering of edges in the SPA. `session.transitions` carries
no roles, by design, so a member sees every edge of a role-split type (see Q5).
The "may move" hint stays true for every caller.
- Changes to the channel store, the `OutputMark` wire format, or any
storage backend.
- The Configure space: `realtime.yaml` is locked in
`internal/configedit/allowlist.go`, so there is no config editor to update.
There is no JSON Schema for `realtime.yaml` itself; `realtimecfg.PropsSchema`
describes message props only.

**Acceptance Criteria:**

1. **The list form loads, the single form is unchanged.** `on.decide:` as a
list of two entries loads. Every existing `realtime.yaml` fixture and docs
example loads with the same compiled result. A one-entry list compiles to the
same transition as the mapping. Test: `internal/realtimecfg/session_test.go`
`TestSession_BranchList` (table: mapping, one-entry list, two-entry list)
compares `Session.Branches(state, type)`; the existing `load_test.go`,
`docs_example_test.go` and `testdata/valid.yaml` pass untouched.
2. **The first entry that applies wins.** A `decide` type with a required
enum `decision` and the list `[{to: working, when: "message.decision ==
'approve'"}, {to: planning, when: "message.decision == 'revise'"}]`: a post with
`revise` moves `plan-review -> planning`; one with `approve` moves to `working`.
With two entries that both hold, the first wins. Test:
`internal/realtime/session_test.go` `TestSessionBranches_FirstWins` through
`Service.Post`, asserting the notice `to`, the generation and
`PostResult.Session.State`.
3. **Each entry has its own poster rule.** Entry 1 `{to: escalated, roles:
[lead]}`, entry 2 `{to: noted}`: a lead's post goes to `escalated`, a member's
to `noted`, a trigger's to `noted`. An entry with `by: trigger` is skipped for a
person's post. Test: `TestSessionBranches_PosterRulePerEntry` in
`internal/realtime/session_test.go` (person with role, person without, trigger
through `systemWriter.post`, as in `trigger_test.go`).
4. **No entry applies: stored, no move.** A post of `decide` whose props
match no entry (an optional enum left out, or a value with no entry) is stored
with 201, the generation stays, no notice is appended. A list in which every
entry has a `when` loads with a warning at `on.<type>`. Test:
`TestSessionBranches_NoMatchStoresWithoutMove`; a
`TestSession_BranchListWarnings` case in `internal/realtimecfg/session_test.go`
(warning without a catch-all, none with an unguarded last entry, none for the
mapping form).
5. **A guard that fails to evaluate fails closed and the next entry is
tried.** Entry 1's guard errors at runtime; entry 2 holds; the session moves
along entry 2. The warning says the entry was skipped, names its `to` and its
key (`t.Key()`), and is logged once per entry per loaded configuration. Test:
`TestSessionBranches_GuardErrorFallsThrough` with a log capture, posting twice
and asserting one warning with the entry key.
6. **Load checks refuse bad lists, by key.** Refused, with the key path
`channel_types.<ct>.session.states.<s>.on.<type>[<i>]...`: an empty list; more
than 8 entries; an entry that no poster can reach because earlier entries
without `when` (judged by the raw `when:` text, so a guard that failed to
compile still counts as guarded) admit every poster it admits; two entries with
the same `to` when either has outputs; a list on a type whose `in:` leaves out
the state (the existing message, reported once at `on.<type>`). Problems inside
a list entry name the index, such as `on.decide[1].to`. Problems in the mapping
form keep today's keys, including `on.<type>: ~`, which still reports "name the
state this transition moves to" at `on.<type>.to`. Test: table-driven
`TestSession_BranchListProblems` in `internal/realtimecfg/session_test.go`,
asserting exact keys and reasons, including: a duplicate `to` without outputs
loads; a broken guard in entry 1 gives only its compile error, not a second "can
never fire" on entry 2. Key-walk cases go in `load_test.go` (next to
`TestParse_BoolKeysUnchanged`): an unknown key inside a list entry
(`on.decide[0].nope`), `on.decide[0].run_as` with the existing reason, and a
scalar (`on.decide: working`, "must be a mapping or a list of mappings").
7. **Graph checks see the entries per role.** A waiting role answered only
by entry 2 of a list passes `answers`. A *shadowed* answer is refused: with
`waiting_for: [owner]` and `[{to: plan-review, roles: [owner]}, {to: working}]`,
the owner always hits the re-entry, so `answers` refuses the owner even though
entry 2 is reachable by the runner. A chain back into a `since: state` state
through entry 2 is refused at `on.<type>[1]`. A trigger-fireable entry 2 with
outputs needs `session.run_as`, unless an earlier unguarded entry admits
triggers. A `set` in entry 2 that writes a role property from free text gets the
handoff warning at its own key. Test: cases added to `waiting_test.go`,
`timers_test.go`, `runas_test.go` in `internal/realtimecfg`.
8. **Outputs follow the entry that fired.** Entry 1 (`approve`) sets
`status: in-progress`, entry 2 (`revise`) sets `status: planning`. A `revise`
post writes `planning`, and the pre-check asks the writer about entry 2's
outputs only. A denial title names `...on.decide[1].outputs[0]`. A retry by the
runner resolves the mark to entry 2 by `(from, type, to)`. Test:
`internal/realtime/outputs_test.go` `TestOutputs_BranchUsesFiredEntry`, and
`outputs_runner_test.go` `TestResolveMark_Branch` (mark with `From/To` of entry
2 resolves to entry 2's outputs; a mark whose `To` matches no entry is removed).
9. **The gate stops a move along an entry with unchecked outputs.** With
`expect_generation`, state, generation and `entered_at` cannot change between
the pre-check and the lock, so only a `session.reminders` counter can switch the
entry. Entry A is guarded by `session.reminders['nudge'] < 2` and has outputs;
entry B has outputs too. The pre-check sees count 1 and checks A. Under the lock
the count is 2, so B applies first. The post is stored and the session does not
move, as today for a gate mismatch. A switch under the lock to an entry
*without* outputs still moves; this is accepted and documented. Test:
`TestSessionBranches_GateRefusesOtherEntry` in
`internal/realtime/outputs_lock_test.go`: run the step's `Decide` once on a
channel with reminder count 1 (pre-check), then on one with count 2 (lock);
assert no move. A second case where B has no outputs asserts the move to B.
10. **Read views list every edge.** For the list in AC 2,
`session.transitions` holds two rows `{decide, working, guarded}` and `{decide,
planning, guarded}`, sorted by message type and then in file order.
`_actions.transitions`, the inbox `transitions` and the waiting list
`transitions` hold `decide` once. Test:
`internal/realtime/session_affordance_test.go` `TestSessionView_BranchEdges`;
`inbox_test.go` case asserting no duplicate; `internal/dataentry` channels wire
test asserting the two rows reach `v1.ChannelSessionTransition`.
11. **The SPA composer says "may move" for several targets.** With two
edges for `decide`, the hint reads "Sending this may move the session to Working
or Planning." (nl: "Dit bericht kan de sessie op Working of Planning zetten.").
One target keeps today's hint. The decision button for `decide` opens the form
and has no `toLabel`. Test:
`frontend/src/components/entity/channels/channelView.test.ts` cases for
`composerTransitions` (two edges, one type: `toLabels` has both) and
`decisionViews` (branching type opens the form); a hint case in
`frontend/src/components/entity/ChannelsPanel.test.ts`, which mounts the
composer, for en and nl; `RlChannelComposer.stories.ts` gets a branching story.
12. **Docs, MCP and OpenAPI describe branching.** `docs/realtime.md` has an
example with a branch list that loads (pinned by a new case in
`docs_example_test.go`). MCP `read_channel`/`read_session` descriptions and the
OpenAPI `transitions` description say a type may appear more than once, in the
order rela tries the entries. MCP `post_message` and the docs say: "If no entry
applies, the message is stored and the session stays; compare session.generation
in the answer." Test: `docs_example_test.go` new case; OpenAPI golden files
regenerated (`internal/dataentry/testdata/golden/*/openapi.*.golden`);
`internal/openapi/channels_test.go` unchanged or updated; an MCP tool
description test, if one pins the text, updated.
13. **A form decision carries `expect_generation`.** A decision button that
opens the form captures the head's session generation at the click. The
composer's Send for that type posts with it. The captured value is cleared when
the user selects another type, when the session generation in the head changes
(the decision is stale: the app shows "The session moved on. Choose again." as
for one-click), after a successful send, and on reset. A plain composer post
with no decision behind it sends no generation, as today. A 409 `session_moved`
on the form's Send goes through the same refusal path as a one-click decision.
Test: `frontend/src/components/entity/channels/useSessionDecisions.test.ts`
cases: open form at generation 3, send, assert the post carries 3; change type,
send, assert no generation; head moves to 4 before send, assert the form
decision is dropped and the stale message shows; 409 `session_moved` on send
shows the stale message.

## Research

- [x] ~~For larger features: run `/research` to create a structured research doc~~ (N/A: the shape follows directly from the existing guard and decider; the open choices are recorded below)
- [x] Searched for existing libraries that solve this problem
- [x] Checked codebase for similar patterns or reusable code
- [x] Looked for reference implementations in other projects
- [x] Reviewed relevant rela concepts for prior art

**Research Doc:** N/A

**Existing Solutions:**

- Libraries: none needed. The guard engine is `internal/predicate`, already
used for `on.<type>.when` (`validator.guard`, `sessionGuardEnv`, `guardHolds`).
- Codebase patterns reused:
  - `internal/realtimecfg/session.go`: `validator.transition` compiles one
entry; it is called once per list entry unchanged, with an entry key.
  - `internal/realtime/session.go`: `sessionDecider`, `poster.admits`,
`guardHolds`, `fireable`, `transitionRoles`, `sessionView`.
  - `internal/realtime/outputs.go`: `precheckOutputs`, `outputGate`,
`personMark`, `triggerMark`, `resolveMark` all look up `Session.Transition(from,
type)`; they move to `Session.TransitionTo(from, type, to)`.
  - `internal/realtimecfg/keys.go`: `keyChecker` walks the YAML tree beside
the raw type by reflection; it gets one special case for the one-or-many type.
  - SPA `decisionViews` in `channelView.ts` already groups edges per type
and opens the form when edges lead to several states or any edge is guarded.
`composerTransitions` does not group yet (it keeps the last edge per type).
- Reference implementations: XState and SCXML both model guarded
transitions as an ordered list per event where the first enabled transition wins
(XState `on: { EVT: [{target, guard}, ...] }`). Statecharts call this "first
match in document order". This plan uses the same rule and the same shape (a
mapping or a list of mappings).
- rela prior art: `internal/statemachine` transition `When:` (schema.yaml
property state machines) evaluates through predicate; the realtime guard already
follows that rule.

## Approach

- [x] Technical approach chosen and documented
- [x] Approach builds on existing patterns (not reinventing)
- [x] Alternatives considered (document why rejected)
- [x] Dependencies identified (packages, APIs, types)

**Technical Approach:**

1. **Raw shape (`internal/realtimecfg/raw.go`).**
   - Add `type rawBranches struct { entries []rawTransition; list bool }`
with `UnmarshalYAML(*yaml.Node)`: a mapping decodes into one entry
(`list=false`); a sequence decodes each element (`list=true`); any other kind
returns an error. `node.Decode` does not inherit `KnownFields`, which is fine:
`checkKeys` runs first and already reports unknown keys.
   - yaml.v3 does not call `UnmarshalYAML` for a null node (`on.<type>:
~`); it leaves the zero value. The compile step treats a zero `rawBranches` (no
entries, `list=false`) as one empty mapping entry, so the problem stays "name
the state this transition moves to" at `on.<type>.to`, as today. Only an
explicit `[]` gets the empty-list refusal.
   - `rawSessionState.On` becomes `map[string]rawBranches`.
   - `message_in.go` (`rawStates[st].On[mn]`) and `reminders.go`
(`c.states[sn].On[raw.Type]`) keep their presence tests; no change beyond the
type.
2. **Key walk (`internal/realtimecfg/keys.go`).** In `keyChecker.walk`,
before the kind switch: if `t == reflect.TypeFor[rawBranches]()`, walk a mapping
as `rawTransition` at `path`, a sequence element-wise as `rawTransition` at
`path[i]`, and report anything else as "must be a mapping or a list of
mappings". `refusedKeys` patterns such as `...on.*.run_as` already match
`on.decide[0].run_as`, since `*` matches one dot-separated segment. A test in
`load_test.go` pins this.
3. **Compile (`internal/realtimecfg/session.go`).**
   - `SessionState.on` becomes `map[string][]Transition` (file order per
type).
   - `Transition` gains `key string` (the entry's key path: `...on.<type>`
for the mapping form, `...on.<type>[i]` for the list form), `Key() string`, and
`guarded bool` (the raw `when:` is not empty, set even when the guard failed to
compile). Outputs compile under `key + ".outputs"`, so `Output.key` names the
entry. Add `Output.Key() string`.
   - In `session()`, for each type: refuse an explicit empty list ("name
at least one entry, or drop on.<type>"); refuse more than
`realtimelimits.MaxTransitionBranches` (new const, 8) entries; call
`v.transition(entryKey, mn, &e, s, ct)` per entry; then run `branchChecks(key,
list)`:
     - **Duplicate target with outputs.** A later entry with the same `to`
as an earlier one is refused at `[j].to` when either has outputs: "entry %d
already leads to %s and one of the two has outputs; a move with outputs must
name one entry, so give them different targets or move the outputs to one
entry". Without outputs on either, duplicates load (for example `[{to: working,
roles: [lead]}, {to: working, when: "message.urgent"}]`: a lead moves it, anyone
else only when urgent).
     - **Unreachable.** Entry j is refused at `[j]` when no poster can
reach it: every poster class j admits (each of its roles, or "trigger", or "any
person plus trigger" for an entry with no rule) is already admitted by some
earlier entry with `guarded == false`. Reason: "earlier entries without when
admit every poster this entry admits, so it can never fire; move it up or give
the earlier entry a when". Because the test reads `guarded`, a guard that failed
to compile does not add a second, spurious problem.
     - **No catch-all (warning).** When every entry of a list has
`guarded == true`, warn at `on.<type>`: "every entry has a when; a post that
matches none is stored and does not move the session. End the list with an entry
without when if every post should move it." Not for the mapping form, whose
behaviour is unchanged.
   - **Per-role view.** Add
`func (st SessionState) firesFor(msgType string, i int, who poster) bool` in
`realtimecfg`, where `who` is a role name or the trigger marker. It is true when
entry i admits `who` and no earlier entry with `guarded == false` admits `who`.
An entry with no rule admits every role of the type's `post` and triggers;
`roles` admits those roles; `by: trigger` admits only triggers.
   - Accessors (replace `Session.Transition`, so the compiler flags every
caller):
     - `(s *Session) Branches(state, msgType string) []Transition`: the
entries in file order, copies (roles cloned).
     - `(s *Session) TransitionTo(state, msgType, to string) (Transition,
bool)`: the first entry with that target. It is unique for every entry with
outputs, which is all a mark or the gate reads.
     - `SessionState.Transitions()` returns every entry, sorted by message
type, then file order.
     - `Session.Drives` unchanged in meaning.
   - Walkers use the per-role view:
     - `fireableBy(t, role, mt)` becomes `fireableBy(st, mn, i, role,
mt)`, which also requires `st.firesFor(mn, i, role)`. `isAnswer`, `answers`,
`deadlineReset` (reporting at `t.Key()`), `returnsTo` and `answerGrants` call
it. So a shadowed entry is never counted as a role's answer, nor as a link in a
role's chain.
     - `jobEdges` (`session.go`) adds an entry only when it is `job` and
`st.firesFor(mn, i, trigger)`; its key becomes `t.Key()`. `jobOutputs`
(`grants.go`) uses the same test, so an entry no trigger can reach does not
demand `run_as`.
     - `transitionCause` marks an entry `job` only when a trigger can
reach it: the existing test plus "no earlier unguarded entry admits triggers".
It runs after all entries of the list are compiled.
     - `handoffs` walks every entry (it warns about a value, whoever
fires it).
4. **Decider (`internal/realtime/session.go`).**
   - `sessionDecider`: replace the single lookup with a loop over
`s.Branches(cur.State, mt.Name)`. For each entry: skip if `!p.admits(t)`;
evaluate `guardHolds`; record the first evaluation error in `failed` (with the
entry) and treat it as not holding; the first entry that holds is chosen. Then
apply `gate.admits(chosen, cur.State)`. If the gate refuses, return no move and
do not fall through to later entries. Return `SessionMove{From, To: chosen.To}`.
   - `logGuardFailure`: message "a session transition guard failed to
evaluate; the entry was skipped", with `to` and `entry` (`t.Key()`) attributes.
`WarnGuardOnce` stays per entry.
   - `transitionRoles`: collect roles over all entries of the type in all
states.
   - `fireable`: a type is listed when any entry of it admits the poster;
`slices.Sort` then `slices.Compact`, so each type appears once.
   - `sessionView`: one `SessionTransitionView` per entry, from
`st.Transitions()`; no new fields. The wire already allows a repeated
`message_type`.
5. **Outputs (`internal/realtime/outputs.go`).**
   - What the gate guarantees: a move along an entry *with outputs*
happens only if the pre-check checked that entry's outputs. A switch under the
lock to an entry without outputs still moves; that writes nothing, so it needs
no check. The gate cannot be turned into a 4xx without giving `Decide` an error
result on all four stores (`channels.DecideSession` is the single path; out of
scope), so a gate refusal stores the post without a move, as today.
   - With `expect_generation`, `Admit` already pins state, generation and
`entered_at`, so only `session.reminders` counters can switch the entry between
the pre-check and the lock.
   - `outputGate` gains `to string`; `precheckOutputs` sets `from` and
`to` of the move it checked. While active, `admits(t, from)` is
`len(t.Outputs()) == 0 || (g.from == from && g.to == t.To)`.
   - `precheckOutputs`, `personMark`, `triggerMark`: look up
`sess.TransitionTo(move.From, mt, move.To)`.
   - The denial title uses `o.Key()`; `outputKey` is deleted.
   - `resolveMark`: `sess.TransitionTo(mark.From, cause.Type, mark.To)`;
the `t.To != mark.To` check folds into the lookup. A mark whose entry was
removed or retargeted by a reload is removed, as today.
   - No change to `channels.SessionMove`, `channels.OutputMark` or any
backend: `(from, type, to)` identifies every entry with outputs.
6. **Triggers and reminders.** Triggers use `newSessionPost` with
`poster{trigger: true}`, so they branch with no extra code. The trigger skip
rule for `in:` is per type and unchanged. Reminders post types no transition is
keyed by (`Drives`), unchanged.
7. **SPA.**
   - `frontend/src/components/entity/channels/channelView.ts`
`composerTransitions`: group edges per message type like `decisionViews`;
produce `toLabels: string[]` (deduped, in edge order) and `guarded` when any
edge is guarded.
   - `frontend/packages/rela-components/src/components/channel/types.ts`
`ChannelComposerTransition`: `toLabel: string` becomes `toLabels: string[]`.
   - `RlChannelComposer.vue`: one target keeps `channelSessionHint`;
several call the new `channelSessionBranchHint({ tos })`.
   - `useMessages.ts` (en): "Sending this may move the session to
${orList(tos)}." `messages.nl.ts` (nl): "Dit bericht kan de sessie op
${ofList(tos)} zetten." The wording says "may", because the SPA cannot see roles
or whether a catch-all exists.
   - **Form decisions carry `expect_generation`.**
`useSessionDecisions.ts` `openForm` records `{ messageType, generation }` (the
head's `session.generation` at the click) in a `formDecision` ref it exposes.
`OpenChannel.vue` `onPost` passes `formDecision.generation` as
`expectGeneration` to `postTyped` when `event.type ===
formDecision.messageType`. The ref is cleared when the composer selects another
type, when a head frame brings another generation (then the app shows the
existing "The session moved on. Choose again." message), after a successful
send, and on composer reset. A 409 `session_moved` on that send uses the
decisions' existing `refused` path. `api/channels.ts` already sends
`expect_generation`.
   - Update fixtures in `rela-components/src/fixtures/channels.ts` and
stories that build `ChannelComposerTransition`.
   - `frontend/src/types/channels.ts` `ChannelSessionTransition` doc
comment: a type may appear more than once, in the order rela tries the entries.
8. **MCP, CLI, OpenAPI.**
   - `internal/mcp/tools_channels.go` (`read_channel`, `read_session`) and
`tools_inbox.go`: add "A message type may lead to several states; its entries
are listed in the order rela tries them, and the first whose poster rule and
guard hold moves the session."
   - `post_message`: add "If no transition entry applies, the message is
stored and the session stays; compare session.generation in the answer."
   - `internal/openapi/channels.go`: the first sentence as the
`transitions` array description; regenerate the OpenAPI goldens.
   - CLI: `internal/cli/channel_render.go` does not render transitions; no
change. `rela channel post` already passes props and `--expect-generation`.

**Alternatives considered:**

- *A new key such as `branches:` or `cases:` under the entry.* Rejected:
two ways to say one thing, and the list form matches XState/SCXML.
- *Store the entry index on `OutputMark` so duplicate targets with
outputs work.* Rejected for this slice: it changes the mark JSON on four
backends, and an index is fragile across reloads that reorder entries. Refusing
duplicates only when outputs are involved keeps `(from, type, to)` unique where
it matters. It can be lifted later.
- *Refuse every duplicate target.* Rejected (design review): a guard
cannot read the poster, so entries that differ only by poster rule could not be
written at all.
- *Refuse a post that matches no entry with a typed 409.* Rejected for
now; see Q1.
- *Derive per-value buttons from guards by inspecting the predicate AST.*
Rejected: brittle for anything but `message.p == 'lit'`; see Q4.
- *Fall through to the next entry when the gate refuses.* Rejected: the
pre-check and the store would then follow different rules.

**Dependencies:** `internal/predicate`, `internal/predicatefns` (no change),
`gopkg.in/yaml.v3` (custom unmarshaler), `internal/realtimelimits` (new
constant), `internal/apiwire/v1` (no change), `internal/openapi` (description
only).

**Files to modify:**

- `internal/realtimelimits/limits.go`: `MaxTransitionBranches = 8`.
- `internal/realtimecfg/raw.go`: `rawBranches` and its `UnmarshalYAML`.
- `internal/realtimecfg/keys.go`: one-or-many walk.
- `internal/realtimecfg/session.go`: `on map[string][]Transition`,
`Transition.key/Key()/guarded`, `Branches`, `TransitionTo`, `Transitions()`,
`firesFor`, list compile, `branchChecks`, null handling, `jobEdges`.
- `internal/realtimecfg/outputs.go`: `Output.Key()`, `transitionCause`
per-entry trigger reachability.
- `internal/realtimecfg/waiting.go` (per-role `fireableBy`),
`grants.go` (`jobOutputs`), `message_in.go` and `reminders.go` (type only).
`clone.go` needs no change: it does not copy the session.
- `internal/realtime/session.go`: decider loop, `logGuardFailure`,
`fireable`, `transitionRoles`, `sessionView`.
- `internal/realtime/outputs.go`: gate `(from, to)`, `TransitionTo` in
pre-check, marks and `resolveMark`, `o.Key()` in the denial, delete `outputKey`.
- `internal/mcp/tools_channels.go` (`read_channel`, `read_session`,
`post_message`), `internal/mcp/tools_inbox.go`.
- `internal/openapi/channels.go`; OpenAPI goldens under
`internal/dataentry/testdata/golden/`.
- `frontend/src/components/entity/channels/channelView.ts`,
`useSessionDecisions.ts`, `OpenChannel.vue`, `frontend/src/types/channels.ts`,
`frontend/packages/rela-components/src/components/channel/types.ts`,
`RlChannelComposer.vue`, `composables/useMessages.ts`,
`composables/messages.nl.ts`, `fixtures/channels.ts`,
`RlChannelComposer.stories.ts`.
- Tests (existing files unless marked new):
`internal/realtimecfg/{session,load,waiting,timers,runas,
docs_example}_test.go`; `internal/realtime/{session,
session_affordance,outputs,outputs_lock,outputs_runner,inbox,trigger,
concurrent}_test.go`; `internal/dataentry` channels wire test;
`frontend/src/components/entity/channels/channelView.test.ts`,
`useSessionDecisions.test.ts`,
`frontend/src/components/entity/ChannelsPanel.test.ts`.
- Docs: `docs/realtime.md`; `docs-project/entities/guides/GUIDE-realtime.md`
only if it shows `on:` (it does not today, so likely no change).

## Security Considerations

- [x] Input sources identified (user input, config, external APIs)
- [x] Input validation approach defined (allowlist preferred over blocklist)
- [x] Security-sensitive operations identified (file access, auth, crypto)
- [x] Error handling doesn't leak sensitive information

**Input Sources & Validation:**

- *`realtime.yaml` (operator config).* The list is checked key by key
against `rawTransition` by `keyChecker` (allowlist of keys), capped at 8
entries, each entry compiled as today. Guards compile in `sessionGuardEnv`,
which declares only `message`, `session` and the standard library; a guard that
reads the entity stays refused. Invalid config fails the load with keyed
problems; a reload with problems keeps the last good config (existing
`reload.go` behaviour).
- *`expect_generation` from the SPA form.* An integer the client captured
from the head; the server validates it as today (400 for a negative value, 409
`session_moved` on mismatch). It only narrows when a post is stored.
- *Message props (user input).* Validated by the message type's property
schema before the decider runs, as today. Props only select an entry; they
cannot name a target state. A poster can only reach states that some entry
admits for their roles.

**Security-Sensitive Operations:**

- *Who may move the session.* Each entry keeps its own `roles` / `by`
rule, checked by `poster.admits` before its guard. A person cannot pick an entry
that names a role they do not hold, nor a `by: trigger` entry. The
unreachable-entry check does not loosen this; it only refuses entries that can
never fire. The per-role view in the load checks matches the decider's
first-match rule, so a waiting role is not counted as answered by an entry it
can never reach.
- *Graph writes (outputs).* Written only for the entry that fired, with
the same ACL pre-check (`writer.Check`) and the gate under the lock that stops a
move along an entry with outputs the pre-check did not check (AC 9). An entry
with outputs always has a unique `to` in its list. `resolveMark` re-resolves by
`(from, type, to)` and removes marks that no longer match, so a reload cannot
make a stored mark run another entry's outputs.
- *Read side.* `session.transitions` is config, the same for every reader
(per CLAUDE.md "The configuration is not a secret"); listing several edges
discloses nothing new. `_actions.transitions` stays per caller.
- *Errors.* Load problems name config keys only. A guard failure is
logged server-side with channel type, state, message type, target and entry key,
not message content; the post answer does not reveal which guard failed.
- *Cost under the store lock.* At most 8 guards per post, each bounded by
predicate's step budget; no I/O. Same order as today's single guard.

## Test Plan

- [x] Test scenarios documented for each acceptance criterion
- [x] Edge cases identified and documented
- [x] Negative test cases defined (invalid input, error conditions)
- [x] Integration test approach defined (not just unit tests)

**Test Scenarios:**

| AC | Test |
| --- | --- |
| 1 | `realtimecfg` `TestSession_BranchList`; existing load, docs-example and testdata tests unchanged |
| 2 | `realtime` `TestSessionBranches_FirstWins` via `Service.Post` (memstore channel store, real config) |
| 3 | `TestSessionBranches_PosterRulePerEntry`, person with and without role, trigger post |
| 4 | `TestSessionBranches_NoMatchStoresWithoutMove` |
| 5 | `TestSessionBranches_GuardErrorFallsThrough` with slog capture |
| 4 | `TestSession_BranchListWarnings` (no catch-all warning) |
| 6 | `TestSession_BranchListProblems` (table), key-walk cases in `load_test.go` |
| 7 | new cases in `waiting_test.go` (incl. shadowed answer), `timers_test.go`, `runas_test.go` |
| 8 | `TestOutputs_BranchUsesFiredEntry`, `TestResolveMark_Branch` |
| 9 | `TestSessionBranches_GateRefusesOtherEntry` (reminder-count switch; with and without outputs on B) |
| 10 | `TestSessionView_BranchEdges`, inbox no-duplicate case, dataentry wire case |
| 11 | `channelView.test.ts` `composerTransitions` and `decisionViews` cases, hint case in `ChannelsPanel.test.ts` (en + nl) |
| 12 | `docs_example_test.go` new case, OpenAPI goldens |
| 13 | `useSessionDecisions.test.ts` form-decision generation cases |

Integration: AC 2, 3, 4, 8 run through `Service.Post` against the in-memory
channel store and a compiled config, so the store lock path
(`SessionStep.Decide` run under the lock) is exercised. Add one case to
`internal/realtime/concurrent_test.go`: two concurrent posts of `decide`
(`approve` and `revise`) with the same `expect_generation`; exactly one moves
the session and the other gets 409 `session_moved`. The `storetest` harness
needs no change, because the store does not change. The race detector is on.

**Edge cases:**

- Single mapping form: unchanged behaviour and keys (regression net:
the whole existing test suite).
- One-entry list: same as the mapping, but problem keys carry `[0]`.
- Optional enum left out: every `message.decision == ...` comparison
yields `ErrNilOperand`, so no entry applies; stored without move.
- Last entry without `when` as a catch-all: allowed, fires when no
earlier entry applies.
- Catch-all entry first: later entries refused as unreachable, unless
their poster rule is not covered (for example entry 1 `roles: [lead]` with no
`when`, entry 2 `roles: [member]`).
- Same target, different poster rules, no outputs
(`[{to: working, roles: [lead]}, {to: working, when: "message.urgent"}]`):
loads; `TransitionTo` returns the first.
- Shadowed for one role only: `[{to: a, roles: [owner]}, {to: b}]` loads
(the runner reaches `b`), but the owner's answers and chains see only `a`.
- Guard in entry 1 fails to compile: one compile problem, no extra
"can never fire" on entry 2.
- `on.decide: ~`: today's "name the state this transition moves to" at
`on.decide.to`.
- Form decision opened, then the user switches type and back: no
generation is sent (the capture was cleared on the switch).
- Entry leading to the same state (re-entry): allowed, as today; the
deadline-reset check runs per entry.
- Entry leading to a final state with `on_final: close`: the mark closes
the channel only for that entry.
- Mixed entries: one `by: trigger`, one with `roles`; a trigger's post
takes the first, a person's post the second.
- Stranded or final current state: `Branches` returns none; no move.
- Closed channel: no move, as today.
- Reload between pre-check and append: the decider holds the old
`*Session` it was built with, as today.
- Reload between move and output retry: `resolveMark` finds the entry by
`(from, type, to)` or removes the mark.
- Unicode or odd characters in enum values: compared by predicate as
strings; no special handling.

**Negative Tests:**

- `on.decide: []` refused: "name at least one entry, or drop on.decide".
- `on.decide: working` (scalar) refused: "must be a mapping or a list of
mappings".
- 9 entries refused: "at most 8 entries per message type".
- Two entries with `to: working`, one with outputs, refused at `[1].to`.
- Entry after an unguarded, covering entry refused at `[1]`.
- A waiting role whose only answer is shadowed by an earlier unguarded
re-entry for that role refused at `waiting_for[i]`.
- Unknown key inside an entry refused at `on.decide[0].nope`.
- `run_as` inside an entry refused with the existing reason, at
`on.decide[0].run_as`.
- Guard reading the entity in entry 2 refused at `on.decide[1].when` with
the existing reason.
- `roles` naming a role that may not post the type, in entry 2, refused at
`on.decide[1].roles[0]`.
- State not in the type's `in:` refused once at `on.decide`.
- Runtime: a person's post that would only match a `by: trigger` entry
does not move the session.
- Runtime: an output denial on entry 2 refuses the post with 403
`session_output_denied` naming `on.decide[1].outputs[0]`, and nothing is stored.

## Risk Assessment

- [x] Technical risks assessed with mitigations
- [x] Security risks assessed (see Security Considerations)
- [x] Effort estimated (xs/s/m/l/xl)

**Risks:**

- *A caller keeps looking up one transition per type and silently picks
the wrong entry.* Mitigation: remove `Session.Transition` so the compiler lists
every caller (about 10 sites in `realtime` and `realtimecfg`, plus tests); each
moves to `Branches` or `TransitionTo`.
- *Pre-check and decider disagree on the entry.* Mitigation: the gate
records `(from, to)`; AC 9 pins it. The decider never falls through after a gate
refusal. With `expect_generation`, only reminder counters can cause a switch. A
switch to an entry without outputs still moves, which writes nothing and is
documented.
- *Load checks and the decider disagree about which entries a role can
fire.* Mitigation: one per-role helper (`firesFor`) used by every graph walker;
AC 7 includes a shadowed-answer case.
- *A stale form decision lands on a re-entered state.* Mitigation: AC 13
sends the generation captured at the click.
- *Problem keys change for existing configs.* Mitigation: the mapping form
keeps `on.<type>` keys; only list entries get `[i]`. Existing key tests stay
green.
- *Authors expect a post that matches no entry to be refused.* Mitigation:
documented with the advice to make the enum required and cover every value, or
end with an unguarded catch-all. Q1 leaves room for an opt-in refusal later.
- *Duplicate-target-with-outputs refusal blocks a real config.*
Mitigation: duplicates without outputs load; templated outputs
(`{{message.<p>}}`) cover most "same target, different value" needs; the refusal
can be lifted with an entry index on the mark.
- *SPA library prop change (`toLabel` to `toLabels`).* Mitigation: one
consumer (`OpenChannel.vue`) and the fixtures; type-check catches all.
- *Role-split edges look the same to every caller in the SPA.*
Accepted: `session.transitions` is config and carries no roles; the "may move"
hint stays true. Per-caller edges would be a later wire change.
- *Mixed-version processes.* An older process refuses a list under `on:`
as "must be a mapping", so it fails the load loudly rather than misreading the
config. Documented like `in:` and `expect_generation`.

**Effort:** m.

## Documentation Planning

- [x] User-facing docs identified (skip if internal refactor)
- [x] Docs-checklist will be created when entering implementation

**Documentation Impact:**

- [x] ~~docs/metamodel.md - New metamodel features~~ (N/A: realtime config is documented in docs/realtime.md)
- [x] ~~docs/cli-reference.md - New/changed commands~~ (N/A: no CLI change)
- [x] docs/realtime.md, section "Sessions": the `on.<type>` table row says
"an entry, or a list of entries tried in order"; a new paragraph "**Several
targets for one message type.**" with the `decide` example (enum `decision`, two
guarded entries, `in: [review]`); the first-match rule; per-entry poster rules;
"no entry applies: stored, no move; compare session.generation in the answer";
the no-catch-all warning; the duplicate-target-with-outputs and unreachable
rules; that the waiting checks judge entries per role; that the gate only holds
a move to an entry with outputs; the version note (every process needs this
release). Also: "If you want one-click buttons, two message types remain the
better choice for now; a branching type opens the form."
- [x] docs/realtime.md, "Checks when the file loads": add the list checks.
- [x] docs/realtime.md, "Limits": at most 8 entries per message type and
state.
- [x] docs/realtime.md, "In the web app": the composer says a branching
type may move the session to any of its targets; a branching type opens the
form, where an enum is a select; a form decision sends `expect_generation` like
a one-click decision.
- [x] docs/realtime.md, "Agents": `post_message` answer, the no-match
sentence.
- [x] docs/realtime.md, "HTTP API" / "Agents": `session.transitions` may
repeat a message type, in try order.
- [x] ~~CLAUDE.md - New patterns or conventions~~ (N/A: no new convention)
- [x] ~~README.md - Project-level changes~~ (N/A)
- [x] ~~docs/data-entry.md - UI changes~~ (N/A: the realtime web app is documented in docs/realtime.md)

## Open questions (decided)

- **Q1. What if no entry applies?** Decision: store the message and do not
move, the same as today's single guard that does not hold. Reason: a transition
is part of what a message means, not a permission (the rule in
docs/realtime.md); changing that for lists only would make the mapping and the
one-entry list behave differently. Authors who need a guaranteed move make the
enum `required` and cover every value, or add an unguarded last entry. A list
without a catch-all loads with a warning, and `post_message` and the docs tell
agents to compare `session.generation`. A strict opt-in (for example
`on.<type>.strict: true` giving 409 `session_no_branch` before storing) can be a
follow-up if the PoC asks for it.
- **Q2. How is `when` evaluated?** Decision: through `internal/predicate`
in the existing `sessionGuardEnv`: `message.<prop>`, `session.state`,
`generation`, `entered_at`, `reminders`, and the standard library. No entity.
The key stays `when:` (predicate expression), as it is today; there is no
filter-syntax sibling here, so the `condition:` vs `when:` rule in
`.claude/rules/predicate.md` is not affected and no new key is added.
- **Q3. Load checks.** Decision: refuse an explicit empty list, more than
8 entries, a duplicate `to` in one list when either entry has outputs, and
entries no poster can reach (judged by the raw `when:` text). Warn when no entry
is a catch-all. Graph checks judge entries per role, so a shadowed entry never
counts for that role. `in:` consistency stays per type and state. Final states
still have no `on:`. `on.<type>: ~` keeps today's problem. No check tries to
prove an enum is fully covered by guards (needs AST inspection; documented
advice instead).
- **Q4. Per-value one-click buttons (approve and revise from one enum
type).** Decision: out of scope. The server sends one row per edge with
`guarded: true`; the SPA's `decisionViews` already opens the form for a type
with several targets, and the form shows the enum as a select. Deriving preset
props from guard text is brittle. A follow-up ticket should add an authored
per-entry preset (for example `button: { label, props }`), mapped to
`ChannelDecisionView` with `key: "<type>:<to>"` and `props`; the server needs no
new check because the post is validated like any other. It overlaps TKT-03FQPD
and TKT-TXZZZ2, so it should be planned with them. Proposed follow-up ticket:
**"Authored one-click presets for session transition entries"**: let a
transition entry declare `button: { label, props }` so the web app shows one
one-click decision per entry (such as Approve and Revise from one enum type),
posting the preset props with `expect_generation`. Until then the docs recommend
two message types for one-click buttons.
- **Q5. Affordance fields.** Decision: no new wire fields.
`session.transitions` already carries `message_type`, `to`, `guarded`, `by` and
`outputs` per row, and may now repeat a type. Order: by message type, then file
order. `_actions.transitions`, inbox and waiting lists stay lists of message
types, deduplicated. The rows carry no roles, so the SPA shows every edge of a
role-split type to every caller; the hint says "may move" to stay true. Form
decisions send `expect_generation` (AC 13).
- **Q6. Server decision under the lock.** Decision: `sessionDecider` runs
the same loop the pre-check runs (same props, poster facts and `now`), and the
gate holds a move along an entry with outputs to the pre-checked `(from, to)`. A
gate refusal stores the post without a move; turning it into a 4xx would need an
error result from `Decide` on all four stores, which is out of scope. Triggers
use the same decider. Reminders are unaffected.
- **Q7. MCP, CLI, docs, OpenAPI.** Decision: description updates only in
MCP (`read_channel`, `read_session`, inbox, `post_message`) and OpenAPI; no CLI
change; docs as listed above.
- **Q8. Backwards compatibility.** Decision: the mapping form is
unchanged in meaning and in problem keys. Configs need no migration. An older
process refuses the list form at load.

## Design Review

- [x] Run `/design-review` before starting implementation
- [x] All critical/significant findings addressed in plan

**Design Review Findings:** 14 findings (3 significant, 8 minor, 3 nits) in
`.ignored/dr-fpxtqj.tsv`, resolutions in `.ignored/dr-fpxtqj-res.txt`.
Significant: duplicate `to` now refused only with outputs (1); graph checks
judge entries per role, including `jobEdges` and `transitionCause` (2); form
decisions carry `expect_generation`, AC 13 (3). All minor and nit findings are
addressed in the plan.

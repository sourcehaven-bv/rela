---
id: PLAN-3JOPEX
type: planning-checklist
title: 'Planning: Session option to offer transition types only where they apply'
started: "2026-10-30"
completed: "2026-10-30"
status: done
---

<!-- @managed: claude-workflow v1 -->

## Understanding

- [x] Problem/requirements clearly understood
- [x] Scope defined (what's in/out documented below)
- [x] Acceptance criteria documented with specific test scenarios

**Problem.** TKT-QYFLRH added `in:` on a message type: the session states in
which the type may be posted. A type that decides a session should have one, so
the composer offers it only where it applies and a late post is refused. For
almost every such type, the right `in:` is exactly the states whose `on:` keys
it. The load even refuses an `in:` that misses one of those states. So an
operator writes the same list twice, once as `on:` keys and once as `in:`, and
has to keep both in step.

The test fixture `pocRealtime` (`internal/realtime/session_affordance_test.go`)
shows the cost: 17 of its 20 types carry an `in:` that only repeats the `on:`
keys. The real PoC configs
(`rela-agent-runner-poc/mission/project/realtime.yaml` and
`project/realtime.yaml`) have no `in:` at all. So today the owner's composer
offers all eleven owner types in every state, and a late Allow is stored and
moves nothing. The ticket's claim that the PoC copies `in:` is true of the
fixture that models it, not of the PoC files themselves.

**Terms used below.**

- *Keyed state* of a type: a state whose `on:` has the type as a key, in any
form (one entry or a list).
- *Authored `in:`*: an `in:` the file writes on the message type.
- *Derived `in:`*: the `in:` the load fills in from the keyed states.

**Scope (in):**

- `realtime.yaml`: `channel_types.<ct>.session.default_in`, a closed enum
with one value, `keyed`. Absent keeps today's behaviour. Checked at load, on
reload and by `rela validate`.
- At load, with `default_in: keyed`: every message type that has at least
one keyed state and no authored `in:` gets `MessageType.In` set to its keyed
states, in state declaration order.
- An unexported `derived` flag on `MessageType`, set only by the loader,
and one helper `(ct *ChannelType) InKey(mt *MessageType) string` that builds the
key path that decided `In`. The 409 `session_wrong_state` detail and the trigger
skip warning call it, so a derived `in:` points at `session.default_in` instead
of a key that is not in the file. The zero value gives today's string.
- One load warning when the key changes nothing.
- A key hint for `default_in` misplaced under a state.
- The refusal of an authored `in:` that misses a keyed state also suggests
dropping the `in:` when `default_in` is set.
- Docs in `GUIDE-realtime.md` and `GUIDE-mcp-server.md`, regenerated into
`docs/realtime.md` and `docs/mcp-server.md`.
- One sentence in the OpenAPI post description.

**Scope (out):**

- Any change to the runtime decision of TKT-QYFLRH (`sessionRefusal`,
`admitPost`, `postable`, `sessionAffordances`). See Approach 4.
- Serving `in:` or `default_in` in the schema descriptor, OpenAPI schemas
or MCP tools. Today no `in:` is served anywhere; see Approach 5. Filed as the
follow-up "Serve the effective in: on message type views".
- A way to keep a keyed type postable in every state other than listing
every state in its `in:`. See Q4.
- A warning for an authored `in:` that equals the derived one. See Q5.
- Changing the PoC. Filed as the follow-up "PoC: use session default_in in
the run session".

**Acceptance Criteria:**

1. **The key loads.** `session: { default_in: keyed, ... }` loads. Absent
or null `default_in` loads as today. Any other value is refused at
`channel_types.<ct>.session.default_in` with "default_in may only be keyed",
case-sensitive, as `on_final` is. `default_in` under a state is refused with the
hint "default_in belongs to the session: session.default_in: keyed".
`default_in` on a message type is an unknown key. A reload that adds or removes
the key is accepted. Test: `internal/realtimecfg/default_in_test.go` (new)
`TestParse_SessionDefaultIn` (keyed; absent; null; `all`; `Keyed`; a list; under
a state; on a message type), and rows "default_in added" and "default_in
removed" in the `CheckReload` table of `internal/realtimecfg/session_test.go`.
2. **Keyed types get a derived `in:`.** With `default_in: keyed`, a type
with no authored `in:` gets `In` equal to its keyed states, in the order of
`session.states`, and `ct.InKey(mt)` equal to
`channel_types.<ct>.session.default_in`. Every entry form keys a state: a single
entry, a list of entries, a guarded entry, an entry shadowed by an earlier
unguarded one, a `by: trigger` entry, and an entry whose `roles` no caller
holds. Test: `TestDefaultIn_Derive` table: "single entry"; "list of entries keys
its state once"; "type keyed in two states lists both in declaration order";
"by: trigger entry keys its state"; "shadowed entry keys its state"; "roles-only
entry keys its state".
3. **An authored `in:` widens, and unkeyed types stay free.** A type with
an authored `in:` keeps it, and `ct.InKey(mt)` is
`channel_types.<ct>.message_types.<mt>.in`. The TKT-QYFLRH checks still apply to
it: a keyed state missing from it is refused. With `default_in: keyed` that
refusal ends with "or drop in to take the keyed states from session.default_in".
So an authored `in:` always contains the keyed states and can only add states to
them. A type with no keyed state, such as `text`, a notes type, a reminder's
post type or a `choices` answer that no transition is keyed by, keeps `In ==
nil`. An ephemeral type never has a keyed state, because the load refuses one in
`on:`. Test: `TestDefaultIn_Derive` cases "authored in widens"; "authored in
missing a keyed state is refused and suggests dropping in" (pins the full text);
"unkeyed type stays nil"; "reminder post type stays nil"; "unkeyed choices
answer stays nil".
4. **Without the key, nothing changes.** A file without `default_in`
compiles to the same `In` as before for every type, and `ct.InKey(mt)` is the
`message_types.<mt>.in` path for every type. A `MessageType` built by hand, with
`In` and no flag, gets the same path. Test: `TestDefaultIn_AbsentKeepsToday`;
the expectations of every existing test in `message_in_test.go`,
`session_test.go` (`TestPost_MessageIn*`), `session_affordance_test.go`
(`TestAffordances_MessageIn`) and `trigger_test.go`
(`TestSessionTriggerPost_MessageIn`) are unchanged. The rows of
`TestAffordances_MessageIn` move into a function (AC 6), but no expected value
changes. `usertext_test.go` `sessionWrongState` stays green without edits.
5. **The PoC fixture needs no copied `in:`.** In `internal/realtimecfg`,
`pocRealtime(pocIn minus the 17 keyed types, "      default_in: keyed\n")`
compiles to the same `In` for each of its 19 types as `pocRealtime(pocIn, "")`.
The 17 keyed types have the `session.default_in` key; `progress`, which no
transition is keyed by, keeps its authored `in:` and its
`message_types.progress.in` key; `text` stays free. Test:
`internal/realtimecfg/default_in_test.go` `TestDefaultIn_PoCEquivalent`, using
the existing `pocRealtime(in, session)` and `pocIn` of `message_in_test.go`.
6. **The runtime is unchanged and reads the derived `in:`.** The rows of
`TestAffordances_MessageIn` move into `messageInRows(fixture string)`, and the
harness and every setup closure build their config from `fixture`. The stranded
row renames `working` in that text, so over the default_in fixture it stays a
default_in config, and it asserts that the snapshot it installs has `ct.InKey`
of `finished` equal to the `session.default_in` path.
`pocRealtimeDefaultIn(triggers)` is `pocRealtime(triggers)` with the 17 copied
`in:` lines removed and `default_in: keyed` added. Over it, every row gives the
same `_actions.post` and `_actions.transitions` as over `pocRealtime`, including
the stranded and final-state rows, and `_actions.entries` is the same. A post of
`approve` while working is refused 409 `session_wrong_state`; its title names
`plan-review` and `working`, and its rule is
`channel_types.run.session.default_in`. Nothing is stored and no rate token is
spent. Test: `session_affordance_test.go` `TestAffordances_MessageIn` and
`TestAffordances_DefaultIn`, both running `messageInRows`; `session_test.go`
`TestPost_DefaultIn` (the refusal, its title and rule, the stored head and the
bucket); `internal/dataentry/channels_handler_test.go`
`TestChannelsHandler_SessionWrongState` new case "derived in names
session.default_in" (problem `detail`).
7. **A skipped trigger names the rule.** Over `pocRealtimeDefaultIn` with
the `ready` trigger, a trigger that posts `go` while the session is working is
skipped with the existing warning, whose `in` attribute is
`channel_types.run.session.default_in`. Test:
`internal/realtime/trigger_test.go` `TestSessionTriggerPost_DefaultIn`.
8. **Load checks see the derived `in:`.** The derived `in:` is set before
`choiceChecker` and `typeButtonChecker` run, so the `choices` warning judges the
answer's derived `in:`. With `default_in: keyed`, the docs' `ask` and `answer`
example without `in: [asking]` loads with no warning. A question whose
transitions lead only to states where the answer is not keyed warns "the buttons
never show", as with an authored `in:`. `decision_button` checks are unchanged,
because they read entries, not `in:`. Test: `TestDefaultIn_Checks` cases
"choices answer keyed in the question's target loads clean"; "choices answer
keyed elsewhere warns"; "decision_button on a derived type loads clean".
9. **The key warns when it changes nothing.** With `default_in: keyed` and
no type getting a derived `in:`, the load warns at `...session.default_in`:
"default_in: keyed has no effect, because every message type a transition is
keyed by declares in:; drop the in: lists that only repeat the on: keys, or drop
default_in". A session whose states have only timers warns "default_in: keyed
has no effect, because no transition is keyed by a message type; drop
default_in". A warning does not fail a reload. Test: `TestDefaultIn_Checks`
cases "every keyed type has in: warns"; "timers only warns"; "one derived type
does not warn".
10. **Docs.** `GUIDE-realtime.md` and `GUIDE-mcp-server.md` document the
key, the rule, its limits and the opt-out (see Documentation Impact). The new
example is the plan example of "Where a message type may be posted" with
`default_in: keyed` and without the three copied `in:`. Test:
`default_in_test.go` `TestDocsExample_DefaultIn` loads the block after the
paragraph "**Take `in:` from the transitions.**" with no problems and no
warnings, and asserts `plan` in `[drafting]`, `approve` and `revise` in
`[review]`, each with the `session.default_in` key, and `text` free. The prose
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
  - `messageInCheck` (`internal/realtimecfg/message_in.go`): compiles
`in:` after the states, transitions and reminders. Its check that a keyed state
must be in `in:` already reads the keyed states from `rawStates[st].On[mn]`. The
derivation reads the same thing.
  - `session.on_final` (`session.go`, `raw.go`, `keys.go`): a session-level
closed enum with one value, read as a plain string, refused with "may only be
close", and a hint for the key misplaced under a state. `default_in` copies all
three.
  - `default_view`, `default_sort`, `default_depth` and `default_world` in
`data-entry.yaml` and `schema.yaml`: a `default_<key>` key gives the value of
`<key>` where none is written. `default_in` follows that pattern for `in:`.
  - `MessageType.PostableIn` and `sessionRefusal`
(`internal/realtime/session.go`): the one runtime decision. It reads only `In`,
so a derived value needs no runtime change.
  - `sessionWrongState` (`session.go:484`) and the trigger skip warning
(`trigger.go:1246`): today each builds the rule as `...message_types.<mt>.in` by
hand. `ChannelType.InKey` replaces both.
  - `choicesReachable` (`choices.go`) reads `PostableIn`, so it sees the
derived value as long as the derivation runs first. It does: the session
compiles inside `channelType` before `choiceChecker`.
  - `pocRealtime(in, session)` and `pocIn`
(`internal/realtimecfg/message_in_test.go`): a PoC fixture that takes the `in:`
per type and extra session keys. AC 5 uses it as is.
  - `pocRealtime(triggers)` and `TestAffordances_MessageIn`
(`internal/realtime/session_affordance_test.go`): the fixture and table that
pinned TKT-QYFLRH. AC 6 moves the rows into a function that takes the fixture
text.
- Reference implementations: state-machine tools derive the events a state
accepts from its transitions. XState's `state.can(event)` and its inspector list
only events with a transition out of the current state. Statecharts generally
treat an event without a transition as ignored, which is today's rela behaviour
without `in:`. This option lets an operator choose the stricter reading per
session.
- rela prior art: TKT-QYFLRH (`in:` and its runtime), TKT-FPXTQJ (branch
lists), TKT-S9TLJJ and TKT-TXZZZ2 (decision buttons, read from entries, not from
`in:`), TKT-03FQPD (`choices`).

## Approach

- [x] Technical approach chosen and documented
- [x] Approach builds on existing patterns (not reinventing)
- [x] Alternatives considered (document why rejected)
- [x] Dependencies identified (packages, APIs, types)

**Technical Approach:**

1. **Shape.**

   ```yaml
   message_types:
     text:    { properties: { body: { type: markdown, required: true } }, post: [owner, member] }
     plan:    { properties: { plan: { type: markdown, required: true } }, post: [member] }
     approve: { properties: { note: { type: string } }, post: [owner] }
     revise:  { properties: { feedback: { type: markdown, required: true } }, post: [owner] }
   session:
     initial: drafting
     default_in: keyed
     states:
       drafting:
         on:
           plan: { to: review }
       review:
         waiting_for: [owner]
         on:
           approve: { to: approved, roles: [owner] }
           revise:  { to: drafting, roles: [owner] }
       approved: { final: true }
   ```

`plan` may be posted only in `drafting`, `approve` and `revise` only in
`review`, and `text` in every state.

2. **The key is `session.default_in`, a closed enum.**
   - One value, `keyed`: the keyed states. Absent means today's behaviour:
every type without an authored `in:` may be posted in every state. No explicit
value names the default, as with `on_final`. A second value can be added later
without a break.
   - It sits on the session, because it describes how the session reads
its own `on:` keys, and it applies to every type of the channel type.
   - The name says what the key sets: the `in:` of a type that has none.
That covers all of its effects, not only what the composer offers: a post
outside the derived `in:` is refused 409, and a trigger that posts outside it is
skipped and not retried. It follows the `default_<key>` naming of
`data-entry.yaml` (`default_view`, `default_sort`) and reads well next to
`initial` and `on_final`.
   - The design review rejected the first name, `offer: transitions`.
"transitions" already names `_actions.transitions`, the transitions in
`read_session` and the decision buttons, so an operator would read the key as
"offer the transition buttons". "offer" also hides that the key refuses posts.
Other names considered: `post_in: keyed_states` (repeats "post" from the `post:`
roles key, which means who, not where) and `in_default` (no precedent in rela).
`default_in` is decided before release, because a config key cannot be renamed
later without a migration.

3. **The rule.** Per message type of a channel type with
`default_in: keyed`:
   - An authored `in:` is compiled and checked exactly as today. Because
the load refuses an authored `in:` that misses a keyed state, an authored `in:`
always contains the derived one. An authored `in:` may add states to the keyed
ones; it cannot remove any. In practice the effective `in:` is the union of the
keyed states and the authored states.
   - Otherwise, if the type has at least one keyed state, `In` is the list
of keyed states in the order of `session.states`. File order of the states is
the order a client shows them, and it gives the 409 title a stable list.
   - Otherwise `In` stays nil: the type may be posted in every state.

Which entries key a state: all of them. The question is "may this type be posted
here", not "can this caller move the session here". So a `by: trigger` entry
keys its state, because a trigger's post also goes through `in:` and would
otherwise be skipped. An entry nobody can fire, such as one shadowed by an
earlier unguarded entry, also keys its state. This matches the TKT-QYFLRH check,
which requires every keyed state in an authored `in:` whatever its entries say,
so a derived `in:` is exactly the smallest authored `in:` that check accepts.

The derived `in:` is per type, not per role, like an authored one. A person who
may post the type can post it in a keyed state where only another role or a
trigger moves the session; the post is stored and moves nothing, as today.

4. **The runtime of TKT-QYFLRH is unchanged.** Every runtime reader goes
through `sessionRefusal`, which reads only `mt.In`. Checked consumers:
   - `postable` and so `_actions.post`, `can_post` in `read_channel`, and
`You may post` in `rela channel read`: they shrink to the keyed states for keyed
types. This is the intended change.
   - `sessionAffordances` (`_actions.transitions` and `_actions.entries`),
and through it the inbox (`inbox.go`) and the waiting read (`waiting.go`): they
list types keyed in the current state. Each such type has the current state in
its derived `in:`, so the post set still contains it and the result is the same.
AC 6 pins this.
   - The decision buttons (TKT-T9G1AE, TKT-S9TLJJ, TKT-TXZZZ2) come from
`session.transitions` and `_actions.entries`, so they are unchanged.
`decision_button` load checks use `personClasses` over entries, not `in:`.
   - `choices` buttons: an answer type that no transition is keyed by
stays free, so a click works as today. An answer type that is keyed gets a
derived `in:`; the click carries `expect_generation` and goes through `in:` like
any post, and the load warning (AC 8) tells the operator when the buttons can
never show.
   - Working and busy states: `WorkingStates` and the busy read read
`status:`, not `in:`. A progress type that no transition is keyed by stays free
in a working state.
   - Reminders: the load already refuses a reminder whose post type a
transition is keyed by, so a reminder's type never gets a derived `in:`, and the
reminder-inside-`in:` check never fires for one.
   - `on_final: close`: a keyed state is never final (a final state has no
`on:`), so a derived `in:` never lists only final states.
   - Stranded sessions admit only types without `in:`, so they refuse
derived types too, as they refuse authored ones. Documented.
   - Triggers: a trigger that posts a keyed type in a state that does not
key it is now skipped with the existing warning, where today it would store a
message that moves nothing. The change is marked seen and is not retried
(`trigger.go:1238-1246`). This is the same as with an authored `in:`.
Documented, with the PoC's `ready` trigger as the example.
   - Edits and deletes of stored messages follow `edit_own` and
`delete_own`, not `in:`, as today.

The only runtime edit is the rule string: `sessionWrongState` and the trigger
warning call `ct.InKey(mt)` instead of building `...message_types.<mt>.in`. For
an authored `in:`, and for a `MessageType` built by hand
(`usertext_test.go:82`), the value is the same string, so existing tests stay
green.

5. **Serving.** No `in:` is served today: not in the channel schema
descriptor (`describe.go`), not in `read_channel`'s message types
(`MessageTypeView`, `read.go`), not in OpenAPI schemas or MCP tools. Clients
learn it only through `_actions.post`, `can_post` and the 409. A derived `in:`
follows the same paths, so no wire type changes and no golden changes. Derived
and authored values can be told apart in two places: the 409 `detail` (`Rule`)
and the trigger warning name `session.default_in` or `message_types.<mt>.in`.
With derivation the file is no longer a readable source of the effective `in:`,
so serving it is filed as the follow-up "Serve the effective in: on message type
views": the effective list on `MessageTypeView` and the channel schema
descriptor, additive on the wire.

6. **Config (`internal/realtimecfg`).**
   - `raw.go`: `rawSession` gains `DefaultIn string yaml:"default_in"`.
   - `message_in.go`: `const DefaultInKeyed = "keyed"`.
`withSession(s, rawStates, defaultIn)`: for each type, an authored `in:` goes
through `one` as today; otherwise, with `defaultIn == DefaultInKeyed`,
`keyedStates(s, rawStates, mn)` returns the keyed states in declaration order,
and a non-empty result sets `In` and `derived = true`. It counts derived types
and adds the AC 9 warning when the count is zero. `one` takes the presence it
already computed in `withSession`, so presence is read once. When `defaultIn` is
set, the refusal of a missing keyed state appends "or drop in to take the keyed
states from session.default_in".
   - `session.go`: check `raw.DefaultIn` next to `on_final` ("default_in
may only be keyed"), and pass the checked value to `withSession`. The `Session`
struct gets no new field, because nothing reads the key after the load.
   - `compiled.go`: `MessageType` gains the unexported `derived bool`,
documented as "In was derived from session.default_in; set only by the loader".
`ChannelType` gains `InKey(mt *MessageType) string`:
`ct.messageTypeKey(mt.Name)+".in"` when `derived` is false, and
`"channel_types."+ct.Name+".session.default_in"` when it is true. The zero value
gives today's string, both runtime sites call one helper, and there is no
exported field to keep in step with `In`. `clone` needs no change (the struct
copy covers the flag). No exported field is added, so plimsoll counts do not
move. The `In` doc comment changes from "in file order" to "the states as listed
in in:, or, when derived from session.default_in, in the order of
session.states".
   - `keys.go`: hint for `channel_types.*.session.states.*.default_in`.

7. **Runtime (`internal/realtime`).**
   - `session.go` `sessionWrongState`: `Rule: ct.InKey(mt)`.
   - `trigger.go`: the warning's `in` attribute is `ru.ct.InKey(ru.mt)`,
and the comment above it says the narrow `in:` may be authored or derived.

8. **OpenAPI.** `postMessageText` in `internal/openapi/channels.go`:
"limited to some session states with in:" becomes "limited to some session
states with in: or session.default_in". The MCP tool descriptions in code
already say "a message type may be limited to some session states", which stays
true; no change.

**Alternatives considered:**

- *Always derive, with no key.* Rejected. It would refuse posts that are
stored today in every existing session config, including the PoC, on upgrade.
The ticket asks for an option.
- *A per-type key, such as `in: transitions` or `in: auto`.* Rejected as
the main mechanism. It still has to be written on every keyed type, which is the
repetition the ticket removes. The session-level key states the intent once, and
an authored `in:` remains the per-type way to add states.
- *An open enum or a boolean (`default_in_keyed: true`).* Rejected. A
boolean cannot grow, and an open value would accept typos. A closed enum with
one value follows `on_final`.
- *`offer: transitions` or `post_in: keyed_states` as the name.* Rejected;
see Approach 2.
- *An exported `MessageType.InRule string`.* Rejected in design review.
Two exported fields would have to agree, and a hand-built `MessageType` with
`In` and no rule would give a 409 with an empty detail. The unexported flag and
`InKey` remove that state.
- *Derive only from entries a person can fire (not `by: trigger`, some role
may post it).* Rejected. A trigger's post also goes through `in:`, so a `by:
trigger` state left out would skip the trigger, which is the one poster that
state exists for. It would also make the derived set narrower than the
TKT-QYFLRH check demands of an authored `in:`.
- *Derive per role.* Rejected. `in:` is per type at runtime; a per-role
set needs a new runtime check, which the ticket rules out.
- *Add reminder states to the derived set.* Not needed: a reminder may not
post a keyed type.
- *Serve the derived `in:` now.* Rejected for this ticket; nothing serves
`in:` today, and `_actions.post` is the authority clients already use. Filed as
a follow-up (Approach 5).

**Dependencies:** `internal/realtimecfg`, `internal/realtime`,
`internal/openapi` (one description string). No new imports. No change to
stores, `entitymanager`, wire types, the SPA or rela-components.

**Files to modify:**

- `internal/realtimecfg/raw.go`, `session.go`, `message_in.go`,
`compiled.go`, `keys.go`.
- `internal/realtime/session.go`, `internal/realtime/trigger.go`.
- `internal/openapi/channels.go`.
- Tests: `internal/realtimecfg/default_in_test.go` (new),
`internal/realtimecfg/session_test.go` (two reload rows),
`internal/realtime/session_affordance_test.go` (rows extracted into
`messageInRows`), `internal/realtime/session_test.go`,
`internal/realtime/trigger_test.go`,
`internal/dataentry/channels_handler_test.go`.
- Docs: `docs-project/entities/guides/GUIDE-realtime.md` and
`GUIDE-mcp-server.md`, then `just docs` to regenerate `docs/realtime.md` and
`docs/mcp-server.md`.

## Security Considerations

- [x] Input sources identified (user input, config, external APIs)
- [x] Input validation approach defined (allowlist preferred over blocklist)
- [x] Security-sensitive operations identified (file access, auth, crypto)
- [x] Error handling doesn't leak sensitive information

**Input Sources & Validation:**

- *`session.default_in` in `realtime.yaml` (operator).* An allowlist of
one value, checked at load, on reload and by `rela validate`. A misplaced key is
refused with a hint. A failed reload keeps the last good config.
- No new request input. Posts are judged by the existing path.

**Security-Sensitive Operations:**

- None new. A derived `in:` can only refuse more posts; it never admits a
post that today's config refuses. Post roles, `when:`, transition roles and
`expect_generation` apply unchanged.
- The 409 `detail` names a config key path. The configuration is not a
secret (CLAUDE.md), and the detail already names `message_types.<mt>.in` today.
- Errors and warnings name config paths and values only.

## Test Plan

- [x] Test scenarios documented for each acceptance criterion
- [x] Edge cases identified and documented
- [x] Negative test cases defined (invalid input, error conditions)
- [x] Integration test approach defined (not just unit tests)

**Test Scenarios:**

| AC | Test |
| --- | --- |
| 1 | `TestParse_SessionDefaultIn`; `CheckReload` rows "default_in added", "default_in removed" |
| 2 | `TestDefaultIn_Derive` (entry forms) |
| 3 | `TestDefaultIn_Derive` (authored and unkeyed cases, refusal text) |
| 4 | `TestDefaultIn_AbsentKeepsToday`; existing `in:` expectations unchanged |
| 5 | `TestDefaultIn_PoCEquivalent` (in `internal/realtimecfg`) |
| 6 | `TestAffordances_MessageIn` and `TestAffordances_DefaultIn` over `messageInRows`, `TestPost_DefaultIn`, `TestChannelsHandler_SessionWrongState` "derived in names session.default_in" |
| 7 | `TestSessionTriggerPost_DefaultIn` |
| 8 | `TestDefaultIn_Checks` (choices and decision_button cases) |
| 9 | `TestDefaultIn_Checks` (warning cases) |
| 10 | `TestDocsExample_DefaultIn` |

Integration: `TestAffordances_DefaultIn` and `TestPost_DefaultIn` run the real
`Service` over the in-memory channel store with the PoC fixture, including the
store-side refusal under its lock. `TestChannelsHandler_SessionWrongState` goes
through the HTTP handler and checks the problem body. MCP and the CLI render the
same `DenialError.Rule` field, so they get no new case.

**Edge cases:**

- A type keyed in every non-final state: refused in the final states,
where today it is accepted. Documented.
- A `by: trigger` entry: its state is in the derived `in:`; a person who
may post the type can post it there, and it moves nothing.
- The `go` type of the PoC (no roles, posted by the owner or the `ready`
trigger): derived `[new]`; a trigger firing in another state is skipped with a
warning and not retried.
- An unstored channel reads as the initial state; the derived `in:` of a
type keyed there admits it (the "unstored reads as initial" row).
- A stranded session refuses derived types (the stranded row of AC 6,
which asserts it runs with the key set).
- A chat type that a transition is keyed by (for example, any `text` in
`asking` moves to `working`): it gets a derived `in:` and stops being postable
elsewhere. The docs say so and give the opt-out: list every state in its `in:`.
- An operator migrating an existing file: sets `default_in: keyed` while
every keyed type still has its copied `in:`. The no-effect warning says to drop
those `in:` lines; it goes away once the first one is dropped.
- A reload that adds or removes `default_in` while channels are open: the
next post is judged by the new config; stored messages are untouched. Pinned by
the two `CheckReload` rows.
- Mixed versions: an older process refuses `session.default_in` as an
unknown key. Documented, as for `in:`.

**Negative Tests:**

- `default_in: all`, `default_in: Keyed`, `default_in: [keyed]`: refused
at `...session.default_in` (the list by the decode shape check, as for
`on_final`).
- `states.review.default_in: keyed`: refused with the hint.
- `message_types.plan.default_in`: refused as an unknown key.
- An authored `in:` that misses a keyed state is still refused under
`default_in: keyed`, and the text suggests dropping the `in:`.

## Risk Assessment

- [x] Technical risks assessed with mitigations
- [x] Security risks assessed (see Security Considerations)
- [x] Effort estimated (xs/s/m/l/xl)

**Risks:**

- *The runtime changes by accident.* Mitigated: the only runtime edit is
the rule string, the existing `in:` expectations stay unchanged (AC 4), and the
equivalence tests compare whole affordance tables (AC 5, AC 6).
- *The equivalence table passes for the wrong reason.* Mitigated: the rows
build every config from the fixture text they get, and the stranded row asserts
that its snapshot has the key set (AC 6).
- *A keyed chat type gets narrowed.* The operator opted in, and the docs
name the case and the opt-out. See Q4.
- *Triggers are skipped where they used to post, and not retried.* Same
behaviour as an authored `in:`, already logged as a warning that now names
`session.default_in`. Documented with the PoC's `ready` trigger.
- *Order of load checks.* The derivation must run before `choiceChecker`.
It does today (inside `session`), and `TestDefaultIn_Checks` pins it.

**Effort:** s. About 60 lines of config code, two one-line runtime edits, one
description string, and tests and docs. AC 5 reuses the existing
`pocRealtime(in, session)` helper in `internal/realtimecfg`, so it needs no new
fixture. The affordance table extraction is the largest test edit. This is the
top end of s. One PR.

## Documentation Planning

- [x] User-facing docs identified (skip if internal refactor)
- [x] Docs-checklist will be created when entering implementation

**Documentation Impact:**

- [x] `GUIDE-realtime.md`, regenerated into `docs/realtime.md` by
`just docs`:
  - "Sessions" key table: a row for `session.default_in` ("`keyed`: a
message type that a transition is keyed by, and that has no `in:`, may be posted
only in the states whose `on:` keys it. See 'Where a message type may be
posted'.").
  - "Where a message type may be posted": a new paragraph, "**Take `in:`
from the transitions.**", after the `in:` bullets, with the example of Approach
1 and these points:
    - which entries key a state: all, including `by: trigger`;
    - an authored `in:` may add states to the keyed ones but cannot remove
any, so in practice it works as a union with the keyed states;
    - types that no transition is keyed by stay free;
    - to keep a keyed type free, list every state in its `in:`; that list
goes stale when a state is added, and the new state then refuses the type;
    - the derived `in:` behaves as an authored one (composer,
`_actions.post`, 409, stranded sessions, skipped triggers that are not retried);
    - the 409 `detail` names `session.default_in`;
    - without the key nothing changes;
    - an older process refuses the key.
  - The advice at line 754, "Give a type that decides a session a `label:`
and an `in:`", adds that `session.default_in: keyed` gives every such type its
`in:` without writing one per type.
  - The `detail` bullet ("names the rule, such as
`channel_types.plan.message_types.approve.in`") adds "or
`channel_types.plan.session.default_in` when the `in:` comes from `default_in:
keyed`".
  - The `choices` example (around line 322) stays as is; the new paragraph
covers that its `in: [asking]` could be dropped under the key.
  - "Checks when the file loads": `session.default_in` values, the
misplaced-key hint, the added refusal hint and the no-effect warning.
  - "Agents and other processes": "A message type with `in:`" becomes "A
message type with `in:`, or one that `default_in: keyed` limits".
  - "Errors": the `session_wrong_state` row says the detail may name
`session.default_in`.
- [x] `GUIDE-mcp-server.md`, regenerated into `docs/mcp-server.md`: the
`read_channel` row (line 529) and the `post_message` row (line 531) say "A
message type limited by `in:`". Both become "limited by `in:` or by
`session.default_in`".
- [x] ~~docs/metamodel.md - New metamodel features~~ (N/A: no schema.yaml change)
- [x] ~~docs/cli-reference.md - New/changed commands~~ (N/A: no CLI change)
- [x] ~~docs/data-entry.md - UI changes~~ (N/A: no UI change; the composer follows `_actions.post` as before)
- [x] ~~CLAUDE.md - New patterns or conventions~~ (N/A: follows existing patterns)
- [x] ~~README.md - Project-level changes~~ (N/A)

Follow-ups, filed separately:

- "PoC: use session default_in in the run session". The PoC configs
(`rela-agent-runner-poc/mission/project/realtime.yaml` and
`project/realtime.yaml`) add `default_in: keyed` to their `run` session. They
have no `in:` to drop. Every type except `text` and `progress` then gets a
derived `in:`, and the `ready` trigger's `go` is skipped outside `new`.
- "Serve the effective in: on message type views". See Approach 5.

## Open questions

- **Q1. Key and values?** Decided after design review:
`session.default_in`, closed enum, one value `keyed`; absent is today's
behaviour. See Approach 2.
- **Q2. Which entries key a state?** Decided: every entry, including
`by: trigger`, guarded, shadowed and role-limited ones. See Approach 3.
- **Q3. Is the derived `in:` served, and can a client tell it apart?**
Decided: not served in this ticket, since no `in:` is served today. The 409
`detail` and the trigger warning name `session.default_in` for a derived value.
Serving the effective list is the follow-up "Serve the effective in: on message
type views". See Approach 5.
- **Q4. How does a keyed type stay free?** Decided: list every state in
its `in:`. This follows from the rule in Approach 3: an authored `in:` can only
add states to the keyed ones. The list goes stale when a state is added, and it
refuses the type in a stranded session; the docs say both. A shorter form, such
as `in: any`, would change the `in:` grammar and waits until the case turns up.
- **Q5. Warn on an authored `in:` that equals the derived one?** Decided:
no. Writing it is harmless and lets an operator be explicit. The no-effect
warning of AC 9 stays, because a key that does nothing is worth flagging (as
`grants.go` does for `run_as`), but it names the fix in both directions, so an
operator midway through a migration knows to drop the copied `in:` lines.
- **Q6. Is the ticket's PoC claim right?** No: the PoC files have no `in:`;
the test fixtures named `pocRealtime` that model the PoC do. The plan uses the
fixtures for the equivalence tests. The PoC change is the follow-up "PoC: use
session default_in in the run session".

## Design Review

- [x] Run `/design-review` before starting implementation
- [x] All critical/significant findings addressed in plan

**Design Review Findings:** <!-- List review-response IDs, e.g., RR-xxxx -->

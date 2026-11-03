---
id: PLAN-97H3XU
type: planning-checklist
title: 'Planning: Sessions 4/6: input_required and handoff'
started: "2026-10-26"
completed: "2026-10-26"
status: done
---

<!-- @managed: claude-workflow v1 -->

## Understanding

- [x] Problem/requirements clearly understood
- [x] Scope defined (what's in/out documented below)
- [x] Acceptance criteria documented with specific test scenarios

**Problem:** A session (slices 4/1 to 4/5b) knows its state, timers, reminders
and graph outputs. It cannot say what kind of state it is in. An agent cannot
tell "the session is working" from "the session waits for a person" or "the
session ended in failure". Nobody can see whom the session waits for. The
research (RES-O8I8S7, use case 2, section (g), slice 6) asks for three things.
First, a `status:` per state with the MCP tasks values (`working`,
`input_required`, `completed`, `cancelled`, `failed`). Second, a `waiting_for:`
that names whom an `input_required` state waits for. Third, an MCP way to wait
on a session. Decision 4 says role holders come from entity properties (`from:
entity.<prop>`), and a handoff is an audited entity write: a `set:` output on
that property. A later per-principal inbox derives "waiting on you" entries from
session states, so `waiting_for` must allow a cheap, gated inbox query.

Terms used below:
- **Status**: the MCP tasks value of a session state, from config.
- **Waiting role**: a role named in a state's `waiting_for:`.
- **Named role**: a role with source `from: entity.<prop>`. Its holders are
the values of that anchor property.
- **Bounded role**: a role whose holders are a small, explicit set: a named
role, or a `creator: true` role. Only bounded roles may be waiting roles.
- **Holders**: the values of a named role's property, as the reader's
redacted anchor view shows them.
- **Answer**: a transition out of a state to a different state. A
re-entry (`to:` the same state) is not an answer.
- **Waiting on me**: the channel is stored and open, the current status is
`input_required`, the configuration is not degraded, and the caller holds a
waiting role for update on the anchor (the client ceiling applies).
- **Handoff**: a transition whose `set:` output writes a role's property.
A **claim** is a handoff to the poster (`{{message.author}}`).
- **Unset state**: the empty stored session state of a channel that never
moved. It means the configured `initial` state everywhere.

**Scope:**
- In: `status:` and `waiting_for:` on `session.states.<name>`, defaults and
load checks, in `internal/realtimecfg`.
- In: the deadline-reset refusal: a re-entry that a waiting role can fire,
in a state with `since: state` timers or active reminders.
- In: refused keys with a reason: `session.states.*.awaits` (the inbox
draft's name), `session.status`, `session.waiting_for`,
`session.states.*.on.*.waiting_for`, `session.states.*.after.*.waiting_for`.
- In: `SessionView` gains `status`, `waiting_for` (role, label, holders for
named roles) and `waiting_on_me`. The schema descriptor gains `status` and
`waiting_for` per state.
- In: a stable `entered_at`: it is left out of the session view of a channel
that is not stored, in every channel answer and live frame.
- In: the handoff pattern documented with a hand-over state and a claim, and
two load warnings: a handoff from free text, and an answer whose output no ACL
role can write.
- In: a fresh anchor read in the post answer after an inline handoff.
- In: MCP tool `read_session`, watchable through the existing `watch` tool.
- In: wire, OpenAPI, TS mirror, CLI rendering, SPA stepper badges, docs,
testdata and the docs example.
- Out: the inbox itself (read markers, `GET /api/v1/_inbox`, a store query by
session state). Approach 10 records what it can build on.
- Out: literal principals, and permission, attached or principal-type roles
in `waiting_for`.
- Out: session-scoped holders (research option e2). Decision 4 defers them.
- Out: a re-entry that keeps its timers (`keep_timers`). This is a possible
follow-up; this slice refuses the risky case instead.
- Out: a stored status column. Status is derived from the stored state and
the config.
- Out: MCP push notifications. The server has no resource subscriptions;
`watch` is the long-poll seam.
- Out: normalizing `{{message.author}}` across transports. Identity
resolution belongs to `acl`; this slice documents and tests the rule.
- Out: `count()` guards and the menu filter (slice 7).

**Example (the shape this slice accepts; also the docs example and the new
`testdata/valid.yaml` block):**

```yaml
channel_types:
  breach-response:
    roles:
      lead:   { from: entity.owner, label: the incident lead }
      member: { from: attached }
      dpo:    { permission: privacy.officer, label: the privacy officer }
    message_types:
      handoff:        { properties: { to: { type: ref, to: [risico], required: true } }, post: [lead] }
      accept-handoff: { properties: { note: { type: string } }, post: [lead] }
      claim:          { properties: { note: { type: string } }, post: [member] }
    session:
      states:
        contained:
          label: Contained
          waiting_for: [lead]                 # status defaults to input_required
          on:
            assessed: { to: assessed, roles: [lead], ... }   # the answer
            handoff:
              to: handed-over
              roles: [lead]
              outputs:
                - set: { properties: { owner: "{{message.to}}" } }
          after:
            notify-deadline: { period: 72h, since: session, to: overdue, ... }
        handed-over:
          label: Handed over
          waiting_for: [lead]                 # the new owner
          on:
            accept-handoff: { to: contained, roles: [lead] }
            claim:
              to: contained
              roles: [member]
              outputs:
                - set: { properties: { owner: "{{message.author}}" } }
          after:
            notify-deadline: { period: 72h, since: session, to: overdue, ... }
            not-accepted:    { period: 4h, to: overdue, label: Handoff not accepted }
        overdue:   { label: Notification overdue, on: { notified: ... } }   # working: dpo answers
        notified:  { label: AP notified, final: true }                        # completed
        dismissed: { label: Dismissed, final: true, status: cancelled }
```

The handoff moves to its own state, as in research use case 4 ("acknowledge step
with timeout"). `since: session` keeps the 72-hour deadline across the handoff.
`overdue` waits for the dpo, a permission role, so it stays `working`: only
bounded roles can be waiting roles. `handed-over` is not in the `status-update`
reminder's `in:` list.

`testdata/acl.yaml` gains an ACL role `handler` (`read: [datalek]`, `update:
[datalek]`, no `fields:` block, so every field) assigned to the test user
`USR-1`, and `officer` gains `update: [datalek]`. These are ACL roles; `lead`
and `dpo` are realtime roles whose holders hold them. Neither grant uses
`everyone`, so the `run_as` grant check over the shared testdata keeps its
answers (`TestRunAsGrantCheck` re-runs unchanged).

**Acceptance Criteria:**

1. `status:` accepts only `working`, `input_required`, `completed`,
`cancelled` and `failed`. Any other value (`submitted`, `rejected`,
`auth_required`, `done`, and an explicit empty string) is refused with one
problem at `...states.<s>.status` that lists the five values. The raw field is
`*string`, so `status: ""` differs from no `status:`. Test:
`TestSessionStatus_Values` table in `internal/realtimecfg`.
2. Defaults: a final state reads `completed`. A state that is not final
reads `input_required` when it has `waiting_for`, else `working`. Test:
`TestSessionStatus_Defaults` reads `SessionState.Status` per case.
3. A final state refuses `working` and `input_required` ("a final state has
no transitions, so nothing can answer; use completed, cancelled or failed"). It
refuses `waiting_for` for the same reason. A state that is not final refuses
`completed`, `cancelled` and `failed` ("mark the state final: true"). Test: rows
in `TestSessionStatus_FinalRules`.
4. `waiting_for:` names declared roles of the channel type only. An unknown
name, a duplicate, and `read` (the read role is not under `roles:`) are refused
at `...waiting_for[i]`; an empty list at `...waiting_for`. Test:
`TestSessionWaitingFor_Refs` table.
5. `waiting_for` takes only bounded roles. A role with source `from:
attached`, `permission:` or `principal_type:` is refused at `...waiting_for[i]`
("role dpo is held by everyone with permission privacy.officer; waiting_for
names a role with a known holder: a from: entity.<property> role or a creator
role"). Test: rows in `TestSessionWaitingFor_Refs` for each refused source and
for a creator role on a cardinality-many type (accepted).
6. `status: input_required` without `waiting_for` is refused ("name whom
the session waits for"). `waiting_for` with an explicit `status:` other than
`input_required` is refused. Test: rows in `TestSessionWaitingFor_Status`.
7. Each waiting role must have an answer: a transition out of the state to
a different state, whose message type lists the role in `post`, which is not
`by: trigger`, and whose `roles` (when set) include the role. A guarded answer
counts. Otherwise the load is refused at `...waiting_for[i]` ("role lead cannot
post a message that moves the session out of contained, so the session would
wait for it forever"). Test: `TestSessionWaitingFor_CanAnswer` table with rows
for a role not in `post`, a role narrowed out by `roles`, a `by: trigger`-only
exit, an exit made only of re-entries (refused), and a guarded answer
(accepted).
8. Deadline reset: in a state that has a `since: state` timer, or a reminder
whose `in:` lists the state, a re-entry that a waiting role can fire (same rule
as AC 7: in `post`, not `by: trigger`, passes `roles`) is refused at
`...on.<type>` ("a re-entry restarts this state's timers and reminders, so a
holder could push back the deadline forever; use since: session for a deadline
that survives handoffs, or move the handoff to a separate state"). Without such
timers or reminders, a re-entry loads. Re-entry behaviour itself is unchanged.
Test: `TestSessionWaitingFor_DeadlineReset` table: a `since: state` timer row, a
reminder row, a `since: session`-only row (accepted), a re-entry narrowed to a
role that is not waiting (accepted), a `by: trigger` re-entry (accepted).
9. The refused keys answer with their reasons: `states.*.awaits` names
`waiting_for`; `session.status` and `session.waiting_for` say "set it per
state"; `on.*.waiting_for` and `after.*.waiting_for` say "waiting_for belongs to
the state the session enters". Test: rows in `TestParse_SessionInvalid`
(`session_test.go`).
10. Handoff warning: when `acl.yaml` declares `user_entity_type`, a `set:`
output that writes the property of a named role of this channel type, or of its
`read` role when that is `from: entity.<prop>`, loads with one warning at the
output property key when the value is neither a literal, `{{message.author}}`,
nor a whole `{{message.<p>}}` whose `<p>` is a `ref` to that type. A composite
template (text around a reference) counts as free text. A property no role of
this type reads gets no warning. Without `user_entity_type` no warning appears.
Test: `TestHandoffWarning` reads `Compiled.Warnings()`.
11. Answer grant warning: AC 7 is structural. When `acl.yaml` is loaded and
no ACL role (including `everyone`) grants update with field write on a property
that an answer's `set:` writes (or create on an answer's `create:` type), the
load gives one warning at the output key: no holder could post the answer
without `session_output_denied`. Test: `TestSessionWaitingFor_AnswerGrants` with
a granted row (no warning) and an ungranted row (warning); `TestAnyRoleGrants`
in `internal/acl`.
12. `SessionView.status` is set for every declared state, read from the
current config. A stranded session reads `failed` with `stranded: true` and no
`waiting_for`, so an agent stops waiting. A reload that declares the state again
returns the configured status. Test: `TestSessionView_Status` in
`internal/realtime`, including a reload that changes a state's status and one
that strands the session and then restores it.
13. An unset state reads as the current `initial` state in the view,
including after a reload that changes `initial:`. Test:
`TestSessionView_UnsetIsInitial`.
14. A channel that is not stored has no `entered_at` in its session view, in
`read_channel`, the channel list, the head, live frames and `read_session`. A
stored channel keeps its stored `entered_at`, or its `created_at` when it never
moved. Test: `TestSessionView_UnstoredNoEnteredAt`, and a row in
`TestReadChannel` that reads an unstored channel twice at two clock times and
compares the results byte for byte.
15. `waiting_for` lists the waiting roles in config order with role name and
label. A named role carries `holders`: the property values from the caller's
redacted anchor view (a string or a list, as `propertyNamesCaller` reads them).
It has no `holders` when the view omits the property. A creator role carries no
holders; the channel's `created_by` already names the creator. Test:
`TestSessionView_WaitingFor` with a fake anchor source that redacts `owner` for
one reader and not for another.
16. `waiting_on_me` follows its definition. Test:
`TestSessionView_WaitingOnMe` table: a holder of a named role (true), the
creator on a many type (true), a non-holder, a holder whose client ceiling
denies update, a closed channel, an unstored channel, a degraded snapshot, and
an anonymous reader (all false).
17. List path: `ListAnchor` computes `waiting_on_me` per channel without
leaking one channel type's role answers into another (the role-answer cache is
reset per type). A creator role is answered per channel and never cached. Test:
`TestListAnchor_WaitingOnMeIsolation` with two types that both declare `lead`
with different sources, and two many-channels with different creators.
18. Budget: `ListAnchor` over 20 session channels whose states wait for a
named role and a creator role makes the same store reads as without
`waiting_for` (one anchor read, one `ListEntity`) and no `HoldsPermissionFor`
call for the waiting facts. Test: `TestListAnchor_WaitingForBudget` with a
counting store and a counting access.
19. A post whose inline outputs set a named role's property answers the
session with the new holders and `waiting_on_me` from a fresh anchor read. A
post with no such set reads the anchor once, as today. Test:
`TestPost_HandoffAnswersNewHolder` (memstore, real policy) and the read count in
`TestPost_NoHandoffNoExtraRead`.
20. The example works end to end. The lead (USR-1, ACL role `handler`)
posts `handoff {to: USR-2}`: the session moves to `handed-over`, `datalek.owner`
becomes `USR-2`, and the audit record names USR-1 as the writer. USR-2's next
read shows `waiting_on_me: true`, and USR-2 posts `accept-handoff` back to
`contained`. A member who may not write `owner` posting `claim` is refused `403
session_output_denied` before anything is stored. A member with the grant
claims, and owner becomes their `Author.User`. One claim row runs over an
MCP-stamped principal (`ResolveDirect`): owner becomes the MCP identity, and
that caller's `read_session` then shows `waiting_on_me: true`. Test:
`TestSession_HandoffAndClaim` in `internal/appbuild`.
21. The schema descriptor gives every state `status`, and gives
`waiting_for` as role names where set. Test: `TestDescribe_SessionStatus` in
`internal/realtime` and the OpenAPI schema test.
22. MCP `read_session` returns `{notice, channel: {anchor, type, name,
state}, session, transitions}` with `notice` = `realtime.UntrustedNotice`, and
no `rev`, `last_seq` or messages. A channel type without a session answers a
tool error naming the type, decided from config before the anchor is read (the
same answer for a missing and an existing anchor). A caller who may not read the
channel gets the same error `read_channel` gives. Test: `TestReadSession` table
in `internal/mcp`, with a counting anchor source on the `no_session` row.
23. `watch` accepts `read_session` targets. It returns `changed` on a
transition, on a handoff that changes `waiting_on_me` for the caller, and on an
entity edit of `owner` outside the channel. It does not return on a plain `text`
post. On an unstored channel it times out with no change across several channel
polls. A post from another process is seen by the channel poll. Test:
`TestWatch_ReadSession` in `internal/mcp`, with a short `ChannelPoll` and no
`Waker` for the cross-process row.
24. Wire: `ChannelSession.status`, `waiting_for`, `waiting_on_me`, the now
optional `entered_at`, and `ChannelSessionStateSchema.status` and `waiting_for`
appear in apiwire, OpenAPI (status as an enum) and
`frontend/src/types/channels.ts`. Test: the OpenAPI channel schema test in
`internal/openapi/channels_test.go`, `wireFields.test.ts`, the frontend type
check.
25. CLI: `sessionText` prints the status, and for `input_required` prints
"waiting for <role> (<holders>)" and "(you)" when `waiting_on_me` is set. A
stranded session prints "failed (stranded)". Test: rows in
`channel_render_test.go`.
26. SPA: the stepper shows an "Input required" tag on the current state and
"Waiting for <labels>" or "Waiting for you". A final state shows "Completed",
"Cancelled" or "Failed" in place of "Ended". The text carries the meaning;
colour only repeats it. Test: `RlChannelSession` unit tests per status, stories
`InputRequired`, `WaitingOnMe`, `Cancelled`, `Failed`, and `channelView.test.ts`
for the mapping.
27. Docs and testdata: the example in `GUIDE-realtime.md` is the
`breach-response` block of `testdata/valid.yaml` (`TestDocsExample_Realtime`).
`TestParse_Golden` parses `valid.yaml` with no problems and no warnings. `just
docs` regenerates `docs/realtime.md` and `docs/mcp-server.md` without a diff.

## Research

- [x] For larger features: run `/research` to create a structured research doc
- [x] ~~Searched for existing libraries that solve this problem~~ (N/A: covered by the research doc; the slice reuses internal realtime, MCP watch and output code)
- [x] Checked codebase for similar patterns or reusable code
- [x] Looked for reference implementations in other projects
- [x] Reviewed relevant rela concepts for prior art

**Research Doc:** RES-O8I8S7 (`.ignored/res-sessions-final.md`). Decision 4:
roles come from a graph property first, and a handoff is an audited entity
write. Slice 6 in section (h): `status:` per state with MCP tasks values,
`waiting_for:`, MCP `watch` on sessions, handoff through `set:`. Use case 4
gives the handoff shape: an acknowledge step with a timeout.

**Existing Solutions:**
- Role matching: `call.can` and `propertyNamesCaller`
(`internal/realtime/authz.go`) apply the `is_current_user` rule to the redacted
anchor view. `heldRole` (`session.go`) caches answers per call and applies the
client ceiling through `CapabilityUpdate`.
- `identityProperty` (`internal/realtimecfg/validate.go`) already refuses a
named role on a property that `acl.yaml` restricts with `visible:`.
- `roleProps` already puts every named role's property in the anchor read of
`open` and `ListAnchor`, so holders cost no extra read.
- `currentSession` already reads an unset state as `initial`.
- The MCP `watch` tool (`internal/mcp/tools_watch.go`) long-polls read tools
by digest, with an in-process channel `Waker`, a 2s channel poll for other
processes, and store-event wakes. `channelTarget` maps a target to its channel.
- 5a/5b outputs: `set:` with `{{message.<p>}}` and `{{message.author}}`
templates, written as the poster after a field-write pre-check
(`session_output_denied`). `{{message.author}}` is `Author.User`.
- `acl.JobIdentityGrants` (5b) composes the verb and field primitives for
one identity. The answer grant warning needs the same primitives over all roles.
- `promotable` lets a `ref` message property fill a `string` entity
property, so a hand-to message can carry a user entity id.
- Other projects: MCP tasks (2026-07-28) statuses; A2A task states (the
extra `submitted`, `rejected` and `auth-required` are not adopted); AWS Step
Functions `waitForTaskToken` as the shape of a state that waits for a named
party.

## Approach

- [x] Technical approach chosen and documented
- [x] Approach builds on existing patterns (not reinventing)
- [x] Alternatives considered (document why rejected)
- [x] Dependencies identified (packages, APIs, types)

**Technical Approach:**

1. **Config.** `rawSessionState` gains `Status *string` (`status`), like
`run_as`, and `WaitingFor []string` (`waiting_for`). `SessionState` gains
`Status SessionStatus` and an unexported `waitingFor []string` with a
`WaitingFor()` copy accessor. `SessionStatus` is a string type with five
constants. A new `waitChecker` type in `internal/realtimecfg/waiting.go` (beside
`writeChecker`; `*validator` is at its plimsoll method line) runs per state
after the transitions and timers compiled:
   - Status: parse against the five values; defaults as in AC 2.
   - Final rule (AC 3). It settles the `input_required`-on-final conflict:
a final state has no transitions, so no answer could move it.
   - `input_required` if and only if `waiting_for` is set (AC 6).
   - `waiting_for` entries go through `roleRefs`, then the bounded-source
rule (AC 5): only `SourceEntityProperty` and `SourceCreator`.
   - The answer check (AC 7) counts only transitions to a different state.
One helper, `fireableBy(t Transition, role string, mt *MessageType) bool`, holds
the rule (role in `post`, not `by: trigger`, `roles` empty or containing the
role). AC 7 and AC 8 share it.
   - The deadline-reset check (AC 8): for each re-entry in the state, when
some waiting role passes `fireableBy`, and the state has a `since: state` timer
or `Session.RemindersIn(state)` is not empty, one problem at the transition key.
Only waiting roles are checked, as decided. A broader role that a holder also
holds (for example `member`) is a residual path. The docs say to narrow
re-entries with `roles:`.
   - The two warnings (AC 10, AC 11) run in the same type.
`keys.go` gains the five refused keys of AC 9.

2. **Answer grant warning.** A new function in `internal/acl/jobgrants.go`:

   ```go
   // AnyRoleGrants reports whether some role of p (everyone included)
   // grants op on typ@face and, for a non-empty field, field write by the
   // union rule. It is a policy-wide floor for load warnings, not an
   // authorization answer.
   func AnyRoleGrants(p *Policy, op Op, typ string, face entity.Face, field string) bool
   ```

It reuses the `JobGrants` primitives over all roles, not one identity's.
`waitChecker` calls it for each answer output target. With a nil policy it is
skipped. A miss is a warning, not a problem: local roles and asserted roles can
grant at request time.

3. **Waiting facts, one pure helper.** In `internal/realtime/waiting.go`:

   ```go
   // waitingFacts are the per-reader inputs of a session's waiting view.
   type waitingFacts struct {
       props  map[string]any                // the reader's redacted anchor props
       held   func(r realtimecfg.Role) bool // update on the anchor, ceiling applied
       stored bool                          // the channel has a row
       open   bool                          // the channel is open
       live   bool                          // false: degraded, render config only
   }
   func waitingView(st realtimecfg.SessionState, roles map[string]realtimecfg.Role, f waitingFacts) ([]SessionWaitingView, bool)
   ```

It returns the waiting roles with labels and holders, and the `waiting_on_me`
bit. It reads only its arguments. Holders come only from `f.props`. This is
defence in depth; `identityProperty` already refuses `visible:` on role
properties. The inbox will be a `Service` method in `internal/realtime`, because
it needs `call` and `heldRole`, so this helper stays unexported.

4. **Read path.** `sessionView` takes a `waitingFacts` argument. Every
caller has a `call` with the anchor already read:
   - `channelView` (Head, create, lifecycle answers) and `postedSession`
pass `c.waiting(ch, stored)`. It builds the facts from `c.anchor.Props`,
`c.snap.Degraded`, the channel state and a `held` that uses `c.heldRole` for
named roles. A creator role is answered with `sameAuthor(ch.CreatedBy, c.p)`
plus the ceiling check, per channel, never through the cache.
   - `listType` resets `c.ops.roles` when it switches channel type, so role
`lead` of one type does not answer for `lead` of another.
   - Status is set on every view. A stranded state reads `failed` (MCP
tasks has no "unknown" value; `failed` stops an agent loop, and `stranded: true`
says why). Under a degraded configuration `waiting_on_me` is false, as named
roles already answer false there.
   - For a channel that is not stored, `EnteredAt` is the zero time and is
left out (`omitzero`). The view no longer uses the call's clock time, so
repeated reads of the same unstored channel are identical. This fixes
`read_channel` and live replays as well. Guards are unaffected: the decider
reads the channel the store commits, with its stored `CreatedAt`. No new store
read on any path: the anchor read already holds every role property, and waiting
roles never ask `HoldsPermissionFor`.

5. **Handoff after an inline output.** The anchor in `call` is read before
the post. After `runInlineOutputs` wrote a `set` that names a named role's
property, `sessionAfterOutputs` re-reads the anchor once through the gated
reader, clears the cached answers of named roles, and renders the session again.
Otherwise it reads nothing extra (AC 19). An asynchronous runner write does not
change the post answer; the next read and live frames show it.

6. **Handoff pattern.** No new output kind. The documented shape is the
example: a `handoff` transition to a `handed-over` state that waits for the new
holder, with an accept answer, a claim answer for others, a timeout, and `since:
session` for the absolute deadline. A re-entry handoff is still possible in a
state with no `since: state` timers and no reminders. The docs state:
   - The poster needs field write on the role property (5a), so ACL decides
who may hand off. The write is in the graph audit with `triggered_by` = the
output origin.
   - A claim writes `Author.User`, the identity the poster's transport
resolved. The web edge resolves it through `principal_property`; CLI and MCP
keep the raw identifier (`ResolveDirect`). A role property matches a caller only
through the same value, so identity configuration must agree across transports.
AC 20 pins the MCP case.
   - Concurrent claims: both moves commit and the last `set:` wins. Guards
cannot read the entity, so there is no compare-and-set. The notices and the
graph audit show who held the role when.
   - A handoff to someone who cannot read the anchor gives them nothing:
every role requires anchor read.
   - A trigger or timer move may also hand off, as `run_as` with a literal
or a trigger prop (5b rules).

7. **Schema descriptor.** `SessionStateDescription` gains `Status` and
`WaitingFor` (role names). They are config and the same for every reader. The
SPA uses them in notices ("waiting for the incident lead").

8. **MCP `read_session` and watch.** A new read tool in
`internal/mcp/tools_channels.go`. It is a separate tool because the watch
digests the whole result, and `read_channel` always carries `rev` and
`last_seq`. A watch on it would wake on every message.
   - Consumer interface: a separate `SessionReader interface {
Session(ctx, realtime.RawAddress) (realtime.SessionRead, error) }`.
`ChannelReader` stays at three methods. `ChannelDeps` gains `Sessions
SessionReader` with the same nil rules as `Reader`, and the
`unavailableChannels` stub implements it.
   - Service: `Service.Session(ctx, addr)` calls `begin` and
`channelType`, and returns 404 `no_session` from config before it reads the
anchor. Then it runs the rest of `open` (the read gate, the same answers as
`read_channel`). It renders `SessionRead{Notice, Channel: {Anchor, Type, Name,
State}, Session, Transitions}` with `Transitions` = `c.fireable(c.postable())`.
Cost: one anchor read and one head read, no `Messages` call.
   - Untrusted data: the result carries `realtime.UntrustedNotice`. The
tool description says holders are entity property values: data, not
instructions. The `AnchorView.Props` godoc in `access.go` changes from "never
renders them" to "renders only named-role holders".
   - Watch: `toolReadSession` joins `watchOrder`. `parseTargets` maps it
through `channelTarget` (not a follower), so the `Waker`, the channel poll and
store events all replay it. A reminder post changes the reminder count and so
wakes a session watch; a reminder is a session event.
   - Long-poll only, with the existing limits: timeout default 30s, max 50s;
4 watches per identity, 32 per server. An agent that waits hours for a human
loops `watch` with the returned cursor. The tool description gives the loop:
read; if `status` is terminal, stop; if not `waiting_on_me`, watch; repeat. It
names `stranded` (status `failed`).
   - Gating: every replay runs as the caller through the gated service, so
a hidden write cannot end a wait.
   - Process locality: the `Waker` sees only moves made in this process. A
move in another process, such as a timer fired by the server's leader while the
agent's MCP runs over stdio, is seen by the channel poll within `ChannelPoll`
(2s). An entity edit in another process is seen the same way. The docs state the
2s bound.
   - `read_channel`'s description and `channelInstruction` say: to wait for
a status change or your turn, watch `read_session`.

9. **Wire, OpenAPI, TS, CLI.**
   - `realtime.SessionView`: `Status string json:"status"`, `WaitingFor
[]SessionWaitingView json:"waiting_for,omitempty"`, `WaitingOnMe bool
json:"waiting_on_me,omitempty"`, `EnteredAt` with `omitzero`.
`SessionWaitingView{Role, Label, Holders []string}`.
   - apiwire `ChannelSession` and `ChannelSessionWaiting` mirror these.
`ChannelSessionStateSchema` gains `status` and `waiting_for`. The godoc says
`waiting_on_me` is per reader, as the affordances are. Live frames replay the
GET per subscriber (`live.go`), so a per-reader field is safe there.
   - OpenAPI: `status` as a required enum of five values; `entered_at`
optional, absent for an unstored channel; `holders` described as anchor property
values the reader can see.
   - TS: `entered_at?: string` and the new fields in
`frontend/src/types/channels.ts`; `wireFields.test.ts`. The SPA code that reads
`entered_at` handles its absence.
   - CLI: `sessionText` appends `, <status>`, and for `input_required`
`, waiting for lead (USR-1)` and `(you)` when `waiting_on_me` is set. Holders
pass through `printable`.

10. **Inbox reconciliation.** The config key is `waiting_for`, as in the
ticket and the research; the inbox draft's `awaits:` is refused with a pointer
to it. What the inbox can build on:
    - Status and waiting roles are derived from (type, stored state) under
the current config, where an unset stored state means `initial`. A store query
for a state set that contains `initial` must also match `''`. Nothing derived is
stored, so nothing drifts on reload.
    - The inbox lists stored channels only. An unstored channel has no row,
and `waiting_on_me` is false for it, so the view and the inbox agree.
    - Waiting roles are bounded. A named role becomes "entities of the
attach_to types whose property names me", which the predicate pushdown already
lowers (`is_current_user`), joined with channels of the type in the waiting
states. A creator role becomes "channels I created" in those states. Neither
needs a per-row lookup or an ACL call per row. The row gate is one batched
anchor read per page.
    - `waitingView` is the one evaluation, called per row on facts batched
per page, under the `collection-reads.md` budget rule.
    - The store query (`ListBySessionState(type, states)`, an index on
`(type, session_state)`) belongs to the inbox ticket.

11. **SPA.**
    - `ChannelSessionView` gains `status`, `waitingFor: {label: string}[]`
and `waitingOnMe`. `channelView.ts` maps them from the head and the descriptor.
    - `RlChannelSession`: an `input_required` current state shows an amber
`RlTag` "Input required" and a caption "Waiting for the incident lead". When
`waitingOnMe` is set, it shows an accent tag "Waiting for you". A final state
shows the status label in place of "Ended": a check and "Completed", or
"Cancelled", or a red "Failed". Holder ids are not shown: the entity page shows
the owner field, and titles would need a lookup.
    - The notice view appends "waiting for <labels>" when the target state
has `waiting_for` in the descriptor.
    - New `useMessages` keys. The stepper stays non-live: the notice
announces moves. A handoff made by editing the form is not announced; the next
live frame updates the tag (a store change runs a full live round, which replays
channel heads).
    - Stories for the four new statuses, from the fixtures.

**Alternatives rejected:**
- Literal principals in `waiting_for`: hard-codes people in a repo file,
and gives the inbox a second matching rule. A named role reaches a person.
- Broadcast roles (attached, permission, principal type) in `waiting_for`:
every reader, or every permission holder, would be "waited on", and the inbox
would need a full scan with an ACL call per row.
- Counting a re-entry as an answer: any self-loop would pass the check, so
the check would prove nothing.
- A warning instead of a refusal for the deadline reset: an escalation that
a holder can postpone forever is a defect, not a style issue.
- `keep_timers` on re-entry now: more surface; the hand-over state covers
the use case. It stays a possible follow-up.
- `working` for a stranded state: an agent following the loop would wait
forever. Leaving `status` out would make clients branch on absence.
- `entered_at` set to the call's clock for an unstored channel: every
replay differs, so a watch never waits.
- Resolving holders of every source: needs principal enumeration per read.
- A stored status column: it would drift on a config reload.
- A parameter on `read_channel`: the result keeps `rev` and `last_seq`.
- MCP push notifications: no resource subscription seam exists.
- A new `handoff:` output kind: `set:` already writes with the poster's
authority and audit (decision 4).
- `Session.StatesWaitingFor`: no consumer in this slice; the inbox adds
what it needs.

**Files to modify:**
- `internal/realtimecfg/raw.go`, `keys.go`, `session.go`, `waiting.go`
(new: `waitChecker`, `SessionStatus`), tests (`session_test.go`,
`load_test.go`), `testdata/{valid,acl}.yaml`.
- `internal/acl/jobgrants.go` (`AnyRoleGrants`, + test).
- `internal/realtime/waiting.go` (new), `session.go`, `render.go`,
`service.go` (`ListAnchor`, `listType`, `sessionAfterOutputs`, `Session`),
`describe.go`, `access.go` (godoc), tests.
- `internal/mcp/tools_channels.go` (`SessionReader`, `read_session`),
`tools_watch.go` (`watchOrder`, target mapping), the `ChannelDeps` wiring and
stub, tests, golden tool list.
- `internal/apiwire/v1/channels.go`, `internal/openapi/channels.go`
(+ test).
- `internal/cli/channel_render.go` (+ test).
- `frontend/src/types/channels.ts`, `wireFields.test.ts`,
`frontend/src/components/entity/channels/channelView.ts` (+ test),
`frontend/packages/rela-components/src/components/channel/{types.ts,
RlChannelSession.vue, RlChannelSession.stories.ts}`, `useMessages.ts`, fixtures.
- `docs-project/entities/guides/GUIDE-realtime.md`, `GUIDE-mcp-server.md`
(then `just docs`).

## Security Considerations

- [x] Input sources identified (user input, config, external APIs)
- [x] Input validation approach defined (allowlist preferred over blocklist)
- [x] Security-sensitive operations identified (file access, auth, crypto)
- [x] Error handling doesn't leak sensitive information

**Input Sources & Validation:**
- `realtime.yaml` `status:` is an allowlist of five values. `waiting_for:`
takes declared bounded roles only. Both are operator config and not secret, so
the schema endpoint serves them to every reader.
- Holders are anchor property values. They come only from the caller's
redacted anchor view, read through the gated reader in `open` or `ListAnchor`.
`identityProperty` refuses a named role on a property that `visible:` restricts,
so a holder shown is a value the caller can already read with a GET of the
entity. Row level: the session is read only after the read role passed, which
requires anchor read.
- Holders are free text that anyone with field write can set. MCP returns
them under `realtime.UntrustedNotice`; the CLI passes them through `printable`;
the SPA does not render them.
- `read_session` takes the address arguments of `read_channel`.
- Handoff values are message props. A `ref` prop is resolved and gated as in
5a. Free text and composite templates get the AC 10 warning.

**Security-Sensitive Operations:**
- `waiting_on_me` is a one-bit answer about the caller's own roles, decided
by the same path that gates posting, client ceiling included. A read-only agent
client is never told it is its turn.
- Handoff writes the graph as the poster through `PatchEntity`, after the
5a field-write pre-check, audited with the output origin. A job move hands off
only with `run_as` grants and config-fixed values (5b).
- Deadlines: the AC 8 refusal stops a waiting role from postponing a
`since: state` deadline or a reminder escalation by re-entering.
- Per-reader fields are never cached across readers: role answers reset per
type, creator roles per channel (AC 17).
- Error answers: `no_session` names a config type and is decided before the
anchor read, so it reveals nothing about the anchor. Denials reuse the
`read_channel` answers, so `read_session` adds no existence oracle.

## Test Plan

- [x] Test scenarios documented for each acceptance criterion
- [x] Edge cases identified and documented
- [x] Negative test cases defined (invalid input, error conditions)
- [x] Integration test approach defined (not just unit tests)

**Test Scenarios:** one test or table row per acceptance criterion, named in the
criterion. New tests are created under those names; refused keys and the valid
testdata use the existing `TestParse_SessionInvalid` and `TestParse_Golden`.
`realtimecfg` and `acl` tests are table-driven unit tests over `testdata`.
`internal/realtime` tests use the service harness with a fake anchor source (for
redaction) and a counting store and access (for budgets). `internal/appbuild`
runs the handoff end to end over memstore with the real `acl.Policy`, the
field-gated entity manager and a capturing audit sink. MCP tests drive the
registered tools, including `watch` with a shortened tick and poll.

**Edge Cases (each has a test):**
- A re-entry in a state with only `since: session` timers loads, and its
timers keep their due time (AC 8 row plus `TestHandoff_SessionTimerSurvives`).
- A handoff to the current holder through the hand-over state: the set
reports no change, and the holder sees `waiting_on_me` in `handed-over`.
- A named role on a property holding a list: holders lists every value, and
`waiting_on_me` uses `has_current_user`.
- A closed channel in an `input_required` state: status stays
`input_required`, `waiting_on_me` is false; reopen restores it.
- A config reload that removes `waiting_for` from the current state: the
next read drops it, and a running watch wakes once.
- A config reload that changes `initial:` while a stored channel never moved
(AC 13).
- Two channel types on one anchor that both declare `lead` (AC 17).
- A watch with both a `read_session` and a `read_channel` target on one
channel: a text post changes only the `read_channel` index.
- The new `testdata/acl.yaml` grants leave `TestRunAsGrantCheck` and
`TestRunAsGrantCheck_AgreesWithRequest` unchanged.

**Negative Tests:** AC 1 (bad status values), AC 3 (final rules), AC 4 and AC 5
(bad role refs and broadcast sources), AC 6 (status and `waiting_for` mismatch),
AC 7 (roles without an answer), AC 8 (deadline reset), AC 9 (refused keys), AC
16 (false rows), AC 20 (a member without field write is refused), AC 22 (no
session, read denied).

## Risk Assessment

- [x] Technical risks assessed with mitigations
- [x] Security risks assessed (see Security Considerations)
- [x] Effort estimated (xs/s/m/l/xl)

**Risks:**
- Role-answer caching across types or channels in `ListAnchor` could show
`waiting_on_me` wrongly. Mitigation: reset per type, creator answered per
channel, AC 17.
- Changing the `sessionView` signature and `entered_at` touches every
channel answer and the SPA. Mitigation: one `call.waiting` constructor; the
compiler finds every call site; the TS field becomes optional so the type check
finds every reader.
- The deadline-reset check covers waiting roles only. A re-entry open to a
broader role that a holder also holds can still reset timers. Mitigation:
documented; narrow re-entries with `roles:`.
- `AnyRoleGrants` is a floor that ignores local and asserted roles, so the
AC 11 warning can be a false alarm. Mitigation: it is a warning, and the message
says so.
- Claims across transports can write identities that do not match.
Mitigation: documented and pinned for MCP (AC 20).
- Session watches wake on reminders. Acceptable and documented.
- Security: a per-reader bit on a shared view type. Live frames and MCP
replay per caller, and no cache keeps views across readers.

**Effort:** m

## Documentation Planning

- [x] User-facing docs identified (skip if internal refactor)
- [x] Docs-checklist will be created when entering implementation

**Documentation Impact:** Enhancement. `GUIDE-realtime.md` (generates
`docs/realtime.md` via `just docs`): a "Status and waiting_for" subsection under
"Sessions" (the five values and what each means to an agent, defaults, the final
rule, bounded roles, the answer check and that it is structural, the
deadline-reset rule, stranded reads `failed`, holders and how they are gated,
`waiting_on_me`, the unset state). A "Handoff" subsection (the hand-over state
example, claim, field write grants, audit, identity across transports,
last-writer-wins, the free-text warning). The load rules go in "Checks when the
file loads". The view fields, including the optional `entered_at`, go in "HTTP
API". `read_session` and the watch loop, with the 2s cross-process bound and the
untrusted notice, go in "Agents and other processes". `GUIDE-mcp-server.md` gets
a `read_session` row and a note on the `watch` row. OpenAPI descriptions carry
the status enum and the per-reader note.

## Design Review

- [x] Run `/design-review` before starting implementation
- [x] All critical/significant findings addressed in plan

**Design Review Findings:** the findings of `/design-review` are recorded as
review responses linked to this ticket. All 16 are folded into this plan:
answers exclude re-entries and the example has real exits (AC 7); the
deadline-reset refusal and the hand-over state (AC 8, Approach 6); a stable
`entered_at` for unstored channels (AC 14, AC 23); the unset state is `initial`
everywhere and the inbox lists stored channels only (AC 13, Approach 10);
bounded waiting roles only (AC 5, Approach 10); the untrusted notice on
`read_session` (AC 22); a separate `SessionReader`; `*string` for status (AC 1);
the answer grant warning (AC 11); ACL-role grants in the testdata; the claim
identity rule documented and tested (AC 20); stranded reads `failed` (AC 12);
last-writer-wins documented; the widened handoff warning (AC 10);
`StatesWaitingFor` dropped and the inbox location named (Approach 3); real test
names and `no_session` checked first (AC 22).

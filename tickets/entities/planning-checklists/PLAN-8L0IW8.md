---
id: PLAN-8L0IW8
type: planning-checklist
title: 'Planning: Sessions 4/5: graph outputs'
started: "2026-10-25"
completed: "2026-10-26"
status: done
---

<!-- @managed: claude-workflow v1 -->

## Understanding

- [x] Problem/requirements clearly understood
- [x] Scope defined (what's in/out documented below)
- [x] Acceptance criteria documented with specific test scenarios

**Problem:** Slices 1 to 4 (TKT-Y6NJ6U, TKT-190E1N, TKT-INM3AZ, TKT-36Z6X3,
merged in #1988) gave a channel type a flat session state machine. Typed
messages and `after:` timers move it, and reminders post into it. Nothing a
session does reaches the graph. The GDPR breach use case needs "when the DPO
posts `notified`, set the breach's status to `notified`" and "when the lead
declares containment, add a timeline entry". After that the room should close by
itself. The sessions research (RES-O8I8S7, option f2) chose transition
`outputs:` written through the entity manager, with pending and done marks so a
failed write is visible and can be retried.

The design review split the slice. This ticket is **5a**: outputs on
message-driven transitions, written as the poster. **5b** (a separate ticket)
adds `run_as` and outputs on timer-fired and trigger-fired transitions. This
slice is part of FEAT-GWF8YH phase 4.

Terms used below:
- **Output**: one entry of a transition's `outputs:` list. It is either
`set:` (patch properties of the channel's anchor entity) or `create:` (create
one entity, optionally related to the anchor).
- **Message-driven transition**: an `on.<type>` entry of a state, fired by a
person's or agent's post.
- **Trigger-reachable transition**: a message-driven transition that a
trigger post can also fire. That is a transition with `by: trigger`, or a
transition without `roles:` whose message type a trigger rule posts into this
channel type (`poster.admits`, `internal/realtime/session.go`).
- **Writer**: the principal an output writes as. In 5a this is always the
poster of the cause message.
- **Outputs mark**: the record on a transition's notice (the system message
each move appends) that says which outputs are pending, done or failed. It is
the notice's action, named `outputs`, with a new `Outputs` field.
- **Claim**: a compare-and-set on the mark that gives one runner the right
to run the next attempt. It holds an attempt number, a random token and the time
of the holder's last write.
- **Runner**: `realtime.Outputs`, the leader component that retries marks
whose claim has expired.
- **Template**: a property value in an output. It is a literal, a
whole-value reference such as `{{message.risk}}`, or a string with references
inside.
- Notice, generation, decider, timer, reminder and round keep their meaning
from the earlier plans.

**Scope:**
- In: `outputs:` on `session.states.<s>.on.<type>`, with `set:` and
`create:`, compiled templates, and load-time validation with one problem per
key.
- In: refusal at load of outputs on timer transitions and on
trigger-reachable transitions, with a reason that names the follow-up.
- In: `session.on_final: close`.
- In: the outputs mark on the notice in every channel store backend,
written in the same store step as the move. No schema step is needed; see
Approach, decision 5.
- In: a pre-check that refuses a post with 403 when the poster may not
write the outputs of the transition it would fire.
- In: the first attempt inline in the poster's request, and the runner
that retries failed or interrupted attempts.
- In: output status on notices on the wire (HTTP, MCP, CLI), an output
count on the transition descriptor, OpenAPI, the frontend wire mirror (types
only), and docs.
- Out (5b): see "Deferred to 5b" at the end of Approach.
- Out: outputs on reminders and on state entry (refused keys; see decision
1). Out: `entity` in templates (as in guards). Out: `delete`, relation-only and
`unset` outputs, and `content` (the body) in `set:`. Out: a moderator "retry" or
"abandon" action for a failed mark; the operator fixes the entity by hand. Out:
UI components (slice 6 renders output status). Out: mail or HTTP side effects
(`jobs.md`; automations that need them already enqueue jobs).

**Example (the `breach-response` type in `testdata/valid.yaml` gains these keys;
the docs use it):**

```yaml
    session:
      label: GDPR breach response
      initial: detected
      on_final: close
      states:
        detected:
          label: Detected
          on:
            contained:
              to: contained
              roles: [lead]
              outputs:
                - create:
                    type: timeline-entry
                    properties:
                      title: "Contained by {{message.author}}: {{message.note}}"
                      at: "{{now}}"
                    relate: { type: about }
            # ... dismiss, escalated as in slice 1 ...
          after:
            no-containment: { period: 24h, to: overdue, label: Not contained within a day }
        contained:
          label: Contained
          on:
            assessed:
              to: assessed
              roles: [lead]
              when: "message.risk ~= 'none'"
              outputs:
                - set: { properties: { risk: "{{message.risk}}" } }
            escalate-stalled: { to: overdue, when: "session.reminders['status-update'] >= 3" }
        assessed:
          on:
            notified:
              to: notified
              roles: [dpo]
              outputs:
                - set: { properties: { status: notified, reference: "{{message.reference}}" } }
        # ... overdue (notified with the same outputs), notified, dismissed ...
      reminders:
        # ... as in slice 4 ...
```

`testdata/schema.yaml`: `datalek` gains `risk: {type: risk-level}` and
`reference: {type: string}`; `timeline-entry` gains `at: {type: datetime}`. Each
transition with outputs names `roles:`, so no trigger can fire it.

**Acceptance Criteria:**
1. The example compiles. `Transition.Outputs()` returns copies, in file
order, each with `Kind` (`set` or `create`), `Properties()` (name to compiled
`Template`), and for `create`, `Type`, `Face` and `Relate`. `Session.OnFinal()`
is `close`. Test `TestParse_SessionOutputs` in
`internal/realtimecfg/outputs_test.go`; `TestParse_Golden` updated.
2. Each invalid output reports its key path and reason, all in one load.
Table-driven `TestParse_SessionOutputsInvalid`, one row per rule in decision 2:
more than 4 outputs; two `set:` in one transition; an entry with both or neither
of `set` and `create`; empty `properties`; an undeclared property; a computed,
file or `external_ref` property; a literal of the wrong type; an enum literal
not in the enum; an unknown reference; `{{entity.*}}`; `{{message.x}}` for an
undeclared message property; a whole-value reference whose type cannot fill the
property (the `promotable` rule); a mixed template in a property that is not
string or markdown; a mixed template naming a non-text reference; an
unterminated `{{`; a template over 1024 bytes; `create.type` unknown, an alias
or `id_type: manual`; a missing required property; a bad `create.face` (the
`promote_to` face rule); a `relate.type` that cannot link the type to every
`attach_to` type; the `set.face` rows of decision 1; an `on_final` other than
`close`; and an action named `outputs` on a message type of a session type.
3. Outputs that 5a cannot run are refused with a reason that names the
follow-up ("... arrive with run_as in a later release (TKT-<5b>)"). Rows in
`TestParse_SessionOutputsInvalid`: outputs on a `by: trigger` transition;
outputs on a transition without `roles:` whose message type a trigger rule posts
into this channel type; and the refused keys `session.states.*.after.*.outputs`
and `session.run_as`.
4. Other refused keys, each with its reason: `session.states.*.outputs`,
`session.reminders.*.outputs`, `session.states.*.on.*.run_as`,
`session.states.*.on_final`. Rows in `TestParse_Invalid`.
5. Adding, removing and changing outputs and `on_final` pass `CheckReload`
(rows in `TestCheckReload_SessionChangesAllowed`). The compiled outputs are
immutable: unexported slices and maps, and accessors return copies (row in
`TestCompiled_SessionIsReadOnly`).
6. Template evaluation, `TestTemplate_Eval` in
`internal/realtimecfg/template_test.go`, table: a literal returns its typed
value; a whole `{{message.risk}}` returns the enum value; an absent optional
whole-value reference returns "absent" and the property is left out; a mixed
template with an absent reference substitutes the empty string; a substituted
value containing `{{x}}` is not scanned again; `{{now}}` into a datetime
property is the notice time, and into a string property its RFC 3339 form in
UTC; `{{today}}` is the notice date in UTC; `{{session.from}}`,
`{{session.to}}`, `{{session.generation}}`, `{{channel.key}}`,
`{{channel.type}}`, `{{channel.title}}` and `{{message.author}}` resolve; the
same inputs give the same output (no clock read).
7. Store conformance in `channelstest.RunAll` (file store on disk and on
MemFS, sqlite, postgres), new `channelstest/outputs.go`:
   - `testOutputsMarkOnAppendMove`: an `Append` whose step's `Mark`
callback returns a mark for the move stores the notice with action `outputs`,
status pending, the claim fields, and `PendingAt` equal to the given eligibility
time. A step whose decider does not move, or whose callback returns nil, stores
no mark.
   - `testOutputsClaimCAS`: two concurrent claims of the next attempt on an
eligible mark: exactly one succeeds, the other is `ErrConflict`.
   - `testOutputsProgress`: a progress write by the holder (same attempt and
token) stores the results and moves `PendingAt` and `ClaimedAt` forward. A
progress write with another token or attempt is `ErrConflict`. The done write
moves the action to done and writes one channel audit record with status done
and `By` the writer.
   - `testOutputsEligibility`: `StalePending(now)` lists a mark only once
`now` has passed its `PendingAt`, oldest first.
   - `testOutputsOnClosedChannel`: progress and done writes succeed on a
closed channel.
   - `testOutputsRefusals`: each of these is `ErrInvalidMessage` or
`ErrTooLarge`, and nothing is stored: an `outputs` action on an append without a
session step; a pending-to-pending write on any other action name; a claim or
progress write that changes `Writer`, `Roles`, the generation, `From`, `To` or
`Cause`; more than 5 results; an oversized mark.
   - `testOutputsSurviveMoves`: the mark is kept across `SetLifecycle`
close and reopen, `Rekey`, and `Tombstone` then `Untombstone`.
8. File-tier crash window: a test in `filechannels` writes meta.yaml with a
pending ref at a seq the log does not hold (the crash between meta and log). The
next `Append` drops the ref before it decides, and the new message at that seq
carries no mark. Test `TestAppend_DropsDanglingPending`.
9. Legacy data: a slice-4 notice without a mark reads with no outputs on
every backend (rows in `testSessionStep`). No schema version changes:
`TestSchemaVersion` in `internal/sqlitedb` still expects 26, and the newest
postgres migration is still `0033`.
10. Inline outputs, `internal/realtime/outputs_test.go`, service over the
file store on MemFS with a fake writer that records requests:
    - `TestOutputs_MessageSet`: the dpo posts `notified` in `assessed`. The
writer gets one `set` on the anchor at its face, with `status: notified` and
`reference` from the message. It runs as the poster with tool `realtime` and
`triggered_by` `session:<key>/assessed->notified#<gen>/0`, and it uses the grant
from the pre-check. The notice reads `outputs.status: done`, and its writer
shows the poster's original tool. The channel is closed by `on_final`, and the
lifecycle audit record has `triggered_by` `session:breach-response/on_final`.
    - `TestOutputs_MessageCreate`: `contained` creates a `timeline-entry`
related `about` to the anchor. The notice result holds the created id as a ref.
    - `TestOutputs_LiteralNotAuthored`: literal values reach the writer as
not authored, and interpolated message values as authored, so the field gate
applies only to the interpolated values.
    - `TestOutputs_PreCheckRefuses`: table. A poster whose update on the
anchor is denied, a poster who may not create the type, and a poster who may
create it but lacks the relation write of `relate` each post a message that
would fire outputs. Each gets 403 `session_output_denied` naming the output key,
and nothing is stored. A transition whose guard does not hold, or whose roles
the poster lacks, needs no pre-check and posts as before. A post that would fire
outputs while the snapshot is degraded is refused with 403
`session_output_denied` ("outputs are suspended while realtime.yaml is
degraded").
    - `TestOutputs_PreCheckRace`: the session moves between the head read
and the append into a state where the message fires a transition whose outputs
the poster may not write. The post is stored, and the session does not move (the
decider backstop).
    - `TestOutputs_ValidationFails`: the writer returns a validation
refusal. The move stays, the notice reads `failed` with code `invalid` and the
property names, no retry is scheduled, and the MCP post answer carries the
status.
    - `TestOutputs_Warnings`: a create that succeeds with warnings is done,
with the warning codes in the result.
    - `TestOutputs_Order`: outputs run in file order. A failure stops the
rest, which read `skipped`.
    - `TestOutputs_DisjointSetsBothWritten`: posts A (set `risk`) and B
(set `status`) move the session in turn, and B's outputs run first. Both
properties are written.
    - `TestOutputs_SameSetSuperseded`: A and B both set `status`, and B's
outputs finish first. A skips `status` and reads `superseded` for it, and writes
its other properties.
    - `TestOutputs_NoOutputsNoMark`: a transition without outputs, into a
state that is not final or without `on_final`, writes no mark.
    - `TestOutputs_FinalWithoutOutputs`: a move into a final state with only
`on_final: close` writes a mark with no outputs and the close flag, and closes
the channel.
    - `TestOutputs_ReopenDoesNotReclose`: a reopened channel in a final
state stays open through later runner rounds.
    - `TestOutputs_CauseRemovalBlocked`: a moderator removal of the message
whose notice has a pending mark answers 409 `outputs_pending`. After the mark is
done, the removal succeeds.
    - `TestReleaseAction_RefusesOutputsMark`: releasing an `outputs` mark,
even past `PendingRelease`, answers 409 `outputs_pending`.
    - `TestSweep_SkipsOutputsMark`: the stale-pending report and its
"a moderator can release it" warning leave out `outputs` marks.
11. Runner, `internal/realtime/outputs_runner_test.go`, with the test clock:
    - `TestRunner_RetryAfterCrash`: a mark whose holder stopped writing is
retried as attempt 2 once `OutputClaimTTL` (90 s) has passed since the holder's
last progress write. A holder that keeps writing progress every 60 s is never
taken over. The TTL is at least 2 x (`promoteTimeout` + progress-write budget);
a test asserts that relation on the constants.
    - `TestRunner_RetryAsPoster`: a retry resolves the stored writer
through `IdentityResolver.ResolveDirect` and builds
`principal.VerifiedFrom(user, "realtime", {PrincipalType, OrgID, Roles})` from
the mark. A retry after `OutputRetryWindow` (30 min) from the notice is not run
and reads `failed` with code `expired`.
    - `TestRunner_DanglingRef`: a pending ref whose seq holds no notice
with a pending `outputs` mark is logged once and skipped.
    - `TestRunner_TransientRetries`: an unknown error, or a version
conflict from `PatchEntity`, retries up to `MaxOutputAttempts` (5), then reads
`failed` with code `error`.
    - `TestRunner_TransitionRemoved`: a reload removed the transition. The
retry reads `failed` with code `transition_removed`.
    - `TestRunner_Degraded`: under a degraded snapshot the runner does
nothing; it retries once the config is valid again.
    - `TestRunner_NoJoinCreateNotRepeated`: when `txChannelsFor` does not
join (the file store, or a channel store on another database), a create whose
claimed attempt may have written (no progress recorded and not `WroteNothing`)
is not retried. It reads `failed` with code `unconfirmed`.
    - `TestRunner_LeaderOverlap`: two runners over one store take the same
mark. Exactly one claim succeeds, and the writer is called once.
    - `TestRunner_Cap`: 40 eligible marks. At most `MaxOutputRunsPerRound`
(16) run per round, oldest first, and `More` is set.
12. Atomic creates where the stores join,
`internal/appbuild/realtime_outputs_db_test.go` (sqlite; postgres under
`RELA_TEST_DATABASE_URL`): with `txChannelsFor` joining, a create whose progress
write fails rolls back the entity and the relation, and a retry then creates
exactly one entity (`TestOutputs_CreateExactlyOnce`). A wiring where the channel
store does not share the database falls back to the `unconfirmed` rule
(`TestOutputs_NoJoinUsesUnconfirmed`).
13. Writer over the entity manager, `TestOutputWriter_*` in
`internal/appbuild/realtime_outputs_test.go`:
    - `set:` goes through `UpdateGrant.PatchEntity`. An unnamed property
hidden from the writer is preserved, pinned like
`TestPatchEntity_PreservesUnnamedProperties`.
    - Table `TestOutputWriter_Errors` maps errors to codes. A denied write
is `forbidden`, a field-gate refusal of an authored property is `field_denied`,
a validation error is `invalid`, and a unique conflict is `conflict`, all final.
`ErrEntityNotFound` is a final `not_found` with no id. A `store.ErrConflict`
version loss is retryable. No code or detail quotes the error text or an entity
id.
    - The audit records carry the writer, tool `realtime` and the
`triggered_by`.
    - `TestCheckRelate` in `internal/entitymanager` checks the probe:
allowed, denied, no audit row written. `TestAuthorizeAndAuditFollowsReadGate`
passes with `CheckRelate` in `noReadGateNeeded`.
14. Wire: `TestChannelWireGolden` is regenerated.
`channel_wire_fields.json` gains `outputs` on the message,
`ChannelMessageOutputs`, `ChannelMessageOutputResult`, and `outputs` on
`ChannelSessionTransition`. `channel_wire.json` gains a notice with a done mark
and one with a failed mark. The mark's role claims are not on the wire.
`frontend/src/types/wireFields.test.ts` passes with the mirrored fields.
`TestChannelsHandler_NoticeOutputs` checks that a created entity the reader
cannot read renders `restricted: true` without its title, and that the notice
offers no `_actions`.
15. OpenAPI: `TestOpenAPI_ChannelSession` is extended for the transition's
`outputs`, the message's `outputs` referencing `ChannelMessageOutputs`, and the
`session_output_denied` and `outputs_pending` problem types.
16. Cost: `channels_budget_test.go` with notices holding created refs. The
page endpoint's store and resolve budget is unchanged, because the ids join the
existing batch resolve. `TestOutputs_PostBudget`: a post with one `set:` costs
one `AuthorizeUpdate`, one supersede read, one `PatchEntity` and two mark
writes. `TestRunner_Budget`: with 100 pending marks, of which 16 are eligible,
one round makes one `StalePending` call, at most 16 `Get` calls and at most 16
claims.
17. MCP: `TestPostMessage_ReturnsOutputs` (done, failed) and
`TestReadChannel_NoticeOutputs` in `tools_channels_test.go`.
18. CLI: `TestRenderRead_NoticeOutputs` in `channel_render_test.go` renders
"moved from assessed to notified; outputs done: set datalek, created TL-1", and
a failed output as "outputs failed: set datalek (invalid: status)". `rela
channel post` prints the outputs line.
19. Docs: the guide example is the `breach-response` block of
`testdata/valid.yaml`, which AC 1 pins. `just docs` regenerates
`docs/realtime.md` and `docs/mcp-server.md` without a diff.

## Research

- [x] For larger features: run `/research` to create a structured research doc
- [x] ~~Searched for existing libraries that solve this problem~~ (N/A: covered by the research doc; outputs reuse the entity manager, the promote path and the action marks)
- [x] Checked codebase for similar patterns or reusable code
- [x] Looked for reference implementations in other projects
- [x] Reviewed relevant rela concepts for prior art

**Research Doc:** RES-O8I8S7 (`.ignored/res-sessions-final.md`). The decisions
of 2026-10-21 that apply here: the session runs only while the channel is open,
and `on_final: close` may close it (decision 2); roles come from graph
properties first, and a handoff is an audited entity write (decision 4), which
`set:` provides. The per-type `run_as` for timer-driven writes (decision 3)
moves to 5b. The research sketch put `outputs:` on a state; this plan puts them
on transitions (decision 1).

**Existing Solutions:**
- The promote path (`internal/realtime/promote.go`,
`internal/appbuild/realtime_promote*.go`): a `Promoter` over the field-gated
entity manager writes as the caller, with `audit.WithTriggeredBy`.
`CreateLinked` with `InTx` and `txChannelsFor` commits the entity, the relation
and the action mark in one transaction when the stores join. `promoteDenial`
maps create errors to codes without quoting them. `ErrNothingWritten` and
`WroteNothing` decide whether a retry is safe. Promoted ids render as
`RefValue`.
- Action marks (`channels.Action`, `SetAction`, `StalePending`, the
`pending_at` column and partial index, filechannels `meta.Pending`). The outputs
mark is an action on the notice, so storage and the index exist.
- `Manager.AuthorizeUpdate` (`internal/entitymanager/manager.go:1306`):
read gate, then the real update question (local roles and conditions included),
returning an `UpdateGrant` whose `PatchEntity` skips the second check. This is
the pre-check for `set:`.
- `CheckCreate` (`linked.go:318`), an affordance probe without audit, for
`create:`. Its sibling `CheckRelate` (new) probes `RelationCreateRequest` the
way `prepareLinkedRelation` asks it.
- `PatchEntity`: partial writes merged against the raw stored row, so a
writer who cannot see a property cannot erase it.
- `IdentityResolver.ResolveDirect` (`internal/acl/identity.go:175`) for a
stored, already resolved user id.
- Lifecycle `applyTransition` with a `TriggeredBy` label (`auto:` rules):
the model for the `on_final` close.
- Interpolation: `ingest.Interpolate` has the single-pass rule (a
substituted value is never scanned again). `automation.Interpolate` has the
`{{now}}` and `{{today}}` names. Decision 3 explains why neither is called.
- Prior art (research table): the transactional outbox pattern, where the
intent is written atomically with the state change and then delivered and
marked; Temporal activities, which run at least once with idempotency keys and a
heartbeat timeout; Step Functions `Retry` with a bounded attempt count. The
outputs mark is an outbox row on the notice, and progress writes are its
heartbeat.

## Approach

- [x] Technical approach chosen and documented
- [x] Approach builds on existing patterns (not reinventing)
- [x] Alternatives considered (document why rejected)
- [x] Dependencies identified (packages, APIs, types)

**Technical Approach:**

1. **Where outputs sit.** Outputs sit on message-driven transitions:
`session.states.<s>.on.<type>.outputs`. A transition knows its cause, so
`{{message.risk}}` is typed against one message type at load. A state can be
entered by several message types and timers, so outputs on a state could not
type their references. They would also run for causes the operator did not think
about. `session.states.<s>.outputs` is a refused key: "outputs belong to a
transition: put them under on.<type>.outputs". `session.reminders.<n>.outputs`
is refused: "a reminder posts a message only; let a guard on its count fire a
transition with outputs".

Outputs that need a writer other than the poster are refused in 5a. That covers
`after.<name>.outputs`, outputs on a `by: trigger` transition, and outputs on a
transition without `roles:` whose message type a trigger rule (`Trigger.Channel`
and `Trigger.PostType`) posts into this type. A trigger post's author is
`system:realtime`, which must not write the graph on a session's behalf. The
reason for each names 5b, for example: "outputs on a transition a trigger can
fire need a run_as identity, which arrives in a later release (TKT-<5b>); add
roles: so only people fire it, or remove the outputs". `session.run_as` is
refused with the same pointer.

Shapes (`raw.go`):

```go
type rawOutput struct {
    Set    *rawSetOutput    `yaml:"set"`
    Create *rawCreateOutput `yaml:"create"`
}
type rawSetOutput struct {
    Face       string         `yaml:"face"`
    Properties map[string]any `yaml:"properties"`
}
type rawCreateOutput struct {
    Type       string         `yaml:"type"`
    Face       string         `yaml:"face"`
    Properties map[string]any `yaml:"properties"`
    Relate     *rawRelate     `yaml:"relate"`
}
// rawTransition gains Outputs []rawOutput.
// rawSession gains OnFinal string `yaml:"on_final"`.
```

- `set:` targets the channel's anchor, resolved per anchor type:
  - On a face-anchored type, the target is the channel's own anchor
face, and `set.face` is refused ("the channel is anchored to one face; set
writes that face").
  - On a family-anchored type, `set.face` is required when any
`attach_to` type declares faces. Every faced `attach_to` type must declare that
face. An anchor whose type is unfaced uses the zero face, which is correct for
an unfaced type.
  - So a mixed `attach_to` loads. The face is never picked by rank,
which CLAUDE.md forbids for writes.
  - The writer passes the face-qualified address, so no bare
`entity.Ref` literal is added (`archguard/bareref_test.go` is unchanged).
- `create:` is `promote_to` with `properties:` templates instead of
`map:`. Its type, face, required-property and `relate` checks are the
`promote_to` checks (`v.promote`), refactored to take a property source, so the
two cannot drift. `relate` links the new entity to the anchor (new entity to
anchor, as `promote_to` does).
- Limits (`realtimelimits`): `MaxTransitionOutputs = 4`, at most one
`set:` per transition, `MaxOutputTemplateBytes = 1024`.

2. **Validation** (new `internal/realtimecfg/outputs.go`, called from the
transition compile in `session.go`). Each problem is reported once, per key,
with the messages AC 2 to AC 4 list. The main rules:
   - Property checks against the metamodel: the property is declared, not
computed, not a file property, and not `external_ref`. Literals are checked
against the property type and enum values with the metamodel's value check.
   - References come from a fixed list: `{{message.<p>}}` (a declared
property of the cause type), `{{message.author}}`, `{{session.from}}`,
`{{session.to}}`, `{{session.generation}}`, `{{channel.key}}`,
`{{channel.type}}`, `{{channel.title}}`, `{{now}}`, `{{today}}`. An unknown
reference reports "unknown reference x; use message.<property>, session.from,
...". `{{entity.*}}` reports "outputs cannot read the entity; a value from the
entity would be written with the writer's authority and could copy a hidden
field".
   - A whole-value reference must fill the property, by the `promotable`
rule. A mixed template is allowed only for string and markdown properties, and
only with text references: string, markdown, enum and ref values, the session
and channel names, `now` and `today`.
   - `on_final`: only `close` ("on_final may only be close"). A
state-level `on_final` is a refused key.
   - Reserved action name: in a session type, a message type may not
declare an action named `outputs` ("outputs is reserved for the session's output
marks").
   - Reload: `CheckReload` is unchanged, because outputs decide nothing
about keys.

3. **Templates** (new `internal/realtimecfg/template.go`).
   - A compiled `Template` is a typed literal, a whole-value reference, or
a list of text and reference parts.
   - `Eval(vars OutputVars, target metamodel.PropertyType) (any, bool)`.
`OutputVars` holds the message props, the author, the session
from/to/generation, the channel key, type and title, and the notice time.
   - `now` and `today` come from the notice time in UTC, never from the
clock, so a retry writes the same values. This differs from automations, which
format `now` in the local time zone; the docs say so.
   - A whole-value reference to an absent optional property returns
`false`, and the property is left out (the `promoteProps` rule). A mixed
template substitutes the empty string. A substituted value is never scanned
again.
   - `automation.Interpolate` is not reused, for four reasons. It is
string-only. It takes an entity. It adds git-config user variables that mean
nothing here. And it scans a substituted value again for `{{new.*}}`, so a
message value could inject a reference. `ingest.Interpolate` belongs to another
subsystem, and its rule is a 20-line loop.

4. **Writer and pre-check.**
   - The writer is the poster. The inline attempt uses the request's
principal with `Tool` set to `principal.ToolRealtime` on a copy. The copy keeps
the verified claims, so the client ceiling applies to the write as it does to
the post.
   - The mark's `Writer` is the cause message's author, including its
original tool (MCP, data-entry). It shows on the wire and in the channel done
audit record, so an agent's output can be told apart from a person's. The graph
audit records tool `realtime`, with `triggered_by` `session:<channel
key>/<from>-><to>#<gen>/<index>` through `audit.WithTriggeredBy`.
   - Pre-check before the append. The check runs for the transition that
`mt` fires in the head-read state, when that transition admits the poster and
its guard holds. It runs on the stamped context (tool `realtime`) and checks
each output:
     - `set:`: `Manager.AuthorizeUpdate(ctx, row)`, where the row carries
the type, id and face of the anchor the call already read, at the `set:` target
face. This runs the read gate and the real update question, including local
roles and conditions, so a DPO whose update comes from a local role passes. A
denial writes the manager's denied-write audit row, which is correct: a refused
post is a real refused attempt. The returned `UpdateGrant` is kept for the
inline write.
     - `create:`: `entitymanager.CheckCreate` for (type, face), and, when
`relate` is set, the new package function `entitymanager.CheckRelate(ctx, m,
relType, fromType, fromFace)`. It asks `RelationCreateRequest` as an affordance
probe (no audit), as `prepareLinkedRelation` does. A relation write is decided
from the source type and the caller's global roles only, so the probe names no
stored row. It joins `noReadGateNeeded` in `readgateguard_test.go` with that
reason. It is a package function because `Manager` is at its method cap.
     - On a denial the post is refused with 403 `session_output_denied`.
The title names the output key and the ACL rule; config is not secret.
     - The decider gets the answers. A transition whose outputs were denied
does not move, which is the backstop for a race where the state changes between
the head read and the append. The post is stored and nothing moves, as for a
role the poster lacks.
   - What the pre-check cannot see is recorded as a failed output after
the move: validation, the field gate on authored values, unique conflicts, a row
that became hidden, and transient errors.
   - Under a degraded snapshot, a post that would fire outputs is refused
with 403 `session_output_denied` ("outputs are suspended while realtime.yaml is
degraded"). The acl.yaml that broke the config may restrict what the outputs
write. This follows the lifecycle condition rule (`lifecycle.go:155`). Posts
that fire no outputs are unaffected.
   - Why there is no single atomic step across both stores. The decider
runs under the channel store's lock. The entity write runs validation,
automations and the cascade, which is slow work that must not run inside a `Tx`
callback or a channel lock. On the file tier the two stores cannot share a
transaction at all. So the move commits with a durable pending mark in the same
channel step (an outbox), and the writes follow.

5. **The outputs mark, and why there is no migration.** The mark is the
notice's action: `Action{Name: "outputs", Status: pending, PendingAt: <eligible
at>, Outputs: &OutputMark{...}}`.

   ```go
   // OutputMark records a session move's graph outputs (TKT-98TR9A).
   type OutputMark struct {
       Writer    Author         `json:"writer"`           // the cause's author, original tool kept
       Roles     []string       `json:"roles,omitempty"`  // writer's verified role claims; never on the wire
       Gen       int64          `json:"gen"`              // generation after the move
       From      string         `json:"from"`
       To        string         `json:"to"`
       Cause     int64          `json:"cause"`            // seq of the cause message
       Attempt   int            `json:"attempt"`
       Claim     string         `json:"claim,omitempty"`  // random token of the current claim
       ClaimedAt time.Time      `json:"claimed_at,omitzero"` // holder's last write
       Close     bool           `json:"close,omitempty"`  // on_final close still due
       Results   []OutputResult `json:"results,omitempty"`
   }
   type OutputResult struct {
       Kind     string    `json:"kind"`               // set | create | close
       Status   string    `json:"status"`             // done | failed | skipped | superseded
       Code     string    `json:"code,omitempty"`     // forbidden, field_denied, invalid, conflict, not_found, unconfirmed, expired, transition_removed, error
       Props    []string  `json:"props,omitempty"`    // set: properties written; invalid: properties named
       Entity   *Promoted `json:"entity,omitempty"`   // the created entity
       Warnings []string  `json:"warnings,omitempty"` // warning codes
   }
   ```

   - `channels.Action` gains `Outputs *OutputMark`
(`json:"outputs,omitempty"`). sqlchannels keeps the action in the message `doc`
JSON. The existing `pending_at` column and its partial index (sqlite DDL,
postgres `0026_channels.sql`) find pending marks. filechannels keeps the mark in
the log line and in `meta.Pending`. So no column, no sqlite rung after v26, and
no postgres file after 0033 is needed (AC 9). If a later review wants a
dedicated index, the next free numbers are sqlite v27 and postgres 0034.
   - `PendingAt` of an `outputs` mark means "eligible for a retry at". It
is set to `ClaimedAt + OutputClaimTTL` on the creating step and on every claim
and progress write. `StalePending(now)` therefore returns only eligible marks,
oldest first, through the existing index and `pendingRef`. No claim data is
added to `PendingAction`, and the runner needs one `Get` per eligible mark only
(AC 16).
   - `SessionStep` gains one field, `Mark func(move SessionMove)
*OutputMark`, a pure callback like `Decide`. When it returns a mark for a move,
`SessionNotice` attaches the pending action and the sql backends set
`pending_at`. The service's callback returns a mark only for a move whose
transition has outputs, or whose target is final with `on_final: close`. The
mark arrives already claimed (attempt 1, a token, `ClaimedAt` the post time),
because the request runs it. `SessionOutcome.Notice` returns the notice seq.
   - `SetAction` contract, with no new method. A pending-to-pending write
is allowed only for the `outputs` action. It must be a claim (the attempt is the
stored attempt + 1, with a new token, allowed once `PendingAt` has passed) or a
progress or done write by the holder (same attempt and token). `Writer`,
`Roles`, `Gen`, `From`, `To` and `Cause` never change after the creating step.
`channels.CheckOutputStep` holds these rules for every backend; anything else is
`ErrConflict` or `ErrInvalidMessage`. A done write records the existing audit
record. Done and progress writes are allowed on a closed channel.
   - `PrepareAction` caps the mark at `MaxTransitionOutputs` + 1 results
and a size under `MaxMessageBytes`. `PrepareMessage` and the action endpoints
refuse the `outputs` name on a message that is not a notice. `releaseGateCheck`
refuses an `outputs` action with 409 `outputs_pending`, so a moderator cannot
erase the outbox. `Sweeper.stalePending` and the lifecycle sweep's "a moderator
can release it" warning (`realtime_worker.go`) skip `outputs`.
   - File-tier crash window. `filechannels.Append` writes meta.yaml (the
moved session and `meta.Pending`) before the log line. A crash in between leaves
a pending ref at a seq the log does not hold, and the next append reuses that
seq. So `Append` (and `SetSession`) drop `meta.Pending` refs above the log's
last seq before deciding (AC 8). The runner also checks that the message at the
seq is a notice with a pending `outputs` mark. Otherwise it logs once per
process and skips the ref. The moved session without its notice is the slice-1
window, which is unchanged and documented.
   - Plimsoll: no new `Store` method. `SessionStep` gains one field and
`Action` one field, both under the field cap.

6. **Running outputs** (new `internal/realtime/outputs.go`):

   ```go
   // OutputWriter writes session outputs to the graph. The wiring site
   // implements it over the field-gated entity manager.
   type OutputWriter interface {
       Check(ctx context.Context, req OutputCheck) (OutputGrants, error) // *DenialError on refusal
       Write(ctx context.Context, req OutputRequest) (OutputResult, error)
   }
   ```

`OutputRequest` carries the anchor, kind, target ref, create type, face and
relate, the props, `Authored` (the interpolated values; literals are config),
the `triggered_by` origin, the grant from `Check` (may be empty on a retry), and
the holder's progress write. `NoOutputs` (like `NoPromoter`) refuses every
check. In a process that cannot write the graph, a post that would fire outputs
gets 403 "outputs are not available in this process".

The appbuild implementation `managerOutputs` mirrors `managerPromoter`. `set:`
goes through `UpdateGrant.PatchEntity`, or `PatchEntity` on a retry, with
`Patch.Properties`. `create:` goes through `CreateLinked` with `Authored` and
with `InTx` applying the progress write when `txChannelsFor` joins. Errors map
to codes: create errors through the `promoteDenial` classifier, and patch errors
through a new `patchDenial`. `ErrEntityNotFound` is a final `not_found`, and
`store.ErrConflict` is retryable.

- Inline run, in `Service.Post` after a committed move with a mark. The
service builds the requests from the current config's transition and evaluates
templates with the cause message and the notice time.
  - It takes the per-channel in-process lock and runs the outputs in
order. Each runs on a context the client cannot cancel, with `promoteTimeout`
each. After each output it writes progress, which refreshes `ClaimedAt` and
`PendingAt`.
  - Then it closes the channel if the mark says so, and writes done.
  - A failed output stops the rest, which read `skipped`. The mark is
then done with a failed result.
  - A refusal is final. An unknown error or a retryable conflict leaves
the mark pending, and the runner retries it once it is eligible.
  - `PostResult.Outputs` carries the view.
- Supersede per property. Before a `set:` the run reads the notices
after this one (`Read` from notice seq + 1, at most 64 messages). It skips each
property that a later mark's done `set:` result lists in `Props`, and writes the
rest. A set with every property skipped reads `superseded`. The read and the
write happen under the per-channel lock, so in one process the newest move's
value wins whatever order the inline runs take the lock. Across processes a
window remains (documented). A `create:` always runs, because a timeline record
of an earlier move stays true.
- Runner (`realtime.Outputs`, run by the leader in the realtime worker
on the minute tick):
  - It calls `StalePending(now)` once and keeps the `outputs` actions.
It takes at most `MaxOutputRunsPerRound` (16), oldest first.
  - For each it does a `Get` and the dangling check, then claims (CAS)
and runs the mark as the inline run does.
  - It does nothing under a degraded snapshot or when realtime is off.
- Retry identity. A retry resolves `Writer.User` through
`IdentityResolver.ResolveDirect`, because the stored id is already resolved. It
builds `principal.VerifiedFrom(user, ToolRealtime, Claims{PrincipalType, OrgID,
Roles: mark.Roles})`. The stored role claims keep a retry working for SSO users
whose grants come through `asserted_role_assignments`. They are bounded by
`OutputRetryWindow` (30 min after the notice), after which the mark fails
`expired`. Scopes are not stored, so a retry is never wider than the request. A
removed account fails `forbidden`. The raw IdP user name is not kept, so the
audit shows the resolved id.
- Claims and timing.
  - `OutputClaimTTL` is 90 s, counted from the holder's last progress
write. One step takes at most `promoteTimeout` (30 s) plus a progress write
(budget 10 s), so the TTL is more than twice a step.
  - A live holder is therefore never taken over, and a dead one is
retried within 90 s plus a minute.
  - `MaxOutputAttempts` is 5.
- Exactly-once rules.
  - The move happens exactly once (CAS), and the mark is written with
it.
  - `create:`: when `txChannelsFor` joins the channel store into the
entity transaction (sqlite or postgres with a channel store on the same
database), each create commits with its progress write, so creates happen
exactly once.
  - Without a join (the file store, or a channel store on another
database), a create whose claimed attempt may have written (no progress, and not
`WroteNothing`) is not retried. It fails `unconfirmed`, so a create is never
doubled and never silently dropped.
  - `set:` runs at least once with identical values. A retry after a
lost progress write patches the same values again, and update automations see
it.
- Recompute on retry. The outputs come from the current config, found
by the mark's `From`, `To` and the cause message's type. If the config no longer
declares that transition, the mark fails with `transition_removed`. Changed
outputs run as now declared, which is documented. The cause message keeps its
props: its author may not edit or delete it (`sessionMessage`, slice 1), and a
moderator removal while its notice has a pending mark answers 409
`outputs_pending`.
- Automations and validation run, because outputs go through the
manager. A validation refusal (422) fails the output with the property names.
Warnings (200 with warnings) complete it with the warning codes. A follow-up
failure completes it with `promote_followup_failed`. An output that changes the
anchor can fire an entity trigger that posts into the session. The trigger
cooldown (one post per rule and entity per minute) bounds such a loop, and the
docs warn about it.

7. **`on_final: close`.**
   - This is a session-level key: a move into any final state closes the
channel.
   - The close is the last step of the mark (the `Close` flag), so a retry
finishes it after a crash, and it happens once per move.
   - A reopened channel in a final state is not closed again, because its
mark is done.
   - The close goes through `applyTransition(open -> closed)` with
`systemAuthor()` and `TriggeredBy: "session:<type>/on_final"`. That writes the
lifecycle audit record and the system message, as `auto.after` does. A channel
that is already closed returns `ErrConflict`, which is ignored.
   - A failed output does not block the close. The conversation is over,
and the failure stays visible on the notice.
   - The close needs no declared lifecycle `close` operation. Reopening
still needs a declared `reopen`.

8. **Wire and UI exposure.**
   - Notices get `MessageView.Outputs *OutputsView`:
     - Fields: `{status: pending|done|failed, writer: AuthorView, results:
[{kind, status, code, props, entity: RefValue, warnings}]}`.
     - The status is failed when any result failed.
     - Role claims are not rendered.
     - The created entity is a `RefValue`, so a reader who cannot read it
sees `restricted: true`, as for promote.
     - This block replaces the notice's `ActionView` for `outputs`, and
notices still offer no `_actions`.
   - `SessionTransitionView.Outputs int` (`outputs,omitempty`): the number
of outputs. This is config, so a UI can say "this updates the breach".
   - `apiwire/v1` gains `ChannelMessageOutputs` and
`ChannelMessageOutputResult`, and `dataentry/channels_wire.go` maps them. The
HTTP post answer stays 201 with the message (slice 1 decision). The notice and
its status arrive through the page and SSE. A refused pre-check is the 403 body.
   - `frontend/src/types/channels.ts`: mirror types and
`CHANNEL_WIRE_FIELDS` only.
   - `internal/openapi/channels.go`: the schemas, a sentence on the
outputs mark in the notice description, and the `session_output_denied` and
`outputs_pending` problem types.
   - MCP: `messageResult.Outputs` for `post_message`. Notices in
`read_channel` carry it, and `watch` wakes on mark writes because they raise
rev.
   - CLI: notice rendering and the `rela channel post` line (AC 18).

**Alternatives rejected:**
- Outputs on state entry (the research sketch). References cannot be
typed, and the outputs would run for causes the operator did not consider.
- Trigger-fired outputs written as `system:realtime`. That is rela's own
worker, it has no grant check, and it fails `forbidden` whenever acl.yaml
exists. 5b gives them `run_as`.
- Running outputs inside the channel step, or the move inside an entity
`Tx`. That puts slow work under a lock, is impossible on the file tier, and
breaks the "no slow I/O inside `Tx`" rule.
- Committing the move and recording a failed output when the poster lacks
the grant. Any poster could then move a session whose outputs they may not
write, and leave a predictable failure behind.
- A grants-only `CheckUpdate` probe. It ignores local roles and
conditions, so it would refuse posters whose grant comes from a graph-property
role. It would also fail the read-gate guard test.
- Refusing the post when validation would fail. Validation needs the write
itself, and a dry run would duplicate the manager pipeline.
- Generation-wide supersede. It drops writes of unrelated properties.
- A separate outputs table or columns (sqlite v27, postgres 0034). The
action fields and the `pending_at` index already store and order pending marks.
- Storing the resolved writes in the mark instead of recomputing them. The
props come from the cause message, which cannot change while the mark is
pending, and `now` is the notice time, so recomputing gives the same values from
less stored data.
- Retrying creates without a transaction join. A crash between the entity
write and the progress write would create duplicates without any sign.
- Retrying without role claims. Retries would fail for every SSO user.
Storing scopes as well was rejected because a retry must not be wider than the
request.
- A moderator "release" or "abandon" for outputs marks. Release would erase
the outbox and the close. Abandon can come with the retry button in a later
slice.
- Reusing `automation.Interpolate`; see decision 3.
- A new `Store.SetOutputs` method. filechannels is at the plimsoll cap, and
`SetAction` with the outputs rule covers it.

**Deferred to 5b:**
- `session.run_as` (a `system:` or `integration:` name). realtimecfg
checks the prefix itself, because `principal.ValidateRunAs` accepts plain names.
It refuses `system:realtime` and other rela identities, and warns when `run_as`
is unused.
- Outputs on `after.<name>` timer transitions and on trigger-reachable
transitions, written as `run_as`. Timer outputs refuse `{{message.*}}` and
`{{channel.title}}`.
- The load-time grant check. It needs a new `acl.Policy.PrincipalGrants`
for create, update and delete, plus a separate read helper over `RoleDef.Read`
and worlds, because `acl.Op` has no read verb. It must count the built-in
`everyone` role. It must document that a run_as whose only grant is a local role
is refused at load. No `ceilingguard_test.go` edit is needed, because
`policy.go` is exempt as a file.
- What an acl.yaml reload that drops a run_as grant does. Either accept
that the whole realtime config degrades and document it, or treat it as a
per-type problem whose timer outputs fail `forbidden`.
- Timer marks created unclaimed by `SetSession` (the `Mark` callback on
the timer step), the runner woken by `TimerReport.Marks`, and the timer writer
rule. The writer comes from the current config's `run_as`, or the mark fails
`run_as_changed`; the stored `Writer` is for display only.
- testdata: `system:breach-bot` with a `breach-bot` role in
`testdata/acl.yaml`, and the `notify-deadline` output `escalated: true` in the
example.

**Files to modify:**
- `internal/realtimecfg`: `raw.go`, `keys.go` (refusals), new `outputs.go`
and `template.go`, `session.go` (transition outputs, `OnFinal`, the
trigger-reachable check), `validate.go` (`promote` shared with `create:`, the
reserved action name), `testdata/valid.yaml`, `testdata/schema.yaml` (+
`outputs_test.go`, `template_test.go`, `load_test.go`, `compiled_test.go`,
`reload_test.go`).
- `internal/realtimelimits/limits.go`: `MaxTransitionOutputs`,
`MaxOutputTemplateBytes`, `OutputClaimTTL`, `MaxOutputAttempts`,
`MaxOutputRunsPerRound`, `OutputRetryWindow`.
- `internal/channels`: `channels.go` (`Action.Outputs`, `OutputMark`,
`OutputResult`, `PrepareAction`, `PrepareMessage`), `session.go`
(`SessionStep.Mark`, `SessionOutcome.Notice`, notice builder), new `outputs.go`
(`CheckOutputStep`), `store.go` (contracts), `channelstest/outputs.go`.
- `internal/channels/sqlchannels/sql.go`,
`internal/channels/filechannels/ops.go` (notice with pending action, `SetAction`
rule, dropping dangling refs).
- `internal/realtime`: new `outputs.go` and `outputs_runner.go` (+ tests),
`session.go` (pre-check answers in the decider, views), `service.go` (inline
run, `PostResult.Outputs`), `actions.go` (`releaseGateCheck`), `sweep.go` (skip
`outputs`), `msgops.go` (409 `outputs_pending`), `render.go`, `errors.go`.
- `internal/entitymanager/linked.go` (`CheckRelate`),
`readgateguard_test.go` (`noReadGateNeeded` entry) (+ test).
- `internal/appbuild`: new `realtime_outputs.go` (`managerOutputs`,
`patchDenial`, wiring), `realtime_worker.go` (runner on the leader, the sweep
warning) (+ `realtime_outputs_test.go`, `realtime_outputs_db_test.go`).
- `internal/apiwire/v1/channels.go`; `internal/dataentry/channels_wire.go`
(+ golden files, handler and budget tests).
- `internal/openapi/channels.go` (+ test).
- `frontend/src/types/channels.ts` (+ `wireFields.test.ts`).
- `internal/mcp/tools_channels.go` (+ test).
- `internal/cli/channel_render.go`, `internal/cli/channel.go` (+ test).
- `docs-project/entities/guides/GUIDE-realtime.md`, `GUIDE-mcp-server.md`,
the audit-log guide; `docs/*.md` by `just docs`.

## Security Considerations

- [x] Input sources identified (user input, config, external APIs)
- [x] Input validation approach defined (allowlist preferred over blocklist)
- [x] Security-sensitive operations identified (file access, auth, crypto)
- [x] Error handling doesn't leak sensitive information

**Input Sources & Validation:**
- `realtime.yaml` (operator config, not secret). Outputs, templates and
`on_final` are checked at load against the metamodel. References come from a
fixed list, template size is capped, and outputs per transition are capped.
Outputs that would write as anyone but the poster are refused.
- Message props (poster input) are already validated by `validateProps`.
A template copies them only into properties whose type they can fill, in a
single pass with no rescan.
- The mark is written only by the store's session step and by `SetAction`
under `CheckOutputStep`. No request can create, claim, release or edit an
`outputs` action, and the writer and role claims cannot change after the move.

**Security-Sensitive Operations:**
- Escalation through outputs. Every output writes as the poster through
the manager. The row ACL, the read gate, the client ceiling, local roles and
validation all apply, so a poster cannot write a row they could not write over
HTTP.
- Literal config values are not field-gated, as recipe literals and
automation values are not. That is the operator's choice. The docs say that a
transition with a literal `set:` lets any poster who can fire it and update the
row set that field to that value. Interpolated values are authored and are
field-gated.
- Trigger posts cannot fire outputs in 5a (load refusal), so
`system:realtime` never writes the graph for a session.
- Retry principal. It is rebuilt from the stored author and verified role
claims, without scopes, through `ResolveDirect`, and only within 30 minutes. A
role revoked at the identity provider within that window still counts for a
retry. This is the accepted cost of a retry that works for SSO users, and the
docs name it.
- Field visibility. Outputs read no entity values, so they cannot copy a
hidden field or act as an oracle. `PatchEntity` preserves properties the writer
cannot see. Results carry codes and property names (config), never error text or
ids. A created entity renders as a `RefValue` gated per reader.
- The pre-check uses `AuthorizeUpdate`, which runs the read gate first. A
poster who cannot read the anchor gets the same answer the post's own anchor
checks already give, so the check adds no existence signal. The 403 names the
output key and ACL rule, which are config.
- acl-ceiling: no runtime deny is added. `CheckRelate` asks the existing
relation ACL.
- configedit: `realtime.yaml` is not a Configure file, so nothing changes.
If it ever becomes editable there, `outputs` and `on_final` must be locked and
`protective`, because they decide what writes the graph.
- jobs: outputs are graph writes, not external side effects. The manager's
automations enqueue their own jobs after commit.
- No Lua runs on the read path, and templates are not Lua.
- Denial of service: at most 4 outputs per move, 16 runner attempts per
round, 5 attempts per mark, a 30-minute retry window, and background work on the
leader only.
- Errors: config problems name keys and values. Runner logs name the
channel key and notice seq only.

## Test Plan

- [x] Test scenarios documented for each acceptance criterion
- [x] Edge cases identified and documented
- [x] Negative test cases defined (invalid input, error conditions)
- [x] Integration test approach defined (not just unit tests)

**Test Scenarios:** each acceptance criterion names its test or table row. The
layers are:
- realtimecfg unit tests (AC 1 to 6);
- store conformance on four backends (AC 7, 9), with postgres gated on
`RELA_TEST_DATABASE_URL` and run with `just test-postgres`, plus the file-tier
crash test (AC 8);
- service tests over the file store on MemFS with the test clock and a
fake writer (AC 10);
- runner tests with swapped snapshots and two runners (AC 11);
- the real writer over the entity manager on fs, sqlite and postgres
(AC 12, 13);
- HTTP golden, frontend mirror, OpenAPI, budgets, MCP and CLI (AC 14 to
18);
- docs (AC 19).

The race detector is on. `testOutputsClaimCAS`, `TestRunner_LeaderOverlap`,
`TestOutputs_PreCheckRace` and `TestOutputs_DisjointSetsBothWritten` exercise
the concurrency.

**Edge Cases (each has a test):**
- An optional message property is absent: the whole value is left out, and
a mixed template gets the empty string (AC 6).
- Faced, unfaced and mixed `attach_to` with `set.face` (AC 2, AC 13).
- A move into a final state without outputs, with `on_final` (AC 10).
- Reopen after an `on_final` close (AC 10).
- Degraded snapshot at the post (AC 10) and at a retry (AC 11).
- A crash after the move, after a create and after a set (AC 11, AC 12).
- A crash between meta.yaml and the log on the file tier (AC 8, AC 11).
- A slow but live holder (AC 11).
- Disjoint and overlapping sets out of order (AC 10).
- The transition removed by a reload (AC 11).
- A moderator removal of the cause, or a release of the mark, while
pending (AC 10).
- Mark writes on a closed channel (AC 7).
- A created entity the reader cannot read (AC 14).
- Not covered by a test: a cascade loop through a trigger, beyond the
existing cooldown tests (documented).

**Negative Tests:**
- AC 2 to 4: every load rule, including the 5b refusals.
- AC 7: forged marks, a wrong claim, a changed writer, an oversized mark.
- AC 10: the pre-check 403s (update, create, relation, degraded), the race
backstop, a validation failure, the stop after a failure, and the 409s on
removal and release.
- AC 11: no takeover of a live holder, no create retry without a join,
the attempt cap, the retry window, a removed transition, dangling refs.
- AC 13: the refusal codes leak no text or ids.

## Risk Assessment

- [x] Technical risks assessed with mitigations
- [x] Security risks assessed (see Security Considerations)
- [x] Effort estimated (xs/s/m/l/xl)

**Risks:**
- Overloading the action mark. Ordinary action code (purge reports,
release, `_actions`) could treat an `outputs` mark as a person's action.
Mitigation: the name is reserved, `CheckOutputStep` owns the pending-to-pending
rule, release and the sweep skip `outputs`, and notices offer no `_actions` (AC
7, AC 10, AC 14).
- Reusing `PendingAt` as "eligible at" changes its meaning for one action
name. Mitigation: godoc on `Action.PendingAt` and `OutputMark`; release and the
sweep skip `outputs` (AC 10).
- Cross-process supersede window. Mitigation: per-property supersede under
the in-process lock; the remaining window is documented.
- A `set:` repeated after a lost progress write re-fires update
automations. Mitigation: identical values and a short window; documented.
- Stored role claims outlive an IdP revocation for up to 30 minutes.
Mitigation: the window is bounded and documented, and scopes are not stored.
- Runner cost on the file tier: `StalePending` walks every `meta.yaml`.
Mitigation: once a minute, as the sweep already does (AC 16).
- 5b depends on the mark format of 5a. Mitigation: `OutputMark` already
has the fields 5b needs (an unclaimed attempt 0, `Writer`), and
`CheckOutputStep` allows a claim from attempt 0.
- Plimsoll caps (`filechannels.Store`, `Manager`). Mitigation: no new
methods, only package functions; `just plimsoll` is on the implementation
checklist.

**Effort:** l

## Documentation Planning

- [x] User-facing docs identified (skip if internal refactor)
- [x] Docs-checklist will be created when entering implementation

**Documentation Impact:** This is an enhancement. Edits go into
`docs-project/entities/guides/GUIDE-realtime.md`, and `just docs` regenerates
`docs/realtime.md`.
- "Sessions": a new "Outputs" part after "Reminders" that covers:
  - the example;
  - the `set:` and `create:` keys and the target face rule;
  - the template references, and the UTC `{{now}}` and `{{today}}`, which
differ from automations;
  - who outputs write as, and how a retry rebuilds the poster;
  - why timer and trigger outputs are refused for now, and how `roles:`
keeps triggers out;
  - the pre-check 403, and what fails after the move instead;
  - literal values versus field grants;
  - order, stop on failure, and per-property `superseded`;
  - retries, the claim TTL, the attempt cap and the retry window;
  - exactly once, at least once and `unconfirmed`, by store join;
  - automations, validation and warnings, and the trigger loop warning;
  - `on_final: close` and reopen;
  - the outputs block on notices, and the 409s on removing a cause or
releasing a mark.
- "Checks when the file loads": the output, template and `on_final` rules
and the refused keys.
- "Limits": the output limits.
- "HTTP API" and "Agents and other processes": `outputs` on notices and
transitions, `session_output_denied` and `outputs_pending`.
- "Reloading": changed or removed outputs.
- The audit-log guide: `triggered_by` `session:<key>/<from>-><to>#<gen>/<i>`
and tool `realtime`.

`GUIDE-mcp-server.md`: the `post_message` and `read_channel` rows mention
outputs. Code: godoc on `OutputMark`, `CheckOutputStep`, the `SetAction` and
`SessionStep` contracts, `OutputWriter`, `realtime.Outputs` (the retry and
exactly-once rules), `Template` and `entitymanager.CheckRelate`.

## Design Review

- [x] Run `/design-review` before starting implementation
- [x] All critical/significant findings addressed in plan

**Design Review Findings:** the 20 findings of `/design-review`
(`.ignored/dr-98tr9a.tsv`) are recorded as review-response entities linked to
TKT-98TR9A via `has-review-response`, with the resolutions in
`.ignored/dr-98tr9a-res.txt`. All five significant findings are addressed in
this plan. Finding 1 is addressed by a load refusal in 5a, and its writer for
trigger outputs moves to 5b. The findings that only 5b needs are listed under
"Deferred to 5b".

---
id: PLAN-ZU5X29
type: planning-checklist
title: 'Planning: Sessions 4/5b: timer and trigger outputs (run_as)'
started: "2026-10-26"
completed: "2026-10-26"
status: done
---

<!-- @managed: claude-workflow v1 -->

## Understanding

- [x] Problem/requirements clearly understood
- [x] Scope defined (what's in/out documented below)
- [x] Acceptance criteria documented with specific test scenarios

**Problem:** Slice 4/5a lets a message-driven session transition write to the
graph (`set:` and `create:` outputs) as the person who posted the message. It
refuses three things because they have no person to write as: outputs on
`after:` timers, outputs on `by: trigger` transitions, and outputs on
transitions a trigger can fire (`realtimecfg.runAsLater`, "TKT-RAN1YA"). The
research (RES-O8I8S7, decision 3, option f2) chose a per-session-type `run_as`
principal that `acl.yaml` grants, as the scheduler does (DEC-O59WM4). A missing
grant must fail loudly at load. This slice adds that principal and lifts the
three refusals. It also closes two 5a gaps: a timer move into a final state does
not close the channel under `on_final: close`, and a trigger move carries only a
close mark, never outputs.

Terms used below:
- **run_as**: `channel_types.<t>.session.run_as`, the identity that writes
the outputs of timer and trigger moves of that session type. It is a job
identity: `system:<name>` or `integration:<name>`. It is not a person and has no
user entity.
- **Person move**: a move fired by a message a person (or agent) posted.
Its outputs write as the poster, unchanged from 5a.
- **Job move**: a move fired by a timer, or by a message a trigger posted
(`system:realtime`). Its outputs write as run_as.
- **Mark**: the `outputs` action on a session notice
(`channels.OutputMark`). The outputs runner claims and runs it.
- **Job mark**: a mark written for a job move. It carries the immutable flag
`Job: true`, set when the mark is created.
- **Job-move output**: an output on a timer, on a `by: trigger` transition,
or on a transition a trigger can reach (no `roles`, no `by`).
- **Grant check**: the load-time check that run_as holds every grant its
job-move outputs need, computed from `acl.yaml` alone.

**Scope:**
- In: `session.run_as` in `realtime.yaml`: parsing, the job-identity check,
the refusal of rela's own identities, "required when a job-move output exists",
and a warning when it is set but unused.
- In: `outputs:` on `after.<name>` timers, on `by: trigger` transitions and on
trigger-reachable transitions. Every job-move output refuses
`{{message.author}}` and `{{channel.title}}`; timer outputs also refuse
`{{message.<p>}}`.
- In: one acl function, `acl.JobIdentityGrants`, returning a narrow
`JobGrants` value for the grant check (verb on type and face, field write with
the union rule and options, relation write). It counts the `everyone` role and
ignores local roles and claims.
- In: strict loading everywhere. An `acl.yaml` change that removes a run_as
grant makes the realtime config fail to load the same way on every path. A
running server keeps the config it has until its next reload or boot, as today.
Writes always go through the real ACL, so a dropped grant fails `forbidden` at
write time.
- In: timer marks: `SessionStep.Mark` on a timer step, written by
`SetSession` in every backend, `TimerReport.Marks`, `TriggerReport.Marks`, the
worker waking the outputs runner on either, and the `run_as_changed` failure
code.
- In: `on_final: close` on timer moves into a final state; trigger moves carry
their transition's outputs.
- In: audit attribution of job-move writes: user = run_as, tool `realtime`,
`triggered_by` = the output origin.
- In: testdata (`system:breach-bot`, role `breach-bot`, `escalated` property)
and the docs example (`notify-deadline` sets `escalated: true`).
- In: the new code in OpenAPI, apiwire docs, the TS mirror, CLI rendering and
docs; the timer `outputs` count on `ChannelSessionTimer`.
- Out: per-transition `run_as` (`session.states.*.on.*.run_as` stays
refused, now with a reason that names `session.run_as`). One identity per
session type is the research decision.
- Out: a `{{session.timer}}` reference. The timer name is on the notice; no
example needs it in an entity property. It can be added later without a breaking
change.
- Out: local-role grants for run_as. A run_as has no user entity, so
`role_relations`, `inherit` and membership can never reach it. The grant check
ignores them and the docs say so.
- Out: client ceilings. A run_as has no `principal_type` and no scopes, so no
`client_baselines` entry applies. `ceilingguard_test.go` is not edited.
- Out: a read check and a delete verb in the grant check. `Policy.Validate`
already requires read coverage for every update grant
(`validateVerbReadCoverage`), and no output deletes.
- Out: per-type suspension on reload (design review: dropped; strict
everywhere instead).
- Out: UI changes beyond what the wire already renders (5a shows marks and
results generically).

**Example (the shape this slice accepts; also the docs example and the new
testdata):**

```yaml
channel_types:
  breach-response:
    session:
      label: GDPR breach response
      initial: detected
      on_final: close
      run_as: system:breach-bot
      states:
        contained:
          on:
            escalate-stalled: { to: overdue, when: "session.reminders['status-update'] >= 3" }
          after:
            notify-deadline:
              period: 72h
              since: session
              to: overdue
              label: 72 hours since detection
              outputs:
                - set: { properties: { escalated: true } }
```

`testdata/acl.yaml` gains:

```yaml
assignments:
  system:breach-bot: breach-bot
roles:
  breach-bot:
    read: [datalek]
    update: [datalek]
```

`testdata/schema.yaml` gives `datalek` a property `escalated: {type: boolean}`.

**Acceptance Criteria:**

1. `session.run_as: system:breach-bot` loads. `run_as` values that are not a
job identity are refused with one problem at `...session.run_as`: a plain name
(`alice`), `unknown`, an empty string, a malformed system name (`system:Breach
Bot`, `system:a::b`), and every rela identity (`system:realtime`,
`system:scheduler`, `system:automation`, `system:webhook:x`,
`system:subscription:x`). `integration:breach-bot` loads. Test:
`TestSessionRunAs_Validation` table in `internal/realtimecfg`.
2. A session with a job-move output and no `run_as` is refused at the first
such output, naming `session.run_as`. Test: table rows for a timer output, a
`by: trigger` output and a trigger-reachable output, each without run_as.
3. A `run_as` on a session with no job-move output loads with one warning
("session.run_as is not used: no timer or trigger move of this session has
outputs"). Test: `TestSessionRunAs_UnusedWarns` reads `Compiled.Warnings()`.
4. Timer outputs compile with the 5a rules and refuse `{{message.<p>}}`
("a timer move has no message"). Every job-move output (timer, `by: trigger`,
trigger-reachable) refuses `{{message.author}}` and `{{channel.title}}` with a
reason naming the run_as authority. Job-move outputs accept `session.from`,
`session.to`, `session.generation`, `channel.key`, `channel.type`, `now`,
`today` and literals. Test: `TestJobOutputs_Templates` table with a row per
reference per output kind.
5. `by: trigger` and trigger-reachable transitions accept outputs whose
templates read the trigger's message properties. A transition with `roles` keeps
the 5a rules (it is never a job move). `session.states.*.on.*.run_as` stays
refused with a reason naming `session.run_as`. Test:
`TestTriggerOutputs_Compile`, plus the keys test row.
6. Load refuses a run_as that lacks a grant its job-move outputs need, one
problem per missing grant at the output key, naming the identity, verb, type and
face:
   - a set on a family-anchored type: update on the named face (or the type
when unfaced) for every `attach_to` type;
   - a set on a face-anchored channel type: update on every face each
`attach_to` type declares, because the channel may anchor any of them;
   - a create: create on the type at the output's face;
   - field write for every set property (update verb) and every create
property (create verb), by the union rule;
   - an option grant for every literal enum value;
   - the relation write for `relate:`.
A grant through `everyone` counts; one only through a local role or an asserted
claim does not. With no `acl.yaml` (nil policy) the check is skipped. Test:
`TestRunAsGrantCheck` table in `internal/realtimecfg`, with rows for an unfaced
type, a face-anchored faced type, a family-anchored faced set, a faced create, a
fields-blocked role, an `everyone`-only grant, a local-role-only grant and an
asserted-only grant; `TestJobIdentityGrants` in `internal/acl`.
7. The grant check agrees with the request-time evaluation. Test:
`TestRunAsGrantCheck_AgreesWithRequest` in `internal/appbuild` builds, for each
row of a policy table, a bound request for the run_as principal and compares:
verb and face answers with `acl.Request` decisions, field and option answers
with `affordances.PolicyResolver.FieldVerdicts` (set) and `CreateFieldVerdicts`
(create) on an entity of the type and face, and the relation answer with
`AuthorizeRelation`. Rows without a `when:` must agree exactly; rows with a
`when:` are counted as granted at load (the documented floor).
8. A timer fire on a timer with outputs writes the notice with an unclaimed
mark (`Job` true, `Writer` = run_as, `Timer` = timer name, `Cause` 0) and
records it pending, in every channel backend. Test: a `channelstest.RunAll` case
`SetSessionWithMark` (filechannels disk and MemFS, sqlitechannels, pgchannels)
asserts the action, `StalePending` lists it and `CheckOutputStep` accepts a
claim and refuses a change of `Job` or `Timer`.
9. `TimerReport.Marks` and `TriggerReport.Marks` count the marks a round
wrote. The worker starts an outputs round right after a timer or trigger round
with `Marks > 0`, without waiting for the minute tick. Test:
`TestTimers_ReportsMarks`, `TestTriggers_ReportsMarks` in `internal/realtime`;
`TestRealtimeWorker_MarksWakeOutputs` in `internal/appbuild` with a fake clock
asserts one outputs round follows each.
10. The runner writes a timer mark's outputs as the current config's run_as:
the entity changes, the audit record has user `system:breach-bot`, tool
`realtime` and `triggered_by` = `session:<key>/<from>-><to>#<gen>/<i>`. Test:
`TestOutputs_TimerSetAsRunAs` (appbuild integration over memstore, real policy,
real audit sink).
11. When the current config's run_as differs from a job mark's stored
`Writer` (the operator changed `run_as` from `system:breach-bot` to
`system:breach-bot-2` between the move and the run), the mark fails with
`run_as_changed` for every output and writes nothing; the close still runs.
Test: `TestOutputs_RunAsChanged`.
12. A trigger post that fires a transition with outputs writes a job mark
with `Writer` = run_as. The runner writes the outputs as run_as. A trigger move
into a final state with no outputs keeps the 5a close-only mark. Test:
`TestOutputs_TriggerMoveWritesAsRunAs`, `TestOutputs_TriggerFinalCloseOnly`.
13. A person who posts on a trigger-reachable transition writes as the
poster. When the inline write fails with a retryable error, the runner's retry
also writes as the poster, never as run_as and never with `run_as_changed`.
Test: `TestOutputs_PersonOnTriggerReachableRetryWritesAsPoster`.
14. A timer move into a final state on a type with `on_final: close` gets a
close mark; the runner closes the channel after the outputs (or alone, when
there are none). Its result is kind `close`. Test:
`TestOutputs_TimerFinalCloses`.
15. Strict loading on every path: the same `realtime.yaml` with an ungranted
run_as is refused at boot, by `rela validate`, by `RealtimeConfig.Reload`
(previous snapshot stays), by the Configure validation, and by the schema/acl
hot-reload successor (5a degraded fallback keeps the previous config). Test:
`TestRunAsGrantCheck_StrictEverywhere` in `internal/appbuild` runs each loader
on the same files.
16. A running worker whose `acl.yaml` lost the run_as grant (no reload yet)
fails the job mark's outputs with `forbidden` from the write path. Test:
`TestOutputs_RunAsWriteReauthorized`: the policy handed to the writer lacks the
grant; a second row has a field `when:` false at write time and yields
`field_denied`.
17. The runner loads the config snapshot once per mark. Test:
`TestOutputs_OneSnapshotPerMark` with a counting `RealtimeSnapshot` source
asserts one load per `runOne`, and a reload between claim and write does not mix
the old outputs with the new run_as.
18. The wire carries the new code `run_as_changed` (OpenAPI enum, TS mirror,
apiwire doc) and `ChannelSessionTimer.outputs`. The CLI renders `set DL-1
(run_as_changed)`. Test: `TestOpenAPI_OutputResultCodes`,
`channel_render_test.go` row, frontend type check, and the head answer test for
the timer count.
19. The testdata and the docs example load clean with the policy:
`TestParse_ValidTestdata` asserts no problems and no warnings, and the docs
example test (`TestDocsExample_Realtime`) parses the same YAML.
20. Budget: a timer round adds no store reads per mark to the timer path,
and the outputs run for a timer mark reads the notice in the same single
`Messages` page a 5a mark uses. Test: `TestOutputs_TimerMarkReadBudget` with a
counting store.

## Research

- [x] For larger features: run `/research` to create a structured research doc
- [x] ~~Searched for existing libraries that solve this problem~~ (N/A: covered by the research doc; the feature reuses internal ACL, principal and outbox code)
- [x] Checked codebase for similar patterns or reusable code
- [x] Looked for reference implementations in other projects
- [x] Reviewed relevant rela concepts for prior art

**Research Doc:** RES-O8I8S7 (`.ignored/res-sessions-final.md`). The user chose
"Per-type run_as" (option f2, decision 3): timer-driven graph writes use a
per-session-type `run_as` granted in `acl.yaml`; a missing grant fails at load.

**Existing Solutions:**
- Scheduler jobs: `run_as` in `schedules.yaml`, stamped as
`principal.Principal{User, Tool: ToolScheduler}` plus `audit.WithTriggeredBy`
(`internal/scheduler`, DEC-O59WM4). The run_as principal here follows the same
shape.
- Automation jobs: `internal/appbuild/automationjobs.go` binds a configured
user with `principal.With` and `acl.BindRequest`. Same binding path.
- CLI `--run-as`: `internal/cli/identity.go` `validateRunAs` and
`ownJobIdentity` already implement the job-identity check (prefix, system name
grammar, refuse rela identities). The shared part moves to
`principal.ValidJobIdentity` so the two cannot drift.
- ACL primitives: `grantsVerb`, `GrantsVerbOnState`,
`relationPermissionFor`, and the relation rule in
`Request.authorizeRelationWrite`. The field fold is the union rule in
`internal/affordances/dimension.go` (BUG-Y5UF3K), used by
`PolicyResolver.FieldVerdicts` and `CreateFieldVerdicts`.
- `writeChecker` in realtimecfg is the precedent for a checker type beside
`*validator`, which is at the plimsoll method line.
- Other projects: GitHub Actions `permissions:` on a workflow token and
Kubernetes service accounts both give automation a named, separately granted
identity rather than borrowing a person's or an all-powerful one.

## Approach

- [x] Technical approach chosen and documented
- [x] Approach builds on existing patterns (not reinventing)
- [x] Alternatives considered (document why rejected)
- [x] Dependencies identified (packages, APIs, types)

**Technical Approach:**

1. **`session.run_as` validation.** `rawSession` gains `RunAs string`
(`run_as`); `keys.go` drops the `session.run_as` refusal. A new
`principal.ValidJobIdentity(name string) error` holds the rule that
`internal/cli/identity.go` has today: the name starts with `system:` or
`integration:`; a `system:` name passes `ValidSystemName` and is not
`IsRelaIdentity`; then `ValidateRunAs(name, ownJobIdentity)`. The prefix check
is explicit because `ValidateRunAs` alone accepts plain names. The CLI calls the
new function and keeps its flag wording. `Session.RunAs()` returns the value.
With a job-move output and no run_as: one problem at the first such output. With
run_as and none: one `v.warn` at `session.run_as`.
`session.states.*.on.*.run_as` stays refused with the reason "run_as is set once
per session type, at session.run_as". `runAsFollowUp`, `runAsLater` and the
three refusals that use them are removed.

2. **Job-move templates.** `writeChecker.outputs` takes a `cause` argument:
`causePerson` (a transition with `roles`), `causeJobMessage` (`by: trigger`, or
no roles and no `by`, which `poster.admits` lets a trigger fire), and
`causeTimer`. `refKind` refuses for every job cause `{{message.author}}` (on a
job move it is always `system:realtime`, written with run_as authority) and
`{{channel.title}}` (free text from the channel's creator; reading it would let
a participant steer a value written with run_as authority; on a trigger target,
a cardinality-one channel, it is always empty anyway). For `causeTimer` it also
refuses `message.*`. The refusal comment says these are security rules, so
adding either reference later is judged as a security change. A person move on a
trigger-reachable transition is compiled with the job rules, because the same
outputs may run as a job.

3. **Timer outputs.** `rawSessionTimer` gains `Outputs []rawOutput`;
`keys.go` drops the `after.*.outputs` refusal. `SessionTimer` gains `outputs
[]Output` and `Outputs()`. `compileTimer` stays a free function; the outputs
compile in `validator.session` after the timers, through `writeChecker.outputs`
with `causeTimer` and a nil message type. The `mt == nil` early return becomes
"skip only when the cause is a message and its type is unusable". The create
check for an optional message property does not apply to a timer. A timer output
into a final state with `on_final: close` runs before the close, as for
messages.

4. **Trigger outputs.** `writeChecker.outputs` stops refusing `by: trigger`;
`validator.transition` stops refusing trigger-reachable outputs. Their templates
may read the trigger post's literal props.

5. **One acl function for job grants.** `*acl.Policy` is at 40 methods, so
nothing is added to it. In a new `internal/acl/jobgrants.go`:

   ```go
   // JobIdentityGrants answers what acl.yaml grants a job identity
   // (system: or integration:) through its assignment and `everyone`.
   func JobIdentityGrants(p *Policy, user string) JobGrants

   type JobGrants struct { /* roles: assignment + everyone */ }
   func (g JobGrants) Verb(op Op, typ string, face entity.Face) bool
   func (g JobGrants) FieldWrite(op Op, typ string, face entity.Face, field string) bool
   func (g JobGrants) Option(op Op, typ string, face entity.Face, field, value string) bool
   func (g JobGrants) Relates(relType, fromType string, fromFace entity.Face) bool
   ```

   - The roles are `p.Assignments[user]` plus `EveryoneRole`. Asserted roles
and local roles are left out: a job identity carries no claims and has no user
entity. The name and godoc limit the type to job identities: its answer is blind
to ceilings and local roles, so it must not look authoritative for a person.
   - `Verb` supports `OpCreate` and `OpUpdate` only. It uses
`GrantsVerbOnState(role, op, typ, face)`, so a state grant (`datalek@legal`)
counts and a zero face on an unfaced type works as a bare grant.
   - `FieldWrite` and `Option` follow the union rule of
`affordances/dimension.go`: allowed is the union over roles of what a role's
`fields:` (or `options:`) block grants for the type, or every declared field
(option) when the role has no block and holds `op` on the type and face. Closed
world applies only when some role opts in. A row with a `when:` counts as
granted (the floor; AC 16 pins the write-time re-check). The fold is not
imported from affordances (acl cannot import it); AC 7 pins agreement with
`FieldVerdicts` and `CreateFieldVerdicts`.
   - `Relates` checks the delegate-X permission first, then
`relation_grants` (with the update-tail rule for a faced source), then the
source-type create verb, as `authorizeRelationWrite` decides.
   - No read helper: `Policy.Validate` already refuses an update grant
without read coverage (`validateVerbReadCoverage`), so update implies read. No
delete: no output deletes.

It reads only `Policy`, never a `Request`, so `ceilingguard_test.go` and
`staticguard_test.go` need no change.

6. **Where the grant check runs.** A new `grantChecker` type in
`internal/realtimecfg/grants.go` (like `writeChecker`; `*validator` is at 40
methods) runs after compile, per session type with run_as, over its job-move
outputs only. Person-move outputs are re-checked per poster at write time, as in
5a. Faces:
   - set on a family-anchored channel type: the output's face (or zero face
for an unfaced type) on every `attach_to` type;
   - set on a face-anchored channel type: every face each `attach_to` type
declares (`metamodel.FaceOrderOf`), because the runner writes the channel's own
anchor face (`key.Face`) and a channel may anchor any;
   - create: the output's face (zero for an unfaced type).
Checks: `Verb(update)` and `FieldWrite(update)` for every set property (literals
included, as 5a gates every set property); `Verb(create)` and
`FieldWrite(create)` for every create property (templates such as
`{{session.to}}` are authored); `Option` for every literal enum value; `Relates`
for `relate:`. A templated enum value (a trigger message prop) is a literal at
the trigger, so it is checked against that literal; a timer has no enum source
other than literals. Each miss is one problem: "system:breach-bot cannot update
datalek@legal: no role assigned to it or to everyone grants it". With nil policy
the step is skipped.

7. **Reload behaviour: strict everywhere.** No `Parse` option, no
suspension. The grant check is part of `Parse`, so every loader refuses the same
files: boot, `rela validate`, `RealtimeConfig.Reload` (keeps the previous
snapshot), Configure validation, and the schema/acl hot-reload successor on MCP
(keeps the previous config degraded, the 5a fallback). A running server keeps
its loaded config until it reloads or restarts, as today. In that window the
write path re-authorizes each output against the live policy, so a dropped grant
fails `forbidden` (AC 16). The docs say that after removing a run_as grant the
next boot fails and names the output key, and the fix is to restore the grant or
remove the outputs.

8. **Job marks: classification and the writer rule.** `OutputMark` gains
`Job bool` and `Timer string`, both immutable (added to `CheckOutputStep`'s
list). `Job` is set at creation: true for timer marks and for marks
`systemWriter.post` writes on a trigger move, false for person marks. The runner
and the writer branch on `mark.Job` only; they never infer it from the
transition shape or the `Writer` prefix (a person may post as `integration:` or
`system:` through `--run-as` or MCP).
   - Job mark: the writer is the current config's `session.RunAs()`. If it
is empty or differs from `mark.Writer.User`, every output fails `run_as_changed`
(new `OutputCodeRunAsChanged`) and the close still runs. The stored `Writer` is
for display and this comparison only.
   - Person mark: 5a unchanged (retry with the stored identity, verified
binding, `userExists`).
   - `managerOutputs.as` checks `RunAs` before `Retry`, so a job mark's retry
takes the run_as branch.
   - `prepareOutputMark` requires exactly one of `Cause > 0` and
`Timer != ""`, and `Timer != ""` only with `Job`. A changed run_as with outputs
still present is the only way to reach `run_as_changed`: a type that loses
run_as but keeps job-move outputs does not load (AC 2), and without outputs the
mark is close-only.

9. **Timer marks.** In `internal/channels`: `PrepareTimerStep` accepts
`Mark`; `TimerNotice` attaches it through `timerMark(step, move, gen)`, a
sibling of `sessionMark` that fills Gen/From/To/Timer, sets `Job`, leaves Cause
10. `filechannels` `SetSession` calls `setPending` for the notice action, as
`moveSession` does; `sqlchannels` `SetSession` writes the action and
`pending_at`, as its append path does. In `internal/realtime`: `timerStep` sets
`Mark` when the timer has outputs or the move enters a final state under
`on_final: close`; the mark is unclaimed (Attempt 0, PendingAt = notice time),
`Writer` = `Principal{User: run_as, Tool: realtime}`, or `system:realtime` for a
close-only mark. `TimerReport.Marks` counts the outcome notices with a mark.
`outputEngine.load` accepts a notice with `mark.Timer != ""` from the same
`Messages` page; `r.cause` stays zero and `vars` leaves Message and Author
empty.

11. **Trigger marks.** `systemWriter.post` builds the mark from the fired
transition: with outputs, `Job` true and `Writer` = run_as; with none and a
final state under `on_final`, the 5a close-only mark. `outputsFor` drops the
"Writer is system:realtime means no outputs" shortcut. A close is decided by the
transition having no outputs. `TriggerReport.Marks` counts the posts whose move
wrote a mark.

12. **One config snapshot per mark.** `Outputs.runOne` loads the snapshot
once and resolves into the `markRun`: the transition or timer (by `(mark.From,
cause.Type)` or `(mark.From, mark.Timer)`, with `To == mark.To`, else
`transition_removed`), its outputs, `readsTitle`, the session's run_as and
`on_final`. `run` and `outputsFor` read only the `markRun`. This follows
"capture state once per operation".

13. **Waking the runner.** `realtime_worker.go` calls `startOutputs` after a
timer round or a trigger round with `Marks > 0`. `startOutputs` keeps its
single-flight guard, so a busy round is not doubled; the minute tick picks up
anything left.

14. **The run_as identity on the write (system principal identity).**
`OutputRequest` gains `RunAs string`. For `RunAs != ""`, `managerOutputs.as`
builds `principal.Principal{User: runAs, Tool: principal.ToolRealtime}`, stamps
it with `principal.With`, and binds it with `o.bind` (`acl.BindRequest`), the
automation-job path. No `VerifiedFrom`, no claims, no user-entity lookup. The
ACL resolves the name through `assignments` like any user. `system:` grants
nothing by itself: allow-all is never inferred from identity, and this path
never takes the `AllowAllReader` capability. The read side of a write (set
diffing, the create join) uses the gated manager as for a person. The leader's
`system:realtime` context is replaced for the write, not nested.

15. **Audit.** Graph audit records of job-move writes carry user = run_as,
tool `realtime`, `triggered_by` = `outputOrigin`
(`session:<key>/<from>-><to>#<gen>/<i>`). The channel audit for the timer move
stays `system:realtime` with `timer:<type>/<state>/<name>` (5a). The notice's
`OutputsView.writer` shows run_as.

16. **Wire, OpenAPI, TS, MCP, CLI.**
    - `run_as_changed` added to `outputResultSchema.code` (OpenAPI), the
`OutputResult.Code` doc in `internal/channels/outputs.go`, the apiwire doc, and
`frontend/src/types/channels.ts`.
    - `ChannelSessionTimer` gains `Outputs int` (`outputs,omitempty`), the
timer twin of `ChannelSessionTransition.Outputs`; OpenAPI and TS follow.
    - `Job` and `Timer` stay off the wire: the notice already carries the
timer name, and the writer shows who wrote.
    - MCP: no new tool. The `read_channel` docs row mentions the new code.
    - CLI: `outputsText` already prints any code; one render test row.
    - The schema endpoint does not expose `run_as`. It is config and not a
secret, but no client needs it; it is visible on each mark's writer.

17. **Testdata and docs example.** As in the Example: `run_as:
system:breach-bot`, `notify-deadline` output `escalated: true` in both
`contained` and `assessed`, `datalek.escalated: boolean`, and the `breach-bot`
role and assignment in `testdata/acl.yaml`. `datalek` is unfaced, so the grant
check needs only `update: [datalek]`. `docs/realtime.md` (generated from
`GUIDE-realtime.md`) shows the same YAML.

**Alternatives rejected:**
- Writing job moves as `system:realtime`: makes a rela identity hold graph
grants, and every session type would share them. The research rejected it.
- A run_as per transition: more surface and more grant checks for no use case
the research found.
- Per-type suspension on an `acl.yaml` reload: its only caller is the MCP
successor, which runs no worker, and it would let a refused `realtime.yaml` edit
in at the next schema save. Strict loading plus write-time re-authorization
covers the same risk.
- Degrading the whole realtime config at runtime on an `acl.yaml` drop: a
running worker host never reassembles on an `acl.yaml` change, and the write
path already denies.
- Classifying job marks from the transition shape or the `Writer` prefix: a
person mark on a trigger-reachable transition, or a person posting as
`integration:`, would be misread. An immutable flag cannot drift.
- Using the stored `Writer` for a job mark: an old mark would write as an
identity the operator replaced.
- Four `*Policy` methods: break the plimsoll 40-method line and look usable
for people.
- Counting `when:` field rows as not granted at load: refuses valid configs
whose condition holds for the entities involved. The write re-check (AC 16) is
the real gate.
- `affordances.WithStaticGrants` for the check: allowlisted to offline
analysis by `staticguard_test.go`, and it works per request, not per policy.

**Files to modify:**
- `internal/principal/principal.go` (+ test): `ValidJobIdentity`.
- `internal/cli/identity.go`: call `ValidJobIdentity`.
- `internal/acl/jobgrants.go` (new, + test): `JobIdentityGrants`,
`JobGrants`.
- `internal/realtimecfg/raw.go`, `keys.go`, `session.go`, `outputs.go`,
`template.go`, `validate.go`, `grants.go` (new: `grantChecker`), tests and
`testdata/{valid,acl,schema}.yaml`.
- `internal/channels/outputs.go`, `session.go` (+ tests); `channelstest`
(new case); `filechannels/ops.go`; `sqlchannels/sql.go`.
- `internal/realtime/outputs.go`, `outputs_runner.go`, `timers.go`,
`trigger.go`, `system.go`, `session.go` (timer outputs count) (+ tests).
- `internal/appbuild/realtime_outputs.go` (run_as branch first),
`realtime_worker.go` (wake), agreement and strictness tests.
- `internal/apiwire/v1/channels.go`, `internal/openapi/channels.go`,
`frontend/src/types/channels.ts`.
- `internal/cli/channel_render_test.go`.
- `docs-project/entities/guides/GUIDE-realtime.md` (then `just docs`),
`GUIDE-mcp-server.md` (`read_channel` row), `docs/acl-security.md` (run_as
section).

## Security Considerations

- [x] Input sources identified (user input, config, external APIs)
- [x] Input validation approach defined (allowlist preferred over blocklist)
- [x] Security-sensitive operations identified (file access, auth, crypto)
- [x] Error handling doesn't leak sensitive information

**Input Sources & Validation:**
- `realtime.yaml` `session.run_as`: operator config. Allowlist grammar
(`system:` segments of lowercase ASCII, digits, `_`, `-`, or an `integration:`
name), then a denylist of rela identities on top. The grammar check comes first
so a look-alike of a rela name cannot pass the registry lookup (the CLI rule).
- `acl.yaml`: operator config; the grant check reads it and never writes it.
- Job-move templates: config literals, session and channel key/type values,
and (for trigger moves) the trigger post's literal props. No message value from
a person, no author, no channel title, no entity value reaches a job write, so a
participant cannot steer a value written with run_as authority and a job write
cannot copy a hidden field.
- Causation: anyone who may post the message that moves a session into a
timed state, create a cardinality-many channel, or edit the entity a trigger
watches can cause a run_as write they could not make themselves. The write's
values are config-fixed, the target is the channel's anchor or a configured
type, and run_as holds only the grants the operator gave it.
`docs/acl-security.md` states this so operators grant run_as the least needed.

**Security-Sensitive Operations:**
- A job identity writes to the graph. It holds exactly the grants
`acl.yaml` assigns to its name or to `everyone`; `system:` confers nothing. It
cannot be a person (prefix rule), cannot be a rela identity (registry rule),
cannot receive local roles (no user entity) and carries no claims. This keeps
the ACL rule "allow-all is never inferred from identity".
- Every write is re-authorized by entitymanager at write time against the
live policy; the load-time check only rejects configs that can never work.
- Job vs person marks are classified by an immutable flag, so a retry cannot
switch a person's write to run_as or the reverse.
- A run_as change cannot make old marks write as the new identity
(`run_as_changed`).
- Audit names run_as as the writer and the output origin, so every job write
is attributable to its session move.
- Error codes stay codes. Load problems name config keys, identities, verbs,
types and faces only (config is not secret); runtime results never quote an
error or an id.

## Test Plan

- [x] Test scenarios documented for each acceptance criterion
- [x] Edge cases identified and documented
- [x] Negative test cases defined (invalid input, error conditions)
- [x] Integration test approach defined (not just unit tests)

**Test Scenarios:** one test or table row per acceptance criterion, named in the
criterion. `realtimecfg` and `acl` tests are table-driven unit tests.
`channelstest` runs the mark case on every backend (filechannels disk and MemFS,
sqlitechannels, pgchannels through the schema-pinned DSN). The `appbuild` tests
are integration tests over memstore with a real `acl.Policy`, the affordances
resolver, the field-gated entity manager and a capturing audit sink, so the
run_as principal goes through the same bind and ACL path as production. The
worker test uses the fake clock already used by `realtime_worker_test.go`.

**Edge Cases (each has a test):**
- A timer whose move enters a final state with outputs and `on_final: close`:
outputs run, then the close (AC 14 row).
- A timer mark claimed and expired (`OutputClaimTTL`) is re-claimed and does
not double-create (5a join rule, timer origin).
- A supersede: a person move after a timer move that set the same property;
the retried timer set reports `superseded` (5a rule, timer mark).
- A set on a face-anchored channel type over a faced `attach_to` type: the
grant check requires update on every declared face; a role granting only
`datalek@legal` is refused naming the missing face (AC 6 row).
- A role without a `fields:` block that holds update (directly or through
`everyone`), plus a run_as role whose block omits the property: allowed by the
union rule (AC 6 and AC 7 rows).
- The same, but only the blocked role applies: refused (AC 6 and AC 7 rows).
- A create on a type where the run_as role has create and a `fields:` block
without the authored property: refused, agreeing with `CreateFieldVerdicts`.
- A literal enum value outside the role's `options:` grant: refused,
agreeing with `FieldVerdicts` options.
- A run_as listed under `assignments` and also matching an `asserted_roles`
claim name: only the assignment counts.
- A person posting as `integration:x` on a trigger-reachable transition: a
person mark (`Job` false); its retry writes as `integration:x` (AC 13 row).
- Two timers due in one round on two channels: `Marks == 2`, one outputs
round.
- The outputs round is already running when a timer or trigger round ends:
no second round; the minute tick finishes the work.
- An `integration:` run_as works the same as a `system:` one.

**Negative Tests:** AC 1 (every refused name), AC 2 (missing run_as), AC 4
(refused references per output kind), AC 6 (each missing grant kind,
local-role-only, asserted-only), AC 11 (`run_as_changed`), AC 15 (every loader
refuses), AC 16 (write-time denial despite load-time pass), AC 8
(`CheckOutputStep` refuses changing `Job` or `Timer`), and `prepareOutputMark`
refusing a mark with both or neither of `Cause` and `Timer`, or `Timer` without
`Job`.

## Risk Assessment

- [x] Technical risks assessed with mitigations
- [x] Security risks assessed (see Security Considerations)
- [x] Effort estimated (xs/s/m/l/xl)

**Risks:**
- The grant check can drift from the request-time ACL and the affordances
field fold. Mitigation: `JobGrants` composes the same acl primitives, and AC 7
compares it with `acl.Request`, `FieldVerdicts` and `CreateFieldVerdicts` over a
policy table.
- The field `when:` floor lets a config load that denies at write time.
Mitigation: documented, and the failure is a visible `field_denied` result on
the mark (AC 16).
- Strict loading means a run_as grant removed from `acl.yaml` breaks the
next boot. Mitigation: the problem names the output key, identity, verb, type
and face; `rela validate` reports it before a deploy; the docs say so.
- Changing `SetSession` touches four backends. Mitigation: the conformance
case in `channelstest`, and the pending bookkeeping reuses each backend's
append-path helper.
- A busy outputs runner delays job outputs to the next minute tick.
Acceptable; the retry window is 30 minutes.
- Security: a job identity with broad grants is a standing capability, and
participants can cause its writes. Mitigation: one per session type, explicit in
`acl.yaml`, config-fixed values, audited per write; `docs/acl-security.md`
advises least-privilege roles for run_as.

**Effort:** l

## Documentation Planning

- [x] User-facing docs identified (skip if internal refactor)
- [x] Docs-checklist will be created when entering implementation

**Documentation Impact:** Enhancement. `GUIDE-realtime.md` (generates
`docs/realtime.md` via `just docs`): `session.run_as`, timer and trigger
outputs, the reference restrictions on job-move outputs, the grant check and its
faces, strict loading and what a running server does after an `acl.yaml` change,
`run_as_changed`, `on_final` for timer moves, audit attribution, and the updated
example. `docs/acl-security.md`: a "Session run_as identities" section next to
"Field write grants on MCP and scheduled scripts" (which grants count, why local
roles and ceilings do not apply, the causation channel, why `{{channel.title}}`
and `{{message.author}}` are refused, least privilege). `GUIDE-mcp-server.md`:
the `read_channel` row names the new code. OpenAPI descriptions carry the code
and the timer `outputs` count.

## Design Review

- [x] Run `/design-review` before starting implementation
- [x] All critical/significant findings addressed in plan

**Design Review Findings:** the findings of `/design-review` are recorded as
review responses linked to this ticket. All 11 are folded into this plan: faces
in the grant check for face-anchored sets and faced creates (Approach 5, 6; AC
6); the union field rule, create fields and options with an agreement test
against `FieldVerdicts` and `CreateFieldVerdicts` (Approach 5; AC 7); job marks
classified by an immutable `Job` flag, with run_as taking precedence over retry
(Approach 8; AC 13); suspension dropped for strict loading everywhere (Approach
7; AC 15, 16); one acl function with a narrow type and a separate realtimecfg
checker type for plimsoll (Approach 5, 6); `{{message.author}}` and
`{{channel.title}}` refused on every job-move output, with the security reasons
documented (Approach 2; AC 4); one snapshot per mark (Approach 11; AC 17); no
read helper and no delete (Approach 5); AC 11 and AC 20 reworded; the runner
woken by trigger marks too (Approach 12; AC 9).

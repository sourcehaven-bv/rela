---
id: PLAN-HWROBZ6
type: planning-checklist
title: 'Planning: Flows M1f: predicate starts and change events from every write'
started: "2026-11-25"
completed: "2026-11-25"
status: done
---

<!-- @managed: claude-workflow v1 -->

## Understanding

- [x] Problem/requirements clearly understood
- [x] Scope defined (what's in/out documented below)
- [x] Acceptance criteria documented with specific test scenarios

**Scope:**

In: `change` events from every write that goes through
`entitymanager.updateCore` (data-entry PATCH, MCP, CLI `update`, Lua
`update_entity` incl. scheduled sync scripts, mapped import, CalDAV, ingest);
`on: predicate` starts on update and on create; durable once-only decisions;
loop bound; docs.

Out: `on: schedule` (TKT-2JOU3VK). A change on a related entity as an event for
the subject (documented trigger pattern instead). Writes that bypass the manager
(`rela import` of an export, `normalize`, seed), rename, conversion, restore,
copy, delete: no events, as for create starts.

**Acceptance Criteria:**
1. Update through the manager moves the subject's open run along the first
admitted `change` branch whose `when` holds on the post-write entity. Test:
appbuild cross-backend test, PATCH via manager as `hr` moves `collect ->
review`.
2. Author roles: a writer without the branch's role does not move it. Test: same
write as a user without `hr`.
3. No-op write (no property or body change) is no event. Test: re-patch same values.
4. A run's own writes (ctx carries `audit.FlowStep`) are not events. Test: a
`set` output that would satisfy a change branch does not move the run.
5. Run move commits with the write: a write that fails after the hook (rolls
back) leaves the run where it was. Test on postgres/sqlite: forced txCheck
failure.
6. A change while the token is settling (pending mark, e.g. a job step) is kept
on the mark and taken once the state's outputs and job are done, when no `done`
branch applies. Test: engine unit test with a pending run.
7. Burst: two writes open then merged in order take the branches in order.
Test: two patches, second moves from the state the first entered.
8. A forked run: each live branch token gets the event; a waiting `when:` join
is tested after commit. Test: fork test with a join `when` that the write
satisfies.
9. `on: predicate` starts a run on a false-to-true edge on update, and on a
create that matches. No run when it already matched before the write.
`per_subject: one` honoured. Tests: engine + appbuild.
10. Predicate starts caused by a flow's own writes are bounded by the create
depth (8). Test: chain bounded test.
11. A sync script (scheduled Lua, `run_as: integration:github`) write moves a
run on a branch with `roles: [integration]`; the timeline records `By`.

## Research

- [x] For larger features: run `/research` to create a structured research doc
- [x] Searched for existing libraries that solve this problem
- [x] Checked codebase for similar patterns or reusable code
- [x] Looked for reference implementations in other projects
- [x] Reviewed relevant rela concepts for prior art

**Research Doc:** N/A: the approach copies the create-start pattern
(TKT-I1OLJ3V); no open approach question.

**Existing Solutions:**
- `entitymanager.CreateHook` + `appbuild.flowCreateStarts` (in-Tx run start on a
joined run store, settle after commit). Same shape for updates.
- `flowengine.Engine.Change` (person step): branch decision with `flow.Decide`,
`Admits` by roles, `Holds` by guard. Reused through the same `flow.Decide`.
- `createDepth` / `maxCreateDepth` as loop bound.
- Transactional outbox pattern: the decision is stored with the write.

## Approach

- [x] Technical approach chosen and documented
- [x] Approach builds on existing patterns (not reinventing)
- [x] Alternatives considered (document why rejected)
- [x] Dependencies identified (packages, APIs, types)

**Technical Approach:**

1. `entitymanager.UpdateHook` (consumer-side, beside `CreateHook`):
`Watches(typ)`, `PrepareUpdate(ctx, subjectID) (ctx, error)` outside the Tx,
`EntityUpdated(ctx, view, old, new) (afterCommit, error)` inside the Tx after
the row and audit record. `updateCore` forces a Tx when watched, runs the hook
in `write`, owns a commit list when it opened the Tx and runs it after the
cascade. `Deps.UpdateHook`.
2. `appbuild.flowUpdateEvents` implements it. Prepare binds `system:flow`
(prebound, as create) and computes the author's role set on the subject
(`req.ForEntity`) before the Tx; no acl.yaml: every role held; no Request: fail
closed (no role held). In the Tx: joins the run store, computes depth
(`createDepth`), calls the engine, returns an after-commit that replays the
deferred header patches and settles the touched runs on a ctx of its own as
`system:flow` (30 s), as `settleStarted`.
3. `flowengine.OnChange` (Engine is at its method cap) `Changed(ctx, runs,
old, new, by, holds, depth) (ChangeOutcome, error)`:
   - no property/body difference: nothing.
   - predicate starts: per flow on the type with `on: predicate`, start when
no start held on `old` and one holds on `new` (`begin`, ErrSubjectBusy skips).
   - change events: skipped when ctx carries an `audit.FlowStep` (run's own
writes, steps, flow jobs). Else every open run on the subject (List, paged,
bounded by `MaxRunsPerSubject`), every live token: `flow.Decide` over the
state's change branches, roles by `holds`, guard on `new`. A move uses an engine
copy whose `Runs` is the joined store and whose `Headers` defers `PatchHeader`
to after commit (a failed statement aborts a postgres Tx). A token with a
pending mark stores the decision on the mark (`flowrun.Pending.Change{Branch,
By, At}`, first decision wins).
   - `StartCreated` also honours `on: predicate` (a created subject that
matches has come to match).
4. `settleStep`: after outputs and job, when no `done` branch applies and the
mark carries a deferred change for this generation, take that branch
(`ViaChange`, its `By`). `mark` keeps the field.
5. Settle after commit settles forked runs too, so a `when:` join is tested at once.

Alternatives rejected:
- Queue the event and decide later on the worker: loses intermediate states
(criterion 7) and loses events on a crash between write and enqueue.
- Call `Engine.Change` after commit: not atomic with the write; a crash loses
the event; reads a later subject state.
- Related-entity propagation in the engine: ACL/fan-out/loop questions; triggers cover it.

**Changes after design review (RR-NM1G2WT .. RR-ALOHB7G):**
- An ordinary write takes only `change` branches that have a `when:` reading a
property the write changed (`Guard.Fields`). Step completion is unchanged.
- Change events are skipped for machine writes (ctx carries `audit.FlowStep`
without a step-write marker). A person's step write (marked in
`FlowControl.complete`) skips only its own run.
- Header patches are written inside the transaction as `system:flow`, bound in
`PrepareUpdate` (only when the subject has open runs or the type has predicate
starts). No replay after commit.
- A token with a live claim by another principal gets no event (logged).
- After commit, every forked run on the subject with arrived branches is
settled (join tested), through a settle that does not skip forked runs.
- The in-Tx engine copy has a `Subjects` that errors; guards use `new`.
- Runs started in the same call get no event; only runs on `new.Face`.
- Without preparation, roles fail closed. The hook watches every flow subject type.

**Files to modify:**
internal/entitymanager/{update_hook.go(new),manager.go,create_hook.go};
internal/flowengine/{onchange.go(new),oncreate.go,step.go};
internal/flowrun/flowrun.go (Pending.Change, checks, copy);
internal/appbuild/{flowupdateevents.go(new),piles.go,appbuild.go}; tests in
flowengine, flowrun, appbuild; docs-project flows guide (generates
docs/flows.md).

## Security Considerations

- [x] Input sources identified (user input, config, external APIs)
- [x] Input validation approach defined (allowlist preferred over blocklist)
- [x] Security-sensitive operations identified (file access, auth, crypto)
- [x] Error handling doesn't leak sensitive information

**Input Sources & Validation:**
- The write itself: already authorized by the manager (row ACL, field rules,
`outside_edits`). A refused write never commits, so it moves nothing.
- The author: `principal.From(ctx)`, stamped at the request edge; never from the
payload. Roles from the bound `acl.Request` (fail closed without one).
- Flow config (`when`, `roles`): operator-authored, compiled at load; guards may
read only the subject's unrestricted properties (load-time rule).
- Deferred mark on the run state: validated by `flowrun.CheckState` (branch
index >= 0, By length <= MaxClaimBy).

**Security-Sensitive Operations:**
- Moving a run: gated by branch `roles` against the author. A role-less branch
admits any writer who may make its `when` true: the operator's choice; docs
advise `roles:` for sync-driven branches.
- Header writes as `system:flow`, prebound before the Tx.
- Guard evaluation errors may quote values: logged at debug only.

## Test Plan

- [x] Test scenarios documented for each acceptance criterion
- [x] Edge cases identified and documented
- [x] Negative test cases defined (invalid input, error conditions)
- [x] Integration test approach defined (not just unit tests)

**Test Scenarios:** criteria 1-11 above; engine unit tests in
`flowengine/onchange_test.go`; cross-backend in
`appbuild/flowupdateevents_test.go` (fs default, `-tags postgres`, `-tags
sqlite`); flowrun state round-trip for `Pending.Change`.

**Edge Cases:** stranded state (skip); run ended between list and move (lost
CAS: skip); forked run with arrived branches; subject with 100 runs; guard error
on old (counts as not matching) and on new (no start, logged); write by
`system:*` principal with empty roles; body-only change; flow removed from
flows.yaml while runs open (flowOf pins version).

**Negative Tests:** author without role; guard false; rolled-back write;
FlowStep on ctx; no-op write; already-matching predicate.

## Risk Assessment

- [x] Technical risks assessed with mitigations
- [x] Security risks assessed (see Security Considerations)
- [x] Effort estimated (xs/s/m/l/xl)

**Risks:**
- Write latency on subject types with flows (list runs + guards per update):
bounded by indexed List and MaxRunsPerSubject; only watched types.
- Nested writes in the update Tx (header create for predicate start): same as
create starts, proven there.
- Postgres Tx abort from a failing statement: header patches deferred to after commit.
- Loops between flows: change events skip flow writes; predicate starts depth-bounded.

Effort: l

## Documentation Planning

For enhancements: identify what documentation needs updating.

- [x] User-facing docs identified (skip if internal refactor)
- [x] Docs-checklist will be created when entering implementation

**Documentation Impact:**
- [x] docs-project flows guide (generates docs/flows.md): Status note, Starting
on a predicate, change events from writes, sync example, related-entity trigger
pattern.

## Design Review

- [x] Run `/design-review` before starting implementation
- [x] All critical/significant findings addressed in plan

**Design Review Findings:** RR-NM1G2WT, RR-1JMLRXA, RR-OIZNBLY, RR-SXH35LW,
RR-0RFX0M4 (significant, addressed in plan); RR-NWJTCDP, RR-HJQAH9F, RR-Z92NGQB,
RR-JRGR9D8, RR-E6L5ME6, RR-JQK3739, RR-47NM6O4 (minor); RR-ALOHB7G (nit).

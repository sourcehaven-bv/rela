---
id: PLAN-HXSENZ
type: planning-checklist
title: 'Planning: Automation action that enqueues a Lua script as a background job'
started: "2026-10-07"
completed: "2026-10-07"
status: done
---

<!-- @managed: claude-workflow v1 -->

## Understanding

- [x] Problem/requirements clearly understood
- [x] Scope defined (what's in/out documented below)
- [x] Acceptance criteria documented with specific test scenarios

**Problem.** An automation `lua_file:` action runs inline, in the request that
saved the entity. An integration push calls an external API, so every save would
wait on that API, and a failure would only be logged. The action should be able
to run as a background job on `jobs.Queue` instead (FEAT-XYQMUB).

**Facts from the survey that shape the design.**
- The automation cascade runs after the write is persisted, not inside `store.Store.Tx`. A direct enqueue from the cascade is therefore already after commit.
- `jobs.WithDeferral` is wired nowhere. The queue does not carry the principal to the handler: handlers stamp it from the payload (`scheduler.stampTaskAuditContext`).
- arch-lint forbids `autocascade`, `script` and `entitymanager` from importing `jobs`.

**In scope.**
- New action keys, all valid only together with `lua_file:`:
  - `background: true`;
  - `run_as: <user>` (default `system:automation`);
  - `retry: never|bounded|persistent` (default `bounded`).
- The enqueue goes through a consumer-side interface in `autocascade`. An appbuild adapter builds the `jobs.Job`.
- The job kind `automation:run-lua` and its handler, wired in appbuild.
- Load-time refusal of invalid combinations.
- Docs: document `lua:`, `lua_file:` and `background:` in docs/metamodel.md, which lists only set/create actions today.

**Out of scope.**
- Inline `lua:` in the background. The handler re-reads the action from config by automation name and file, and an inline body has no stable identity.
- `allow_acl_bypass` with `background`, refused at load. There is no cascade mutator on a worker; a later ticket can add it if sync needs it.
- `old_entity` in the job. It would be stale, and sync (TKT-SM20FG) takes its base from history.
- Wiring `WithDeferral` into `Store.Tx`. No enqueue happens inside a Tx.

**Acceptance.**
1. An automation with `lua_file: x.lua` and `background: true` enqueues a job on create/update instead of running inline. The save returns before the script runs, and the script sees `entity` as stored at run time.
2. Three quick saves of the same entity, while the job is still pending, give one run (idempotency key `automation:<name>:<ref>`). Saves of two entities give two runs.
3. The job runs as `run_as`, or as `system:automation` with tool `automation-job`. Reads are ACL-bound, writes are row- and field-gated (`ScheduledLuaWriteDeps`), and capabilities come from the action. The audit log has `triggered_by=automation-job:<name>`.
4. The payload is not trusted for authority. The handler looks up the automation by name in the live metamodel and finds the background action with the same `lua_file`. If none matches, the job is dropped with a warning; it does not fail. The entity is re-read through the gated reader. A hidden or deleted entity drops the job.
5. A write made by the job does not enqueue the same automation for the same entity again (ctx marker), so a script that writes its own entity does not loop.
6. Retry follows the declared intent. A script error with `bounded` is retried by the queue and then logged.
7. The following are refused at load, with the automation name: `background` without `lua_file`; `background` with inline `lua`, set, create_relation or create_entity in the same action; `run_as` or `retry` without `background`; an unknown `retry`; `background` with `allow_acl_bypass`.
8. `just arch-lint` passes.

## Research

- [x] ~~For larger features: run `/research` to create a structured research doc~~ (N/A: the decision was made in FEAT-XYQMUB; the survey is recorded above)
- [x] ~~Searched for existing libraries that solve this problem~~ (N/A: the job queue exists)
- [x] Checked codebase for similar patterns or reusable code: the scheduler's run-task job (`scheduler/jobs.go`) for the payload, stamping and handler; soft-delete GC (`appbuild/softdeletegc.go`) for appbuild-registered kinds; `script.LuaScriptRunner` for capabilities.
- [x] ~~Looked for reference implementations in other projects~~ (N/A: internal seam)
- [x] Reviewed relevant rela concepts for prior art: background-jobs rules in CLAUDE.md (idempotency key, not deadline; flat retry enum), TKT-YH52OM capabilities, TKT-D8T148 elevation, TKT-0XL8MF field gate.

## Approach

- [x] Technical approach chosen and documented
- [x] Approach builds on existing patterns (not reinventing)
- [x] Alternatives considered (document why rejected)
- [x] Dependencies identified (packages, APIs, types)

1. **metamodel:**
   - `AutomationAction` gains `Background bool`, `RunAs string` and `Retry JobRetry`.
   - `JobRetry` is a string enum (never/bounded/persistent) that rejects unknown values when unmarshalled.
   - The loader validates the combinations in acceptance 7.
2. **automation:** `Action` and `LuaToExecute` carry the new fields through `convertFromMetamodel`, which copies field by field, so each new field gets a test. `executeAction` emits the item with `Background` set.
3. **autocascade:**
   - New consumer-side `BackgroundScripts interface { EnqueueScript(ctx, BackgroundScript) error }`.
   - `BackgroundScript{Automation, LuaFile, Ref entity.Ref, RunAs, Retry}`.
   - It is set on the Runner. In `executeScriptActions`, a background item is enqueued instead of run.
   - Self-trigger suppression: the handler marks ctx with the running job's (automation, ref). The runner skips an enqueue that matches.
   - A missing enqueuer is an outcome error, like a missing ScriptRunner.
4. **appbuild (`automationjobs.go`):**
   - `automation:run-lua` kind. The payload holds `{automation, lua_file, ref, run_as, retry}`.
   - The adapter over `jobs.Client` sets `IdempotencyKey` and `Retry`.
   - The handler:
     - stamps the principal and audit trigger;
     - re-resolves the action from `s.meta`;
     - re-reads the entity through `ScheduledLuaWriteDeps().VisibleReader`;
     - runs `script.Engine.ExecuteFileWithCapabilities` with that entity and the action's capabilities.
   - `ErrDuplicateJob` counts as success.
   - The kind is registered in `buildRuntimeServices`, after the queue and before the entity manager.
5. **principal:** `UserAutomation = "system:automation"` and `ToolAutomationJob = "automation-job"`.

**Alternatives rejected.**
- *Enqueue from Lua (`rela.jobs.enqueue`).* This gives scripts a general queue surface, which is much wider than needed.
- *Run the job as the triggering user.* The ticket asks for the declared identity, as scheduled tasks have. A user's grants also change over time, while a job may run hours later.
- *A deadline for coalescing.* CLAUDE.md forbids it: a deadline drops work under load.

## Security Considerations

- [x] Input sources identified (user input, config, external APIs)
- [x] Input validation approach defined (allowlist preferred over blocklist)
- [x] Security-sensitive operations identified (file access, auth, crypto)
- [x] Error handling doesn't leak sensitive information

- **Sources.** The action comes from operator config (`schema.yaml`). The payload comes from the queue table, which in postgres anyone with DB access could edit. The handler therefore takes authority only from the live config: identity, capabilities and file are re-read, and the payload only selects among configured actions.
- **Validation.** `retry` is an allowlisted enum. The script path is validated by `lua.ReadScript` (`fs.ValidPath`, `.lua` suffix, rooted). `run_as` is a user name that goes through the same path as the scheduler's `run_as`.
- **Identity.** The job never inherits the trigger's principal. A user who can edit an entity cannot gain the job's grants beyond what the script does by itself; the operator chose the script.
- **Errors.** Script errors go to the server log with the automation name, as scheduled tasks do.

## Test Plan

- [x] Test scenarios documented for each acceptance criterion
- [x] Edge cases identified and documented
- [x] Negative test cases defined (invalid input, error conditions)
- [x] Integration test approach defined (not just unit tests)

- AC1, AC3: an appbuild end-to-end test, like `TestScheduler_EndToEnd_LuaWritesThroughTheQueue`. It uses a memory queue and an automation with `background: true`. The script writes a property and records `rela.principal`. Assert that the property is unset right after the save and set after the job runs. Also assert the principal, the audit `triggered_by`, and a field-gated refusal.
- AC2: three saves against an unstarted queue give one pending job; two entities give two.
- AC4: payload tampering. A job whose automation no longer exists is dropped, a non-matching lua_file is dropped, and a hidden entity is dropped.
- AC5: a script that updates its own entity runs once.
- AC6: a payload round-trip for retry, and the job's `Retry` field.
- AC7: table-driven loader tests.
- Unit tests: the autocascade runner with a fake enqueuer, and the `convertFromMetamodel` field copy.

## Risk Assessment

- [x] Technical risks assessed with mitigations
- [x] Security risks assessed (see Security Considerations)
- [x] Effort estimated (xs/s/m/l/xl)

- **Risk:** the fs/desktop memory queue loses pending jobs on exit. This is accepted per CLAUDE.md and documented.
- **Risk:** a write from the job triggers *other* automations, including other background ones. This is intended; the self-trigger suppression covers only the same automation on the same entity.
- **Risk:** the worker ctx has no request deadline. The script engine's own budgets apply.
- **Effort:** m.

## Documentation Planning

- [x] User-facing docs identified (skip if internal refactor)
- [x] Docs-checklist will be created when entering implementation
- [x] docs/metamodel.md - New metamodel features (Lua actions and `background`)
- [x] ~~docs/cli-reference.md - New/changed commands~~ (N/A)
- [x] ~~docs/data-entry.md - UI changes~~ (N/A)
- [x] ~~CLAUDE.md - New patterns or conventions~~ (N/A: follows the existing jobs rules)
- [x] ~~README.md - Project-level changes~~ (N/A)
- [x] ~~N/A - Internal change, no user-facing docs needed~~ (N/A: user-facing)

## Design Review

- [x] Run `/design-review` before starting implementation
- [x] All critical/significant findings addressed in plan

The design review found two critical and seven significant problems. The plan
above is amended as follows. Where this section and the sections above disagree,
this section wins.

- **C1. An edit during a run was lost.** The idempotency key also collapses against a RUNNING job, so an edit made after the script read the entity was dropped.
  - Fix: a trailing-run guarantee. Each trigger writes a fresh random token to `state.KV` under `automation-jobs/<sha256(key)>`, then enqueues. `ErrDuplicateJob` is still success.
  - The handler reads the token, runs, and reads it again. If the token changed, it runs again, up to 5 runs.
  - A per-key mutex in the handler keeps a retry and a fresh job of one key from overlapping in one process.
  - On postgres, `state.KV` is shared, so a token written on another node is still seen. Two nodes can still overlap in the rare case where a retry runs while a new job does. This is documented: background scripts must converge, which sync (TKT-SM20FG) does by design.
  - Tokens are never deleted, because deleting races with a new trigger. There is one small row per (automation, file, entity).
- **C2. One-shot processes lost jobs.** The CLI starts a memory queue and stops it on exit. On postgres a CLI process also claims shared jobs.
  - Fix: delivery mode belongs to the entry point. By default the background action runs in the foreground, right after the cascade, through the same handler (same identity, re-resolution and token). `appbuild.WithBackgroundAutomationJobs()` switches to the queue. Only rela-server and rela-desktop set it.
  - The pre-existing problem, that a postgres CLI process drops unregistered kinds such as scheduler jobs, is filed as a bug.
- **S1.** The key includes the file: `automation:<name>:<lua_file>:<ref>`.
- **S2.** Refused at load: a background action in an automation whose name is empty, or not unique; and a repeated `(name, lua_file)` pair among background actions.
- **S3.** A hop count rides on ctx into the payload, used only to limit depth. An enqueue past 8 hops is refused with a warning that names the automation. This stops A→B→A loops across jobs.
- **S4.**
  - `run_as` is validated at load: trimmed, non-empty, no control characters.
  - The enqueuing principal is logged at enqueue, for attribution only.
  - docs/metamodel.md states that `run_as` is an elevation the operator chooses, and that `entity` is untrusted input.
- **S5. Rename.** The job service is a `store.EntityObserver`. On `EntityRenamed` it triggers the background actions of automations on that type for the new ref. Over-triggering is harmless. Restore after a soft delete is out of scope; noted in the docs.
- **S6. Wiring.**
  - The cascade runner takes the enqueuer in `autocascade.Deps`. `buildAutomation` therefore moves after `buildRuntimeServices`, where the queue exists.
  - The trigger side needs only the queue and `state.KV`. The handler needs the services, so the job kind is registered at the end of `assemble`, and a failed registration fails assembly.
  - Schema hot reload (`rela mcp`) closes the memory queue, which only matters in queue mode; noted.
- **S7.** A drop because the entity is missing or hidden names the principal and adds "or this identity cannot read it". docs/metamodel.md shows the `acl.yaml` assignment for `system:automation`.
- **M1.** Configuration errors (action gone, file refused, entity not readable) end the job with an error log and no retry. Script errors are returned, so retry applies. The docs say background scripts must be idempotent.
- **M2.** The coalescing test uses a started queue with a handler blocked on a channel.
- **M3.** The payload carries only `{automation, lua_file, id, face, hops}`. Retry is set on `Job.Retry`. `run_as` and capabilities come from config.
- **M4.** The shared pool can be starved by slow scripts. Noted as a risk; there is no per-kind limit in v1.
- **M5.** Documented: the trigger condition is not re-checked when the job runs, and `old_entity` is nil.
- **M6.** Unknown action keys are still ignored, as before. Deferred to the follow-up for strict action parsing.
- **N1.** The payload stores `id` and `face` separately, and the handler reads `GetAddress(entity.FormatStateRef(id, face))`.
- **N2.** Re-resolving from config is described as protection against stale and redelivered jobs.

**Acceptance criteria, revised.**
- AC1 covers create and update only; delete and rename run no automations.
- AC1 also distinguishes the delivery modes: in queue mode the save returns first; in foreground mode the script runs after the cascade, in the same request.
- New AC9: an edit while the script runs gives one more run (C1).
- New AC10: a rename re-triggers the job for the new id (S5).
- New AC11: an A→B→A loop stops at 8 hops (S3).

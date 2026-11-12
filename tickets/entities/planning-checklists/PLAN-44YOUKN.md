---
id: PLAN-44YOUKN
type: planning-checklist
title: 'Planning: Flows M4b: job steps'
started: "2026-11-12"
completed: "2026-11-12"
status: done
---

<!-- @managed: claude-workflow v1 -->

## Understanding

- [x] Problem/requirements clearly understood
- [x] Scope defined (what's in/out documented below)
- [x] Acceptance criteria documented with specific test scenarios

**Scope:** In: a `job:` key on a flow state with exactly one target (`script:`,
`action:` or `kind: webhook`); the attempt state in the run store; the engine's
attempt state machine (enqueue, start, outcome, retry, fail); the queue handler
in appbuild; a Lua permanent error (`error({message = ..., retry = false})`);
operator retry from `failed`; the attempt in the run explanation, the flow view
and the SPA; GUIDE-flows. Out: `kind: mail` (TKT-VVDY2RM: it must render as the
recipient's principal, not run_as); the loop guard for job writes that start
runs (noted on TKT-GYXOTQC, which adds write-triggered starts); `set:` actions,
`request:` actions and actions with `available_on:`, `allow_acl_bypass` or
`elevated_updates` as targets (refused when the services are built, and again at
runtime after a data-entry.yaml reload); flow runs in tenant export
(BUG-EWG0R90); `db load` of runs (TKT-TWLN8UB).

**Acceptance Criteria:**
1. A job that keeps failing with a retryable error runs `attempts` times,
with backoff between attempts, and then the run is `failed` with kind
`exhausted`.
2. A permanent error (a webhook 4xx other than 429, a Lua `retry = false`,
a missing script or action) fails the run on the first attempt with kind
`permanent`.
3. An attempt that passes its timeout fails the run with kind `timeout`; it
is not run again.
4. An attempt whose worker died (started, never finished, deadline plus
grace passed) fails the run with kind `lost`.
5. With `idempotent: true`, a timeout and a lost worker are retried like a
retryable error.
6. An attempt that never started (queued past its deadline, or never
enqueued) is enqueued again without using the budget.
7. Operator retry of a failed job starts a fresh attempt count; retry of a
waiting job only brings the next attempt forward; retry of a queued or started
attempt is refused.
8. Each attempt gets the key `<run>/<state>.<generation>/<seq>`, passed to
the target (script `rela.params.idempotency_key`, webhook `Idempotency-Key`
header) and to the queue. The target also gets the stable step key
`<run>/<state>.<generation>` (`rela.params.step_key`, `X-Rela-Step`), so it can
drop a repeat across attempts.
9. A job that succeeds follows the state's `done` branch.

## Research

- [x] ~~For larger features: run `/research` to create a structured research doc~~ (N/A: the design was approved on the ticket; this plan only maps it onto the code)
- [x] Searched for existing libraries that solve this problem
- [x] Checked codebase for similar patterns or reusable code
- [x] Looked for reference implementations in other projects
- [x] Reviewed relevant rela concepts for prior art

**Research Doc:** N/A

**Existing Solutions:** `internal/appbuild/automationjobs.go` (a job handler
registered on the queue, the run-as principal, `BindIdentity`,
`ScheduledLuaWriteDeps`, `ExecuteFile` with declared capabilities).
`internal/dataentry/actions_ops.go` (action permission, static params,
`ExecuteActionRequest`). `appbuild/scheduled_mail.go` (`RunScheduledTemplate`).
The engine's pending marks and backoff (`flowengine/step.go`, `branches.go`).
Temporal and AWS Step Functions model retries the same way: the orchestrator
owns the retry policy and each attempt is a separate task with a timeout; an
unknown outcome is not retried unless the activity is idempotent. River and the
memory queue already give at-least-once delivery, a deadline and an idempotency
key, so no new library is needed.

## Approach

- [x] Technical approach chosen and documented
- [x] Approach builds on existing patterns (not reinventing)
- [x] Alternatives considered (document why rejected)
- [x] Dependencies identified (packages, APIs, types)

**Technical Approach:**

- flowcfg: `rawState.Job`, compiled into `State.Job *Job{Target, Script,
Action, Kind, URL, Secret, Template, Capabilities, Idempotent, Retry}`. Exactly
one of script, action, kind. `retry: false` or `{attempts 1..10, backoff
{initial, factor 1..10, max}, timeout}`, fixed periods only, timeout 1s..10m
(below the queue's 15m handler cap), backoff max at most 1h. Defaults: 3
attempts, 10s, factor 2, 5m max, 5m timeout. A job state is not final and has no
`waiting_for`, `writes`, `on.change` or `after`; it needs at a last `done`
branch without `when:`, so a success always has a way out. `run_as` is required.
A script path is relative, clean, `.lua`, without `..`. A webhook URL is http or
https with a host.
- flowrun: `Pending.Job *JobAttempt{Seq, Failed, Unsent, Phase, EnqueuedAt,
StartedAt, Deadline, NextAt, Outcome, FailKind}`, checked in `checkPending`
(enum values, ranges, at most 1<<20 sequence). No error text is stored.
`copyPending` deep-copies it. The JSON envelope carries it in every backend.
- flowengine: `Deps.Jobs` (enqueuer; nil leaves the attempt waiting for a
process that has a queue) and `Deps.JobRunner`. In `settleStep`, after the
outputs and before the `done` branch, `jobStep` drives the attempt:
  - none or waiting with NextAt passed: next seq, phase queued, deadline
= now + timeout, save, then enqueue. Due at the deadline.
  - queued past the deadline: never started. Back to waiting with a backoff
by Unsent; the budget is untouched.
  - started past the deadline plus 30s grace: lost.
  - finished: ok clears the attempt and follows `done`; retryable retries
while Failed+1 < attempts, else fails as exhausted; permanent fails; timeout or
lost fail unless idempotent.
  - failed: stays parked, never due.
A parked token returns a sentinel; `settle` skips it like a failed branch,
without an error, so other branches settle. `pendingToken` and `Pending` treat a
token whose job is parked (in flight before its due time, or failed) as settled,
so `advanceBranches` still fires the other branches' timers.
  - An enqueue error puts the attempt straight back to waiting with the
Unsent backoff.
  - `mark` refuses to drop a job attempt (it is only called before the job
starts); `backoff` copies the attempt.
- `Engine.HandleAttempt(ctx, AttemptRef)`: move queued to started, run the
target with ctx bounded by the deadline, record the outcome, then settle the run
with a bounded context. Both writes loop: re-read the run, check that the token,
generation, seq and phase still match, write, and retry on a conflict, so a
concurrent write to another branch never drops an outcome. Only a mismatch or a
gone run stops it (cancel, redelivery). The outcome is classified from the
attempt context first: its deadline passed means timeout; a cancel that is not
the deadline (shutdown) records nothing and leaves the lost path to decide. Then
`ErrPermanent` means permanent, and any other error retryable.
- A failed job leaves the header status as it is: `failed` is terminal, and
the run is not over. The explanation reports the run as `failed` with the
failure kind, and the flow view and API show it. The run stays stored, so it can
be retried or cancelled. `Retry` resets every failed attempt of the run (Failed
0, Unsent 0, NextAt now) and moves a waiting attempt's NextAt to now; a run
whose only pending work is a queued or started attempt returns `ErrJobRunning`.
- lua: `ScriptError.Permanent`, set when the raised value is a table with
`retry = false`; its `message` field becomes the message.
- script: `Engine.ExecuteJob(ctx, path, deps, JobInvocation{Subject, Params,
Capabilities})` runs a `scripts/` file through `NewWriterRuntime` with
`rela.params`.
- appbuild: `flowJobs` registers kind `flow/job-attempt` on the queue and
adapts `Deps.Jobs` (enqueue with `RetryNever`, the key, the deadline) and
`Deps.JobRunner`. The runner stamps the run_as principal (Tool flow), binds
identity, sets the audit flow step, and dispatches:
  - script: `ExecuteJob` with the declared capabilities; subject read as
run_as.
  - action: data-entry.yaml action, script-only; refuses `set`, `request`,
`available_on`, `allow_acl_bypass`, `elevated_updates` and a param named like a
flow param; run_as must hold its `permission`; runs `ExecuteActionRequest` with
its own capabilities.
  - webhook: POST of ids only (flow, run, state, subject, attempt, keys),
with `Idempotency-Key`, `X-Rela-Step`, `X-Rela-Timestamp` and, when `secret:`
names a global secret, `X-Rela-Signature` = HMAC-SHA256 over
`<timestamp>.<body>`, so a receiver can refuse a replay. 2xx ok; 429, 5xx and
network errors retryable; other statuses permanent. Redirects are not followed.
The client is the services' HTTP seam when set, else a default client; the
response body is drained up to a cap and dropped.
- The action checks (script-only, no elevation, no request, permission set
or not) also run in `buildFlowEngine` against data-entry.yaml, so a flow that
names a missing or refused action fails the build.
- Only long-running processes (`WithBackgroundAutomationJobs`) get
`Deps.Jobs`; the flow worker then enqueues waiting attempts on its tick.
- Explanation: `PendingStep.Job{Attempt, Attempts, Phase, FailKind,
NextAttempt}`, carried by `flowview.Pending` to FlowWaiting.vue with en and nl
strings.

**Alternatives rejected:** a per-call retry policy on `jobs.Job` (jobs.md
forbids widening the enum; the engine owns the loop and enqueues each attempt as
`RetryNever`); running the target inline in settle (a side effect on the write
path, and no timeout isolation); storing the error text on the run (it can carry
subject values and secrets; it goes to the log only); a separate attempts table
(the run envelope already persists per backend under `store.Tx`).

**Files to modify:** `internal/flowcfg/{raw,states,compiled,job}.go`,
`internal/flowrun/flowrun.go` and `flowruntest`,
`internal/flowengine/{engine,step,person,job}.go`,
`internal/lua/scripterror.go`, `internal/script/job.go`,
`internal/appbuild/{flowruntime,flowjobs,flowsteps,appbuild}.go`,
`internal/flowview`, `frontend/src/api/flows.ts`, FlowWaiting.vue, the en and nl
locales, `docs-project/entities/guides/GUIDE-flows.md` and the generated
`docs/flows.md`.

## Security Considerations

- [x] Input sources identified (user input, config, external APIs)
- [x] Input validation approach defined (allowlist preferred over blocklist)
- [x] Security-sensitive operations identified (file access, auth, crypto)
- [x] Error handling doesn't leak sensitive information

**Input Sources & Validation:**

- flows.yaml (operator config): strict decoding, one target, allowlisted
kinds, path and URL checks, bounded retry values. Invalid config fails the load.
- Stored run state: `CheckState` on every write checks the attempt enums and
ranges, so a corrupt row is refused rather than acted on.
- Queue payload (run id, token, generation, seq): never trusted; the
handler re-reads the run and acts only when the stored attempt matches.
- Webhook responses: only the status code is read; the body is drained up
to a cap and dropped.

**Security-Sensitive Operations:**

- Script and action execution as run_as, bounded by its ACL; capabilities
only as declared (none by default).
- Outbound HTTP to an operator-configured URL; HMAC secret from
`.rela/secrets.yaml` by name, never in flows.yaml or on the wire.

**Threat model (POLICY-015 §3, PROCEDURE-j5mq step 1).**

- Identity. Every job runs as the flow's `run_as`, stamped with Tool flow
and bound at the handler, which is the edge. No allow-all reader; the
`system:flow` header writer is not the job identity. A missing run_as fails the
load. An action's `permission:` is checked for run_as at every attempt.
- Data disclosure. The webhook body carries ids only, so no property value
leaves through it whatever run_as may read. Error text never reaches the run
store, the explanation or the API; only the failure kind does. Logs carry the
error for the operator.
- Duplicate side effects. The handler moves an attempt from queued to
started with a compare-and-set, so a redelivered job or a second worker does
nothing. An unknown outcome is not retried unless `idempotent: true`. The
idempotency key lets the target drop a repeat.
- Unbounded work. Attempts are capped at 10, the timeout at 10m, the backoff
at 1h; a never-started attempt backs off up to 1h. A failed run is never due, so
it costs nothing until an operator acts.
- Escalation through actions. Actions that elevate (`allow_acl_bypass`,
`elevated_updates`) or read a request are refused as targets, so a flow cannot
reach a capability the action's HTTP surface gates differently.
- SSRF. The URL is operator config, the same trust as a script's granted
`http` capability; redirects are not followed and the response body is not read.
- Multi-tenant. Each tenant has its own queue (River in the tenant schema)
and run store; the payload names a run id in that tenant only.

## Test Plan

- [x] Test scenarios documented for each acceptance criterion
- [x] Edge cases identified and documented
- [x] Negative test cases defined (invalid input, error conditions)
- [x] Integration test approach defined (not just unit tests)

**Test Scenarios:**

- flowengine (fake queue, fake runner, fake clock): exhausted retries (1),
permanent (2), timeout (3), lost worker (4), idempotent timeout and lost (5),
never started (6), operator retry (7), the key per attempt (8), success follows
done (9), a redelivered attempt runs once, a cancel during an attempt.
- flowcfg: every refusal (two targets, none, final job state, change branch,
timer, calendar period, ranges, bad path, bad URL, unknown kind, missing run_as)
and the defaults.
- flowrun: `CheckState` refusals and a round trip of `Pending.Job` in the
conformance suite (kv, sqlite, postgres).
- lua: `retry = false` sets Permanent; a plain error does not.
- appbuild: a flow with a script job end to end on the memory queue; webhook
status classification and signature against httptest; action permission refusal
and the build-time action check.
- flowengine: a concurrent write to another branch during an attempt keeps
the outcome; a fork with a job branch and a timer branch fires the timer; a
shutdown cancel leaves the attempt to the lost path; an enqueue error goes back
to waiting.

**Edge cases:** a job state in a parallel branch; a job state with outputs
(written before the job); a success just after the deadline (counts as success);
a run version change while an attempt is in flight (the stored definition is
used).

**Negative tests:** stale seq or generation in the payload; an attempt past its
deadline is not started; a corrupt attempt in the store is refused.

## Risk Assessment

- [x] Technical risks assessed with mitigations
- [x] Security risks assessed (see Security Considerations)
- [x] Effort estimated (xs/s/m/l/xl)

**Risks:**

- The fs and desktop queue is ephemeral: an attempt queued at exit never
starts. Mitigated by the never-started path, which enqueues it again after its
deadline.
- A run parked on a job holds its subject like any open run. Mitigated by
operator retry and cancel, and by the failure kind in the explanation.
- Clock skew between processes on postgres can mistake a slow attempt for a
lost one. Mitigated by the 30s grace and by `idempotent`.

Effort: l.

## Documentation Planning

For enhancements: identify what documentation needs updating.

- [x] User-facing docs identified (skip if internal refactor)
- [x] Docs-checklist will be created when entering implementation

**Documentation Impact:** GUIDE-flows in docs-project (job steps, retries,
idempotency key, failure kinds, operator retry), regenerated into
`docs/flows.md`.

## Design Review

- [x] Run `/design-review` before starting implementation
- [x] All critical/significant findings addressed in plan

**Design Review Findings:** RR-5L41TIL, RR-K3DMOQ1, RR-48HQ9T9 (critical,
addressed); RR-0JQWN6W, RR-H3Z72TJ, RR-GZ335ZV, RR-IED8VHF, RR-62RAH37,
RR-T77TZ2O (significant, addressed); RR-QDPDTMV (significant, deferred to
TKT-GYXOTQC); RR-6X3AUZN, RR-UQ615NV, RR-BACHJMD (minor, addressed).

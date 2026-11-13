---
id: TKT-74TOWJM
type: ticket
title: 'Flows: explicit failure path for job steps'
kind: enhancement
priority: medium
status: backlog
---

## Problem

A flow `job` step (TKT-PY2G11W) can fail for good: its retries run out
(`exhausted`), the job reports a permanent error (`permanent`), or the outcome
is unknown (`timeout`, `lost`). Today the run then stays in the job's state. Its
header status stays non-terminal, and only the run's explanation says `failed`
with the failure kind. An operator can press Retry, which starts the job again
with a fresh attempt count.

This came from review response RR-48HQ9T9. `flow.Status.Terminal` treats
`failed` as ended, so a header status of `failed` would make `flowsteps` and
`closeOrphan` read the run as over while it is still retryable. Keeping the
header non-terminal fixed that, but it hides a decision in a status rule.

Seen as a flowchart, a failed job is followed by a decision: go on along a
failure path, or wait for a person. The flow author cannot make that decision
today, and the flowchart does not show it. The diagram draws only the job's
`done` edge, so a reader cannot see that the run can stop there.

## Options

### A. An `on_failure:` edge to a named state

```yaml
provision:
  job: { script: flows/provision.lua }
  on:
    done: { to: provisioned }
    failed: { to: provision-failed }
```

A new event `failed` (or a key `on_failure:` on the job) names the state the run
enters when the job fails for good. The branch may carry a `when:` over the
failure kind, for example `when: "job.fail_kind == 'permanent'"`. The target is
an ordinary state: it can wait for a role, write outputs, run another job or be
final with `status: failed`.

- Flowchart: one extra edge from the job state, labeled `failed` (or with the
kind). `flowcfg.Flow.MermaidAt` already draws one edge per branch.
- Pro: uses the existing state machine. Escalation, compensation and "mark the
subject as blocked" are all plain states. No new runtime concept.
- Con: an author who wants "wait for an operator, then retry" has to model the
retry as a branch back to the job state. That re-entry is a new generation, so
it naturally gets a fresh attempt count, but every flow repeats the same three
states.

### B. An explicit "wait for operator" node

The job state gets an implicit (or declared) failure node, drawn in the
flowchart, with three actions: retry, skip and cancel.

```yaml
provision:
  job:
    script: flows/provision.lua
    on_failure:
      wait_for: [ops]
      skip: { to: provisioned }
  on:
    done: { to: provisioned }
```

- Flowchart: the job state has a child node "failed: waiting for ops" with
three edges: retry (back to the job), skip (to the named state) and cancel (to
the end).
- Pro: matches the operator's mental model and today's Retry button.
- Con: a second kind of waiting next to `waiting_for:`. The roles, the task
list (Flow tasks page), `_actions` and MCP tools need a new case. "Skip" has to
name a target, which is option A in disguise.

### C. A terminal `failed` that stays retryable

The run's header status becomes `failed`. `flow.Status.Terminal` gets a second
notion, "ended but resumable", and Retry and Cancel accept a failed run.

- Flowchart: the job state is marked red. No edge, because nothing in the
definition changes.
- Pro: the run list and status filters show failed runs directly.
- Con: it is the hidden rule this ticket wants to remove, moved from the
explanation to the header. Every reader of `Terminal` (`flowsteps`,
`closeOrphan`, retention, `per_subject: one`, the worker) has to learn the
difference. Retention could delete a run that an operator meant to retry.

## Flowchart view

`flowcfg.Flow.MermaidAt` draws the diagram from branches and timers, and
`FlowWaiting.vue` (with `flowview.Pending.Job`) shows the job line.

- A: the `failed` edge appears in the diagram with no change to the renderer
beyond adding the event. When the run takes the edge, the current-state mark
moves to the failure state. `FlowWaiting.vue` shows the failure kind in the
timeline entry for the move.
- B: the renderer needs a pseudo-state per job state with an `on_failure:`.
`FlowWaiting.vue` shows the failure line plus the three actions.
- C: no diagram change. `FlowWaiting.vue` keeps today's alert line.

Whichever option wins, a job state without a failure path should still be drawn
so a reader sees it: for example a dashed edge to an implicit "stopped: waiting
for retry" node.

## Retry policy, `idempotent: true` and operator retry

- The retry policy (`attempts`, `backoff`, `timeout`) and `idempotent: true`
stay as they are. They decide *when* a job has failed for good. This ticket only
decides *what happens next*.
- A: taking the `failed` edge leaves the job state. A later branch back into
the job state starts a new generation, so the attempt count starts fresh and the
idempotency key (run + step + attempt) must include the generation, or a
receiver could drop the new attempt 1 as a repeat. The `step_key` (run + step)
stays the same across re-entries; decide whether that is wanted.
- B: retry from the failure node keeps today's semantics: same generation,
fresh attempt count. The idempotency key needs the retry round in it for the
same reason.
- C: same as today.
- In all options, an unknown outcome (`timeout`, `lost`) on a job that is not
idempotent must not be retried automatically by the failure path. A failure
branch that loops straight back to the job state would do exactly that; `flow
validate` should refuse such a loop, or require `idempotent: true`.

## ACL

Today Retry and Cancel need the `permission:` of the flow's `on: manual` start
(see "Running flows" in `docs/flows.md`).

- A: no new permission. Who moves the run on from the failure state follows
that state's `waiting_for:` and branch `roles:`, as for any other state. A
`done` or `failed` branch takes no `roles`, because nobody causes it.
- B: the node needs its own roles for retry and skip. Skip is the dangerous
one: it moves the run past work that did not happen. It should need a role that
the author names, not the start permission.
- C: keeps the start permission for retry and cancel.
- In all options, the action is recorded in the audit log with `by`, and
`_actions` on the run reports whether each action would be accepted.

## Recommendation

Option A, a `failed` event on job states, with these rules:

1. A job state without a `failed` branch keeps today's behaviour: the run stays
in the state, retryable, and the diagram draws a dashed implicit "waiting for
retry" edge so the stop is visible.
2. A `failed` branch may have a `when:` over the failure kind, and its target
is an ordinary state. Operators wait there through `waiting_for:`, and a
`change` branch back to the job state is the modelled retry.
3. Re-entering a job state starts a fresh attempt count. The idempotency key
includes the generation.
4. `flow validate` refuses a `failed` branch that leads straight back to a
non-idempotent job state without passing a state that waits for a person.
5. The run's header status stays non-terminal until the run reaches a final
state, so `flow.Status.Terminal` keeps one meaning. An author who wants a failed
run to end uses a final state with `status: failed`.

Option A makes the decision part of the definition, reuses states, roles, timers
and the diagram renderer, and leaves `Terminal` alone. Option B's operator node
can come later as a shorthand that expands into A's states.

## Acceptance

- `flows.yaml` accepts a `failed` branch on a job state and refuses it
elsewhere; `when:` can read the failure kind.
- The diagram draws the `failed` edge, and a dashed implicit edge for a job
state without one.
- Re-entering a job state gives a fresh attempt count and a new idempotency
key.
- `flow validate` refuses an unguarded retry loop on a non-idempotent job.
- `docs/flows.md` "Job steps" describes the failure path.

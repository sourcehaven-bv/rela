---
id: TKT-XWZIOB
type: ticket
title: 'Scheduler for_each: expand one occurrence into recipient-scoped jobs'
kind: enhancement
priority: medium
effort: l
tags:
    - needs-design
    - security
status: done
---

## Description

Add `for_each` to scheduled tasks: select entities from the graph, resolve each
to a principal, and execute one independently retryable background job per
entity under that principal. Mail is the first consumer, but the mechanism is
general enough for per-user scripts, exports, and cleanup tasks.

Built on TKT-YOED3R / PR #1444. The scheduler owns cadence and creates one
expansion job for an occurrence; `internal/jobs` owns child execution and retry.
There is no second scheduler retry/state implementation for derived jobs.

## Configuration

```yaml
tasks:
  - name: daily-digest
    template: overdue_digest
    every: day
    for_each:
      entity_type: person
      where: ["active = true"]
      limit: 1000
```

`where` reuses `filter.ParseAll`. `entity_type` must exist. `limit` defaults to
1000 and must be 1..10000. A task without `for_each` follows the existing
scheduler path byte-for-byte. A `for_each` task may not specify `run_as`: its
identity is the selected entity's resolved principal, so accepting both would
create an ambiguous and potentially privileged fallback.

This first slice permits `for_each` only on calendar schedules (`day`, weekday,
or `week`). Their occurrence is a stable local date. Interval schedules are
rejected: a retry can cross an interval slot, and the existing state does not
retain the original slot, so claiming at-most-once child creation would be a
false guarantee.

## Two-phase execution

1. When a declaration is due, the scheduler enqueues one expansion job carrying
the declaration and a stable occurrence ID.
2. The expansion handler queries at most `limit+1` matching entities and
enqueues one child job for each entity within the limit.
3. A child handler reloads the entity, derives its principal from the configured
ACL `principal_property`, obtains an `acl.Request` through the existing
resolver, installs it on the context, and invokes the task action.

Expansion succeeds after its intended children are accepted; it never waits for
them. Each child uses `jobs.RetryBounded`, so a failure retries only that
entity. The scheduler remains free to evaluate later declarations.

Payloads contain only stable task, occurrence, and selected-entity identifiers,
never action configuration, graph content, rendered output, email addresses,
roles, capabilities, or an ACL request. The child reloads the current task
declaration; authority and capabilities are always re-derived by the worker from
current config, policy, and graph state. A removed/changed declaration does not
execute stale serialized instructions.

## Occurrence and duplicate suppression

The child identity is `<task>/<occurrence>/<entity-id>`. The occurrence is the
selected local calendar date. The expansion job and every child use that
identity as their pending `jobs.IdempotencyKey`.

The existing pending `jobs.IdempotencyKey` collapses concurrent expansion and
children that are still queued or running. It deliberately does not retain a
history after completion. Consequently an expansion retry after partial enqueue
can recreate a child that already finished. That is the queue's documented
at-least-once boundary and matches the external effect: a process can also die
after SMTP accepts DATA but before job completion. A separate claim table would
add another pool, migration and retention policy without making delivery
exactly-once, so this ticket does not invent one.

## ACL invariants

`for_each` is identity selection, never capability:

- The selection query runs under the declaration's scheduler identity and only
decides which principals receive child work.
- Each child reads through the selected principal's existing row gate and
TKT-BUYEW1 field redactor.
- No shared `run_as` fallback exists.
- An entity with no usable principal mapping is logged by ID and skipped; it
never runs as scheduler/system.
- Write preparation remains raw internally, while writes continue through
entitymanager's own ACL checks.

Attenuation is deferred. The earlier draft proposed scope intersection but no
existing scheduler config surface can express it without adding new ACL policy
semantics. Recipient identity alone is the safe, mechanical first slice.

## Failure behavior

- Selection/query failure: expansion fails and is retried; pending child keys
collapse work still queued or running, while completed children remain in the
documented at-least-once replay window.
- One child failure: only that child follows `RetryBounded`.
- Principal resolution failure: log entity ID, count as skipped, continue peers.
- Limit exceeded: process the deterministic first `limit`, log and count the
dropped remainder, then finish successfully. Retrying the same bounded prefix
cannot make progress on deliberately excluded rows.
- Removed recipient after enqueue: child reload fails or mapping disappears;
log/finish without falling back to another principal.

## Scope: IS NOT

- No broadcast or shared-principal graph mail.
- No parallelism controls beyond the process-scoped job worker pool.
- No scheduler-state retry ladder changes (TKT-N52HRC remains independent).
- No address lookup or mail rendering (TKT-U2R7GU).
- No widening/attenuation language in this slice.

## Acceptance criteria

1. A `for_each` declaration creates one child job per matching entity, with a
stable task/occurrence/entity identity.
2. Each child runs with the selected entity's resolved principal on the context.
3. Row-denied entities and `visible:`-redacted fields remain unavailable through
the existing scheduled read dependencies.
4. `run_as` plus `for_each`, interval schedules with `for_each`, unknown entity
types, malformed filters, invalid limits, and unknown keys fail config
validation with named diagnostics.
5. Tasks without `for_each` retain their current config, state key, queue job,
retry ladder, and audit behavior.
6. One child failure retries only that child and does not block or replay peers.
7. Concurrent expansion and pending children collapse by their stable
idempotency keys; the post-completion replay window is documented as
at-least-once.
8. An unresolvable or removed principal is skipped with a warning naming the
entity and never falls back to the scheduler identity.
9. Expansion is bounded, deterministic, and reports the dropped count.
10. Payload round-trips through JSON and contains only task, occurrence and
entity IDs—no address, rendered content, action config, capability, role, or
serialized ACL request.
11. Audit attribution names the schedule while the context principal identifies
the selected user.

## Risks

- **ACL exfiltration:** authority is reconstructed inside the child and content
reads use the existing row/field visibility bundle.
- **Duplicate side effects:** stable pending idempotency limits concurrent
duplicates; post-completion replay remains honestly at-least-once.
- **Fan-out amplification:** strict bounded selection plus worker-pool backpressure.
- **Stale payload authority:** payload carries identity hints only; worker reloads
and resolves against current state.
- **Cross-backend drift:** the existing job conformance suite covers both tiers.

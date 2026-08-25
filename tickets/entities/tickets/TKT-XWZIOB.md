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
status: in-progress
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

Pending-only deduplication is not enough when expansion partially succeeds: a
completed child frees its queue key, so an expansion retry could recreate it.
Expansion therefore maintains a persistent child claim through an injected
`OccurrenceClaims` interface:

```go
type OccurrenceClaims interface {
    Claim(ctx context.Context, task, occurrence, subject string) (bool, error)
    Release(ctx context.Context, task, occurrence, subject string) error
}
```

`Claim=false` means already posted. A claim is retained after enqueue and
expires after 35 days; it is released only when enqueue itself fails, allowing
an expansion retry to fill the hole. Durability matches the queue: FS/desktop's
ephemeral memory queue uses in-memory claims, so a restart may recreate work the
queue lost; PostgreSQL uses a durable unique-key table alongside its durable
queue. A persistent FS claim in front of an ephemeral queue would turn a crash
into guaranteed lost work. Backend conformance tests pin atomic claim and
cross-instance behavior where the backend promises it.

This provides at-most-once job creation per occurrence. It cannot provide
exactly-once external effects: a process can die after SMTP accepts DATA but
before the job records completion. That residual at-least-once crash window is
documented rather than hidden.

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

- Selection/query failure: expansion fails and is retried; existing child claims
prevent replay of children already posted.
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
7. Expansion retry after a child completed does not enqueue that child again;
an enqueue failure releases only that child's claim.
8. Memory and PostgreSQL claim backends pass one conformance suite, including
concurrent claim of the same identity; their durability matches their queue.
9. An unresolvable or removed principal is skipped with a warning naming the
entity and never falls back to the scheduler identity.
10. Expansion is bounded, deterministic, and reports the dropped count.
11. Payload round-trips through JSON and contains only task, occurrence and
entity IDs—no address, rendered content, action config, capability, role, or
serialized ACL request.
12. Audit attribution names the schedule while the context principal identifies
the selected user.

## Risks

- **ACL exfiltration:** authority is reconstructed inside the child and content
reads use the existing row/field visibility bundle.
- **Duplicate side effects:** persistent child claims prevent expansion replay;
the unavoidable external-effect acknowledgement window is documented.
- **Fan-out amplification:** strict bounded selection plus worker-pool backpressure.
- **Stale payload authority:** payload carries identity hints only; worker reloads
and resolves against current state.
- **Cross-backend drift:** claim implementations share a conformance suite.

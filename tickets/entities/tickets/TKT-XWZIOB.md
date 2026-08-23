---
id: TKT-XWZIOB
type: ticket
title: 'Scheduler for_each: run a task once per selected user, as that user'
kind: enhancement
priority: medium
effort: l
tags:
    - needs-design
    - security
status: backlog
---

## Description

A scheduled task has exactly one identity today: `run_as`, a single string
defaulting to `system:scheduler` (`internal/scheduler/config.go:42`, stamped
once at `scheduler.go:298`). There is no way to say *"run this for every active
person, as that person"*.

That gap surfaced while designing per-recipient mail digests, but it is not a
mail feature. It is a scheduler capability that mail happens to be the first
consumer of. Others that want it:

- A daily digest per person, each seeing only their own items.
- A per-user report or export written into that user's scope.
- A per-user validation or cleanup pass.
- Anything where the correct answer is "the user's view", not "a system view".

Building it inside mail would produce a mail-shaped version of a general
mechanism, and the next consumer would either duplicate it or bend mail's config
to reach it.

## The shape

Selection is a query over the graph; execution is the existing task, repeated:

```yaml
tasks:
  - name: daily-digest
    script: reports/digest.lua      # or a mail template, per TKT-U2R7GU
    every: day
    for_each:
      entity_type: person
      where: ["active = true"]
```

For each matching entity the scheduler resolves that entity to a principal,
stamps it on a fresh ctx, and runs the task body. One task declaration, N scoped
runs.

**Identity resolution already exists.** `acl.Declarative.ResolvePrincipal`
(`internal/acl/declarative.go:135`) maps a raw principal string to an entity ID
via `principal_property`; the reverse — entity to principal — is reading that
same property off the entity. `aclmap/whocan.go:158` is the existing precedent
for opening an `acl.Request` for an arbitrary, non-ctx principal:

```go
req, err := e.resolver.ForPrincipal(
    principal.Principal{User: user, Tool: principal.ToolCLI, RawUser: rawShown})
```

So the mechanism is: `ForPrincipal(p)` → `acl.WithRequest(ctx, req)` → run. No
new ACL machinery.

## `run_as` stays identity, not capability

DEC-O59WM4 holds: naming a principal grants nothing, `acl.yaml` decides what it
reads. `for_each` does not elevate — it *narrows*, running as a real user rather
than as a system identity, which is strictly less privilege than
`system:scheduler` with a broad role.

**Attenuation is supported, defaulting to the user's own grants.** A `for_each`
run reads as the user by default — no more, no less. An optional ceiling narrows
it further, for a job that should see less than the user does:

```yaml
    for_each:
      entity_type: person
      where: ["active = true"]
      attenuate:
        scopes: [tasks:read]      # or a baseline role
```

This is not new ACL machinery either: `client_baselines` / `scope_grants`
(TKT-IAC8TX) already compiles exactly this kind of ceiling at `acl.yaml` load
time, into plain allowlists, with `Request.roleFor` as the clamp point. The
ceiling only ever NARROWS (`effective = user_grants ∩ (baseline ∪ scopes)`), so a
bug fails toward less access.

Attenuation must never widen. A `for_each` run can see at most what the user can
see, whatever the config says — that invariant is what makes the feature safe to
reason about, and it is worth a test that tries to widen and fails.

## Field-level redaction is fixed for ALL scheduled jobs, not just `for_each`

RR-7408F5 is open: `appbuild.ScheduledLuaWriteDeps` wires a **nil redactor**
(`appbuild.go:415-426`), so scheduled jobs get row gating only — a job reading
`person` receives every property, including ones a human with the same role
would see redacted in the UI.

That is not a considered trade-off, it is an accident of wiring. There is no
principled reason a scheduled job should see MORE than the same identity sees
interactively: `run_as` is an identity (DEC-O59WM4), and an identity's field
policy should not depend on which entry point happens to be reading. So this
closes for every scheduled job, not merely the iterated ones.

**It is wiring, not new machinery.** `luaWriteDepsFor` already takes a
`visibility.FieldRedactor` (`appbuild.go:403`) and only ever gets `nil`. The
redactor is `visibility.NewPolicyRedactor(*affordances.PolicyResolver)`
(`adapters.go:96`), and `affordances.New(meta, lookup, declarative)`
(`resolver.go:125`) needs exactly three things `Services` already holds: the
metamodel, a relation lookup (the store), and `aclDeclarative`
(`appbuild.go:115`). The dataentry equivalent is `appRedactor` (`app.go:386`).

Behaviour when no ACL policy is configured must stay byte-identical to today —
`affordances.New` returns a resolver with a nil policy, which redacts nothing,
so the NopACL path is unaffected.

This is a **behaviour change for existing deployments**: a scheduled Lua job that
reads a `visible:`-restricted property will stop seeing it. That is the point,
but it needs calling out in the changelog rather than landing silently, since a
script could be relying on the leak.

## Failure handling: continue, collect, retry only the failures

A run that fails does **not** stop the pass. The scheduler continues to the next
user and collects the ids of the users whose runs failed. Only that subset is
retried.

This is the only sensible reading once `for_each` exists: one bad user out of 200
must not deny the other 199 their digest, and re-running the 199 successes to
retry one failure would send each of them a duplicate message. So the retry unit
is the failed subset, never the whole task.

**This does not fit the existing ladder as-is, and that needs a decision.** Today
a failed task gets `state.NextRetry[task.Name]` (`state.go:41`), a persisted
per-task time, and the ladder **replaces** the schedule until it succeeds
(`scheduler.go:19-24`) — so a failing *daily* task retries in 5 minutes. Two
things follow that the current model has no answer for:

- The retry unit becomes a `(task, user)` set, not a task. `NextRetry` is keyed
  by task name only, so it cannot express "retry these 3 of 200".
- A daily digest that fails for one user would, under the existing ladder, re-fire
  the whole task 5 minutes later. For mail that is 199 duplicate messages — the
  ladder's "speeds a failing daily one up" behaviour is right for a sync job and
  wrong for a notification.

Two candidate resolutions, to settle in planning:

**(a) In-pass retry, no state change.** Retry failed users within the same
execution (a bounded number of immediate attempts), then report the task
succeeded-with-failures and let the *next* scheduled occurrence pick them up
naturally. `NextRetry` is never set by a partial failure, so the daily digest
stays daily. Simple, no schema change, and no duplicate mail. The cost is that a
user whose mail server was down for an hour waits until tomorrow.

**(b) Persist the failed subset.** Extend state with a per-task failed-user list
and let the ladder drive a retry pass over just those users. Recovers within
minutes rather than a day, at the cost of new persisted state, a growth bound on
that list, and reconciling it with a `for_each` query whose membership may have
changed between passes (a user who no longer matches must be dropped).

(a) is the smaller, safer v1 and is what the acceptance criteria below assume;
(b) is the natural follow-up if operators find a day too long to wait. Either
way the invariant is the same: **a user whose run succeeded is never re-run
within the retry of the same occurrence.**

Also true regardless: the failed-user list is per-execution and unpersisted under
(a), so a restart mid-pass loses it — consistent with the scheduler's existing
state model and with mail's best-effort delivery. A task reports failure if any
user still failed at the end, so a persistent fault stays visible rather than
being swallowed; the log names the affected users.

## Scope: IS NOT

- No new scheduling syntax beyond `for_each` (`every:` unchanged).
- **No parallel execution.** `for_each` means *iteration*, not concurrency. The
scheduler is deliberately sequential (`scheduler.go:193-225`) — one Lua VM, one
job at a time — and this ticket does not change that. N users means N runs, one
after another. The cost of that needs documenting rather than hiding (see
criterion 5).
- No general per-user scheduler state: the only per-run state is the failed-user
list described below, held for the duration of one task execution.

## Open questions

1. **Bounding.** N users means N sequential runs; a large graph makes a "daily"
task not daily. Needs a cap with a loud log, not silent truncation. Open: what
the default cap is, and whether it is a count, a wall-clock budget, or both. A
wall-clock budget is more honest for a scheduler (the thing that actually breaks
is the schedule, not the count), but it makes which users get skipped depend on
timing.

**Resolved — a user with no usable principal.** Earlier framed as
"unresolvable users"; the framing was backwards. `ResolvePrincipal` runs *raw
identifier → entity ID*, whereas `for_each` starts from entities a query already
returned, so there is nothing to resolve in that direction. The real case is a
selected entity whose `principal_property` is empty, or shared with another
entity — a data-integrity fault of exactly the kind the resolver already refuses
to guess through (`declarative.go:155`, ambiguous natural key). It is handled by
the rule above: log naming the entity, skip, add to the failed list. Not a
separate policy decision.

## Acceptance criteria

1. `for_each` runs the task once per matching entity, each with that entity's
principal on the ctx.
2. A run sees only what that user may see — both row gating **and** field-level
`visible:` redaction.
2a. Field redaction applies to **every** scheduled job, iterated or not
(closes RR-7408F5): a plain `run_as` Lua task no longer reads a
`visible:`-restricted property.
2b. With no ACL policy configured, scheduled reads are byte-identical to today.
2c. `attenuate:` narrows a run below the user's grants, and a config that
attempts to WIDEN beyond them grants nothing.
3. A task without `for_each` behaves exactly as today.
4. An entity with no usable principal (empty or ambiguous `principal_property`)
is skipped with a warning naming it, not silently and not fatally, and lands on
the failed list.
4a. A failing run does not stop the pass: the remaining users still run, and only
the failed subset is retried. A user whose run succeeded is never re-run within
the retry of the same occurrence.
4b. A partial failure does not drag the whole task onto the retry ladder — a
daily digest that fails for one user stays daily and does not re-fire in 5
minutes (see the failure-handling decision).
5. `for_each` is bounded, and hitting the bound logs what was dropped.
6. Audit records name the per-run principal, not a generic scheduler identity.
7. `rela validate` reports an unknown `entity_type` or unparseable `where:` in
`for_each`.

## Risks

- **Privilege confusion** — mitigated by keeping DEC-O59WM4 explicit: `for_each`
narrows, never elevates, and `attenuate:` can only narrow further.
- **Silent behaviour change** — closing RR-7408F5 removes properties a scheduled
script may currently read. Intended, but it belongs in the changelog: a script
relying on the leak will start seeing empty values.
- **Runtime blowup** — N sequential runs, plus a retry pass over the failures;
criterion 5 bounds it.
- **Duplicate side effects on retry** — the existing ladder retries a whole task
and *replaces* its schedule, so one failed user in a daily digest would re-fire
the task within 5 minutes and mail the other 199 again. This is the sharpest
constraint on the design; criteria 4a/4b exist to prevent it, and it is why the
retry unit cannot simply be the task.
- **Half-enforced scoping** — the reason RR-7408F5 is in scope rather than
deferred.

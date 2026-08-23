---
id: TKT-XWZIOB
type: ticket
title: 'Scheduler fan-out: run a task once per selected user, as that user'
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

The "perhaps with additional limitations" question is worth deciding explicitly:
whether a fanned-out run should be further attenuated below the user's own
grants (the `client_baselines` / `scope_grants` mechanism from TKT-IAC8TX
already compiles exactly this kind of ceiling, and would fit here as an optional
`as_role:` or scope restriction). Defaulting to the user's real grants is the
honest starting point; an attenuation knob can follow if a use case needs it.

## Field-level redaction must be closed with this

RR-7408F5 is currently open: `appbuild.ScheduledLuaWriteDeps` wires a **nil
redactor** (`appbuild.go:415-426`), so scheduled jobs get row gating only — a
job reading `person` receives every property, including ones a human with the
same role would see redacted.

That is tolerable today because a Lua job's read stays in-process. It is **not**
tolerable for fan-out, whose entire premise is "this run sees what that user
sees". Half-enforcing that is worse than not offering it, because the config
reads like a guarantee. The existing doc already names the fix: wire an
affordance resolver into appbuild.

## Scope: IS NOT

- No new scheduling syntax beyond `for_each` (`every:` unchanged).
- No parallel execution — the scheduler is deliberately sequential
(`scheduler.go:193-225`); fan-out runs serially, and the cost of that needs
documenting rather than hiding.
- No per-user state or retry granularity in v1: a failing run for one user
follows the existing task-level retry ladder. Whether that is right is an open
question below.

## Open questions

1. **Failure granularity.** One user's run fails — does the whole task enter the
retry ladder, or does it skip and continue? Skipping is probably right (one bad
user should not stop the other 200), but it changes what `recordFailure` means.
2. **Bounding.** N users means N runs; a large graph makes a "daily" task not
daily. Needs a cap with a loud log, not silent truncation.
3. **Attenuation.** Should a fanned-out run be further restricted below the
user's own grants? See above.
4. **Unresolvable users.** An entity with no `principal_property` value, or an
ambiguous match — skip with a warning, or fail the task?

## Acceptance criteria

1. `for_each` runs the task once per matching entity, each with that entity's
principal on the ctx.
2. A run sees only what that user may see — both row gating **and** field-level
`visible:` redaction (closes RR-7408F5).
3. A task without `for_each` behaves exactly as today.
4. An entity that cannot be resolved to a principal is skipped with a warning
naming it, not silently and not fatally.
5. Fan-out is bounded, and hitting the bound logs what was dropped.
6. Audit records name the per-run principal, not a generic scheduler identity.
7. `rela validate` reports an unknown `entity_type` or unparseable `where:` in
`for_each`.

## Risks

- **Privilege confusion** — mitigated by keeping DEC-O59WM4 explicit: fan-out
narrows, never elevates.
- **Runtime blowup** — N sequential runs; criterion 5 bounds it.
- **Half-enforced scoping** — the reason RR-7408F5 is in scope rather than
deferred.

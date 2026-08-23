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

**Attenuation is supported, defaulting to the user's own grants.** A fanned-out
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

## Field-level redaction is fixed for ALL scheduled jobs, not just fan-out

RR-7408F5 is open: `appbuild.ScheduledLuaWriteDeps` wires a **nil redactor**
(`appbuild.go:415-426`), so scheduled jobs get row gating only — a job reading
`person` receives every property, including ones a human with the same role
would see redacted in the UI.

That is not a considered trade-off, it is an accident of wiring. There is no
principled reason a scheduled job should see MORE than the same identity sees
interactively: `run_as` is an identity (DEC-O59WM4), and an identity's field
policy should not depend on which entry point happens to be reading. So this
closes for every scheduled job, not merely the fanned-out ones.

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
4. **Unresolvable users.** An entity with no `principal_property` value, or an
ambiguous match — skip with a warning, or fail the task?

## Acceptance criteria

1. `for_each` runs the task once per matching entity, each with that entity's
principal on the ctx.
2. A run sees only what that user may see — both row gating **and** field-level
`visible:` redaction.
2a. Field redaction applies to **every** scheduled job, fanned-out or not
(closes RR-7408F5): a plain `run_as` Lua task no longer reads a
`visible:`-restricted property.
2b. With no ACL policy configured, scheduled reads are byte-identical to today.
2c. `attenuate:` narrows a run below the user's grants, and a config that
attempts to WIDEN beyond them grants nothing.
3. A task without `for_each` behaves exactly as today.
4. An entity that cannot be resolved to a principal is skipped with a warning
naming it, not silently and not fatally.
5. Fan-out is bounded, and hitting the bound logs what was dropped.
6. Audit records name the per-run principal, not a generic scheduler identity.
7. `rela validate` reports an unknown `entity_type` or unparseable `where:` in
`for_each`.

## Risks

- **Privilege confusion** — mitigated by keeping DEC-O59WM4 explicit: fan-out
narrows, never elevates, and `attenuate:` can only narrow further.
- **Silent behaviour change** — closing RR-7408F5 removes properties a scheduled
script may currently read. Intended, but it belongs in the changelog: a script
relying on the leak will start seeing empty values.
- **Runtime blowup** — N sequential runs; criterion 5 bounds it.
- **Half-enforced scoping** — the reason RR-7408F5 is in scope rather than
deferred.

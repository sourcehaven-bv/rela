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

## Model it as a task-per-user factory

`for_each` is a **task factory**: at scheduling time it expands one declared task
into N derived tasks, one per matching entity, each carrying that entity's
principal. `runDueTasks` then iterates derived tasks exactly as it iterates
declared ones today.

This is the design decision that makes the retry question disappear rather than
needing an answer. Every state map is keyed by task NAME
(`state.go:38-41` — `Tasks`, `Failures`, `NextRetry`, all `map[string]...`), so
a derived task gets its own entry in each:

- `Tasks[name]` is that user's last **successful** run. A user who succeeded is
  not due again, so "never re-run a succeeded user" is a property of the existing
  schedule check, not bookkeeping this ticket has to add.
- `Failures[name]` / `NextRetry[name]` give each user their **own ladder**. One
  user failing suppresses only that user's cadence.
- "The ladder replaces the schedule" (`scheduler.go:19-24`) becomes correct
  instead of dangerous: it replaces *that user's* schedule. The other 199 keep
  their normal daily run; the failing one retries at 5m, 10m, 20m.

So the duplicate-mail hazard is not mitigated, it is **structurally impossible** —
a successful user is never re-run because their `Tasks` entry says they are not
due. And "persist the failed subset" is already built: it is `NextRetry`, keyed
per derived task.

There is also no "continue past a failure" special case to write.
`runDueTasks` already moves to the next task after one fails
(`scheduler.go:225-275`); with derived tasks, that IS continue-and-collect.

**Supersedes the earlier (a)/(b) fork.** Both options existed only because the
retry unit was assumed to be the declared task; making the derived task the unit
removes the question. Sequential execution is unchanged — N derived tasks run one
after another in the same goroutine.

### What the factory model does cost

State is now keyed by a set that CHANGES between ticks, which the current model
never had to handle:

1. **Stale entries.** A user who stops matching (`active = false`) leaves
   `Tasks`/`Failures`/`NextRetry` entries behind. Needs pruning against the
   current expansion, or `.rela/scheduler-state.json` grows without bound.
   Pruning must not disturb declared tasks — only derived names.
2. **New members run immediately.** A user with no `Tasks` entry hits the
   "first run, executing immediately" branch (`scheduler.go:264-268`). A person
   added at 14:00 gets a digest at 14:00 and another at the normal hour.
   Acceptable for a digest, wrong for a task with side effects — so a derived
   task's first occurrence should probably inherit the declared task's schedule
   rather than firing on sight. Needs deciding, not inheriting.
3. **Name collisions.** Derived names must be stable (same user → same name
   across restarts, or state is lost) and unable to collide with a declared task
   name. Use a separator that cannot appear in a hand-written name — `#` — so the
   namespaces are provably disjoint and a state-file entry is self-describing:
   `daily-digest#PERS-JV`.
4. **Log volume.** The per-task lines (`scheduled task`, `first run`,
   `retrying failed task`) fire per DERIVED task: a 200-user expansion turns one
   startup line into 200. Log the expansion once with a count, and keep per-user
   lines to failures.

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

2. **First run of a newly-matching user** — fire immediately (current
"unrecorded task" behaviour) or wait for the declared schedule? See cost 2 above.

**Not in scope — a missing email address.** Earlier listed here as
"unresolvable users". It does not belong to this ticket at all: `for_each`
resolves an entity to a *principal*, and whether that principal has a usable
address is a mail question. A per-user export or cleanup pass needs no address.
It goes to TKT-U2R7GU, where the recipient field is named in config and can be
asserted up front rather than discovered mid-send.

The only thing in scope here is an entity that cannot be mapped to a principal at
all: skipped with a warning naming it, counted as a failed run.

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
4. An entity that cannot be mapped to a principal is skipped with a warning
naming it, not silently and not fatally, and counts as a failed run. Address
validation is not in scope; see TKT-U2R7GU.
4a. A failing run does not stop the others: each derived task carries its own
`Failures`/`NextRetry` entry, so one user's failure suppresses only that user's
cadence and retries only that user.
4b. A partial failure does not drag the whole task onto the retry ladder — a
daily digest that fails for one user stays daily for the other 199.
4c. A user whose run succeeded is not re-run by another user's retry (falls out
of per-derived-task `Tasks` entries; worth a test that pins it).
4d. Derived-task state is pruned when a user leaves the selection, and declared
tasks are untouched by that pruning.
5. `for_each` is bounded, and hitting the bound logs what was dropped.
6. Audit records name the per-run principal, not a generic scheduler identity.
7. `rela validate` reports an unknown `entity_type` or unparseable `where:` in
`for_each`. This stays syntactic — `scheduler.Config.validate`
(`config.go:195`) has no store access.

## Risks

- **Privilege confusion** — mitigated by keeping DEC-O59WM4 explicit: `for_each`
narrows, never elevates, and `attenuate:` can only narrow further.
- **Silent behaviour change** — closing RR-7408F5 removes properties a scheduled
script may currently read. Intended, but it belongs in the changelog: a script
relying on the leak will start seeing empty values.
- **Runtime blowup** — N sequential runs, plus a retry pass over the failures;
criterion 5 bounds it.
- **Duplicate side effects on retry** — was the sharpest constraint: the ladder
retries a whole task and replaces its schedule, so one failed user would re-fire
a daily digest in 5 minutes and mail the other 199 again. The task-per-user
factory removes it structurally (a succeeded user is not due), so this is now a
regression to pin with a test rather than a design problem to solve.
- **Unbounded state growth** — derived entries accumulate as the selection
changes; criterion 4d.
- **Half-enforced scoping** — the reason RR-7408F5 is in scope rather than
deferred.

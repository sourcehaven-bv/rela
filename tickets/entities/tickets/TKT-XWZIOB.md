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

## Field redaction: split out to TKT-NJ91LX

RR-7408F5 (scheduled jobs wired with a nil field redactor) applies to every
scheduled job, not just iterated ones, so it landed in its own ticket:
**TKT-NJ91LX**. This ticket depends on it — a `for_each` run that saw
unredacted fields would be half-enforced scoping, which is worse than none.

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

### The seam: a TaskProducer

The factory is an interface, and there are two implementations:

```go
// TaskProducer supplies the tasks the scheduler should consider this tick.
type TaskProducer interface {
    Tasks(ctx context.Context) ([]TaskConfig, error)
}
```

- **Config producer** — returns `cfg.Tasks` verbatim. Total, infallible, no store
  access. This is today's behaviour, unchanged.
- **Query producer** — evaluates `for_each`, expanding one declared task into N
  derived `TaskConfig`s, each with a computed `Name` and that entity's principal
  in `RunAs`.

`TaskConfig` already carries `RunAs`, so a derived task needs no new type: the
engine, the ladder and the state maps keep working on `TaskConfig` by name and
learn nothing about users.

The seam is narrow because `config.Tasks` has exactly two consumers:
`runDueTasks` (`scheduler.go:229`) and `pruneOrphanedState` (`:418`). Both become
producer calls.

### A task identity, not a name

`task.Name` currently does three unrelated jobs: it is the **state key**
(`state.Tasks/Failures/NextRetry`), a **log field**, and the **audit identity**
(`audit.WithTriggeredBy(ctx, "schedule:"+taskName)`, `scheduler.go:153`). A
derived task needs all three to differ per user, and a string cannot say which
producer minted it. So the identity becomes a struct:

```go
type TaskID struct {
    Producer string // "config" | "for_each"
    Task     string // declared name, e.g. "daily-digest"
    Subject  string // derived entity id, e.g. "PERS-JV"; empty when declared
}
```

`Subject == ""` distinguishes declared from derived **structurally** — no code
parses a name to find out.

What this buys beyond provenance:

- **It removes the `#` separator hazard.** Collision-avoidance was previously
  "pick a character nobody would type in schedules.yaml", i.e. a convention
  enforced by hope. With a struct the distinction is a field, and `#` demotes to
  a serialization detail confined to one encode/decode pair.
- **It scopes liveness correctly.** Pruning today diffs ALL state against one
  live set (`:418-430`). With two producers that is wrong: a config expansion
  must not judge `for_each` entries dead, or a producer that errors (or is
  removed from the config) prunes another producer's ladders. `Producer` makes
  the diff per-producer, which is a correctness fix, not tidiness.
- **It gives audit an honest identity.** `schedule:daily-digest` for 200 users
  makes their writes indistinguishable. Note the principal ALREADY carries the
  user (`RunAs` → `principal.With`, `:149-152`), so `triggered_by` should name
  the task and let the principal name the user — encoding the user twice invites
  two fields that disagree.

Two constraints on the encoding:

1. **State is JSON keyed by string** (`state.go:38-41`), so `TaskID` needs a
   stable string form that round-trips. This is a compatibility surface: an
   existing state file has bare `daily-digest` keys, and the format is explicitly
   NOT forward compatible (`state.go:27-32`). The encoding MUST leave a declared
   task's key byte-identical to today, or every in-flight ladder resets on
   upgrade. A declared `TaskID` encodes to just `Task`.
2. **Config uniqueness is unchanged.** `config.go:206` still rejects duplicate
   declared names; that check operates on the config, before any identity is
   minted.

### Liveness is a batch question, asked only when a decision depends on it

**Batch, not per-task.** The producer is asked for the whole expansion at once
and liveness is set membership — there is no `IsLive(name)` method. That matches
what `pruneOrphanedState` already does (`:418-430`: build `live` once, diff all
three maps against it), it is one query instead of N, and it avoids an
inconsistency a per-task check would allow — the selection changing mid-tick, so
a user is present at derived task 3 and absent at task 40.

**And the expansion is NOT needed every tick.** For a daily digest that mostly
succeeds, the set is needed roughly once a day. The scheduler ticks every 60s
(`tickInterval`, `:48`), but `IsDue` for a `dayKind` schedule is
`truncateToDay(now) != truncateToDay(lastRun)` (`config.go:67-69`) — false for
all 1,439 intervening ticks. Querying each tick would be ~1,440 store queries a
day to answer a question that changes once.

The set is only needed when a decision actually depends on it: something is due,
or a retry is pending. Both can be checked BEFORE expanding, provided the check
does not itself need derived names:

1. **A pending retry is due.** `NextRetry` is non-empty only while something is
   failing — a pure map check, no query. If a retry is due, expand: the live set
   is needed to decide retry-vs-drop (criterion 4e).
2. **The declared task is due.** Requires a per-declared-task watermark (see
   below), so group dueness is decided before expansion.
3. **Neither.** No query. This is the steady state.

Cost for a daily digest with no failures: about one query per day, plus one per
failing user's retry — not one per tick.

**This needs one new piece of state.** The maps are keyed by DERIVED name, so
"is this group due?" cannot be answered from them without knowing the names,
which is what the expansion produces — circular. Record a per-declared-task
watermark (the declared name as a key in `Tasks`, distinct from derived entries
by the `#` separator) updated when a group pass completes. Without it, every tick
must expand just to discover it had nothing to do, which is the cost this section
exists to avoid.

**A pending retry must still be re-checked against a fresh set — but the
producer is not what asks.** `pruneOrphanedState` is already a degenerate
liveness check, except it derives its set from config and runs **once, in
`loadState` (`:403`)**. Sound when membership changes only on a config edit plus
restart; not sound for a query producer. The failure is concrete: a user is
deactivated at 09:00 holding a `NextRetry` entry; nothing re-prunes, so the
scheduler retries a task for a non-matching user on the 2h rung indefinitely.

Case 1 closes that, but the dependency direction matters (see the next section):
the ENGINE refreshes the set before honouring a pending retry, because it will
not act on stale liveness. The retry mechanism does not consult the producer, and
the producer never learns that a retry is why it was called.

### One run per occurrence

The occurrence idempotency key (`daily-digest@2026-08-23`) is specified in
**TKT-N52HRC**. For `for_each` the key extends with the subject
(`daily-digest#PERS-JV@2026-08-23`), which settles two questions that would
otherwise need answering here:

- **New members.** "Fire immediately or wait?" becomes "does today's occurrence
  exist for this user?" Someone added at 14:00 has no record for today, so they
  get that day's run once and the normal cadence after — no special case in the
  expansion.
- **Restart mid-pass.** Users already completed today are recorded and are not
  re-run.

### The producer does not know about retries

Retry is **engine state**, not producer state. The producer answers exactly one
question — *what tasks exist right now* — as a pure function of the world. It has
no notion of failure, backoff, or attempt count, and nothing in its interface
mentions them.

The engine owns `Failures`/`NextRetry`, decides when to run, and reconciles its
own state against whatever set the producer last returned. So liveness is not a
question the retry path *asks*; it is a property the engine checks against a set
it already holds.

This is why the dependency runs **engine → producer** and never
**producer ← retry**. A producer that had to be told "this is a retry" would be
one that could behave differently on a retry, which is exactly the coupling that
makes a scheduler hard to reason about.

The one thing that cannot be decoupled: detecting a retry pending for a task that
no longer exists requires comparing engine state to a FRESH producer result. That
is the engine refusing to act on stale liveness — same store query, dependency
pointing the right way.

### Retry semantics: split out to TKT-N52HRC

Bounded retries (a task always gives up rather than retrying forever) and the
per-occurrence idempotency key that must land with them are **TKT-N52HRC**. Both
apply to ordinary single-identity tasks, so they are not `for_each` work.

They matter here for two reasons, which is why this ticket depends on that one:

- Per-user expansion multiplies the current unbounded ladder: 200 users stuck at
  the 2h rung is ~2,400 failing executions a day.
- The occurrence key is what makes "one digest per user per day" true. Without
  it, a `for_each` mail digest can double-send across a day boundary.

Per-task `retry:` properties (`base`/`max`/`attempts`) belong on `TaskConfig`,
emitted by the producer as ordinary task data — which is what keeps the
producer/retry separation above intact: the producer emits a task that
*describes* its policy, the engine *implements* it. A derived task inherits the
declared task's props, so N users share one policy without the producer knowing
an attempt count exists.

### What the factory model costs

1. **A producer can fail, and empty must not mean gone.** Config cannot fail; a
   query can — store error, bad `where:`, a partially-loaded graph. If a
   transient failure yields an empty set and that is treated as authoritative,
   pruning deletes every derived entry: ladders and last-run times are wiped, and
   on recovery every user looks unrecorded and fires "first run, executing
   immediately" (`:264-268`) — a mass re-send. A producer error must therefore
   mean *skip this tick, change nothing*, and be distinguishable from a
   legitimately empty result. That is why `Tasks` returns `error` rather than
   just a slice, and it needs a test that pins "producer error does not prune".
2. **I/O on the decision path.** `runDueTasks` is currently pure map lookups at
   a 60s tick. Expanding only when a decision depends on it (above) keeps a
   healthy daily task at ~1 query/day, so a refresh-interval cache is NOT needed
   for the common case. What remains: a short-interval task (`every: 30m`)
   expands on every occurrence, and a task stuck on the 5m rung expands every 5
   minutes. Both are bounded and proportionate, but the query cost should be
   logged so an operator can see it.
3. **New members run immediately.** A user with no `Tasks` entry hits the "first
   run, executing immediately" branch (`:264-268`), so someone added at 14:00
   would get a digest at 14:00 and another at the normal hour. Resolved by the
   occurrence key: they have no record for today's occurrence, so they get it
   once. Listed here because the fix is the occurrence key, not a special case in
   the expansion.
4. **Identity encoding.** A derived `TaskID` must encode stably (same user →
   same key across restarts, or state is lost) and must not collide with a
   declared key. `TaskID` makes the distinction structural; the encoding is the
   remaining risk, and is pinned by a round-trip test plus one asserting a
   declared task's key is byte-identical to a pre-upgrade state file.
5. **Log volume.** Per-task lines (`scheduled task`, `first run`, `retrying
   failed task`) fire per DERIVED task: 200 users turns one startup line into
   200. Log the expansion once with a count; keep per-user lines to failures.

## Not a job queue (yet)

This ticket derives task producers, per-task identity, bounded retries and
occurrence keys — the components of a job queue, one at a time. That is a real
signal, and it is already captured: **IDEA-WIJ2H1** proposes a `Queue` seam with
memory / postgres / redis backends and a conformance suite, in the
`internal/store/storetest` posture.

**It stays out of scope here, and an off-the-shelf queue does not change that.**
River (`riverqueue.com`, MPL-2.0) was considered and does not fit as the
persistence design, because the scheduler is not database-backed on every build:

- `WorkspaceProvider` requires `State() state.KV` (`scheduler.go:83`), and
  `state.KV` is backend-swapped by build tag — FSKV under `.rela/` by default,
  `pgstore.StateKV` on postgres (TKT-VC27L3).
- Three entry points start it: `rela scheduler` (`cli/scheduler.go:32`),
  `rela-server` (`main.go:521`) and **`rela-desktop`** (`main.go:208`). A desktop
  app that needs a PostgreSQL instance to send a daily reminder is a non-starter.
- CI hard-asserts the default build does not link pgx
  (`ci.yml:488-496`), so such a library could not be imported unconditionally.

River needs a SQL database: Postgres (its target), or SQLite, which its own docs
call **experimental** and which runs at roughly a quarter of the Postgres
throughput. There is no in-memory client — a driver can be constructed with a nil
pool for insert-only tests, but worker execution needs a real database. It also
requires **5 tables of its own** (`river_job`, `river_leader`, `river_queue`,
`river_migration`, `river_notification`) plus an enum, via its own migration line.

So it could only ever sit BEHIND the queue seam as one backend, with a
memory/fs implementation still required and still needing every semantic derived
here. **The seam is the work, and adopting a library does not remove it.**

Three further mismatches, recorded so this is not re-litigated from the feature
list alone:

- **Its periodic scheduling is the wrong half.** River's `PeriodicJobs` keeps
  schedules **in memory** and resets them across restarts; *durable* periodic
  jobs are a **Pro** (paid) feature. Runtime add/remove exists but only applies
  to the cluster leader. Since durable scheduling is exactly what we would be
  adopting it for, the free tier does not supply it.
- **Its uniqueness is not our occurrence key.** `UniqueOpts.ByPeriod` rounds the
  current time down to a period multiple, which is a *time-window* key, not the
  schedule-derived occurrence we specified. And it guarantees single INSERTION —
  execution stays at-least-once, so "one digest per user per day" would still
  need enforcing above it.
- **Its retry defaults are a different posture.** 25 attempts over ~3 weeks with
  `attempts^4` backoff, exhaustion moving to `discarded` for 7 days; a dead-letter
  queue is again Pro. Our bounded ladder returning a task to its ordinary
  schedule has no equivalent.

It is also still **0.x** (v0.44.x as of 2026-08), actively changing its schema,
and its `riverdriver.Driver` is a wide producer-side interface — against this
repo's consumer-side-interface rule, so it would want a narrow local interface at
the enqueue site regardless.

Sequencing follows IDEA-WIJ2H1's existing argument: the consumer proves the
shape first. The semantics specified in this ticket are what a queue would have
to honour anyway, so they are the conformance suite written in advance against a
real workload, not throwaway work.

## Scope: IS NOT

- No new scheduling syntax beyond `for_each` (`every:` unchanged).
- **No parallel execution.** `for_each` means *iteration*, not concurrency. The
scheduler is deliberately sequential (`scheduler.go:193-225`) — one Lua VM, one
job at a time — and this ticket does not change that. N users means N runs, one
after another. The cost of that needs documenting rather than hiding (see
criterion 5).
- No durable job queue, and no external queue library — see above. State stays in
`.rela/scheduler-state.json` via the existing `state.KV`.

## Open questions

1. **Bounding.** N users means N sequential runs; a large graph makes a "daily"
task not daily. Needs a cap with a loud log, not silent truncation. Open: what
the default cap is, and whether it is a count, a wall-clock budget, or both. A
wall-clock budget is more honest for a scheduler (the thing that actually breaks
is the schedule, not the count), but it makes which users get skipped depend on
timing.

2. **Occurrence for interval schedules.** `every: 30m` has no natural
occurrence boundary — derive one from the slot, or exempt interval tasks from the
idempotency key. (The newly-matching-user question is resolved by the occurrence
key; see above.)

**Not in scope — a missing email address.** Earlier listed here as
"unresolvable users". It does not belong to this ticket at all: `for_each`
resolves an entity to a *principal*, and whether that principal has a usable
address is a mail question. A per-user export or cleanup pass needs no address.
It goes to TKT-U2R7GU, where the recipient field is named in config and can be
asserted up front rather than discovered mid-send.

The only thing in scope here is an entity that cannot be mapped to a principal at
all: skipped with a warning naming it, counted as a failed run.

## Acceptance criteria

Numbered fresh; the retry/redaction criteria moved with their tickets.

1. `for_each` runs the task once per matching entity, each with that entity's
principal on the ctx.
2. A run sees only what that user may see — row gating and, via TKT-NJ91LX,
field-level `visible:` redaction.
3. `attenuate:` narrows a run below the user's grants, and a config that attempts
to WIDEN beyond them grants nothing.
4. A task without `for_each` behaves exactly as today, including the existing
`pruneOrphanedState` semantics for declared tasks.
5. An entity that cannot be mapped to a principal is skipped with a warning
naming it, not silently and not fatally, and counts as a failed run. Address
validation is not in scope; see TKT-U2R7GU.
6. One user's failure suppresses only that user's cadence and retries only that
user: each derived task carries its own `Failures`/`NextRetry` entry.
7. Derived-task state is pruned when a user leaves the selection, declared tasks
are untouched by that pruning, and pruning is scoped per producer — one
producer's expansion or failure never removes another's state.
8. A pending retry for a task the producer no longer yields is dropped, not
retried indefinitely.
9. A producer error leaves state untouched: no pruning, no execution, and no mass
"first run" on the following tick.
10. A task that is neither due nor retrying does not query the store: a daily
digest in steady state expands about once a day, not once a tick.
11. `for_each` is bounded, and hitting the bound logs what was dropped.
12. `TaskID` round-trips through its string encoding, and a declared task's state
key is unchanged from a pre-upgrade state file — upgrading does not reset an
in-flight ladder.
13. The `TaskProducer` interface mentions nothing about retries, failures or
attempts, and a producer cannot observe that a call was prompted by a pending
retry.
14. Audit records distinguish per-user runs of the same declared task; the user
comes from the principal, not from a second encoding in `triggered_by`.
15. `rela validate` reports an unknown `entity_type` or unparseable `where:` in
`for_each`. This stays syntactic — `scheduler.Config.validate`
(`config.go:195`) has no store access.

## Risks

- **Privilege confusion** — mitigated by keeping DEC-O59WM4 explicit: `for_each`
narrows, never elevates, and `attenuate:` can only narrow further.
- **Runtime blowup** — N sequential runs; criterion 11 bounds it.
- **Duplicate side effects on retry** — was the sharpest constraint: the ladder
retries a whole task and replaces its schedule, so one failed user would re-fire
a daily digest in 5 minutes and mail the other 199 again. The task-per-user
factory removes it structurally (a succeeded user is not due), so this is now a
regression to pin with a test rather than a design problem to solve.
- **Unbounded state growth** — derived entries accumulate as the selection
changes; criteria 7/8.
- **State-key format change** — encoding `TaskID` into the existing string-keyed
JSON risks resetting every in-flight ladder on upgrade; criterion 12.
- **Cross-producer pruning** — a shared live set would let one producer delete
another's entries; criterion 7.
- **Mass re-send after a store blip** — a producer error misread as "no tasks"
prunes every derived entry and makes every user look like a first run. Criterion
9; the single most damaging failure mode this design introduces.
- **Query volume** — bounded by expanding only when a decision needs the set
(criterion 10); the residual is short-interval tasks and fast retry rungs.
- **Half-enforced scoping** — a `for_each` run without field redaction would be
worse than none; the dependency on TKT-NJ91LX exists for that reason.

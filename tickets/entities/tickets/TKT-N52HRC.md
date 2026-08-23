---
id: TKT-N52HRC
type: ticket
title: Bounded retries and per-occurrence idempotency for scheduled tasks
kind: enhancement
priority: medium
effort: m
tags:
    - needs-design
status: backlog
---

## Description

Two defects in the scheduler's retry model, split out of TKT-XWZIOB because they
are independent of `for_each` and affect **every** scheduled task today.

### 1. Retries are unbounded

`retryDelay` (`scheduler.go:373`) climbs 5m → 2h and then repeats at the cap
forever (`:51-52`). A permanently broken task retries ~12 times a day
indefinitely.

The current comment (`:53-56`) defends this: the cap *"keeps a persistently
broken job to roughly a dozen attempts a day rather than silencing it"* — the
fear being that a bounded retry means a silently dead job. That fear is answered
by what "gives up" means below, not by retrying forever.

**A task always gives up after a set number of attempts.** Retrying past the
ladder's length has no value: if the ladder has run out, the fault is not the
transient blip retry exists to absorb.

**Giving up ends the LADDER, not the task.** This distinction is the whole
design, and the other reading is dangerous:

- *Wrong:* the task is abandoned permanently. A daily digest failing during a
two-hour mail outage would be dead until someone restarts the process — a
transient fault silently promoted to a permanent one. Strictly worse than today.
- *Right:* on exhausting attempts, clear `NextRetry`/`Failures` and log at ERROR.
The task returns to its ORDINARY schedule; tomorrow's run happens normally.

Nothing is silenced either way: `persistentFailureThreshold` (4, `:64`) still
escalates WARN→ERROR *before* the give-up point.

Note this bounds the *ladder*, not lifetime attempts. An `every: 30m` task that
is permanently broken returns to its own cadence and still fails ~48 times a day
— at the schedule the operator declared, which is the right answer.

Default the bound to `maxLadderSteps` (`:71-78`), the count at which doubling
first reaches `maxRetryDelay`. Beyond that every retry is identical, so it is
exactly where continuing stops adding information. Derive it rather than writing
a literal — that variable exists for this reason.

### 2. An occurrence can run twice

`IsDue` for `dayKind` is `truncateToDay(now) != truncateToDay(lastRun)`
(`config.go:67-69`), and `Tasks[name]` is stamped **only on success**
(`scheduler.go:325`). A task that fails all Monday never stamps, so at midnight
it is due again — correctly, as a fresh run for Tuesday. But its ladder is still
live, so two things want to fire: the 2h-rung retry of *Monday's* run and
*Tuesday's* scheduled run.

Today this is masked: a pending `NextRetry` suppresses the ordinary schedule
entirely (`:234-240`), so the retry wins and Tuesday is silently skipped — one
run, for the wrong day.

**These two changes must land together.** Bounding the ladder clears
`NextRetry`, which removes the suppression that currently hides the overlap.
Shipping bounded retry alone would turn a silent skip into a duplicate run — for
a mail digest, a duplicate message.

**Key runs by occurrence.** The occurrence is the natural idempotency key:

```
daily-digest@2026-08-23
```

"Has this already run?" becomes a state lookup rather than an inference from
timestamps, and the retry semantics fall out: a retry targets the SAME
occurrence key, so it cannot produce a second execution. It also settles the
give-up case — yesterday's occurrence is abandoned and can never run today,
because today is a different key.

**Retain the last occurrence, not the history.** Keeping every occurrence grows
state without bound in a file rewritten on every `saveState`. Only the most
recent occurrence per task is needed to answer "is this occurrence done?", so
state stays at one entry per task, exactly as today.

That makes this a change of MEANING to an existing field rather than a new
table: `Tasks[id]` stores the last completed **occurrence** rather than a bare
timestamp. It is therefore a state-format change — an old file's timestamp must
map onto an occurrence without resetting an in-flight ladder.

## Decided: interval schedules are exempt from the occurrence key

`dayKind` and `weekdayKind` have obvious boundaries; `every: 30m` does not, and
an interval is a **soft request** — a bit more or less elapsed time is fine by
design. So interval tasks keep today's `lastRun`-relative behaviour
(`config.go:75`, `now.Sub(lastRun) >= s.interval`) and carry no occurrence key.

The occurrence key exists to stop a *calendar* double-run: Monday's retry firing
alongside Tuesday's scheduled run. An interval task has no calendar boundary to
collide across, so it cannot suffer that failure.

Deriving a slot was considered and rejected. `parseSchedule` (`config.go:140-178`)
accepts any positive duration, so slots need not divide a day: a `7h` interval
gives 3.43 slots/day, which drift across midnight. `time.Truncate` also works on
absolute time since the epoch, so on a DST changeover a local 14:00 truncates to
12:00 CEST — the wrong boundary. And it would change `IsDue` semantics for every
existing interval task, firing on slot edges instead of N after the last run: a
silent behaviour change nobody asked for.

Consequence: `Tasks[id]` holds an occurrence for day/weekday schedules and a
timestamp for interval ones, so the encoding must be tagged and the difference
documented.

**Constraint to preserve** (`docs/scheduled-tasks.md:219-230`): a scheduler down
Monday→Wednesday performs ONE catch-up run, not two. The key records the last
COMPLETED occurrence; it does not enumerate missed ones.

## Note: interval parsing has no minimum, and a regex wart

`parseSchedule` enforces only `d > 0` — `every: 1m` is accepted despite
`tickInterval` being 60s (`scheduler.go:48`), so it can fire at most once per
tick. Out of scope here, but worth knowing.

Separately, `durationRe` (`config.go:119`) admits uppercase `M`/`H` as its prefix
gate while `time.ParseDuration` rejects them, so `every: 7H` passes the regex and
then fails with a confusing error. One-line fix whenever that file is next
touched.

## Scope: IS NOT

- Not `for_each` (TKT-XWZIOB) — this applies to ordinary single-identity tasks.
- Not a durable queue (IDEA-WIJ2H1). State stays in
`.rela/scheduler-state.json` via the existing `state.KV`.

## Acceptance criteria

1. Retries are bounded for every task: after the configured number of
consecutive failures the ladder is abandoned, `NextRetry`/`Failures` are
cleared, and an ERROR is logged naming the task.
2. A task that gave up still runs at its next scheduled occurrence. Pinned by a
test: fail past the bound, advance the clock to the next occurrence, assert it
runs.
3. A given occurrence runs at most once: a retry targets the same occurrence and
cannot produce a second execution, and a task whose ladder is still live when
the next occurrence arrives does not run both.
4. Upgrading from an existing `scheduler-state.json` does not reset an in-flight
ladder or re-run a completed occurrence.
5. The give-up bound derives from `maxLadderSteps`, not a literal.

## Risks

- **Bounded retry read as "task disabled"** — if giving up were taken to mean
abandoning the task, an outage would permanently kill a daily job. Criterion 2
exists to stop that reading being implemented.
- **Duplicate runs if landed piecemeal** — clearing the ladder without the
occurrence key introduces the double-run described above. The two halves are one
change.
- **Behaviour change for existing deployments** — a permanently failing task
stops retrying every 2h and reverts to its schedule. Belongs in the changelog.

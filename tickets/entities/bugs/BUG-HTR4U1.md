---
id: BUG-HTR4U1
type: bug
title: Docs describe weekday schedules as ISO-week-change; code fires on target-weekday-passed
description: docs/scheduled-tasks.md 'Missed Run Detection' says week tasks are missed if the ISO week changed since the last run. Schedule.IsDue actually fires when the most recent occurrence of the target weekday is after lastRun. The two disagree in BOTH directions, so the doc misleads rather than merely simplifying.
priority: low
effort: xs
status: backlog
---

## Symptom

`docs/scheduled-tasks.md` § "Missed Run Detection" documents weekday schedules
as ISO-week-based:

> - **Week tasks**: missed if the ISO week changed since the last run

`Schedule.IsDue` (`internal/scheduler/config.go:70-74`) implements something
different — it fires when the most recent occurrence of the **target weekday**
is after `lastRun`:

```go
case weekdayKind:
    target := mostRecentWeekday(now, s.weekday)
    return target.After(lastRun)
```

ISO week number is never computed anywhere in the package (`ISOWeek` appears
nowhere in `internal/scheduler`).

## The two disagree in BOTH directions

Demonstrated against the real `IsDue` with a `friday` schedule:

| Case | lastRun → now | ISO week | Docs | Actual |
| --- | --- | --- | --- | --- |
| A | Thu 2026-04-09 → Fri 2026-04-10 | 2026-W15 → 2026-W15 (**same**) | not due | **due** |
| B | Sat 2026-04-11 → Mon 2026-04-13 | 2026-W15 → 2026-W16 (**changed**) | due | **not due** |

So this is not a doc that simplifies the truth — it is wrong in each direction.
Case B is the one that bites an operator: they read "ISO week changed → it
runs", restart the scheduler on Monday expecting Friday's missed job to fire,
and it does not (correctly, per the actual design — a `friday` task waits for
Friday).

The actual behaviour is the more useful one, and matches the `Schedule` godoc
and the "Schedule Values" section, which both describe weekday names as "once
per week on that weekday". Only the Missed Run Detection bullet is wrong.

## Fix

Documentation only — the code is right. Correct the bullet in the **source of
truth** `docs-project/entities/guides/GUIDE-scheduled-tasks.md` (roughly line
222; `docs/scheduled-tasks.md` is auto-generated from it and must be regenerated
with `scripts/generate-docs.sh`, never hand-edited — see RR-U2LC1P).

Suggested wording:

> - **Weekday tasks**: missed if the target weekday has occurred since the
>   last run

Worth also adding a `config_test.go` case pinning the two rows above, so the
described semantics are enforced rather than just asserted in prose.

## Provenance

Found while fixing BUG-ZKK2UL (scheduler failure backoff). Recorded rather than
swept into that change, since it is unrelated to the retry ladder and touches a
different section of the same guide.

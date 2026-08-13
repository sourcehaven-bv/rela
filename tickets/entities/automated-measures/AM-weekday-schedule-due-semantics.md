---
id: AM-weekday-schedule-due-semantics
type: automated-measure
title: Weekday schedule due-semantics match the documented rule
description: Table test pinning Schedule.IsDue for weekdayKind against the two cases where the old ISO-week wording diverged, so the prose and the code cannot drift apart again.
kind: test
location: internal/scheduler/config_test.go (test to be written with the fix)
status: proposed
---

Regression test for BUG-HTR4U1.

Pins `Schedule.IsDue` for `weekdayKind` against the two cases that proved the
ISO-week wording wrong in both directions (a `friday` schedule):

| lastRun → now | ISO week | Expected |
| --- | --- | --- |
| Thu 2026-04-09 → Fri 2026-04-10 | same (2026-W15) | **due** |
| Sat 2026-04-11 → Mon 2026-04-13 | changed (W15 → W16) | **not due** |

The first case would be "not due" under ISO-week semantics; the second would be
"due". Asserting both directions is what makes the test a guard against the
documentation drifting back toward an ISO-week description, rather than merely
restating the current implementation.

Worth adding a same-day case (`lastRun` and `now` both on the target weekday,
after a run) to pin that the task does not re-fire within the day.

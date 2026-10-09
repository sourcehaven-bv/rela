---
id: RR-PWKM1W
type: review-response
title: Backlog tests silently depend on the driver batch size
finding: If a SweepNow driver's batch exceeds sweepBacklogRows, every backlog test passes without testing anything.
severity: minor
resolution: NeverCapturedBacklogDrains asserts that one tick leaves rows uncaptured, so a driver batch at or above the backlog fails the test.
status: addressed
---

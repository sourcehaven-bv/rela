---
id: BUG-WKL0M2
type: bug
title: Collapsed background-job trigger resets the hop count
description: A trigger that collapses into a running background job made the job rerun with its own hop count, so jobs that trigger each other while running could outlast maxAutomationJobHops. Seen as a flaky TestAutomationJobs_QueuedChainStops.
priority: medium
why1: The job's rerun loop ran with the hop count from its own payload, not from the trigger that caused the rerun.
why2: The collapsed trigger left only an opaque token in the state store, so its hop count was lost.
why3: The hop limit was designed per enqueued job, and the collapse path added later bypasses enqueue.
why4: The collapse path was added in code review to cover a save racing a running job, and its tests checked that the save was handled, not the chain's hop count.
why5: Invariants that span two code paths (enqueue and the rerun loop) had a test for only one of them, and the overlap only shows under load, so a timing test hid it as a flake.
prevention: The trigger token now carries its hop count and a rerun adopts the higher count. The test asserts the exact hop sequence 1..8, so a reset fails it every time it occurs instead of by a timeout.
started: "2026-10-08"
completed: "2026-10-08"
status: done
---

## Description

Failed once in the Test job on PR #1809 (run 37799559868): `Condition never
satisfied` after 5 s at internal/appbuild/automationjobs_internal_test.go:316.
Passed on rerun. Locally it passes 170 runs, including 8 parallel copies with
-cpu=1.

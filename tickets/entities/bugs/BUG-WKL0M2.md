---
id: BUG-WKL0M2
type: bug
title: TestAutomationJobs_QueuedChainStops flaky in CI
description: 'TestAutomationJobs_QueuedChainStops failed once in CI on PR #1809 (condition never satisfied after 5 s) and passed on rerun; it does not reproduce locally.'
priority: low
status: backlog
---

## Description

Failed once in the Test job on PR #1809 (run 37799559868): `Condition never
satisfied` after 5 s at internal/appbuild/automationjobs_internal_test.go:316.
Passed on rerun. Locally it passes 170 runs, including 8 parallel copies with
-cpu=1.

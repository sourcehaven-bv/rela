---
id: AM-automation-job-chain-test-deterministic
type: automated-measure
title: Queued chain test asserts the hop sequence
description: TestAutomationJobs_QueuedChainStops records the hop count of every run and requires exactly 1..8, so a hop reset fails the test directly.
kind: test
location: internal/appbuild
status: active
---

The queued-chain hop-limit test must not depend on wall-clock timing; it waits
on a completion signal from the queue.

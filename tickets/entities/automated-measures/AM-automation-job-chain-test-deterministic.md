---
id: AM-automation-job-chain-test-deterministic
type: automated-measure
title: Automation job hop-limit test is deterministic under CI load
description: The queued-chain hop-limit test waits on a completion signal from the queue instead of a wall-clock deadline.
kind: test
location: internal/appbuild
status: proposed
---

The queued-chain hop-limit test must not depend on wall-clock timing; it waits
on a completion signal from the queue.

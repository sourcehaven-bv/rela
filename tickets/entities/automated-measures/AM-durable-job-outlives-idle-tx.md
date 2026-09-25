---
id: AM-durable-job-outlives-idle-tx
type: automated-measure
title: A durable job that runs longer than the idle-transaction timeout completes exactly once
description: Runs a handler longer than the backend's idle-in-transaction timeout on NewPostgresQueue and asserts one execution; a processed row; and that the idempotency key is released.
kind: test
location: internal/jobs/jobstest (postgres conformance)
status: proposed
---

---
id: AM-unknown-job-kind-not-dropped
type: automated-measure
title: A queue process never consumes a job kind it has no handler for
description: 'A jobstest conformance case: a queue with no handler for a kind leaves a pending job of that kind for a process that registers it, on every backend.'
kind: test
location: internal/jobs/jobstest
status: proposed
---

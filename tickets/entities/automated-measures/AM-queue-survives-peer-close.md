---
id: AM-queue-survives-peer-close
type: automated-measure
title: A queue keeps processing after another process on the same database closes its queue
description: Starts a second queue on the same database and closes it; asserts the first queue still runs a newly enqueued job.
kind: test
location: internal/jobs/pgqueue_stall_test.go (TestPostgresQueue_SurvivesAnotherProcessClosing)
status: active
---

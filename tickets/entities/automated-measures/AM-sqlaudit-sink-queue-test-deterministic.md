---
id: AM-sqlaudit-sink-queue-test-deterministic
type: automated-measure
title: Sink queue-bound test pins the writer before filling the queue
description: TestSink_QueueIsBounded holds the writer goroutine inside its first append before it fills the queue. The writer cannot take a batch off the full queue so the dropped-record assertion no longer depends on scheduling.
kind: test
location: internal/audit/sqlaudit (TestSink_QueueIsBounded)
status: active
---

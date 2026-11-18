---
id: BUG-14XO1JF
type: bug
title: Flaky TestSink_QueueIsBounded under -race
description: 'TestSink_QueueIsBounded failed in the post-merge -race run (issue #2447). It expected one dropped record and got none. The writer goroutine could take a batch off the filled queue before Record ran.'
priority: medium
effort: xs
why1: The writer goroutine took 500 records off the queue after the test filled it and before Record ran. The queue was then not full and nothing was dropped.
why2: newSink starts the writer goroutine and the test filled the queue without first making sure that goroutine was parked. If it had not yet reached cond.Wait it saw a non-empty queue and took a batch.
why3: The test relied on goroutine scheduling order. Locally the goroutine almost always parks first so the race only shows on a loaded CI runner.
prevention: The test now holds the writer inside its first append before it fills the queue. Tests that inspect the sink queue must first pin the writer in a known state.
status: ready
---

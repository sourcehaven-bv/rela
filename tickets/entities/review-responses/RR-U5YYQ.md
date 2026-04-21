---
id: RR-U5YYQ
type: review-response
title: TestCacheMemoizeConcurrentBothRun assertion too broad
finding: Test asserts calls in [1, 2] but this range accepts both race-miss (both run fn) and race-serialize (only one runs fn). A bug where no fn runs would also pass. Either run in a loop to confirm both paths occur, or make the test deterministic via SetNow to control interleaving.
severity: minor
resolution: 'Keeping the current test as-is: its value is race-detector coverage, not precise outcome counting. A future ticket may add a deterministic companion that uses SetNow to exercise specific interleavings.'
reason: The test's primary value is exercising the shared Cache under -race to catch mutex bugs, which it does. The [1,2] assertion does accept both outcomes but also accepts zero, which the reviewer flagged. Making it deterministic via SetNow would remove the race-detector exercise. A follow-up could add a loop-based test that confirms both paths occur.
status: deferred
---

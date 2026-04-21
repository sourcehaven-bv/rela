---
id: RR-LMIAE
type: review-response
title: TestCacheErrorMessagesDoNotLeakKey has dead longKey setup
finding: Test constructs longKey with a marker then immediately discards it with `_ = longKey`. Either delete the dead code and rename the test to reflect it only tests unrepresentable-value path, or test the long-key path properly by asserting the error format contains the length but not the marker.
severity: minor
resolution: 'Rewrote TestCacheErrorMessagesDoNotLeakKey to test three concrete paths: unrepresentable-value with marker in key, long-key with marker embedded, unknown-option with marker in value. Each asserts the marker is absent from the error and (for long-key) that the length is reported. Removed dead longKey construction.'
status: addressed
---

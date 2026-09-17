---
id: 'docscapture-capture-retry'
type: 'automated-measure'
title: 'Test: a screenshot capture retries a transient failure and fails fast on a deterministic one'
description: 'Guards against BUG-EB2BGR. Three layers, because the two halves of the policy can regress independently. (1) TestRetryableCapture pins the classifier against the two error strings CI actually produced (the devtools dial timeout and the capture deadline) plus each deterministic sentinel. (2) TestCapture_RetryPolicy pins the loop as attempt counts through a test seam: a transient failure that clears is invisible, two of them still succeed, a persistent one exhausts the budget, and a deterministic one fails on the FIRST attempt so a broken figure is not slowed threefold. (3) TestCapture_RetriesAgainstRealBrowser runs a real capture, kills the browser mid-flight and asserts a PNG is still produced — the mocked seam cannot catch a wrongly wired relaunch, which is the bug that actually occurred while making this fix. Mutation-tested: captureAttempts=1 fails the transient cases, bypassing the classifier fails the deterministic case, and removing the browser reset reproduces the original ''context canceled''.'
kind: 'test'
location: 'internal/docscapture/retry_test.go'
status: 'active'
---

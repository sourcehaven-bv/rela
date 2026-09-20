---
id: 'docscapture-capture-retry'
type: 'automated-measure'
title: 'Test: a screenshot capture retries a transient failure and fails fast on a deterministic one'
description: 'Guards against BUG-EB2BGR. Four layers, because the two halves of the policy can regress independently. (1) TestRetryableCapture pins the classifier against the two error strings CI actually produced (the devtools dial timeout and the capture deadline) plus each deterministic sentinel. (2) TestCapture_RetryPolicy pins the loop as attempt counts through a test seam: a transient failure that clears is invisible, two of them still succeed, a persistent one exhausts the budget, and a deterministic one fails on the FIRST attempt so a broken figure is not slowed threefold. (3) TestCapture_RetriesAgainstRealBrowser runs a real capture, kills the browser mid-flight and asserts a PNG is still produced — the mocked seam cannot catch a wrongly wired relaunch, which is the bug that actually occurred while making this fix. (4) internal/docs/island_timeout_test.go pins the ceiling that makes the retry reachable at all: a screenshot island must get the screenshot build''s deadline rather than the bare Tier-A buildTimeout, and the build-wide context must still bound the total so widening the per-island cap does not let N islands escape. Mutation-tested: reverting rlua.WithTimeout(deadline) to buildTimeout reproduces the exact CI failure (lua: <string>:2: context deadline exceeded), captureAttempts=1 fails the transient cases, bypassing the classifier fails the deterministic case, and removing the browser reset reproduces the original ''context canceled''.'
kind: 'test'
location: 'internal/docscapture/retry_test.go, internal/docs/island_timeout_test.go'
status: 'active'
---

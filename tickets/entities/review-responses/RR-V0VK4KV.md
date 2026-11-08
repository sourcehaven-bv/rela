---
id: RR-V0VK4KV
type: review-response
title: tooLarge splits on local addresses without limit
finding: 'An automated push security review (MEDIUM): tooLarge() split a span at local address bounds and gave up the count check. A remote that answers too_large to every fingerprint drove the split down to single elements and so learned local addresses, bypassing the small-span rule.'
severity: significant
resolution: 'tooLarge now splits a single-type span only when it holds more than bucketRows (1500) local elements, once, into parts of at most bucketRows; any other span is listed. A remote learns at most one local address per 1500 local elements this way. Tests: TestDiff_TooLargeAlwaysLeaksFewBounds and TestDiff_TooLargeSmallSpanIsListed use a remote that answers too_large to every fingerprint. Midpoint bounds that reveal no local address are deferred: they gain little for dense ids.'
status: addressed
---

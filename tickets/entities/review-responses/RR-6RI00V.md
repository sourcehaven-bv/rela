---
id: RR-6RI00V
type: review-response
title: 'Minor cleanups: duplicated return type, weak cache test, arity-coupled assertions, e2e comment tense'
finding: Four minors from the review. (7) The list return shape was restated inline on fetchList, fetchAllList and fetchListInternal. (8) The test "does not serve a single-page cache entry to an all-pages caller" only asserted a call count, so it passed on a cold cache even with the mode removed from the key -- it could not fail for its own reason. (9) toHaveBeenCalledWith('ticket', undefined) coupled tests to the call arity rather than its meaning. (11) The e2e spec's WHY section described the pre-fix code in present tense.
severity: minor
resolution: (7) Extracted a named ListResult type used by all three. (8) Gave the two mocks distinguishable payloads and asserted the caller receives the all-pages DATA; mutation-verified -- removing the mode from listCacheKey now fails this test, which it did not before. (9) Switched to checking the first argument only; this immediately caught a pre-existing test that broke when signal was added, demonstrating the coupling. (11) Rewrote in past tense and noted the fix.
status: addressed
---

---
id: RR-SB0IPG
type: review-response
title: Whole-cache clear + undebounced RelationChange per relation write — cost cliff once membership refresh is real
finding: 'Code review S2: pumpStoreEvents broadcast a RelationChange per relation store event, undebounced, and runSSELoop cleared the WHOLE verdict cache each time. Cheap only because of the S1 bug (memoized Globals → re-resolve was a map lookup); fixing S1 would invert it to a member-of walk per relation per connection — a thundering herd on a bulk relation import.'
severity: minor
resolution: 'The S1 fix coalesces RelationChange through the SAME flush window: a burst of relation writes sets a single regate flag and triggers ONE fresh-gate re-derive per connection per window, not one per edge. Whole-cache clear is retained (over-invalidation is safe and cheap), but the expensive part (the member-of re-walk) is now bounded to once per window regardless of relation-write volume. Pinned implicitly by the debounce tests + the membership test.'
status: addressed
---

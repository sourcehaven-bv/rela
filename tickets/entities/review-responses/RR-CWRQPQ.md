---
id: RR-CWRQPQ
type: review-response
title: Merging breaks no-op suppression, the disappeared-key sweep, and commitImmediately
severity: significant
resolution: 'Approach step 8 enumerates all three: per-entry no-op suppression with an all-suppressed short-circuit; the widened disappeared-key sweep; and commitImmediately''s drain loop. The unchanged-in-shape claim is retracted.'
status: addressed
finding: Plan step 5 claims queueTail, abort, commitImmediately and mergeServerResponse are unchanged in shape; three are not. (a) No-op suppression (useAutoSave.ts:297) is per-property; fireDue must apply it per entry while batching and handle all-suppressed by sending nothing, or it fires an empty properties PATCH where today it fired zero requests. (b) mergeServerResponse's disappeared-key sweep (:521) now spans the whole merged batch. (c) commitImmediately iterates Object.keys(timers) calling fireProperty per key; under fireDue the first call drains all ripe entries, so the claim of no change is false and it needs review.
---

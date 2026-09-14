---
id: RR-3JBQ7Y
type: review-response
title: 'Leverage: make the unsafe path unrepresentable with a rawValue type, and fuzz the invariant'
finding: 'Two leverage suggestions. (1) "producer value" and "safe string" are both string, so the compiler cannot help; a named type (type rawValue string, returned by resolve and consumed by flattenToLine) would make interpolate the only place that can convert one to the other, and any future path writing a rawValue into content would fail to compile — the Go idiom is template.HTML vs string. (2) The invariant is crisply falsifiable, so it is a natural fuzz target: Go''s fuzzer would explore delimiter-splicing cases far more thoroughly than hand-written rows, and would stop the seam moving wrong a third time.'
severity: minor
resolution: 'Deferred, deliberately, and not silently. Both are good and both are larger than this change. The rawValue type would touch resolve, flattenToLine, stringifyWebhookValue and lookupPath to add compile-time enforcement to a single-caller unexported method — worth doing on its own ticket where the API change is the subject, not folded into a security fix that needs to stay reviewable. The fuzz target is the stronger of the two and is a genuinely new test asset rather than a refactor; it belongs with the existing fuzz sweep infrastructure. Filed here so the reasoning survives rather than being rediscovered.'
status: deferred
reason: 'Both are API or test-infrastructure changes larger than this behaviour fix. Folding a type-level refactor into a security-relevant diff would make the diff harder to review, which is the wrong trade for a change whose correctness argument is the point. Recorded for a follow-up ticket.'
---

Both suggestions are sound. Neither belongs in this diff.

The fuzz target in particular is worth its own ticket: the seam has now moved
twice, and a property test is what stops a third move going wrong.

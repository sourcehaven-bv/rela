---
id: RR-2L6XJB
type: review-response
title: 'Version capture omits FromFace and the create half is asynchronous'
finding: 'capturer.relationDelete builds RelationVersionInput without setting FromFace (run.go:354-358) though the struct has one (store.go:1024-1028), so capturing a state-tailed edge''s delete records history against the wrong relation. Pre-existing, but a step whose subject matter is edges that may carry a tail is what makes it bite. Separately the plan never states that the CREATE half is captured by the debounced sweep rather than synchronously, so on a fast-exiting CLI run the new edges'' initial versions may not be captured before the process ends.'
severity: significant
resolution: 'Both halves documented in the plan. The create half is sweep-captured and asynchronous (attribution is still correct via store.WithAttribution); the delete half is synchronous. The FromFace omission in capturer.relationDelete is recorded as pre-existing and out of scope, made moot by the pre-flight tail refusal - with the note that this is precisely why that refusal cannot be relaxed without fixing the capture first.'
status: addressed
---

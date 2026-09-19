---
id: RR-2L6XJB
type: review-response
title: 'Version capture omits FromFace and the create half is asynchronous'
finding: 'capturer.relationDelete builds RelationVersionInput without setting FromFace (run.go:354-358) though the struct has one (store.go:1024-1028), so capturing a state-tailed edge''s delete records history against the wrong relation. Pre-existing, but a step whose subject matter is edges that may carry a tail is what makes it bite. Separately the plan never states that the CREATE half is captured by the debounced sweep rather than synchronously, so on a fast-exiting CLI run the new edges'' initial versions may not be captured before the process ends.'
severity: significant
status: open
---

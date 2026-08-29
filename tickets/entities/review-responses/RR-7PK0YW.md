---
id: RR-7PK0YW
type: review-response
title: 'Name the two redaction insertion points: zero redacted date roles BEFORE the fold; Redact exactly once at the boundary'
finding: 'Two distinct leaks conflated by ''gate then fold''. (a) FIELD-level: a date property hidden by visible: policy is exactly the payload this endpoint serves. visibility.Redact is serializer-local and NOT composable — policyreader.go:222-227 states its input must be a raw store entity, never a prior Redact output (the nothing-hidden path returns the input pointer, so a stale Redacted list survives and misreports; RR-Q1VCKR). A recursive fold is precisely the shape that stacks redaction by accident. (b) The fold itself: if it reads raw date values of ROW-visible entities whose date FIELD is hidden, the parent''s rolled span launders the hidden value into a readable field. Resolution: two named insertion points — (1) before the fold, zero out any date-role property the principal''s field verdicts hide, so the fold never sees a hidden value; (2) after the fold, run visibility.Redact exactly once per entity at the response boundary, mirroring views.go:56-68 raw-traverse-redact-once. The plan must state both; ''gate then fold'' alone covers only row-level.'
severity: critical
resolution: 'Plan now specifies a five-step pipeline with both insertion points named: step 2 zeroes any date-role property the principal''s field verdicts hide BEFORE the fold (so the fold never sees a visible:-hidden value), and step 5 runs visibility.Redact exactly once at the response boundary, citing the non-composability precondition (policyreader.go:222-227, RR-Q1VCKR). AC7(b) adds the differential test: a row-visible entity whose date field is hidden contributes nothing to any ancestor''s span.'
status: addressed
---

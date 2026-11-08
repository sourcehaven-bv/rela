---
id: RR-4VN0KPA
type: review-response
title: Collapsed run gate drops the subject-type constraint
finding: With every target fully readable the gate became an untyped _carries predicate, admitting a run whose stored edge points at a non-target type.
severity: minor
resolution: Every branch keeps its endpoint type; a single branch skips Any. Pinned by TestFlowRunReadQuery_EdgeOutsideTargets.
status: addressed
---

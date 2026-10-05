---
id: RR-F682HK
type: review-response
title: No read budget or fail-closed tests for the new orphan path
finding: VisibleTracer.FindOrphans had no storetest.Counting pin, and deleting tracer_typesof_test.go removed the empty-store and gate-error cases.
severity: significant
resolution: 'Added internal/visibility/tracer_orphans_test.go: a Counting budget test (same reads at 10 and 50 families), an empty-store test, and a gate-error test asserting the type is dropped.'
status: addressed
---

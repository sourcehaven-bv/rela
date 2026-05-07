---
id: RR-QIKA2
type: review-response
title: Performance regression documented but not optimized
finding: AC18 gate was ≤2x; actual is ~2.5–3x on kitchen-sink fixture. Reviewer suggests preallocated table sizes and an empty-inlines sentinel as quick wins worth ~30–40% of the regression.
severity: minor
reason: 'Optimizations deferred to a dedicated perf ticket so they can be benchmarked carefully. Documented in user-facing docs (Limitations: Performance section). Bench results captured in IMPL-SE1K0 verification evidence.'
status: deferred
---

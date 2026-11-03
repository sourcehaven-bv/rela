---
id: RR-YUAA3E
type: review-response
title: The seed closure in TestReopenPreservesStateKeyOrdering is now called once
finding: 'internal/store/fsstore/states_reopen_order_test.go:36-50: `seed` was a closure because two subtests each built a fresh MemFS. With the table gone it runs exactly once at :53. Inline it, or keep it and drop the `t *testing.T, fs` parameters that only served the subtests.'
severity: nit
resolution: Inlined the seeding in TestReopenPreservesStateKeyOrdering.
status: addressed
---

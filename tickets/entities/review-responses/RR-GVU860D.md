---
id: RR-GVU860D
type: review-response
title: Budget test did not pin rows for matching padding
finding: measureEvaluate padded only non-matching rows, so removing the Limit from the lowered rule query still passed every test.
severity: significant
resolution: 'Added TestEvaluate_LoweredRuleReadsAtMostTheCap: it pads with matching rows at 1100 and 2000 and asserts the same cost, 2 calls and MaxMatchesPerRule+1 rows. Verified it fails with the Limit removed.'
status: addressed
---

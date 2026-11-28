---
id: RR-RL5CFVP
type: review-response
title: Weak push-down test
finding: TestListCondition_RollupNeverPushedDown looked for a Lower() method that does not exist.
severity: minor
resolution: Replaced by TestListCondition_CountRollupNotPushedDown calling Lower(ctx, gate).
status: addressed
---

---
id: RR-NLHAKMX
type: review-response
title: Shared helper for the two hidden-property tests
finding: TestHiddenProperty_NotCompared and TestHiddenProperty_RealPolicy repeat the seed, plan and probe block.
severity: nit
reason: Kept the fix minimal; the two tests use different gates and second halves, so a helper would cover only part of each.
status: wont-fix
---

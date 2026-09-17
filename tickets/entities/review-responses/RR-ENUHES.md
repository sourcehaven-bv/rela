---
id: RR-ENUHES
type: review-response
title: Fixture asserted only the upper half of the straddle
finding: The breadth fixture asserted that the visible universe exceeds the allowance, but never that the emitted tree stays under it. If nestedNodeBudget were raised, both the allowance and the emitted tree would grow together, and the test would keep passing while losing its ability to distinguish emitted from visible.
severity: minor
resolution: 'Added the lower assertion: the test now fails with ''fixture does not overflow'' if emitted >= visibleChildren, so both halves of the straddle are checked.'
status: addressed
---

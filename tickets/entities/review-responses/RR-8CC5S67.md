---
id: RR-8CC5S67
type: review-response
title: Timeout not named as one
finding: An expired --timeout surfaced as a bare context deadline error.
severity: minor
resolution: Diff wraps it as ErrBudget naming the deadline. TestDiff_DeadlineIsBudget.
status: addressed
---

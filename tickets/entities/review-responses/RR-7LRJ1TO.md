---
id: RR-7LRJ1TO
type: review-response
title: Date pushdown not exact on fs time.Time values
finding: yaml.v3 decodes an unquoted date to time.Time; the naive matcher compares fmt.Sprint of it, so byte bounds disagree with Go.
severity: critical
reason: 'Not reachable in this ticket: date and datetime comparisons are never lowered to the store (LowerCondition declines ordered comparisons and calls; PushdownPrefilters skips date properties), so Go evaluates every row. Exact lowering, including fs time.Time values, is TKT-4Z2BIRG.'
status: deferred
---

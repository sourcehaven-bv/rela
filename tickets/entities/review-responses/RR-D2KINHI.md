---
id: RR-D2KINHI
type: review-response
title: Guide overstates when an id filter is pushed down
finding: GUIDE-data-entry says an id filter is part of the store query when the relation is not content-scoped. It also declines for faced target types not read whole, more than one value and the [] form.
severity: minor
resolution: The guide now states when id mode applies, the warning throttle, the SPA rewrite, the 400 on two values and the exact pushdown conditions; docs regenerated.
status: addressed
---

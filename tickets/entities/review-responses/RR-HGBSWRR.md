---
id: RR-HGBSWRR
type: review-response
title: Body-task refusal depended on data
finding: The 400 for filter/sort on a body_tasks roll-up ran after the empty-population early return, so an empty type answered 200.
severity: significant
resolution: Refusal moved before the empty return; test covers an empty type.
status: addressed
---

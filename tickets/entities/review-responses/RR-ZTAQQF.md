---
id: RR-ZTAQQF
type: review-response
title: Close select may report an error when handlers did finish
finding: If done and ctx.Done are both ready, select may pick the error branch.
severity: minor
resolution: Close checks drained again before returning the error.
status: addressed
---

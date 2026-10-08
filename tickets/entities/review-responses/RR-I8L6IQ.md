---
id: RR-I8L6IQ
type: review-response
title: Move path skips relation meta-field writability gate
finding: A separate /move route bypasses affordances.relationMetaDenial and the path/peer gates handleV1UpdateRelation applies (write_handler.go:1358-1440).
severity: critical
resolution: 'Plan updated: Reuse PATCH .../relations/{rel}/{target} with a position body; the handler computes the value and runs the same gate chain incl. relationMetaDenial on the order key. Test with an affordance denying _order_out.'
status: addressed
---

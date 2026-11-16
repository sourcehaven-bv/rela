---
id: RR-0BB54DA
type: review-response
title: Card modes beyond count and breakdown lack a gate test
finding: 'dashboard_anchor_test.go covered count, breakdown and table only: no sum/ratio gate test, no outgoing scope, no empty reach.'
severity: minor
resolution: GatesBeforeFolding now has a sum card whose hidden rows carry points. TestEntityDashboard_OutgoingScopeAndEmptyReach covers an outgoing scope and an anchor that reaches nothing. Ratio and matrix fold the same narrowed rows.
status: addressed
---

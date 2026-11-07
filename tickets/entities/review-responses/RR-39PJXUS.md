---
id: RR-39PJXUS
type: review-response
title: 'Design: SQL sum shapes and scanning'
finding: Use COALESCE(SUM(CASE ...)) in sqlite and scan into int64 with a non-negative check.
severity: nit
resolution: Adopted in the plan.
status: addressed
---

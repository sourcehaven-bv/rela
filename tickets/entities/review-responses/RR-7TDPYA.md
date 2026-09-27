---
id: RR-7TDPYA
type: review-response
title: Nested TKT-44PVX2 limit still applies
finding: Related frees only the top-level inbound slot; lowerTraversal still refuses a chained incoming hop into a role-gated endpoint (acl/traversal.go:104-111).
severity: minor
resolution: 'Plan: stated in scope; parity test for 422 with candidates and 200 without, on both paths.'
status: addressed
---

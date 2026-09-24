---
id: RR-8D8IGY
type: review-response
title: Denied traversal falls back to a full scan and double-gates
finding: Restricted principals are the ones most likely denied; fallback gives them a whole-type read and gates twice. Lower also runs before planListPushdown's own eligibility checks.
severity: minor
resolution: 'Plan: run planListPushdown''s cheap checks first; on ErrTraversalDenied keep gating the rest and, if none is refused, return an empty page with total 0 (matches Go path, no oracle).'
status: addressed
---

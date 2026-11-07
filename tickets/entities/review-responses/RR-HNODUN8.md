---
id: RR-HNODUN8
type: review-response
title: 'Code: per-row hidden conditions sent to the store are never used'
finding: Elements recomputes every row of a non-FoldStored type so the per-row grant predicates only cost the store work and would ask it which rows hold a hidden value.
severity: minor
resolution: A type that can hide a field or has roll-up names gets a nil Visibility (fold nothing); otherwise an empty one. hidesSome replaces fieldVisibility.
status: addressed
---

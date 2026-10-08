---
id: RR-D4Q2KB
type: review-response
title: Edges without order values make the first move land wrong
finding: Pre-existing edges have no _order_out; nothing backfills; midpoint against missing values puts the moved row first.
severity: critical
resolution: 'Plan updated: Inside the move Tx: if any sibling lacks a value, densify 1..N in display order first (audited like renumber), then compute. Test starting from all-missing.'
status: addressed
---

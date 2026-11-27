---
id: RR-DQFDRXO
type: review-response
title: Pushdown picks a different target face than the Go path
finding: 'The pushdown endpoint takes the world''s prime face and applies the read query to it; the Go path trims unreadable faces first and then ranks. A faced target whose prime fails a row-level read query matches on the Go path and not in the store. Fix: push down only when the target type is unfaced or its read query is allow-all.'
severity: significant
resolution: lowerRelationFilters declines a faced target type unless its ReadQuery is AllowAll; the Go path then decides.
status: addressed
---

---
id: RR-ILJ049
type: review-response
title: 'Redesign before merge: column_summary and property display format'
finding: 'A sum-only column_total key and a per-card-field currency were too narrow: aggregates should share one generic spec, and a currency belongs to the property so every surface formats it the same way.'
severity: significant
resolution: 'Replaced column_total with column_summary (shared Summary type: count, sum, avg, min, max) and the card field currency with the schema option display: {currency}, applied on cards, summaries, list cells, view rows and entity detail.'
status: addressed
---

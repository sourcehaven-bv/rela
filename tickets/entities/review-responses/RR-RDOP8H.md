---
id: RR-RDOP8H
type: review-response
title: '[security] Raw cardinality counts can reveal hidden edges'
finding: countRelations in internal/dataentry/analyze.go counts over the raw store and prints 'has N', so the difference from visible edges is the number of hidden neighbours.
severity: minor
resolution: 'TKT-5LW875: counts are folded from the edges the caller''s reader yields. Data-entry and MCP pass gated readers, which drop an edge unless both endpoints are readable, so ''has N'' counts visible edges only. Pinned by TestAnalyze_FacedTypes/cardinality_counts_visible_edges_only and TestAnalyzeCardinality_CountsVisibleEdgesOnly.'
reason: Pre-existing. Counting visible edges only is the same change as batching the counts; both are in TKT-5LW875.
status: addressed
---

---
id: RR-Z16WXF
type: review-response
title: Adding items does per-item full reads
finding: visibleReader.address loads one full entity per address; 500 items means 500 reads, the per-row defect CLAUDE.md forbids.
severity: significant
resolution: 'Plan: adds validate all ids with one batched ResolveIDsErr call in the request''s world; if any id is not served the whole request gets the uniform item_not_found. Counting budget tests for POST items and POST /_piles.'
status: addressed
---

---
id: RR-HGQDT
type: review-response
title: 'Response headers: table shape not specified'
finding: 'The plan says response includes ''headers'' but doesn''t specify the Lua table shape. HTTP headers can have multiple values for the same key (e.g., Set-Cookie). Options: (a) first-value-wins flat table (simple, lossy), (b) table of arrays (complete but verbose), (c) flat table with comma-joined values for multi-value headers (matches HTTP spec canonicalization). Recommend (a) for simplicity since this is for calling APIs, not parsing complex responses. Document the limitation.'
severity: minor
resolution: Flat table with lowercase keys, first-value-wins for duplicates
status: addressed
---

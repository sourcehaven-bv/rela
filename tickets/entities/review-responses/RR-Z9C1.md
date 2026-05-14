---
id: RR-Z9C1
type: review-response
title: Plan does not address what `searchEntities` returns when q is short
finding: 'Plan reuses MIN_QUERY_LEN = 2 from CommandPaletteModal. But the picker should also handle the user pasting an exact ID (e.g. `TKT-77JD4`) and expecting to find it. Two characters is fine, but the result list should ALSO surface exact-ID matches early in the ranking — the existing `/_search` endpoint uses Bleve full-text relevance, which may rank a partial title match higher than an exact ID match. Worth verifying in the e2e: type a known ID, assert it appears as the first result.'
severity: minor
resolution: 'Plan §Test Plan: e2e case asserts exact-ID match surfaces as top result. No code change in v1 -- Bleve''s existing relevance handles it; we test rather than assume.'
status: addressed
---

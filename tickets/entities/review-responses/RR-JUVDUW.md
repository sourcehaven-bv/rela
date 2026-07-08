---
id: RR-JUVDUW
type: review-response
title: Loop must trust response meta, not assume per_page=100 was honored
finding: 'parseV1Pagination silently ignores out-of-range per_page values (>100 falls back to 25). If the server cap ever changes, a loop that computes page counts from an assumed page size of 100 would skip or re-fetch ranges. The loop''s advance condition must be purely response-driven: continue while meta.has_more, request meta.page+1, never derive offsets from the requested per_page. The page-count safety cap also guards a pathological server that returns has_more: true with empty data (would otherwise loop forever).'
severity: minor
resolution: 'Plan revised: the loop''s advance condition is purely response-driven — continue while meta.has_more, request meta.page + 1, never derive offsets from the requested per_page. A unit test covers the server-ignores-per_page case (falls back to 25); the 50-page cap bounds the pathological has_more-forever server.'
status: addressed
---

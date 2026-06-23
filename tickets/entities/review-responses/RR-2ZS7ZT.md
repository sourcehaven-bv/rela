---
id: RR-2ZS7ZT
type: review-response
title: Filtered free-text search loads up to the candidate window of entity bodies per request
finding: With property filters present, runVisibleFreeTextSearch passes limit 0; on bleve up to 10k visible hits each get a GetEntity body load before the 1000 cap applies. In-process and tolerable, but the load amplification was undocumented — the kind of thing that surfaces as a latency complaint later.
severity: minor
resolution: 'GUIDE-acl-security candidate-window bullet extended with the load note: filtered free-text defers truncation past the filters, so the generic path may load up to the candidate window of entity bodies per request — named explicitly as a search-latency diagnosis hint. docs/acl-security.md regenerated.'
status: addressed
---

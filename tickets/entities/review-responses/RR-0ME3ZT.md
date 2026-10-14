---
id: RR-0ME3ZT
type: review-response
title: 'Other narrowing on raw values: query scopes and list q='
finding: query_scope props and filters compare stored values, and list ?q= may use the raw searcher. Check both and record what holds.
severity: minor
resolution: List q= was a real leak and is fixed under RR-H3GP12. Query scopes are recorded under RR-MJ8XRF.
status: addressed
---

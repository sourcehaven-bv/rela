---
id: RR-P1FK0IN
type: review-response
title: 'Design: exported scope bound to a principal outlives the request'
finding: An ElementScope kept across requests would keep stale grants for the same principal; CLAUDE.md forbids caching identity across requests.
severity: significant
resolution: 'No exported scope: Fingerprint and List compile the scope from the bound request on every call; the caller passes type names only.'
status: addressed
---

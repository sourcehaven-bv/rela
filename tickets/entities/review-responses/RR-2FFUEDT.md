---
id: RR-2FFUEDT
type: review-response
title: Each card repeats the anchor read and the relation query
finding: dashboardCardAnchor and cardReach run once per card request, so cards sharing a relation repeat the same query.
severity: minor
reason: The SPA loads cards one request each, as plain dashboards do. Each request costs a fixed 6 reads (pinned). Sharing across requests would need a cross-request cache, which the aggregate rule forbids across principals.
status: wont-fix
---

---
id: RR-OUWRZL3
type: review-response
title: Inbox links are API paths, not SPA routes
finding: links.* are /api/v1/... paths; InboxView builds /entity/{type}/{anchor}?channel=&seq= via space.href.
severity: critical
resolution: Extract rowKey()/inboxRoute() from InboxView into utils/inboxRoutes.ts; notification click and page share it.
status: addressed
---

---
id: RR-3CWYDER
type: review-response
title: Dashboard ignores has_more after the page guard
finding: searchAllEntities stops after 50 pages and the dashboard renders the partial set without saying so.
severity: minor
reason: 50 pages is 50000 rows, far past any dashboard card today; a visible notice is a separate UX change.
status: deferred
---

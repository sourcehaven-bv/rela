---
id: RR-O2K38U
type: review-response
title: Add another from the Create menu leaves the tab stale
finding: created-another links the row but refreshes nothing. The entity-created SSE refetch can land before the link POST, and relation writes do not trigger a refetch, so the row is missing from the open tab until reload. The tab path calls refreshAfterCreate after linking.
severity: significant
resolution: usePageCreateLink.linkCreated invalidates the type's list queries after a successful link. Pinned by the add-another test.
reason: ""
status: addressed
---

---
id: RR-1IBKZ0
type: review-response
title: Multi-page fetch is not cancellable (no AbortSignal on listEntities)
finding: listEntities takes no AbortSignal, so a stale multi-page loop keeps issuing requests after the user switches boards or navigates away. Pinia Colada keys cache entries per type, so no wrong-data risk — only wasted requests and a delayed settle. Acceptable for typical board sizes; worth a signal parameter only if large deployments surface it.
severity: minor
resolution: 'Deferral superseded during code review (RR-MZ7NJU): Pinia Colada already supplies an AbortSignal in the query context, so cancellation required only an optional trailing parameter on listEntities/listAllEntities — no caller churn. The multi-page loop is now cancelled between pages and in-flight when a refetch supersedes it.'
status: addressed
---

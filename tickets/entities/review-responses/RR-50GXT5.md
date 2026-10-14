---
id: RR-50GXT5
type: review-response
title: No load-time warning for a configured list filter or sort on a field-restricted property
finding: An operator learns that a configured list filter, sort or group_by names a property some role can hide only when a user gets a 400. Warn at load, like QueryScopeVisibilityConflicts does for query scopes.
severity: minor
resolution: 'Covered by the load-time lint extended for RR-9FKZ9H on BUG-WJSYQT (#1882): configured lists, kanban, calendar, sidebar sorts and client baselines that use a field the gate may refuse are warned about at load.'
reason: A load-time warning needs the per-principal rule evaluated for every role combination at config load; it is a separate operator-UX improvement and does not affect the fix. Runtime behavior is already safe (refused or degraded per entry).
status: addressed
---

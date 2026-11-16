---
id: RR-UIDD3CD
type: review-response
title: DashboardResponse duplicated DashboardConfig's fields
finding: 'Archplan on PR 2349: DashboardResponse (apiwire) and DashboardConfig (dataentryconfig) shared 80% or more of their field names once entity_type was added. weight.types rose from 4203 to 4233.'
severity: significant
resolution: DashboardResponse no longer carries entity_type. The SPA tells an entity dashboard by its cards' scope, which validation requires on every card of an entity dashboard and forbids elsewhere. weight.types is 4191 locally.
status: addressed
---

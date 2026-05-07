---
id: RR-SOOUK
type: review-response
title: Query preservation for back-navigation is silently dropped
finding: 'EntityList.vue forwards from, scope, sort, and bracket-format filters into the query so the EntityView back-button (useBackTarget) works. CustomView''s navigation today drops all of it, but it also doesn''t navigate. Once we fix navigation, users land on EntityView with no back-target. Plan must decide: pass from=<viewId> and scope=view:<viewId>:<sectionId>, or none of it. State the choice up front.'
severity: significant
resolution: Plan now passes from=<viewId> and scope=view:<viewId> on navigation, mirroring EntityList.vue's pattern. Filters and sort omitted (CustomView sections aren't filterable).
status: addressed
---

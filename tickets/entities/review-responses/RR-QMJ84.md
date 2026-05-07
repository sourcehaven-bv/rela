---
id: RR-QMJ84
type: review-response
title: from=<viewId> back-button query param is broken; useBackTarget assumes from is a list id
finding: 'useBackTarget reads ?from and unconditionally builds /list/${from}. Passing a view id sends users to a ''list not found'' page. useScopeNavigation does the same. The plan''s `scope=view:<viewId>` key is read by nothing -- it''s dead code. Two ways to fix: (a) use ?return_to=/view/<viewId>/<entityId> instead (useBackTarget already supports return_to as higher-priority path); no scope-nav from views, but that''s fine for v1. (b) Extend LabelHint and useBackTarget/useScopeNavigation to handle a view kind -- bigger change. Pick (a) for this PR.'
severity: critical
resolution: 'Plan now uses ?return_to=/view/<viewId>/<entityId> instead of ?from=<viewId>+?scope=. useBackTarget already supports return_to as the higher-priority path. No extension to LabelHint or useScopeNavigation needed. Trade-off accepted: no scope-nav (prev/next) when navigating from a view; documented in plan.'
status: addressed
---

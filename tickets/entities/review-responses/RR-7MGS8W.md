---
id: RR-7MGS8W
type: review-response
title: list=<id> ignores the list's own filters and falls back silently
finding: The export core takes request query parameters; the SPA sends the list's filters/sort/query_scope. resolveEffectiveList falls back to the default list for an unknown id.
severity: significant
resolution: 'Plan: build the query with navItemsListQuery(listID, listCfg); type from cfg.Lists[id].EntityType; unknown list id is an error.'
status: addressed
---

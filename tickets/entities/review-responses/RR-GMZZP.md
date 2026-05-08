---
id: RR-GMZZP
type: review-response
title: Empty query must not call /_search
finding: 'Plan says ''when query is empty, show "Type to search entities"''. But the watcher described is just watch(query, debounce(searchEntities)). If wired naively, an empty query (after Backspace, or after re-opening the modal) fires searchEntities(''''). Backend behavior is undefined — likely 400 or unbounded results. Required: in Approach, state the debounced search guards ''if query.trim() === "" then results = [], return'' before the API call. Test: type ''abc'', clear with Backspace, advance timers, assert searchEntities was not called for the empty state.'
severity: minor
resolution: 'Plan updated: the debounced search effect early-returns when query.trim() === '''' — clears results, cancels any in-flight AbortController, and renders the ''Type to search entities'' hint without calling searchEntities. Test added in Edge Cases section.'
status: addressed
---

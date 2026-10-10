---
id: RR-DCTA9Q
type: review-response
title: Stale fetch overrides a local navigation
finding: fetchSeq only advanced in fetchScope; a navigation answered locally left an in-flight fetch current, so a late subtree replaced the forest.
severity: significant
resolution: The navigation watcher calls cancelFetch() first; test 'drops a fetch the user navigated away from' (mutation-checked).
status: addressed
---

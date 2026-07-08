---
id: RR-L78S8H
type: review-response
title: Candidate fetch is not reactive to props.config changes (mount-once)
finding: FilterBar.vue fetches candidates only in onMounted. If props.config.filter_controls changes after mount (component reuse across route params / config reload), candidate lists are never refetched — stale/empty relation widgets. RelationPicker uses watch(immediate:true) for this reason. The surrounding props.filters watcher shows the component otherwise assumes props can change. Either watch the config or document why mount-once is safe.
severity: significant
resolution: 'FilterBar.vue: replaced onMounted with watch(immediate:true) keyed on the relation/direction of each filter control, so a later props.config change refetches candidates. Tested indirectly via the mode-flip test (fetch re-runs on load) and the sibling test (per-control loading).'
status: addressed
---

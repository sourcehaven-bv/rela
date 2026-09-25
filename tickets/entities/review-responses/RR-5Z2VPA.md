---
id: RR-5Z2VPA
type: review-response
title: Prune divergence between backends not pinned by conformance
finding: pg deletes every run of a pruned task; kvstate keeps ended runs of a removed task until their own cut-off. The shared suite does not pin either.
severity: minor
resolution: 'kvstate Prune now also drops the ended runs of a task it removes, matching pgschedstate. No conformance case: a removed task was untouched since the cut-off, and its runs finished no later than that, so both backends already drop them by age; the difference is not observable through the Store API.'
status: addressed
---

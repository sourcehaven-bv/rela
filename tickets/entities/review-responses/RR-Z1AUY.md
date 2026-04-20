---
id: RR-Z1AUY
type: review-response
title: Scheduler state staying in .rela/ contradicts the ticket's threat model
finding: 'scheduler-state.json contains last-run timestamps — activity-pattern leak under Dropbox sync. Cross-machine sync also races with no filesystem lock. Plan rationale that it is ''shareable'' is backwards: execution history is per-machine, not per-repo.'
severity: significant
resolution: 'User decision: move scheduler-state.json to user-state alongside last_seen_version, documents/, ui-state.json etc. Updated scope section. .rela/ now holds only cache.json, fsstore-index.json, repo-id, ai.yaml, secrets.yaml, encryption.yaml.'
status: addressed
---

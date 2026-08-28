---
id: RR-C5DBEB
type: review-response
title: Hide-detection cannot live in the post-flush activeProperties watcher
finding: Plan step 7 calls the watcher a proposal source, but a proposal is pre-mutation and the watcher only fires post-mutation - it fires BECAUSE formData changed. Hide-detection must move into proposeChange, computed from hypothetical bindings before the write. The watcher keeps only error-clearing and reveal-restore, and loses the destructive effect entirely.
severity: critical
resolution: 'Approach step 3: hide-detection moves into proposeChange. The watcher keeps only error-clearing and reveal-restore and loses the destructive scheduleUnset entirely. Risk 6 added to trace the ordering dependency before deleting.'
status: addressed
---

---
id: RR-6SU18P
type: review-response
title: 'Design: idempotency key loses edits during a run'
finding: The key also collapses against a running job, so a save after the script read the entity was dropped.
severity: critical
resolution: 'Trigger tokens in state.KV: each trigger writes a fresh token before enqueue; the handler reruns while the token changed (max 5) under a per-key mutex. TestAutomationJobs_SaveDuringRunRunsAgain.'
status: addressed
---

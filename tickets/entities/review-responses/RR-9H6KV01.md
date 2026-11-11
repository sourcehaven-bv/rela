---
id: RR-9H6KV01
type: review-response
title: The header lags the run store
finding: The header patch after a move is best effort, so the task list can show a stale task.
severity: minor
resolution: Complete revalidates against the run store, so a stale task cannot complete the wrong step. Documented.
status: addressed
---

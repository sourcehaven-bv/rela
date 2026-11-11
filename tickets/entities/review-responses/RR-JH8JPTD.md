---
id: RR-JH8JPTD
type: review-response
title: 'File backend: crash mid-batch or before the log write'
finding: A batch over a segment boundary is two log writes; meta.yaml is written before the log so a crash can leave answer counts without an answer.
severity: minor
reason: Crash-only on the single-user file backend; the fix (rebuilding Asked from the log on repair) is a separate change. Follow-up proposed.
status: deferred
---

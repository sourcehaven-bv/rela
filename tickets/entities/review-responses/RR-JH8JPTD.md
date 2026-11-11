---
id: RR-JH8JPTD
type: review-response
title: 'File backend: crash mid-batch or before the log write'
finding: A batch over a segment boundary is two log writes; meta.yaml is written before the log so a crash can leave answer counts without an answer.
severity: minor
reason: 'The file backend is the developer tier: one user with the project in git. Normal users run sqlite or postgres, which write a batch in one transaction. Crash repair for batches there is not worth its complexity.'
status: wont-fix
---

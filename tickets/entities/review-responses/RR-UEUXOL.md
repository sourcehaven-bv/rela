---
id: RR-UEUXOL
type: review-response
title: Retry rebuilt the server when nothing was pending
finding: POST /migrate always built a candidate, even when the migration state was ready.
severity: minor
resolution: Retry returns early when Migrations.Ready succeeds.
status: addressed
---

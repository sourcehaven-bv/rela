---
id: RR-ABYHPB
type: review-response
title: Task-less processes now log and tick
finding: Every server and desktop project logs scheduler start and ticks with no tasks.
severity: minor
reason: Needed so a file added later is applied; a tick with no tasks is a reap and an empty load, and prune is skipped.
status: wont-fix
---

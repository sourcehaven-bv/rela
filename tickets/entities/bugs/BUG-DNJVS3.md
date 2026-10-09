---
id: BUG-DNJVS3
type: bug
title: A postgres CLI process consumes and drops scheduler jobs
description: Every assembly starts the job queue with workers. On postgres the queue is shared, so a short-lived rela-postgres CLI command can claim a pending scheduler:run-task job. Its dispatcher has no handler for that kind (the scheduler registers only in rela-server and desktop) and marks the job processed, so the scheduled run is lost.
priority: high
status: backlog
---

Found in the TKT-2Q4UFI design review: neoqqueue.go dispatch logs "no handler
registered for kind\, dropping" and returns nil. Fix direction: one-shot
assemblies get an enqueue-only queue (no workers)\, or the dispatcher leaves
unknown kinds for another node.

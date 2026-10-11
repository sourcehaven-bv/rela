---
id: BUG-821TEB
type: bug
title: Failed assembly leaks mail, GC and soft-delete goroutines
description: startBackgroundServices starts mail, GC sweep and soft-delete before finishAssembly, which can still fail; on that path only the job queue is closed. MCP reload leaks them once per bad save.
priority: medium
status: backlog
---

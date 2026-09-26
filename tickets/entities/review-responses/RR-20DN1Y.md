---
id: RR-20DN1Y
type: review-response
title: MCP attachment writes do not take the App write lock
finding: 'WriteAttachment assumes serialized writers per (entity, property); web holds app.writeMu. Concurrent MCP uploads can overshoot max or orphan a file at max 1. Fix: AttachmentDeps carries a sync.Locker (remote: app.writeMu via MCPHostDeps; stdio: process-local mutex) held from preflight through the write; concurrency test at max 2.'
severity: significant
resolution: AttachmentDeps.WriteLock is held for the whole attach/delete. Remote wiring passes &App.writeMu via MCPHost; stdio uses a mutex on mcpServices that outlives reloads. TestAttachments_ConcurrentAttachRespectsMax.
status: addressed
---

---
id: RR-BU8BK
type: review-response
title: Plan must specify workspace.Sync() before each task execution
finding: The plan identifies graph staleness as a risk but leaves mitigation vague ('use workspace file watcher or re-sync'). The workspace already has Sync() (line 523) and SyncLua() (line 563) methods. The plan should explicitly call workspace.Sync() before each task execution to ensure scripts see current entity/relation state. Alternatively, use SyncLua() which is the Lua-friendly wrapper.
severity: significant
resolution: Updated plan to call workspace.Sync() before each task execution
status: addressed
---

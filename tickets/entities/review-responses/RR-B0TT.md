---
id: RR-B0TT
type: review-response
title: Missing SyncLua() in read-only wrapper
finding: WorkspaceInterface includes `SyncLua() error` which reloads the graph from disk. The plan's read-only wrapper must block this method too - it's effectively a mutation (changes in-memory state). The test plan doesn't include a test case for SyncLua being blocked.
severity: significant
resolution: Added SyncLua() to blocked methods list, added test case TestLuaValidation_SyncBlocked
status: addressed
---

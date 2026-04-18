---
id: RR-NCNDE
type: review-response
title: Delete Meta/ProjectRoot fallbacks in executor/action/validation in same PR
finding: Current code has `if svc.Meta == nil { svc.Meta = ctx.GetMeta() }` and `if svc.ProjectRoot == "" { svc.ProjectRoot = ctx.GetProjectRoot() }` at executor.go:64-70, action.go:60-65, and validation/lua.go:43-48. These boilerplate fallbacks exist only because there are two sources of truth (Services struct + ScriptContext). After the refactor, ReadDeps.Meta() and ReadDeps.ProjectRoot() must be authoritative. Delete the fallbacks in the same PR, not as follow-up — otherwise the old smell migrates forward.
severity: significant
resolution: Accepted and planned for same PR. executor.go:64-70, action.go:60-65, and validation/lua.go:43-48 fallbacks all deleted when lua.ReadDeps/WriteDeps becomes the single source of truth. Meta and ProjectRoot are set once by the helper that builds the deps value.
status: addressed
---

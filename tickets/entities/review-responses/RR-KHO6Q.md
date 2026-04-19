---
id: RR-KHO6Q
type: review-response
title: Collapse metamodel.ScriptContext once GetWorkspace() is removed
finding: Removing GetWorkspace() interface{} is right. After removal, ScriptContext reduces to GetMeta/GetProjectRoot/GetEntity/GetOldEntity. Meta and ProjectRoot now duplicate what ReadDeps provides (since the engine accepts lua.WriteDeps). ScriptContext was only living in metamodel to dodge the lua import cycle; with the cycle gone, the interface has no reason to exist. Collapse it either to `type EntityContext struct { New, Old *entity.Entity }` or drop the type entirely and pass two *entity.Entity args to Engine methods.
severity: critical
resolution: 'Accepted. Plan updated: metamodel.ScriptContext is dropped entirely. Engine methods (ExecuteCode/ExecuteFile/ExecuteAction) take lua.WriteDeps and new/old *entity.Entity as separate args. No shim interface lives in metamodel after the refactor.'
status: addressed
---

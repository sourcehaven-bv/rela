---
id: RR-KVBHR
type: review-response
title: wsRaw interface{} lacks runtime type assertion
finding: wsRaw is untyped interface{}. No compile-time or runtime validation that it satisfies lua.WorkspaceInterface.
severity: minor
reason: Import cycle prevents compile-time enforcement. The existing pattern in workspace (scriptContextImpl) uses the same untyped interface{} approach. Runtime assertion happens in script.Engine.execute
status: wont-fix
---

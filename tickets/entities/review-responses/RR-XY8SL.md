---
id: RR-XY8SL
type: review-response
title: Cache ownership should move from script.Engine (architect, round 2)
finding: go-architect argued the cache should live on Workspace, not script.Engine. The Workspace delegation (ws.LuaCache -> scriptExec.LuaCache) is a lie about ownership. Every cache consumer already holds a Workspace.
severity: significant
resolution: Not moved. Will land with the workspace-removal refactor as 'cache created in entry point, passed down with other per-process state'. Current Engine-ownership is acknowledged as a waypoint; docs (cache.go, CLAUDE.md) already clarify the scope is 'engine/invocation' rather than process-absolute.
reason: 'User indicated workspace is planned to be removed in a separate refactor. Moving the cache to a dying type would just be a migration target for the workspace-removal work. The right landing is entry-point-owned: each main() creates one lua.Cache and threads it down via the wiring that replaces Workspace.'
status: deferred
---

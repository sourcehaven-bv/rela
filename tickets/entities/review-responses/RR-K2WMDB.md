---
id: RR-K2WMDB
type: review-response
title: ProjectRoot split between Deps and LuaWriteDeps undocumented
finding: mcp Deps.ProjectRoot is a real t.TempDir() while svc.LuaWriteDeps().ReadDeps.ProjectRoot is the fixture's in-memory /project — inert today but a tripwire for future Lua-write MCP tests.
severity: nit
resolution: Comment added on the ProjectRoot field documenting the intentional split and when to align the two.
status: addressed
---

---
id: RR-TDHV
type: review-response
title: 'F11: Five entry points copy-paste filepath.Join(projectRoot, .rela)'
finding: Four of five Lua-construction sites hand-built the .rela path with the literal string, duplicating the project.CacheDir constant and re-inventing a path that already exists on every Paths struct.
severity: minor
resolution: 'Partial cleanup as part of F8: internal/mcp/tools_lua.go now uses s.ws.Paths().CacheDir directly (both call sites). internal/script/executor.go uses filepath.Join(ctx.GetProjectRoot(), project.CacheDir) — the project.CacheDir constant instead of the literal ''.rela''. The two cli sites already used Paths().CacheDir from the start. The literal magic string is gone from the codebase. A proper LoadProviderForWorkspace helper was considered but deemed overkill for a one-liner that varies slightly per call site (cli has projectCtx, mcp has s.ws, script has ctx.GetProjectRoot).'
status: addressed
---

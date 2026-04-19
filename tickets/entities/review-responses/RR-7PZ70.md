---
id: RR-7PZ70
type: review-response
title: Consolidate wiring boilerplate across 5 call sites
finding: Five wiring sites duplicate the pattern of LuaServices() + WithAIProvider + LoadContextOptions (executor, action, mcp eval, mcp run, cli script, cli flow). The refactor should either (a) leave this as-is and just swap Services for ReadDeps/WriteDeps at each site, or (b) introduce a small helper like `script.NewReaderRuntime(ws, scriptPath, opts...)` / `script.NewWriterRuntime(ws, scriptPath, opts...)` that returns an already-wired runtime. Pick one explicitly in the plan — leaving duplicated boilerplate 'for later' is how the next smell gets born.
severity: significant
resolution: 'Accepted option (b) with user refinement: helpers take lua.ReadDeps/WriteDeps + cacheDir + scriptPath directly, NOT *workspace.Workspace. This avoids moving the fat-provider smell one layer out; script/runtime.go does not import internal/workspace. Workspace gets LuaReadDeps()/LuaWriteDeps() materialization methods (one place that knows how to extract values from workspace); call sites do one-liner `script.NewWriterRuntime(ws.LuaWriteDeps(), cacheDir, scriptPath, ...)`.'
status: addressed
---

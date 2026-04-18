---
id: RR-T9C62
type: review-response
title: cacheDir string redundant with deps.ProjectRoot
finding: 'internal/script/runtime.go:14, 29 and all call sites — cacheDir is always projectCtx.CacheDir which is always filepath.Join(projectRoot, project.CacheDir). Every caller passes deps.ProjectRoot + cacheDir redundantly. Fix: compute cacheDir := filepath.Join(deps.ProjectRoot, project.CacheDir) inside the helper and drop the parameter from 6 call sites.'
severity: minor
resolution: 'Dropped cacheDir parameter from script.NewReaderRuntime and NewWriterRuntime and from script.Engine methods (ExecuteCode/ExecuteFile/ExecuteAction) and from workspace.ScriptExecutor interface. cacheDir is now computed inside script.runtime.go via cacheDirFor(deps.ProjectRoot) = filepath.Join(ProjectRoot, project.CacheDir). 6 call sites updated: cli/script, cli/flow, mcp/tools_lua (2x), script/action, script/executor, scheduler, dataentry/actions.'
status: addressed
---

---
id: RR-NRK1
type: review-response
title: lua_file path validation should use os.OpenRoot
finding: |-
    The plan mentions using `filepath.IsLocal()` for path validation, but the existing `tools_lua.go` uses `os.OpenRoot` for traversal-resistant access which is more secure (handles symlinks safely).

    **Recommendation:** Follow the same pattern as `handleLuaRun` in `tools_lua.go:104-129`:
    1. Use `os.OpenRoot(projectRoot)` to get a rooted filesystem
    2. Use `scriptsRoot.Open(path)` to read script content
    3. Execute the content via `RunString()` rather than `RunFile()`
severity: minor
resolution: loadLuaScript uses os.OpenRoot pattern from tools_lua.go for traversal-resistant file access.
status: addressed
---

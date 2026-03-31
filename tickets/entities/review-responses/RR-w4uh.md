---
finding: |-
    In handleLuaRun (tools_lua.go lines 124-125), after validating the path using os.OpenRoot, the code then builds an ABSOLUTE path and passes it directly to runtime.RunFile:

    ```go
    scriptPath := filepath.Join(projectRoot, scriptRelPath)
    ...
    if err := runtime.RunFile(scriptPath, args); err != nil {
    ```

    The Lua runtime's RunFile then calls L.DoFile(path) with this absolute path, which bypasses the os.Root protection entirely! The L.DoFile function uses regular os file operations, not the traversal-resistant os.Root API.

    This means a script with carefully crafted symlinks in the scripts/ directory could still potentially escape.

    While filepath.IsLocal() + the scriptsRoot.Stat() check provides defense-in-depth, the final execution path does NOT use traversal-resistant APIs.

    Location: /Users/jeroen/Work/sourcehaven/rela-3/internal/mcp/tools_lua.go lines 124-133
id: RR-w4uh
resolution: Changed lua_run to read script content via os.Root (traversal-resistant) and execute using RunString instead of RunFile. This prevents symlink escapes since the script content is read through the sandbox, not via absolute path.
severity: significant
status: addressed
title: lua_run builds absolute path defeating os.OpenRoot protection
type: review-response
---

## The Issue

The validation uses os.OpenRoot for safety:
```go
scriptsRoot, err := root.OpenRoot(scriptsDir)
// ...
if _, err := scriptsRoot.Stat(path); err != nil {  // Traversal-safe check
```

But then execution uses regular path:
```go
scriptPath := filepath.Join(projectRoot, scriptRelPath)  // Regular path join
// ...
runtime.RunFile(scriptPath, args)  // L.DoFile uses regular os.Open
```

## Why This Matters

If an attacker can create a symlink in scripts/ pointing outside the project:
```
scripts/evil.lua -> ../../../../../../etc/malicious.lua
```

The check `scriptsRoot.Stat("evil.lua")` will succeed (symlink exists)
Then `runtime.RunFile("/project/scripts/evil.lua")` follows the symlink

## The Fix

Option 1: Read file content through os.Root, then use RunString:
```go
f, err := scriptsRoot.Open(path)
if err != nil { return error }
defer f.Close()
content, err := io.ReadAll(f)
if err := runtime.RunString(string(content)); err != nil { ... }
```

Option 2: Add O_NOFOLLOW to prevent symlink following (OS-dependent)

Option 3: Document that symlinks in scripts/ are not supported and verify target is regular file

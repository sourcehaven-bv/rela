---
finding: |-
    The handleLuaList function uses filepath.WalkDir to enumerate scripts, which follows symlinks:

    ```go
    _ = filepath.WalkDir(scriptsPath, func(path string, d os.DirEntry, walkErr error) error {
    ```

    If someone creates a symlink in scripts/ pointing to a directory outside the project, WalkDir will happily follow it and list files from outside the project.

    While this only affects listing (not execution, due to separate checks), it's inconsistent with the security posture of lua_run and could leak information about files outside the project.

    Location: /Users/jeroen/Work/sourcehaven/rela-3/internal/mcp/tools_lua.go lines 154-171
id: RR-agc7
reason: lua_list is read-only script enumeration with no execution. Symlinks would just show as filenames. The actual execution path (lua_run) now uses traversal-resistant os.Root for reading script content. Risk is acceptable for listing operation.
severity: minor
status: wont-fix
title: lua_list uses filepath.WalkDir without traversal protection
type: review-response
---

## The Issue

```go
// Walk the scripts directory recursively to find all .lua files
_ = filepath.WalkDir(scriptsPath, func(path string, d os.DirEntry, walkErr error) error {
```

If `scripts/external -> /home/user/private/scripts`:
- lua_list will enumerate files in /home/user/private/scripts
- This leaks information about files that shouldn't be visible

## The Fix

Either:
1. Use os.Root-based directory reading (consistent with lua_run)
2. Skip symlinks during walk

```go
_ = filepath.WalkDir(scriptsPath, func(path string, d os.DirEntry, walkErr error) error {
    // Skip symlinks to prevent following links outside project
    if d.Type()&os.ModeSymlink != 0 {
        if d.IsDir() {
            return filepath.SkipDir
        }
        return nil
    }
    // ... rest of function
})
```

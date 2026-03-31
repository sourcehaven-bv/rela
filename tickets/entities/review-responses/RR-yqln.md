---
finding: |-
    In luaWriteFile (runtime.go lines 354-363), the code creates parent directories using root.Mkdir(dir, 0755). However, it only creates ONE level of directory, not nested directories.

    If `path` is "foo/bar/baz/output.txt", the code calls `root.Mkdir("foo/bar/baz", 0755)` which will FAIL if "foo/bar" doesn't exist (ENOENT).

    Even worse, the error handling then tries to Stat the directory to check if it exists, but this masks the actual failure scenario where intermediate directories don't exist.

    This means:
    1. Scripts cannot create files in new nested directories
    2. The error handling logic is flawed - it catches IsExist but not the nested directory case

    Location: /Users/jeroen/Work/sourcehaven/rela-3/internal/lua/runtime.go lines 354-363

    NOTE: os.Root in Go 1.24 does NOT have MkdirAll - only Mkdir. This is a design limitation that needs explicit handling.
id: RR-yqln
resolution: Simplified by restricting write_file to output/ directory only. Now uses standard os.MkdirAll which is safe because paths are validated with filepath.IsLocal() before joining with the output directory path.
severity: critical
status: addressed
title: Nested directory creation in write_file is not traversal-safe
type: review-response
---

## The Problem

```go
// This code at lines 354-363:
dir := filepath.Dir(path)
if dir != "." && dir != "" {
    if mkdirErr := root.Mkdir(dir, 0755); mkdirErr != nil && !os.IsExist(mkdirErr) {
        // Try to check if it already exists
        if _, statErr := root.Stat(dir); statErr != nil {
            ls.RaiseError("write_file: cannot create directory %s: %s", dir, mkdirErr.Error())
            return 0
        }
    }
}
```

For path="deep/nested/dir/file.txt", this calls:
- `root.Mkdir("deep/nested/dir", 0755)` -> FAILS with ENOENT because "deep/nested" doesn't exist

## The Fix

Need to implement MkdirAll-like behavior by splitting the path and creating each level:

```go
// Ensure parent directories exist (MkdirAll equivalent for os.Root)
dir := filepath.Dir(path)
if dir != "." && dir != "" {
    parts := strings.Split(filepath.ToSlash(dir), "/")
    for i := range parts {
        subdir := filepath.Join(parts[:i+1]...)
        if mkdirErr := root.Mkdir(subdir, 0755); mkdirErr != nil && !os.IsExist(mkdirErr) {
            if _, statErr := root.Stat(subdir); statErr != nil {
                ls.RaiseError("write_file: cannot create directory %s: %s", subdir, mkdirErr.Error())
                return 0
            }
        }
    }
}
```

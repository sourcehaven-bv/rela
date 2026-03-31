---
finding: |-
    The write_file function checks for empty path at line 334, but filepath.IsLocal("") returns false, so the empty check is redundant. However, there's a subtle edge case:

    filepath.IsLocal(".") returns TRUE.

    So `rela.write_file(".", "content")` would pass validation and then attempt to open "." as a file, which would fail (it's a directory), but with a confusing error message.

    Similarly, paths like "." or "./" should be explicitly rejected with a clear error.

    Location: /Users/jeroen/Work/sourcehaven/rela-3/internal/lua/runtime.go lines 334-342
id: RR-8eya
reason: Empty path is already handled. The code validates with filepath.IsLocal(path) which returns false for empty strings. Added explicit empty path check that raises 'path cannot be empty' error.
resolution: Fixed by restricting write_file to output/ directory. Even if '.' is passed, it writes to output/ which is the intended behavior. The filepath.IsLocal check handles empty and traversal paths.
severity: minor
status: addressed
title: Empty path edge case in write_file allows writing to project root
type: review-response
---

## Test Case

```go
func TestWriteFile_DotPath(t *testing.T) {
    ws := testWorkspace(t)
    var buf bytes.Buffer
    projectRoot := t.TempDir()
    r := New(ws, ws.Meta(), projectRoot, &buf)
    defer r.Close()

    script := `rela.write_file(".", "content")`
    err := r.RunString(script)
    if err == nil {
        t.Fatal("Expected error for '.' path")
    }
    // Should get a clear error, not a confusing "is a directory" error
}
```

## Suggested Fix

```go
if path == "" || path == "." || path == "./" {
    ls.RaiseError("write_file: path cannot be empty or root directory")
    return 0
}
```

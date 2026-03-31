---
finding: |-
    There is no test for writing files into nested subdirectories that don't exist yet. Given the bug in directory creation (see related finding), this would have caught the issue.

    Test coverage for write_file includes:
    - TestWriteFile (simple file)
    - TestWriteFile_PathTraversal (../ blocked)
    - TestWriteFile_AbsolutePathOutside (absolute blocked)
    - TestWriteFile_WithinProject (simple file, existing dir)

    Missing tests:
    - Writing to "subdir/output.txt" when subdir doesn't exist
    - Writing to "deep/nested/path/file.txt" when directories don't exist
    - Writing to a path with leading slashes "//foo" (edge case)

    Location: /Users/jeroen/Work/sourcehaven/rela-3/internal/lua/runtime_test.go
id: RR-9ren
resolution: Added TestWriteFile_NestedDirectories test that verifies write_file correctly creates nested directory structure (a/b/c/deep.txt).
severity: significant
status: addressed
title: Missing test for write_file with nested directories
type: review-response
---

## Required Test

```go
func TestWriteFile_NestedDirectory(t *testing.T) {
    ws := testWorkspace(t)
    var buf bytes.Buffer

    projectRoot := t.TempDir()
    r := New(ws, ws.Meta(), projectRoot, &buf)
    defer r.Close()

    // Note: subdir/nested does not exist yet
    script := `rela.write_file("subdir/nested/output.txt", "deep content")`
    tmpFile := filepath.Join(projectRoot, "test.lua")
    if err := os.WriteFile(tmpFile, []byte(script), 0644); err != nil {
        t.Fatal(err)
    }

    if err := r.RunFile(tmpFile, nil); err != nil {
        t.Fatalf("RunFile failed: %v", err)
    }

    outFile := filepath.Join(projectRoot, "subdir", "nested", "output.txt")
    content, err := os.ReadFile(outFile)
    if err != nil {
        t.Fatalf("Failed to read output file: %v", err)
    }

    if string(content) != "deep content" {
        t.Errorf("Expected 'deep content', got %q", string(content))
    }
}
```

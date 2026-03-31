---
finding: |-
    The sandbox removes loadfile, dofile, load, and loadstring from the global environment. However, there are NO TESTS verifying that these functions are actually unavailable. This is a critical oversight because:

    1. A regression could silently re-enable arbitrary code execution
    2. You cannot verify sandbox effectiveness without attempting bypass
    3. gopher-lua updates could change behavior

    The test file has tests for path traversal but nothing for:
    - Attempting to use io.open(), io.read(), io.write()
    - Attempting to use os.execute(), os.remove(), os.rename()
    - Attempting to use debug.getinfo(), debug.setlocal()
    - Attempting to use load(), loadfile(), dofile(), loadstring()

    Location: /Users/jeroen/Work/sourcehaven/rela-3/internal/lua/runtime_test.go

    This WILL break at 3 AM when someone updates gopher-lua and doesn't realize the sandbox regressed.
id: RR-pot8
resolution: Added TestSandbox_DangerousLibrariesUnavailable and TestSandbox_DangerousFunctionsRemoved tests to verify io, os, debug libraries are unavailable and dangerous functions (loadfile, dofile, load, loadstring, rawget, rawset, rawequal, rawlen, getmetatable, setmetatable) are nil.
severity: critical
status: addressed
title: Missing sandbox tests for dangerous Lua functions
type: review-response
---

## Required Tests

```go
func TestSandbox_DangerousFunctionsRemoved(t *testing.T) {
    ws := testWorkspace(t)
    var buf bytes.Buffer
    r := New(ws, ws.Meta(), "/tmp", &buf)
    defer r.Close()

    dangerousFunctions := []struct {
        name   string
        script string
    }{
        {"loadfile", `loadfile("foo")`},
        {"dofile", `dofile("foo")`},
        {"load", `load("print('hi')")`},
        {"loadstring", `loadstring("print('hi')")`},
    }

    for _, tc := range dangerousFunctions {
        t.Run(tc.name, func(t *testing.T) {
            err := r.RunString(tc.script)
            // Should fail because function is nil
            if err == nil {
                t.Fatalf("%s should not be available in sandbox", tc.name)
            }
        })
    }
}

func TestSandbox_IoLibraryNotAvailable(t *testing.T) {
    ws := testWorkspace(t)
    var buf bytes.Buffer
    r := New(ws, ws.Meta(), "/tmp", &buf)
    defer r.Close()

    scripts := []string{
        `io.open("/etc/passwd", "r")`,
        `io.read("*a")`,
        `io.popen("ls")`,
    }

    for _, script := range scripts {
        err := r.RunString(script)
        if err == nil {
            t.Fatalf("io library should not be available: %s", script)
        }
    }
}

func TestSandbox_OsLibraryNotAvailable(t *testing.T) {
    ws := testWorkspace(t)
    var buf bytes.Buffer
    r := New(ws, ws.Meta(), "/tmp", &buf)
    defer r.Close()

    scripts := []string{
        `os.execute("rm -rf /")`,
        `os.remove("/tmp/test")`,
        `os.getenv("HOME")`,
    }

    for _, script := range scripts {
        err := r.RunString(script)
        if err == nil {
            t.Fatalf("os library should not be available: %s", script)
        }
    }
}
```

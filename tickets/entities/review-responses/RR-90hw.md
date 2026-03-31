---
finding: |-
    The code sets RegistrySize: 1024 * 64 to limit registry size, but gopher-lua doesn't have built-in memory limiting. A malicious script can still exhaust memory by:

    1. Creating large strings: `s = string.rep('x', 1000000000)`
    2. Creating large tables in a loop
    3. Recursive data structures

    The CallStackSize limit (1024) prevents stack overflow but not heap exhaustion.

    This is a minor issue because:
    - The tool is designed for trusted users working on their own projects
    - DoS against yourself is not a high-priority threat
    - Go's GC will eventually clean up

    But worth documenting as a known limitation.

    Location: /Users/jeroen/Work/sourcehaven/rela-3/internal/lua/runtime.go lines 48-52
id: RR-90hw
reason: Memory limits would require implementing custom allocator hooks in gopher-lua. CallStackSize and RegistrySize limits are in place. For MCP context, scripts are short-lived and timeout-controlled. Will revisit if abuse is observed.
severity: minor
status: deferred
title: No memory limit on Lua VM despite registry size limit
type: review-response
---

## The Code

```go
L := lua.NewState(lua.Options{
    SkipOpenLibs:  true,
    CallStackSize: 1024,      // Limit call stack depth to prevent stack overflow
    RegistrySize:  1024 * 64, // Limit registry size
})
```

## Memory Exhaustion Example

```lua
-- This will still work and consume arbitrary memory
local t = {}
for i = 1, 1000000000 do
    t[i] = string.rep("x", 1000)
end
```

## Mitigation Options

1. **Context with timeout**: Wrap script execution in context.WithTimeout
2. **Resource monitoring**: Check memory usage periodically (impractical in Lua)
3. **Documentation**: Note this as a known limitation

## Recommended Approach

Add execution timeout as a practical mitigation:

```go
func (r *Runtime) RunStringWithTimeout(code string, timeout time.Duration) error {
    ctx, cancel := context.WithTimeout(context.Background(), timeout)
    defer cancel()
    
    done := make(chan error, 1)
    go func() {
        done <- r.L.DoString(code)
    }()
    
    select {
    case err := <-done:
        return err
    case <-ctx.Done():
        r.L.Close() // Kill the VM
        return ctx.Err()
    }
}
```

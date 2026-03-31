---
finding: |-
    The sandbox removes loadfile, dofile, load, and loadstring. However, several other potentially dangerous base functions are NOT removed:

    - rawget / rawset - Can bypass metamethod protections
    - getmetatable / setmetatable - Can modify metatable of any value
    - rawequal / rawlen - Less dangerous but part of raw access pattern

    While these don't directly enable code execution or file access, they can be used to:
    1. Bypass any future sandboxing attempts using metatables
    2. Modify the behavior of the rela module itself
    3. Access internal implementation details

    In gopher-lua specifically, the _G (global table) is accessible, which combined with setmetatable could modify global function behavior.

    For a production sandbox, these should also be removed or carefully considered.

    Location: /Users/jeroen/Work/sourcehaven/rela-3/internal/lua/runtime.go lines 91-95
id: RR-ytyp
resolution: Added rawget, rawset, rawequal, rawlen, getmetatable, setmetatable to the list of removed globals in openSafeLibraries. Added tests verifying these are nil.
severity: significant
status: addressed
title: rawget/rawset/getmetatable/setmetatable not removed from sandbox
type: review-response
---

## Current Code

```go
// Remove dangerous base functions that could bypass sandbox
ls.SetGlobal("loadfile", lua.LNil)
ls.SetGlobal("dofile", lua.LNil)
ls.SetGlobal("load", lua.LNil)
ls.SetGlobal("loadstring", lua.LNil)
```

## Recommended Addition

```go
// Remove dangerous base functions that could bypass sandbox
ls.SetGlobal("loadfile", lua.LNil)
ls.SetGlobal("dofile", lua.LNil)
ls.SetGlobal("load", lua.LNil)
ls.SetGlobal("loadstring", lua.LNil)

// Remove raw access functions that could bypass metamethod protections
// These could be used to modify rela module internals or bypass future protections
ls.SetGlobal("rawget", lua.LNil)
ls.SetGlobal("rawset", lua.LNil)
ls.SetGlobal("rawequal", lua.LNil)
ls.SetGlobal("rawlen", lua.LNil)

// Consider removing metatable access to prevent modification of rela internals
// ls.SetGlobal("getmetatable", lua.LNil)
// ls.SetGlobal("setmetatable", lua.LNil)
```

## Risk Assessment

If scripts don't need raw access or metatable manipulation (likely they don't), removing these functions is pure security benefit with no functionality cost.

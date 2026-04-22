---
id: TKT-5LCNM
type: ticket
title: Consolidate script path plumbing in script.Engine.ExecuteDocument / ExecuteAction
kind: refactor
priority: low
status: backlog
---

## Problem

Both `script.Engine.ExecuteDocument` (TKT-CGBVW) and `ExecuteAction` pass the
script path to `NewWriterRuntime` for per-script secret lookup AND call
`runtime.SetScriptPath(path)` separately so `rela.cache.*` bindings namespace
correctly. The awkwardness is called out in a 5-line comment in executor.go.

## Options

1. **Add `lua.WithScriptPath(path) Option`** that sets the field inside the runtime constructor. `NewWriterRuntime` can apply it automatically when it knows the path (which it already does for the secrets path).
2. **Have `NewWriterRuntime` call `SetScriptPath` internally** when the caller's scriptPath is non-empty. Simpler API; removes the need for the caller to do it separately.

Option 2 is smaller. Either works.

## Scope

- Apply the cleanup to both `ExecuteDocument` and `ExecuteAction`.
- Verify existing `RunFile` / `RunFileContent` paths still set the script path correctly (they use a different code path).
- Remove the trailing "wire the namespace explicitly for rela.cache.*" comments in both callsites.

## Out of scope

Changing how `rela.cache.*` binds or namespaces.

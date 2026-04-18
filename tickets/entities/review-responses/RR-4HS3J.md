---
id: RR-4HS3J
type: review-response
title: Store concrete deps inside runtime, not accessor interfaces
finding: Accepting interfaces at the constructor boundary is correct. But inside internal/lua, storing interface accessors (r.deps.Store()) means every binding re-invokes the accessor method on every Lua call. Consider defining a private `type deps struct { store store.Store; manager entitymanager.EntityManager; ... }` and having NewReader/NewWriter snapshot the interface methods into that struct once. Bindings then read fields (r.deps.store) which is simpler and sidesteps the 'accessor does construction work' pitfall mentioned in RR-XU0FS.
severity: minor
resolution: 'Subsumed by RR-XU0FS redesign: ReadDeps/WriteDeps are plain value structs, so the ''interface accessor called from every binding'' concern dissolves. Runtime holds the deps struct directly and bindings read fields like r.deps.store, not r.deps.Store().'
status: addressed
---

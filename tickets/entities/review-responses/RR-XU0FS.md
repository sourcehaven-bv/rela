---
id: RR-XU0FS
type: review-response
title: Workspace.Tracer()/Searcher() construct per call — memoize or snapshot
finding: Method-returning-interface accessors like ReadDeps.Tracer() and Searcher() are acceptable only if they are cheap getters. Workspace.Tracer() currently calls tracer.New(w.Store()) on every call and Searcher() allocates a new wsSearcher. A single Lua run may invoke trace_from/trace_to/search multiple times; each call would construct a fresh Tracer/Searcher. Either memoize on Workspace or have NewReader/NewWriter snapshot the accessors into the Runtime struct at construction time so each binding reads a stable reference.
severity: significant
resolution: 'Redesigned per user direction: lua.ReadDeps/WriteDeps are plain value structs (not accessor interfaces). Concrete services (store, tracer, searcher, manager, meta, projectRoot) are constructed once by the helper and passed as values. No per-call accessor overhead. No service-locator shape. Workspace.Tracer()/Searcher() allocation cost is paid exactly once per runtime construction, in the helper.'
status: addressed
---

---
id: PLAN-3VRAW4
type: planning-checklist
title: 'Planning: MCP schema hot-reload'
status: done
---

- [x] Problem understood — MCP bakes the metamodel into `mcp.Deps` and into every service `appbuild.assemble` builds; a schema edit needs a restart.
- [x] Approach chosen — reuse `appbuild.SharedBase` + `Assemble` against the existing store/searcher, publish `mcp.Deps` via an `atomic.Pointer` snapshot.
- [x] Alternatives considered — swapping `Deps.Meta` alone was rejected: the entitymanager holds its own metamodel, so writes would keep validating against the old schema (a half-updated server). Rebuilding the whole `Services` via `Discover` was rejected: it closes the store and forces a full search reindex.
- [x] Risk identified — every `Assemble` starts per-assembly background services (job queue, mail worker, GC sweep) and the only teardown, `Services.Close`, also closes the shared store. Addressed with a scoped `CloseAssembly`.
- [x] ~~Security review~~ (N/A: stdio MCP is wired `acl.NopACL` by design — the filesystem is the trust boundary; the reload changes no gate. The remote transport starts no watcher.)
- [x] Test plan — reload visible to read tools; reload reaches the write path; a handler bound before the reload observes it; bad schema keeps last-good; store/searcher reused; close-after-reload clean; concurrent reload under `-race`.

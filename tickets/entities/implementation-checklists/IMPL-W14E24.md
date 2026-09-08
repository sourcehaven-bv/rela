---
id: IMPL-W14E24
type: implementation-checklist
title: 'Implementation: MCP schema hot-reload'
status: done
---

- [x] `appbuild`: `Services.CloseAssembly` + shared `stopBackgroundServices`; `Services.Base()`; `SharedBase.Config()`.
- [x] `mcp`: snapshot provider (`atomic.Pointer`), `Server.ReloadDeps`, `bind`/`group` so registered handlers resolve the current snapshot per request.
- [x] `cli`: `mcpServices.reload` + `watchSchema`, wired from `rela mcp`.
- [x] Tests added at all three layers, including a `-race` concurrency test.
- [x] `just lint` clean (0 issues).
- [x] `just arch-lint` clean.
- [x] `just plimsoll` clean — load lines re-pinned with rationale.
- [x] `just comment-lint` clean.
- [x] `just coverage-check` passes.
- [x] All build tags compile (default, postgres, memorybackend, sqlite).

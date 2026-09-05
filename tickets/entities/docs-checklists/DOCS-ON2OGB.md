---
id: DOCS-ON2OGB
type: docs-checklist
title: 'Docs: MCP schema hot-reload'
status: done
---

- [x] Code docs — godoc on `Server.ReloadDeps`, `bind`, `snapshotProvider`, `Services.CloseAssembly`, `SharedBase.ForReassembly`/`Config`, `mcpServices.reload`/`watchSchema`, each stating why rather than what.
- [x] Project docs — `docs-project/entities/guides/GUIDE-mcp-server.md` (source of the generated `docs/mcp-server.md`) "File Watching" section rewritten.

Two pre-existing inaccuracies were corrected while there: it claimed
`schema.yaml` was already watched (only `entities/` and `relations/` were), and
it claimed changes were announced via `notifications/resources/list_changed`,
which the go-sdk migration removed. The remote-MCP section's "no change
notifications" bullet now states that a `schema.yaml` edit still needs a restart
on that transport, since it starts no watcher.
- [x] `just docs` regenerated and the generated output committed.
- [x] ~~External docs~~ (N/A: no public API or CLI flag changed — the behavior is automatic.)
- [x] ~~CHANGELOG~~ (N/A: repo keeps no changelog; release notes derive from commits.)

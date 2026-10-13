---
id: DOCS-S75WFR
type: docs-checklist
title: Documentation
status: done
---

<!-- @managed: claude-workflow v1 -->

## Code Documentation

- [x] Comments where logic isn't obvious — `installGraphReads` states what a
read sees and why; `forEachEntity` states that the lua reads depend on its
collect-then-write order; `relationQueryArg` lists every rejected mistake and
why
- [x] Function/type docs if public API — no exported API; package-internal
`graphReader`, `luaGetEntity`, `luaGetFaces`, `luaGetRelations` documented

## Project Documentation

- [x] README updated (if applicable) — regenerated via `just docs`
- [x] ~~CLAUDE.md updated~~ (N/A: follows the existing consumer-side
interface pattern; the datamigration rule "never hand it a write handle" still
holds)
- [x] ~~Help text accurate~~ (N/A: no CLI change)

## External Documentation

- [x] ~~Changelog entry added~~ (N/A: repo keeps no changelog file)
- [x] API docs updated (if applicable) — GUIDE-data-migration gains
"Reading the graph" under "The Lua escape hatch": the three functions, the
classify-by-relation example, read consistency, dry-run behaviour, faces,
argument checks, idempotency and cost. `docs/data-migration.md` regenerated from
it.

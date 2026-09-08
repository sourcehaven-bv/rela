---
id: DOCS-CFGSQL
type: docs-checklist
title: 'Documentation: SQLite-backed config source'
status: done
---

<!-- @managed: claude-workflow v1 -->

## Code Documentation

- [x] Comments where logic isn't obvious — three decisions carry their
  reasoning: why `List` compares a literal prefix rather than using `GLOB`
  (the caller's directory would become a pattern), why `sqlitestore` declares
  its own `ConfigReader` instead of naming `config.Loader` (arch-lint forbids
  the import, and Go matches method sets exactly), and why the accessor exists
  on both `Conn` and `Store` (the wiring site discovers it by type assertion
  on the store).
- [x] Function/type docs if public API — `ProjectFiles`, `ConfigReader`,
  `Load`, `List`, `Put`, `Paths` and `layerStoreConfig` all documented, with
  the disk-first ordering stated as the design rather than an implementation
  detail.

## Project Documentation

- [x] ~~README updated~~ (N/A: no user-visible surface yet — nothing writes
  `project_files` until `db load` lands in Phase C)
- [x] ~~CLAUDE.md updated~~ (N/A: deferred deliberately to Phase C. The
  storage-backend section should describe config-in-the-database once an
  operator can actually use it; documenting a table nothing populates would be
  a note about internals.)
- [x] ~~Help text accurate~~ (N/A: no CLI changes)

## External Documentation

- [x] ~~Changelog entry added~~ (N/A: no changelog file in this repo; behaviour
  is unchanged on every build — the layer is empty until config is baked in)
- [x] ~~API docs updated~~ (N/A: no HTTP or MCP surface touched)

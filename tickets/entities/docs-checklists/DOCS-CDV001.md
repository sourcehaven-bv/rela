---
id: DOCS-CDV001
type: docs-checklist
title: 'Docs: VTODO renderer in internal/calfeed'
status: done
---

<!-- @managed: claude-workflow v1 -->

## Code Documentation

- [x] Godoc on `calfeed.Todo`, `RenderTodo`, `TodoETag` and `CollectionTag`, including why `Todo.Complete(at)` sets the whole STATUS/COMPLETED/PERCENT-COMPLETE trio in one call rather than exposing three independent fields (RFC 4791 §7.8.9 filters on COMPLETED while UIs read STATUS, so a half-set state reads as done in one client and pending in another)
- [x] Doc comment on the Apple fixture tests explaining that assertions compare rendered output against the fixture's own parsed value, so the fixture stays the single source of truth
- [x] ~~CLAUDE.md pattern update~~ (N/A: reuses the existing VEVENT render patterns — `writeProp`/`foldLine`/`escapeText`)

## Project Documentation

- [x] ~~User-facing docs~~ (N/A: `internal/calfeed` is a leaf model→bytes package with no config surface; the operator-facing documentation lands with the CalDAV collection config in TKT-UGYSC8)

## External Documentation

- [x] ~~README / external docs~~ (N/A: internal package)

**Docs verified:** `.gitattributes` pins `*.ics -text` so the byte-exact Apple
fixtures survive checkout; noted in the implementation checklist.

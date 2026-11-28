---
id: DOCS-HD7FGZT
type: docs-checklist
title: 'Docs: Push id relation filters down to the store'
completed: "2026-11-28"
status: done
---

<!-- @managed: claude-workflow v1 -->

## Code Documentation

- [x] Comments where logic isn't obvious (matcher and pass docs name the pushdown)
- [x] Function/type docs if public API (`Resolver.ResolveIDsStrict` documented)

## Project Documentation

- [x] ~~README updated (if applicable)~~ (N/A: no project-level change)
- [x] ~~CLAUDE.md updated (if new patterns)~~ (N/A: no new pattern)
- [x] ~~Help text accurate (if CLI changes)~~ (N/A: no CLI change)

## External Documentation

- [x] ~~Changelog entry added~~ (N/A: the repo has no changelog)
- [x] API docs updated (if applicable): postgres-backend, metamodel and data-entry guides say when an id filter pushes down; `docs/` regenerated

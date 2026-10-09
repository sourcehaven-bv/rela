---
id: DOCS-OW078N
type: docs-checklist
title: 'Docs checklist: Piles (TKT-K3RJLH)'
status: done
---

<!-- @managed: claude-workflow v1 -->

## Code Documentation

- [x] Comments where logic isn't obvious (foreign-push limits, Overflow modes, file lock, rename hook ordering, read-gate ordering)
- [x] Function/type docs if public API (piles, kvpiles, pgpiles, lua/mcp PileFuncs, autocascade PilePusher; comment-lint gate clean, no new comment-report findings)

## Project Documentation

- [x] ~~README updated~~ (N/A: README is generated from docs-project and lists no per-feature APIs)
- [x] CLAUDE.md updated (if new patterns) (internal/piles row in the package table)
- [x] ~~Help text accurate~~ (N/A: no CLI command or flag changes)

## External Documentation

- [x] ~~Changelog entry added~~ (N/A: the repo keeps no changelog; release notes come from PR titles)
- [x] API docs updated (if applicable) (docs/data-entry/api-reference.md "Piles" section; GUIDE-data-entry, GUIDE-lua-scripting, GUIDE-mcp-server, GUIDE-metamodel, GUIDE-postgres-backend updated and regenerated with `just docs`)

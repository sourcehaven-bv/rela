---
id: DOCS-EZ7LPQ
type: docs-checklist
title: 'Docs: lease storage (TKT-6QC5EL)'
completed: "2026-10-11"
status: done
---

<!-- @managed: claude-workflow v1 -->

## Code Documentation

- [x] Comments where logic isn't obvious (package docs for lease, kvlease, pglease, filelock; lock ordering, token floor, version contract)
- [x] Function/type docs if public API (Store interface contract, validators, errors)

## Project Documentation

- [x] ~~README updated (if applicable)~~ (N/A: no user-facing change)
- [x] CLAUDE.md updated (if new patterns) (internal/lease row in the package table)
- [x] ~~Help text accurate (if CLI changes)~~ (N/A: no CLI change)

## External Documentation

- [x] ~~Changelog entry added~~ (N/A: the repo has no changelog)
- [x] ~~API docs updated (if applicable)~~ (N/A: no API surface until TKT-27XVYS)

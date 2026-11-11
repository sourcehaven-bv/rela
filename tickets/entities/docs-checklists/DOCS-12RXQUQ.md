---
id: DOCS-12RXQUQ
type: docs-checklist
title: 'Docs: Headless rela-server build for sqlite and postgres in releases'
started: "2026-11-11"
status: done
---

<!-- @managed: claude-workflow v1 -->

## Code Documentation

- [x] Comments where logic isn't obvious
- [x] Function/type docs if public API

## Project Documentation

- [x] ~~README updated (if applicable)~~ (N/A: README does not list release archives)
- [x] ~~CLAUDE.md updated (if new patterns)~~ (N/A: build tag follows the existing backend-tag pattern)
- [x] ~~Help text accurate (if CLI changes)~~ (N/A: no CLI flags changed)

## External Documentation

- [x] ~~Changelog entry added~~ (N/A: changelog is generated from commits by goreleaser)
- [x] API docs updated (if applicable)

GUIDE-sqlite-backend and GUIDE-postgres-backend gained a "Headless server"
section (binary and archive names, what is served, what answers 404, the `just`
recipe); `docs/` regenerated with `just docs`.

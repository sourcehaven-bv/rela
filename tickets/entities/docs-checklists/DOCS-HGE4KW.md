---
id: DOCS-HGE4KW
type: docs-checklist
title: 'Docs: Deleted-relation history id-reuse disambiguation'
status: done
---

<!-- @managed: claude-workflow v1 -->

## Code Documentation

- [x] Godoc on `ListRelationLifetimes`, `RelationLifetime`, `RelationHistoryQuery`, `resolvePurgeLineage` explains the lifetime model + auth boundary
- [x] Purge multi-lifetime guardrail documented at the DTO and impl

## Project Documentation

- [x] ~~CLAUDE.md relation-versioning section~~ (N/A: the injected-VersionStore + lifetime model is documented in godoc; a follow-up doc pass can fold a summary into the postgres-backend guide)
- [x] CLI `--help` text on the new flags (`--list-lifetimes`, `--lifetime`, `--all-lifetimes`)

## External Documentation

- [x] ~~docs-project postgres-backend guide lifetime note~~ (N/A: deferred to a docs follow-up; the mechanism is complete and self-documented in code)

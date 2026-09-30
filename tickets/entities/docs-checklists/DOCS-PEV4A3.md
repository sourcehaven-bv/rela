---
id: DOCS-PEV4A3
type: docs-checklist
title: 'Docs: Store API takes entity.Ref and RelationKey; queries must select their faces'
status: done
---

<!-- @managed: claude-workflow v1 -->

## Code Documentation

- [x] Comments where logic isn't obvious: migration `0018_face_read_indexes.sql` explains each dropped and created index and the lock it holds; the archguard guards (bareref, tailless, faceselect) carry advice text and a reason per allowlist entry
- [x] Function/type docs if public API: `store.Store` methods (`GetEntity`, `DeleteFace`, `DeleteFamily`, `RenameFamily`, the family attachment methods), `store.FaceSelection` and its constructors, `store.FamilyHeaders`, `entity.RelationKey` and `Relation.Identity()` are documented

## Project Documentation

- [x] ~~README updated~~ (N/A: no user-facing command or config change beyond the address grammar, which docs/cli-reference.md covers)
- [x] CLAUDE.md updated: "Don't add a zero-face read" now describes `entity.Ref` and the `bareref` guard in place of the deleted zero-face allowlist (#1735)
- [x] Help text accurate: docs/cli-reference.md gained "Entity addresses" (`ID@face`) and per-command address notes; docs/lua-scripting.md and docs/mcp-server.md updated for addresses

## External Documentation

- [x] ~~Changelog entry added~~ (N/A: the project keeps no changelog file; the migration 18 lock note is in docs/postgres-backend.md)
- [x] API docs updated: docs/postgres-backend.md describes the migration 18 write lock and advises a maintenance window or `rela db migrate` as a deploy step; docs/acl-security.md updated for per-face gating

---
id: DOCS-FIY2UG
type: docs-checklist
title: Documentation
status: done
---

<!-- @managed: claude-workflow v1 -->

## Code Documentation

- [x] Godoc on new exported types/functions (transform.Registry/Def/Engine/Renderer/EntityRenderer, cmdexec.Runner, metamodel.TransformDef)
- [x] Load-bearing invariants documented in code comments (ACL-gate-before-render, argv-no-shell, batched relation resolution)

## Project Documentation

- [x] CLAUDE.md — added `internal/cmdexec` + `internal/transform` to the package table and a "View export & transforms" rules section (export downstream of a gated view; list renderer lives in dataentry for the ACL gate; Lua override via gated `_documents`; hardened downloads)
- [x] ~~docs-project auto-gen entity~~ (N/A for this pass: `docs/*.md` are auto-generated from `docs-project/entities/`; a standalone `docs/transforms.md` was authored instead. Folding it into docs-project is a reasonable follow-up doc-task, not a blocker.)

## External / User Documentation

- [x] docs/transforms.md — user-facing reference: registering transforms, CLI `rela render`, data-entry export (entity + list), `?document=` override, security notes, v1 limits
- [x] ~~docs/cli-reference.md / docs/data-entry.md updates~~ (N/A: both are auto-generated; the standalone transforms.md covers CLI + data-entry export in one place until a docs-project source entity is added)

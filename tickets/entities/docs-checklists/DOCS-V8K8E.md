---
id: DOCS-V8K8E
type: docs-checklist
title: 'Docs: Add Edit button to data-entry document view'
status: done
---

<!-- @managed: claude-workflow v1 -->

## User-facing docs

- [x] User guide / reference docs updated — `docs/data-entry.md`, Documents
section, paragraph + extended YAML example explaining the new `edit:` sub-block,
including the bare-`edit:`-vs-`edit: {}` YAML caveat surfaced by RR-4AXJR.
- [x] ~~CLI help text~~ (N/A: no CLI changes)
- [x] ~~README.md~~ (N/A: project-level scope unchanged)
- [x] ~~API docs~~ (N/A: no new API surface — config payload extends an
existing schema and is read by the SPA via `/api/v1/_config`)

## Code docs

- [x] Go struct comment on `DocumentEdit` and the `Edit *DocumentEdit`
field on `DocumentConfig` — explains the YAML deserialisation caveat.
- [x] `editEntity` Vue handler — comment documents the deliberate
divergence from `EntityDetail.vue`'s `router.back()` pattern, with the
deep-linkability rationale.
- [x] ~~CLAUDE.md~~ (N/A: no new patterns introduced; pattern follows the
existing `list.edit_form` / `kanban.edit_form` shape)

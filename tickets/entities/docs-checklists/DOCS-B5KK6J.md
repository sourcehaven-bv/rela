---
id: DOCS-B5KK6J
type: docs-checklist
title: 'Docs: Form relation direction inference + required direction for self-referencing relations'
status: done
---

<!-- @managed: claude-workflow v1 -->

## Code Documentation

- [x] Godoc on new exported symbols — `InferDirection` and `DirectionResolution`
(`internal/dataentryconfig/direction.go`) document the rule and why the old
default was unsafe
- [x] Package/type docs updated — `FormRelationDirectionMigration` explains why
it deliberately skips self-referencing bindings; `DataEntryCleanupMigration`
records why `direction` is NOT re-derivable (unlike widget/target_type/label)
- [x] `resolveFormRelations` documents why direction MUST be resolved
server-side (the SPA tests `direction === 'incoming'` literally)

## Project Documentation

- [x] `docs/data-entry.md` — new "How `direction` is resolved" section under
"Reverse (incoming) Relations", with the from/to/both table and an upgrade note
pointing at `rela migrate`
- [x] `direction` row in the form-relations field table updated to state that it
is inferred when omitted and required for self-referencing relations
- [x] ~~Architecture docs~~ (N/A: no package boundary or wiring change; the new
`direction.go` sits inside the existing `dataentryconfig` package)

## External Documentation

- [x] ~~README~~ (N/A: no top-level feature or command added)
- [x] ~~CHANGELOG~~ (N/A: project does not maintain one; the breaking change is
documented in the docs upgrade note and the ticket)
- [x] Migration path documented — `rela migrate` handles unambiguous bindings;
`rela validate` lists the self-referencing ones the author must decide

## Verification

- [x] Docs examples match actual behavior (the from/to/both table mirrors
`InferDirection`'s switch exactly)
- [x] Upgrade note is accurate — verified end-to-end on `tickets/data-entry.yaml`
(26 auto-filled, 8 listed for the author)

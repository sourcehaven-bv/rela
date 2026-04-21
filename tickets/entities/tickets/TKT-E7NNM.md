---
id: TKT-E7NNM
type: ticket
title: 'Data-entry create form: prefix picker for multi-prefix types and manual ID field'
kind: enhancement
priority: medium
effort: m
status: done
---

## Problem

The data-entry create forms currently don't support two metamodel features:

1. **Multi-prefix types** (`id_prefixes: ["DEC-", "ADR-"]`): when an entity type declares more than one ID prefix, the UI must let the user pick which prefix to use for the new entity. Today the backend (`handleV1CreateEntity` in `internal/dataentry/api_v1.go`) doesn't accept a prefix override, and `V1EntityType` only exposes `id_prefix` (the first one in the list). The main create form (`frontend/src/components/forms/DynamicForm.vue`) has no prefix UI. The inline create modal (`frontend/src/components/forms/InlineCreateModal.vue`) has no prefix UI either.

2. **Manual ID type** (`id_type: manual`): when an entity type requires a manually supplied ID, the create form must show an editable text field. `InlineCreateModal.vue` already handles this, but `DynamicForm.vue` (the main form used for `/form/:id` routes) has no ID field at all, so new entities of manual-ID types cannot be created through the main form UI.

## Acceptance criteria

- When the selected entity type has `id_type: manual`, both `DynamicForm.vue` and `InlineCreateModal.vue` show a required editable ID field; on submit the value is sent as `payload.id`.
- When the selected entity type has `id_prefixes` with more than one value, the create form shows a picker (select/radio) to choose the prefix. Single-prefix types keep current behaviour (no picker shown).
- The backend schema endpoint exposes the full list of prefixes (not just the first) to the frontend.
- The backend create endpoint accepts an optional `prefix` override and passes it to `workspace.CreateOptions.Prefix`.
- Behaviour is covered by unit tests (Go handler, Vue components) and an E2E test.

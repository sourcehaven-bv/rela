---
id: TKT-NSC075O
type: ticket
title: Convert dialog uses form widgets and the Milkdown editor
kind: enhancement
priority: medium
effort: m
started: "2026-11-02"
completed: "2026-11-02"
status: done
---

## Description

The convert dialog (`frontend/src/components/entity/ConversionModal.vue`) builds
its own inputs instead of using the shared form machinery. As a result:

- Fields do not get the widgets the normal form uses. Reference pickers, user
pickers, the status control, enum labels and per-field widget choices from the
form config are missing.
- It lists every unmapped target property, required or not. The preview's
`required` list is only used to build one combined error message.
- The body is a plain textarea, not the Milkdown editor.

## Approach

- Render fields through `FieldRenderer`/`FieldShell`, driven by the target
type's resolved form, so widgets, labels and options match the create form.
- List first the required target fields the source cannot supply. Put the
other unmapped fields behind "more fields".
- Show the preview's `required` errors next to each field.
- Replace the body textarea with `MarkdownEditor` (Milkdown). When the mapping
supplies the content (`content_mapped`), show the result read-only.

## Out of scope

Mentions of the old id in other entities: see the companion ticket on
`RenameFamily`.

---
id: TKT-9QNHN
type: ticket
title: Add Edit button to data-entry document view
kind: enhancement
priority: medium
effort: s
status: done
---

## Problem

The document view (`/document/:name/:entityId`) renders a read-only HTML panel
about a specific entity but does not expose a direct way for the user to
navigate to the edit form for that entity. Users currently have to navigate back
to a list, find the entity, and click edit from there — the document is clearly
about an entity, so getting to its edit form should be one click away.

## Proposed change

Add an "Edit" button to the document view's page header (next to Back /
Refresh). Clicking it navigates the user to the edit form for the entity the
document is about (i.e. `/form/<editFormId>/<entityId>`), with `return_to`
pointing back to the current document URL so saving returns the user here.

## Acceptance criteria

- An "Edit" button appears in the document view header when an edit form
exists for the document's `entity_type`.
- The button is hidden when no edit form is configured for that entity type
(or when `entity_type` is not set on the document config).
- Clicking the button navigates to `/form/<editFormId>/<entityId>` with a
`return_to` query param that brings the user back to the document view on
save/cancel.
- Existing Back and Refresh buttons continue to work unchanged.

## Out of scope

- No changes to the document rendering pipeline, server-side renderer, or
`edit://` link rewriting.
- No new "Create" button — only Edit for the entity the document is about.
- No keyboard shortcut wiring (can be a follow-up).

## Affected files (initial)

- `frontend/src/views/DocumentView.vue` — add the button + handler.
- E2E coverage in `frontend/e2e/` for the new affordance.

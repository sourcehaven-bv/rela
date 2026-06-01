---
id: TKT-HOIX1
type: ticket
title: Add widget + editable config surface for view sections
kind: enhancement
priority: medium
effort: s
status: backlog
---

## Goal\n\nAllow view config authors to (a) override which widget renders a given property in a view section, and (b) opt a section into inline-edit mode.\n\n## Scope\n\n- `ViewSectionField` (Go: `internal/dataentryconfig/config.go`) gains an optional `widget` string and an optional `editable` bool.\n- `ViewSection` gains an optional `editable` bool that defaults each contained field to inline-edit.\n- `validate.go` validates widget names against the registry and against the property's type (no `checkbox` widget on a `date` property).\n- Backend rendering (`internal/dataentry/api_v1.go`) populates `V1ViewCell.Widget` from config (the field is already in the wire schema, just not populated).\n- Frontend `ViewSectionField` TS type gains the new fields; the registry consults them when picking widget and mode.\n- Backward-compatible defaults: omitting `widget`/`editable` produces today's behaviour exactly.\n\n## Non-goals\n\n- No new widget types.\n- No content-section inline-edit (final ticket).\n- No bulk operations or row-level actions.\n\n## Why\n\nThe payoff ticket: once this lands, the Daily-Notes 'click checkbox to mark task done' becomes a config line.

---
id: TKT-3R7RF3
type: ticket
title: Widget override for view section fields (`widget:` on ViewSectionField)
kind: enhancement
priority: medium
effort: s
status: backlog
---

## Goal

Let view config authors override which widget renders a given property in a view
section, independent of the type-based default.

Split out of TKT-HOIX1, which now covers only the `render: input | display`
axis. This ticket is the orthogonal *which widget* axis and should land after
it.

## Scope

- `ViewSectionField` (`internal/dataentryconfig/config.go:610-613`) gains an optional
`widget string`.
- `internal/dataentryconfig/validate.go` validates widget names against the registry **and**
against the property's type — no `checkbox` widget on a `date` property.
- Backend populates `V1ViewCell.Widget` from config. The field is reportedly already in the
wire schema but unpopulated — verify before implementing.
- Frontend `ViewSectionField` TS type gains `widget`; `frontend/src/widgets/registry.ts`
consults it when picking the widget, ahead of the `defaultWidgetFor` type
dispatch (`registry.ts:18-28`).
- Omitting `widget` preserves today's type-based selection exactly.

## Why

The payoff case: the Daily-Notes "click checkbox to mark task done" interaction
becomes a config line rather than custom code. Note this needs `render: input`
from TKT-HOIX1 to be useful, since a checkbox you cannot click is just an icon.

## Non-goals

- No new widget types — only overriding selection among registered ones.
- The `render: input | display` axis (TKT-HOIX1).
- Markdown and relation widgets, which do not exist in the registry
(`frontend/src/widgets/registry.ts:105-117`).

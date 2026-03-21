---
effort: m
id: TKT-tv5u
kind: refactor
priority: medium
status: done
title: Remove v1 HTMX UI code after Vue migration
type: ticket
---

Remove all v1 HTMX template-based UI code now that the Vue SPA has feature parity.

## Changes
- Remove templates/ directory and all v1 HTML templates
- Remove v1 static assets (htmx, easymde, slimselect, etc.)
- Remove template parsing from app.go and watcher.go
- Rewrite router.go to serve Vue SPA at root path
- Reduce handlers.go to only handleToggleCheckbox and handleEntityHelp
- Remove unused handlers (kanban, conflict, template-based)
- Clean up unused functions

## Result
- ~14,500 lines of code removed
- Vue SPA now served at root path `/`
- All v1 template rendering eliminated

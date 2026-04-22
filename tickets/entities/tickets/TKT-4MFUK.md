---
id: TKT-4MFUK
type: ticket
title: Refactor document links to app-relative + add Lua router/URL helpers
kind: enhancement
priority: medium
effort: m
status: done
---

## Problem

The data-entry document renderer rewrites `create://type` and `edit://type/id`
links into `/form/...` URLs. This is broken in three ways:

1. **Closed scheme.** A hardcoded regex (`internal/dataentry/document.go:335`) recognizes only `create` and `edit`. Authors can't link to lists, kanbans, entity detail pages, custom views, search, or other documents — even though the SPA router already exposes all of them.
2. **Implicit form selection.** `create://ticket` carries entity type but no form ID. When multiple forms target the same entity type (e.g. `quick_ticket`, `full_ticket`), the link has no way to choose. Worse, the rewriter emits `/form/{entityType}/...` without consulting `createFormForType` / `editFormForType` — it relies on accidental form-id/entity-type coincidence.
3. **Undocumented params.** The `prop.*` / `rel.*` query convention is de-facto only, and `return_to` is appended as a sibling key — an author-written `return_to` silently collides with the injected one.

## Proposed direction

- **Accept app-relative links as-is.** Document authors write real SPA paths (`/form/full_ticket?prop.status=open`, `/entity/ticket/TKT-001`, `/list/all_tasks`). The rewriter's only job is to append `return_to` when the target is a form.
- **Drop `create://` / `edit://` rewriting.** No `form://` sugar — explicit form IDs are the point.
- **Lua URL + router helpers.** Hand-concatenating URLs from Lua is error-prone. Provide:
  - A URL builder that handles encoding and the `prop.*` / `rel.*` conventions.
  - A named-route helper in the spirit of Rails `*_path`, Laravel `route()`, Phoenix `Routes`, Django `reverse()`. Authors reference routes by name and pass params, getting back a validated path.
- **Route discovery.** Add `rela-server routes` subcommand (cf. `rails routes`, `php artisan route:list`, `mix phx.routes`, `python manage.py show_urls`) listing every route name, path, and params. Consider exposing the same list in a developer page inside the UI.

## Out of scope

- Backwards compatibility for existing `create://` / `edit://` links in user projects (migration note + one-pass rewrite in docs; no permanent shim).
- Form selection heuristics (`createFormForType` / `editFormForType`) — those stay for sections/lists, but documents no longer use them.
- Route permissions / auth.

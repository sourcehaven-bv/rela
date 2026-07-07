---
id: TKT-H7E611
type: ticket
title: Render admin-authored header/footer markdown on list views
kind: enhancement
priority: medium
effort: s
status: done
---

## Summary

Add admin-authored info content to the top and bottom of data-entry list views,
rendered as sanitized markdown from `data-entry.yaml`.

## Motivation

Users viewing a register/list page (e.g. "Risicoregister") have no in-context
guidance — how scores work, which guides to read, process notes. Today the list
header shows only the title and a "+ New" button. Admins should be able to add a
short blurb and links to relevant guides at the top (and optionally footer notes
at the bottom).

## Scope (Option A — minimal, from brainstorm)

**In scope:**
- Render a top info region on list views from list config, as sanitized markdown via the existing `renderMarkdown()`.
- Add a `footer` field for a bottom info region.
- Reuse the existing markdown pipeline (`frontend/src/utils/markdown.ts`) including `refResolver` so entity-ref links work and break loudly.
- Authoring is admin-only via `data-entry.yaml` (no UI editing, no end-user authoring).

**Out of scope (left as a seam for later):**
- Entity-backed or query-backed info blocks (the "link one or more entities" / dashboard-card-style option). The render path goes through `renderMarkdown()` on a string, so upgrading a slot to also accept `{entity}`/`{query}` later is additive.
- Per-entity-type default info in the metamodel.

## Design notes

Existing building blocks (confirmed by exploration):
- `List.Description` (`internal/dataentryconfig/config.go`) is already plumbed end-to-end (Go → `/api/v1/_config` JSON → TS `ListConfig.description`) but **never rendered**.
- Render sites in `frontend/src/components/lists/EntityList.vue`: header block (~line 641) for top, after `<Pagination>` (~line 910) for bottom, both inside the `v-if="listConfig"` wrapper.
- `renderMarkdown()` already sanitizes (DOMPurify) and resolves entity refs.

**Open question — field naming:** keep `description` for the top slot (zero
migration) + add `footer`, OR introduce `header` + `footer` as the canonical
symmetric fields with `description` kept as a deprecated alias for `header`.
Leaning toward `header`+`footer` with `description` alias for cleaner YAML
vocabulary at minimal extra cost. To be decided in planning.

## Acceptance criteria

1. A list config with header markdown renders sanitized HTML above the search/filter row.
2. A list config with footer markdown renders sanitized HTML below the table/pagination.
3. Markdown links to entity IDs resolve to in-app links via `refResolver`.
4. Output is DOMPurify-sanitized (no raw HTML/script injection from config).
5. A list with no header/footer configured renders exactly as before (no empty region, no layout shift).
6. Backward compatibility: existing `description` values continue to work per the chosen naming decision.

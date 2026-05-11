---
id: TKT-747O
type: ticket
title: Resolve entity-ID code spans to titled links in data-entry views
kind: enhancement
priority: medium
effort: m
status: done
---

## Problem

When data-entry views render entity markdown body content, references like ``
`TKT-LXYHQ` `` or `` `IDEA-011` `` appear as bare code spans containing opaque
entity IDs. The reader has to manually copy the ID, navigate elsewhere, and
search to discover what's being referenced — even though the project already
knows the title and a navigable in-app URL exists.

The Lua side already solved this (TKT-LXYHQ): `rela.md.resolve_refs     ` plus
`rela.md.entity_refs     ` rewrite single-token code spans that match a known
entity ID into proper markdown links titled with the target's title. Document
renderers (FEAT-023) benefit when they opt in from Lua.

The plain-content render path used by `EntityDetail.vue     ` and similar views
(`frontend/src/utils/markdown.ts     `'s `renderMarkdown     `) does **not**
apply this rewrite, so authors who use the natural convention of wrapping IDs in
backticks in body content get no link affordance in the UI.

## Proposal

Apply the same opt-in code-span resolution at the data-entry markdown render
path so any markdown content rendered in the SPA gets entity-ID code spans
upgraded to titled links pointing into the data-entry app.

Sketch:

- Frontend already has an `entitiesStore     ` with cached entity summaries; surface
a stable lookup `{ id -> title }     ` (or per-id fetch) usable from the
markdown renderer.
- Extend `renderMarkdown     ` (or wrap it in a render helper used by views) so
that, after marked.js parses the markdown, code spans whose text matches a known
entity ID are replaced by `<a href="/entity/<id>">Title</a>     `.
- Only rewrite code spans whose **entire** text is an entity ID — match the
Lua semantics exactly (no bare prose mentions). Unknown IDs are left as plain
code spans.
- Reuse the existing entity URL scheme used elsewhere in the SPA (router
link for `/entity/:id     ` or equivalent) — do not invent a new one.

## Out of scope

- Server-side document rendering (FEAT-023) — already covered by the Lua API.
- `[[ID]]     ` Obsidian-style wiki link syntax (`IDEA-011     `).
- Bare prose mentions outside code spans.
- Hover previews or backlinks panels.

## Acceptance Criteria

1. Markdown rendered in `EntityDetail.vue     ` (entity body and section content)
replaces code spans whose entire content matches a known entity ID with a link
whose text is the target's title and whose href routes to the entity's
data-entry detail view.
2. Unknown IDs are left as code spans (no link, no warning surfaced to users).
3. Code spans containing anything other than a bare ID (e.g. `` `TKT-1 and TKT-2` ``)
are not rewritten.
4. Code blocks, link text, link URLs, and HTML attributes are not affected.
5. The rewrite is sanitization-safe — output continues to pass DOMPurify
without weakening the existing sanitizer config.
6. Unit tests cover: known-ID replacement, unknown-ID passthrough, multi-ID
code spans, ID inside fenced code block, ID inside existing link text, escaping
of entity titles containing HTML-significant characters.
7. Manual verification in the running dev server confirms the link is
clickable and navigates correctly.

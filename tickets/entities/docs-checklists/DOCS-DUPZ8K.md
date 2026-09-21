---
id: DOCS-DUPZ8K
type: docs-checklist
title: 'Documentation: Duplicate an entity from the detail page'
status: done
---

<!-- @managed: claude-workflow v1 -->

## Code Documentation

- [x] Comments where logic isn't obvious
- [x] Function/type docs if public API

Each of the three always-excluded property kinds carries the reason it is
excluded at the point of exclusion, because "why is my attachment missing from
the copy?" is the question a reader will arrive with. `planPrefillRouting` and
`DuplicateConfig.CarriesProperty` carry their nil/direction contracts.

## Project Documentation

- [x] README updated (if applicable) — N/A, no project-level change
- [x] ~~CLAUDE.md updated~~ (N/A: no new pattern; reuses inline_create,
      applyTemplate and the existing batched create)
- [x] ~~Help text accurate~~ (N/A: no CLI surface)

## External Documentation

- [x] ~~Changelog entry added~~ (N/A: this repo has no CHANGELOG file)
- [x] API docs updated (if applicable) — no API change: the feature adds no
      endpoint and rides `POST /{plural}` and `GET /{plural}/{id}/relations`
      as they already exist.

`docs/data-entry.md` documents the Duplicate action and the
`entity_views.<type>.duplicate` block. Written in the GUIDE entity, since the
published file is generated (RR-WGNYNY); `just docs` run twice with no diff and
`just docs-check` green.

The docs state the three always-excluded property kinds as behaviour rather
than caveats, and that the allowlist narrows properties only — the markdown
body always carries, which an operator writing an allowlist could otherwise
reasonably expect it to cover (raised by the security review as a
docs/UX gap).

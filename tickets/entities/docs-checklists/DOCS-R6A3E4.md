---
id: DOCS-R6A3E4
type: docs-checklist
title: 'Docs: Create menu offers only linkable types (TKT-XPFXMG)'
started: "2026-10-12"
completed: "2026-10-12"
status: done
---

<!-- @managed: claude-workflow v1 -->

## Code Documentation

- [x] Comments where logic isn't obvious (`usePageCreateLink().offered`, `pageStore.opening` and the SpaceCreateMenu header say why the menu narrows on an entity page, why it offers nothing until the anchor has loaded, and why the button is then busy rather than removed; the template comment says why the dialog sits outside the menu's `v-if`)
- [x] ~~Function/type docs if public API~~ (N/A: no public API change; the sidebar payload is unchanged)

## Project Documentation

- [x] ~~README updated (if applicable)~~ (N/A: no README change)
- [x] ~~CLAUDE.md updated (if new patterns)~~ (N/A: no new pattern)
- [x] ~~Help text accurate (if CLI changes)~~ (N/A: no CLI change)

## External Documentation

- [x] ~~Changelog entry added~~ (N/A: the repo has no changelog)
- [x] API docs updated (if applicable) (GUIDE-data-entry.md § Spaces `create:` points to Entity pages; § Entity pages describes the narrowed offer, the two-relation case and what to do instead, the hidden button, and the busy button until the entity has loaded; § sidebar payload says the menu reads `links` to choose its types once the page's entity has loaded. Regenerated into docs/data-entry.md with scripts/generate-docs.sh; no further diff)

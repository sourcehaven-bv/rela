---
id: DOCS-7TQ0M
type: docs-checklist
title: 'Documentation: Mobile-friendly responsive page layout and iOS viewport handling'
status: done
---

<!-- @managed: claude-workflow v1 -->

## Code Documentation

- [x] Comments where logic isn't obvious — `useVisualViewportOffset.ts` documents the iOS sticky-vs-visual-viewport quirk it works around; `PageLayout.vue` / `mobile-bars.css` document the safe-area and topbar math.
- [x] ~~Function/type docs if public API~~ (N/A: internal SPA components/composable; no public Go or HTTP API surface.)

## Project Documentation

- [x] ~~README updated~~ (N/A: no setup, command, or behavior change at the project level.)
- [x] ~~CLAUDE.md updated~~ (N/A: no new architectural pattern — shared page chrome follows the existing components/composables layout documented in frontend/CLAUDE.md.)
- [x] ~~Help text accurate~~ (N/A: no CLI changes.)

## External Documentation

- [x] ~~Changelog entry added~~ (N/A: the repository keeps no changelog file.)
- [x] ~~API docs updated~~ (N/A: no API changes.)

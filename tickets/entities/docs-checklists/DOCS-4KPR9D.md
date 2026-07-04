---
id: DOCS-4KPR9D
type: docs-checklist
title: 'Docs: Reposition Properties auto-save indicator inline, hidden when idle'
status: done
---

<!-- @managed: claude-workflow v1 -->

## Documentation Review

Cosmetic UI micro-behaviour change to the data-entry auto-save indicator
(placement + hidden-until-idle + a11y announcement). No user-facing feature,
CLI, metamodel, or API surface changed.

- [x] ~~Code docs (godoc / JSDoc) updated~~ (N/A: component comments updated in-place — AutoSaveIndicator.vue, SectionEditForm.vue, EntityDetail.vue — no doc-comment API contract changed)
- [x] ~~Project docs updated (docs/data-entry.md etc.)~~ (N/A: docs/data-entry.md describes data-entry usage/config, not indicator micro-behaviour; nothing to update)
- [x] ~~External / README docs updated~~ (N/A: no project-level or user-facing behaviour a reader would look up)
- [x] ~~CLAUDE.md patterns updated~~ (N/A: no new convention introduced; reuses the existing `#indicator` slot pattern)

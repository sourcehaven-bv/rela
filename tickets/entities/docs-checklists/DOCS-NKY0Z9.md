---
id: DOCS-NKY0Z9
type: docs-checklist
title: 'Docs: doc-fields in the help modal + About (TKT-DUQBD0)'
status: done
---

<!-- @managed: claude-workflow v1 -->

## Code Documentation

- [x] Godoc on new symbols (gatherEnumHelp, renderEnumHelp, mermaidStateDiagram, mermaidLabel, aboutDescription, EnumHelp/ValueHelp/TransitionHelp) — including the injection-safety rationale on mermaidStateDiagram and the "separate from AppConfig.Description" rationale on aboutDescription + v1.Config.about_description.
- [x] TSDoc on the new store field + config type.

## Project Documentation

- [x] ~~docs/metamodel.md~~ (N/A: the doc-FIELDS themselves are documented in TKT-0YBFT8; this ticket only surfaces them in the UI — no new schema surface)
- [x] ~~docs/data-entry~~ (N/A: the help modal + About are internal UI; no API-reference contract change beyond the additive `about_description` config field, which is self-describing)

## External Documentation

- [x] ~~README / tutorials~~ (N/A: UI enhancement)

## Verification

- [x] The metamodel-doc-field docs (from TKT-0YBFT8) accurately describe fields now rendered by this UI — consistent.

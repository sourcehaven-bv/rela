---
id: DOCS-WZ3NAD
type: docs-checklist
title: 'Docs: Enum values support a display label/title for better UX on snake_case values'
status: done
---

<!-- @managed: claude-workflow v1 -->

## Code Documentation

- [x] Godoc / comments on new exported symbols — `CustomType.Labels`, `PropertyDef.Labels` (Go); `getEnumLabel`, `resolveOptionLabels`, `labelsForProperty` (store JSDoc-style comments)
- [x] ~~README~~ (N/A: no project-level change)

## Project Documentation

- [x] `docs/metamodel.md` — added "Display Labels" subsection under Enum Types documenting the `labels:` map on custom types and inline enums, the display-only/value-stays-identity semantics, surfaces covered, back-compat, and custom-type precedence.
- [x] ~~`docs/data-entry.md`~~ (N/A: metamodel.md is the canonical place for the enum feature; data-entry.md documents UI config, not per-value metamodel shape)
- [x] ~~`docs/cli-reference.md`~~ (N/A: no CLI change)
- [x] CLAUDE.md — no new pattern/convention that belongs there (feature follows existing consumer-side + store-getter patterns)

## External Documentation

- [x] ~~Changelog / release notes~~ (N/A: handled at release time; no separate changelog file in-tree)

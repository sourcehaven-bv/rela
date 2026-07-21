---
id: DOCS-OBV091
type: docs-checklist
title: 'Docs: Metamodel doc-fields (TKT-0YBFT8)'
status: done
---

<!-- @managed: claude-workflow v1 -->

## Code Documentation

- [x] Godoc on the three new fields (Metamodel.Description, CustomType.Descriptions, TransitionDef.Help) explaining they are display-only, doc-generator-facing, and how Descriptions differs from Labels / the type-level Description scalar.
- [x] Doc comment on toV1CustomType pinning the intentional wire-omission phase boundary.

## Project Documentation

- [x] docs/metamodel.md (via source GUIDE-metamodel.md) — added: top-level `description:` in Structure; a "Value Descriptions" subsection (`descriptions:` vs `labels:`); `help:` in the State Machines transition-field table + example; updated the include root-only list to include `description:`. Regenerated; markdownlint clean.

## External Documentation

- [x] ~~README / tutorials~~ (N/A: additive optional schema fields; covered by the metamodel guide)

## Verification

- [x] Docs match code (include root-only list now lists description, matching the include.go guard + its new test)
- [x] Example in the guide matches the real fields (parses; mirrored in prototypes/data-entry/project/metamodel.yaml)

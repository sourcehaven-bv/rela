---
id: DOCS-QXY8CQ
type: docs-checklist
title: 'Docs: direction inference across all relation surfaces'
status: done
---

<!-- @managed: claude-workflow v1 -->

## Code Documentation

- [x] Godoc on new exported symbols — `CheckAmbiguousDirection` and
`AmbiguousDirectionError` state why one shared guard exists rather than one per
surface
- [x] `InferDirection`'s godoc now spells out what `entityType` means per
surface, including that CalDAV anchors on the MEMBER type (`entity_type`), not
`driver_type`
- [x] `resolveConfigDirection` documents why resolution must happen server-side
(every SPA consumer tests the literal `direction === 'incoming'`) and is
explicitly distinguished from the unrelated pre-existing `resolveDirection`
- [x] `RelationDirectionMigration` renamed from `FormRelationDirectionMigration`
and its doc comment updated to list every covered surface

## Project Documentation

- [x] `docs-project/entities/guides/GUIDE-data-entry.md` — the "How `direction`
is resolved" section generalized from forms to all surfaces, with a table naming
what each one anchors to
- [x] Two stale `"outgoing" (default)` table rows (list columns, filter
controls) corrected to "inferred when omitted"
- [x] `docs/` regenerated via `./scripts/generate-docs.sh` — edited the SOURCE
entity, not the generated file (the mistake that failed the Docs job on #1376)

## External Documentation

- [x] ~~README~~ (N/A: no top-level command or feature added)
- [x] ~~CHANGELOG~~ (N/A: project does not maintain one)
- [x] Migration path documented — the upgrade note already points at
`rela migrate`; it now covers the additional surfaces automatically

## Verification

- [x] Docs examples match actual behavior (the anchoring table mirrors the
validation call sites one-for-one)
- [x] `git diff --exit-code docs/ README.md` passes — the generated output
matches the source entity

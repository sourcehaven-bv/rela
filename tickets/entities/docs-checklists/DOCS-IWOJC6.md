---
id: DOCS-IWOJC6
type: docs-checklist
title: 'Docs: relation field-level visible: redaction + sync deferred gap (TKT-B1F5Q1)'
status: done
---

<!-- @managed: claude-workflow v1 -->

## Code Documentation

- [x] `acl.RelationGrant.Visible` godoc: read-side sibling of write-side `Fields`, mirrors RoleDef Fields/Visible split
- [x] `PolicyResolver.RelationFieldVerdicts` godoc: source-keyed, same bindings, historical fail-closed + type-level closed-world
- [x] `PolicyResolver.relationTypesWithVisible` field godoc: relation-history analog of typesWithVisible
- [x] `dataentry.RelationVisibilityResolver` godoc: optional interface type-asserted like TransitionResolver; Nop/Demo don't implement
- [x] `visibleRelationMeta` / `visibleRelationMetaIncoming` godoc: copy-on-redact, read-only result, fail-closed on unresolvable incoming source
- [x] `relationMetaStrip` / `redactRelationMetaStrip` godoc: strip-after-sort rationale, single-source `incoming` warning
- [x] `serveRelationHistoryVersion` godoc + file-header comment updated: fail-closed relation history, reveal permission (replaces the "no redaction" RR-BZNL0S seam comment)

## Project Documentation

- [x] ~~CLAUDE.md~~ (N/A: the "read-out via visibility wrappers" and "never redact a read that feeds a write" rules already cover relations; no new rule needed)

## External / User-Facing Documentation

- [x] `docs/acl-security.md` — "Property-level redaction (`visible:`)": new relation-meta subsection (source-keyed grant, all read shapes, free-form key coverage, no `_title` channel); "Historical field redaction fails closed": relation-history-inherits paragraph; "What still leaks (deferred)": new `/api/sync/` entry
- [x] `docs-project/entities/guides/GUIDE-acl-security.md` — same three edits (byte-consistent mirror)
- [x] ~~docs/metamodel.md, cli-reference.md, data-entry.md, README.md~~ (N/A: no metamodel/CLI/UI/project-level change; `visible:` schema shape already documented for entities)

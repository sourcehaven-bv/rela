---
id: DOCS-E70GOV
type: docs-checklist
title: 'Documentation: Computed properties'
status: done
---

<!-- @managed: claude-workflow v1 -->

## Code Documentation

- [x] Comments where logic isn't obvious (dependency inference, topological
evaluation, write-time snapshots, trusted sync recomputation, portability)
- [x] Function/type docs for the new predicate profile/program API,
`computed.Set`, and `ComputedWriteError`

## Project Documentation

- [x] ~~README updated~~ (N/A: generated overview has no property-reference section)
- [x] CLAUDE.md updated with the typed value-expression and target-portability pattern
- [x] ~~Help text updated~~ (N/A: no new CLI command or flag)

## External Documentation

- [x] ~~Changelog entry added~~ (N/A: this repository has no manual changelog;
release notes derive from PR/commit metadata)
- [x] Generated metamodel reference documents `computed:` syntax, supported
expression subset, dependency/cycle behavior, write-time clock semantics,
read-only surfaces, materialization/indexing, schema drift, SQL portability, and
ACL disclosure considerations
- [x] Data-entry API reference documents `_fields.<name>.writable: false`
- [x] `just docs-check` confirms generated documentation is current

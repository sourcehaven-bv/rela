---
id: DOCS-VXCAJ
type: docs-checklist
title: 'Docs: Document the documents feature and add Lua script renderer'
status: done
---

<!-- @managed: claude-workflow v1 -->

## Code Documentation

- [x] Public APIs have doc comments
- [x] Complex logic has explanatory comments
- [x] Package-level docs updated if applicable

**Evidence:** `WithDocumentMode`, `ExecuteDocument`, `DocumentConfig.Script`,
`documentScriptEngine` interface, and the new fields on `documentService` all
carry doc comments. Cache-policy and singleflight-key comments added inline
where the invariants are non-obvious. `luaOutput` now has a comment explaining
why conversion is deferred past the mode guards.

## Project Documentation

- [x] CLAUDE.md updated if new patterns introduced
- [x] README.md updated if user-facing change

**Decisions:**

- CLAUDE.md: N/A — the feature follows the existing `WithActionMode` pattern exactly, no new architecture.
- README.md: N/A — no project-level changes.

## External Documentation

- [x] User guides updated (`docs-project/entities/guides/`)
- [x] Generated docs regenerated (`just docs`)
- [x] Examples updated / added

**Updated guides:**

- `GUIDE-data-entry.md` — new `## Documents` section (substantial; the documents feature was entirely undocumented before this ticket). Covers YAML config schema for both `command:` and `script:` variants, `edit://` + `create://` URL schemes, caching behavior with the per-renderer split, SSE live-reload caveat, document-mode Lua API, `rela.output` warning behavior, `html.WithUnsafe` trust boundary, config hot-reload caveat.
- `GUIDE-lua-scripting.md` — new `### Document Mode` subsection cross-linking to the data-entry guide; documents `rela.mode`, `rela.document.id`, `rela.document.entry_id`, `rela.output` behavior in document mode, and the cache namespace policy.
- `docs/data-entry.md` and `docs/lua-scripting.md` regenerated via `just docs`.

**Added example:**
`prototypes/data-entry/project/scripts/docs/category_report.lua` — demonstrates
`rela.trace_to`, `rela.cache.memoize` with a version-keyed cache, `edit://` and
`create://` link emission. Wired into
`prototypes/data-entry/project/data-entry.yaml` under `documents:` as
`category_overview` for `entity_type: category`.

## Cross-References

- [x] Links between related docs verified
- [x] Code comments reference relevant docs

**Verified:** GUIDE-lua-scripting → GUIDE-data-entry Documents section
(bi-directional link); prototype `category_report.lua` has a header comment
pointing at the `documents:` YAML shape users need to declare.

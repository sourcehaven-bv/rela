---
id: DOCS-CFJWKI
type: docs-checklist
title: 'Docs: Standalone documents: document: as a navigation entry with optional entity_type'
status: done
---

<!-- @managed: claude-workflow v1 -->

## Code Documentation

- [x] Exported types and functions have godoc
- [x] Non-obvious decisions explain *why*, not just *what*

Key godoc: `DocumentConfig` (the two kinds), `IsStandalone`,
`WithStandaloneDocumentMode` (why a separate constructor rather than `""`),
`ExecuteStandaloneDocument`, `RenderStandalone` (why a string return),
`gateDocumentPermission` (why optional, and why 403 rather than a disguised
404), `handleV1AnchoredDocument` / `handleV1StandaloneDocument`.

## Project Documentation

**`docs/*.md` are GENERATED from `docs-project/entities/`** — the header says
so, and `just docs` overwrites them. Edit the source guide, then run `just
docs`. I initially wrote these edits into the generated files; `just docs`
silently reverted ~172 lines of them. Caught by diffing the regenerated output
against the committed one, which is now the check to run after any docs change:

```console
$ just docs && git diff --stat docs/
```

- [x] `docs-project/entities/guides/GUIDE-data-entry.md` → `docs/data-entry.md`
  - Documents intro rewritten around the two kinds (comparison table)
  - "Standalone documents" section: script-only, nil `entry_id`, nav-entry
constraint, no disk cache, SSE caveat
  - "Gating a document": `permission:`, the ungated default and why, the 403,
the unfiltered menu/config and why, the fail-open/fail-closed divergence from
`commands:`
  - `document:` in the navigation field table
  - Lua context table: `entry_id` nil for standalone + the branch idiom and
the `""`-is-truthy warning
  - Security notes: the two URL shapes; document scripts read through the
ACL-gated reader
- [x] `docs-project/entities/guides/GUIDE-acl-security.md` → `docs/acl-security.md`
  - Extends § "Sidebar menu structure is principal-independent" to the rest of
the config surface; states the 403-vs-404 distinction (config key vs entity id);
notes that UX-motivated menu filtering is still wanted and tracked separately
(TKT-TXDK8U)
- [x] `internal/dataentry/CLAUDE.md` — Documents rules: never let one URL shape
serve the other kind; `permission:` is intent, not the boundary; gate before the
renderer; 403 naming the document; **do not** filter sidebar or `_config` per
principal (with a pointer to the acl-security decision and a note that an
earlier draft did and was reverted)
- [x] Root `CLAUDE.md` — "The configuration is not a secret; the data is"
(added at the user's request; generalizes past this ticket)
- [x] ~~`docs/data-entry/api-reference.md`~~ (N/A: `_documents` is not
documented there; the surface lives in the data-entry guide)
- [x] ~~`docs/lua-scripting.md`~~ (N/A: document-mode context is documented in
the data-entry guide, where it already lived)
- [x] ~~`docs/metamodel.md`~~ (N/A: this is `data-entry.yaml`, not the metamodel)

## Working Example

- [x] `prototypes/data-entry/project` gained a real standalone document
(`scripts/docs/status_review.lua`, aggregating tickets × categories) and a
`document:` navigation entry, so the in-tree demo exercises the feature.

## External Documentation

- [x] ~~Changelog / release notes~~ (N/A: none maintained in-tree; commit
messages carry the rationale)

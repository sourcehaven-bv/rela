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

Key godoc added: `DocumentConfig` (the two kinds), `IsStandalone`,
`WithStandaloneDocumentMode` (why a separate constructor rather than `""`),
`ExecuteStandaloneDocument`, `RenderStandalone` (why a string return),
`gateDocumentPermission` / `permitsDocument` (why optional), `visibleDocuments`
/ `visibleNavigation` (why both projection *and* filtering), `hidesNavEntry`
(why it takes the caller's snapshot), `v1.Document` (what is withheld and why).

## Project Documentation

- [x] `docs/data-entry.md` — rewrote the Documents intro around the two kinds
(table), added a "Standalone documents" section with the rules (script-only, nil
`entry_id`, nav-entry constraint, no disk cache, SSE caveat) and a "Gating a
document" section covering `permission:`, the deliberate ungated default, the
config projection, and the fail-open/fail-closed divergence from `commands:`
- [x] `docs/data-entry.md` — `document:` added to the navigation field table
- [x] `docs/data-entry.md` — Lua context table marks `entry_id` as nil for
standalone, with the branch idiom and the `""`-is-truthy warning
- [x] `docs/data-entry.md` — security notes cover the two URL shapes and that
document scripts read through the ACL-gated reader
- [x] `internal/dataentry/CLAUDE.md` — new "Documents" rules section: never let
one URL shape serve the other kind; `permission:` is intent/UX not the boundary;
gate before the renderer anyway; sidebar filtering is an affordance; a
standalone render has no hash/entities/cache
- [x] ~~`docs/data-entry/api-reference.md`~~ (N/A: the `_documents` endpoint is
not documented there — the whole surface is described in `docs/data-entry.md`,
and adding one endpoint in isolation would imply coverage that doesn't exist)
- [x] ~~`docs/lua-scripting.md`~~ (N/A: document-mode context lives in
`docs/data-entry.md`, which is where it was already documented)
- [x] ~~`docs/acl-security.md`~~ (N/A: `permission:` on a document is a
data-entry config concern and is documented there; it introduces no new ACL
mechanism, only a new consumer of the existing named-permission one)
- [x] ~~`docs/metamodel.md`~~ (N/A: this is `data-entry.yaml`, not the metamodel)

## Working Example

- [x] `prototypes/data-entry/project` gained a real standalone document
(`scripts/docs/status_review.lua`, aggregating tickets × categories) and a
`document:` navigation entry, so the feature is exercised by the in-tree demo
project rather than only by tests.

## External Documentation

- [x] ~~Changelog / release notes~~ (N/A: no changelog is maintained in-tree;
the commit messages carry the rationale)

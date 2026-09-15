---
id: DOCS-3J98YJ
type: docs-checklist
title: 'Docs: export: documents export via transforms, like entity and list views'
status: done
---

<!-- @managed: claude-workflow v1 -->

## Code Documentation

- [x] Exported types and functions have godoc
- [x] Non-obvious decisions explain *why*, not just *what*

Key godoc added or rewritten:

- `resolveAnchoredDocument` / `resolveStandaloneDocument` — the shared gate
chain. The doc states the two load-bearing orderings (read gate above the store
read, type check below the read gate) and *why* each matters, plus why the
function deliberately does not touch `GetCached`.
- `exportSegment` — the routing premise, explicitly flagged as a premise:
`entity.ValidateID`'s leading-underscore rule is well-formedness, not a security
control, and the comment says so rather than leaning on it silently.
- `RenderStandaloneMarkdown` — why every guard lives *there* rather than in the
`RenderStandalone` wrapper (a guard above one of two callers is a guard the
other caller does not have).
- `RenderDocumentMarkdown` — why dispatch is on document kind rather than an
empty entry id, with the concrete consequence: `entry_id` would be `""` instead
of nil and a script branching on it renders the wrong document.
- `handleV1ExportDocument` — the `command:` refusal as a stated *policy* choice,
not a structural impossibility.
- `ReservedExportSegment` — why it is declared in `dataentryconfig` (import
direction) and how the alias makes drift a compile error.
- The `handleV1Documents` dispatch — the four-shape table, and why empty
segments are rejected rather than normalized (RR-1JZTB0).

## Project Documentation

**`docs/*.md` are GENERATED from `docs-project/entities/`** — the header says so
and `just docs` overwrites them. I initially wrote the data-entry change into
the generated file; the `/pr` gate caught the missing docs-checklist, and
checking this item surfaced the mistake before `just docs` silently reverted it.
Moved to the source guide and regenerated; `git diff docs/` is now empty, which
is the check that proves source and output agree.

`docs/transforms.md` is the exception — it has **no** source guide in
`docs-project/entities/` (DOC-WWNE41 tracks folding it into the pipeline), so it
is edited directly. Verified before editing rather than assumed.

- [x] `docs/transforms.md` — edited directly (no generator source)
  - "Exporting from the data-entry app" intro now names documents as the third
surface, with both URL shapes in the endpoint list
  - New "Exporting a document" section: the two shapes and why each rejects the
other kind; that export is gated exactly like the render it comes from (no
separate permission, no opt-out, and why); script-only with the reason; no
caching because renders are per-principal
  - A callout stating plainly that a document export carries entity **bodies**
exactly as the on-screen document does — `visible:` redaction covers property
values, and body redaction is unimplemented on every read path, so the docs must
not imply export is redaction-equivalent to the entity path (RR-MZE4IA)
  - "Not yet supported (v1 limits)" gains the `command:`-renderer limitation
- [x] `docs-project/entities/guides/GUIDE-data-entry.md` → `docs/data-entry.md`
  - Documents section notes the Export menu on both document kinds, that export
is gated like the render, the `command:` limitation, and links to the transforms
guide
- [x] ~~`internal/dataentry/CLAUDE.md`~~ (N/A: the existing Documents rules —
"never let one URL shape serve the other kind", the `permission:` /elevation
split, gate-before-render — already govern this change and needed no amendment.
RR-1JZTB0 was a violation of a rule already written there, not a gap in it.)
- [x] ~~root `CLAUDE.md`~~ (N/A: no new cross-cutting pattern; the transforms
section already describes export as downstream of an authorized view)

## External Documentation

- [x] ~~API reference~~ (N/A: `docs/data-entry/api-reference.md` documents the
`_actions` write-affordance contract; export is a read affordance and the two
existing export endpoints are likewise documented in `docs/transforms.md` rather
than there)
- [x] ~~Migration notes~~ (N/A: purely additive. Two new routes, one new config
rule that only rejects a name no existing project can be using — a document
named `_export` would have had to be authored deliberately)
- [x] ~~Changelog~~ (N/A: not maintained in-repo; release notes derive from
commit messages, and the commits carry the ticket ID)

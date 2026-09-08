---
id: DOCS-D7N6RN
type: docs-checklist
title: 'Docs: Entity commenting stage 1: property and section anchors'
status: done
---

## Code Documentation

- [x] Comments where logic isn't obvious
- [x] Function/type docs if public API

The load-bearing rationale is recorded at the declaration sites: why comments
are not graph entities (`internal/comments/comments.go` package doc), why the
anchor is a kind-discriminated struct (so stage 2 needed no migration), why a
resolved `Range` must be sliced with Start/End rather than the quote's length,
why `applyHighlights` skips code, and why `gateCommentTarget` returns a
face-resolved target rather than leaving it to each call site.

## Project Documentation

- [x] README updated (if applicable)
- [x] CLAUDE.md updated (if new patterns)
- [x] ~~Help text accurate (if CLI changes)~~ (N/A: no CLI surface — commenting
is a data-entry API and SPA feature)

`docs/comments.md` is generated from
`docs-project/entities/guides/GUIDE-comments.md` and the README index picked it
up automatically; `just docs-check` passes.

No root CLAUDE.md change was needed: the feature follows the existing rules
rather than introducing a pattern (consumer-side interfaces, the storetest-style
conformance suite, visibility wrappers on the read path). The one convention
worth recording — that a stored comment key is `entity.FormatStateRef`, so the
default face keeps the bare id — lives on `Target.Key()`, next to the code that
depends on it.

## External Documentation

- [x] ~~Changelog entry added~~ (N/A: this repo keeps no CHANGELOG.md; releases
are tagged and the guide is the user-facing record)
- [x] User-facing docs written

`docs/comments.md` covers enabling the `comments:` block, the six permissions
with worked `acl.yaml` roles, the three anchor kinds and how each behaves under
edits, image/diagram commenting, per-face scoping, the HTTP surface, and the
limits. Every number in it was checked against the constants rather than
recalled — the first draft said 8 KB for the body cap where `MaxBodyBytes` is 16
KB, and that was corrected.

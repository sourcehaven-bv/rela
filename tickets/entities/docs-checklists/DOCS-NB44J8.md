---
id: DOCS-NB44J8
type: docs-checklist
title: 'Documentation: Replace EasyMDE with Milkdown (ProseMirror) in data-entry forms'
status: done
---

## Code Documentation

- [x] Comments where logic isn't obvious
- [x] Function/type docs if public API

The comments that matter here are the ones recording *why*, because several of
these decisions look wrong until you know what they prevent:

- `serializerContract.ts` — why `WHITESPACE_INSENSITIVE_LEAVES` excludes fenced
code, since including it looks harmless and masks corruption.
- `writeBackGuard.ts` — why `decideEmit` compares against `lastEmitted` rather
than `original`, with the revert sequence spelled out.
- `MilkdownEditor.vue` — why `originalValue` and `settledValue` are separate,
naming the bug that re-baselining caused.
- `entityRefNode.ts` — the node-before-mark ordering in `#matchTarget`, and
that `NodeSchema.priority` does not control it.
- `tableCommands.ts` — why three hand-written guards exist where the dry run is
the pattern everywhere else.
- `entityRefIdGrammar.test.ts` and its Go counterpart — what drift they catch.

## Project Documentation

- [x] README updated (if applicable) — N/A: no project-level change.
- [x] CLAUDE.md updated (if new patterns)
- [x] Help text accurate (if CLI changes) — N/A: no CLI surface.

`frontend/CLAUDE.md` gains a section on the editor covering the four properties
new code must not break: the write-back guard, load-time normalization not
counting as an edit, titles coming only from the server's mentions map, and
`.md-body` supplying typography. It also records the toolbar's dry-run
availability check and why `tableCommands.ts` deviates.

The package table's `MarkdownEditor` entry was stale (the file is deleted) and
now points at `src/components/forms/milkdown/`.

## External Documentation

- [x] Changelog entry added — N/A: this project has no CHANGELOG file.
- [x] API docs updated (if applicable)

`docs/data-entry.md` gains "The Markdown Body Editor": what the toolbar does,
how `@` linking works, the table controls, and the promise that opening an
entity without editing it writes nothing. It also states the storage contract a
user can verify for themselves — a reference is stored as a plain code span, so
the file stays readable outside rela — and that titles are ACL-scoped.

The `mentions` field on the v1 entity response is documented in
`internal/apiwire/v1/responses.go` beside the existing `ViewResponse.Mentions`,
noting same shape and semantics, single-entity GET only, per principal.

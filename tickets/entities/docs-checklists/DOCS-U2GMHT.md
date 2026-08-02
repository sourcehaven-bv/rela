---
id: DOCS-U2GMHT
type: docs-checklist
title: 'Docs: History/diff views shareable version URLs (TKT-YOHC3N)'
status: done
---

<!-- @managed: claude-workflow v1 -->

## Code Documentation

- [x] New code carries doc comments explaining the *why*

`useVersionSelectionSync.ts` has a module docstring stating the three rules of
the seed/replace/echo pattern and naming its two in-repo precedents, plus
per-function comments on the non-obvious parts: why `parseSide` must return a
number (the `v-model` type trap), why the watcher getter returns a string rather
than an array (reference comparison would fire on unrelated query changes), why
the allowlist is membership-in-the-real-list rather than a range check, and why
`current` is spelled identically in both views' URLs despite being labelled
differently.

Both views carry a comment at the `load()` call site explaining why
`publishSelection()` runs even with no versions, and why the restore path resets
instead of re-seeding.

## Project Documentation

- [x] `docs/postgres-backend.md` updated

Added a "Sharing a diff" block under the paragraph describing the history UI,
with worked URL examples for both entity and relation history. It states
explicitly that `current` is **live-relative** (so nobody expects a frozen
diff), that omitting the params gives the default view, that an ordinal naming
no existing version falls back *and* gets corrected in the address bar, and that
a shared link is **not a capability** — the recipient still needs their own read
permission and sees the same 404 they would without it.

- [x] ~~`docs/metamodel.md`~~ (N/A: no metamodel change)
- [x] ~~`docs/cli-reference.md`~~ (N/A: no CLI change; the `rela history` /
`rela relation-history` commands are untouched)
- [x] ~~`docs/data-entry.md`~~ (N/A: that file never documents the history
feature — the history UI is described in `postgres-backend.md`, since versioning
is a pgstore-only capability, so the docs went where a reader would actually
look)
- [x] ~~`CLAUDE.md` / `frontend/CLAUDE.md`~~ (N/A: this *uses* the established
URL-sync composable pattern rather than introducing a new convention;
`frontend/CLAUDE.md` already documents `src/composables/`)
- [x] ~~`README.md`~~ (N/A: not a project-level change)

## External Documentation

- [x] ~~Release notes / changelog~~ (N/A: no changelog file in this repo)
- [x] ~~API documentation~~ (N/A: no API change — this is entirely client-side;
the `_history` endpoints already accepted a version ordinal)

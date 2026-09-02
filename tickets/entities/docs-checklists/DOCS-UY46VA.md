---
id: DOCS-UY46VA
type: docs-checklist
title: 'Documentation: search fail-closed guard'
status: done
---

<!-- @managed: claude-workflow v1 -->

## Code Documentation

- [x] Comments where logic isn't obvious — and here the comment was the *bug's
accomplice*, which is the point worth recording.

`searchVisibleHits`'s godoc already **claimed** this guarantee:

> When redaction IS in play but the searcher can't do it, SearchVisibleFields
> fails closed (it does not silently skip)

True of `SearchVisibleFields` — and irrelevant, because the code returned before
ever calling it. Anyone auditing this seam would have read that sentence,
matched it against the intent, and moved on. A comment describing a method you
don't reach is worse than no comment: it launders the gap.

The godoc now describes what **this function** does, names the failure it
prevents (a decorator wrapping the searcher without forwarding the method), and
cites RR-8W40EW so the next reader sees the settled principle rather than
re-deriving it.

- [x] ~~Function/type docs if public API~~ (N/A: unexported free function.)

## Project Documentation

- [x] ~~README updated~~ (N/A)
- [x] ~~CLAUDE.md updated~~ (N/A: applies the existing fail-closed rule rather
than establishing one. The rule already exists in
`search.Visible.SearchVisibleFields`'s godoc; this ticket carried it to the seam
that missed it.)
- [x] ~~Help text accurate~~ (N/A: no CLI change)

## External Documentation

- [x] ~~Changelog entry added~~ (N/A: no CHANGELOG; releases come from commit
history.)
- [x] ~~API docs updated~~ (N/A, and worth stating why rather than waving
through: `docs/acl-security.md` documents the **policy model** — what `visible:`
means, how verdicts resolve, what the read gate promises. This change adds no
policy surface and no operator-visible configuration; it closes an internal
wiring hole so the documented promise actually holds. Documenting "the code now
does what this page already said" would be noise.)

If the guard ever becomes operator-visible — e.g. surfacing as a distinct API
error code rather than the existing `errACLListQuery` — that *would* belong in
the guide.

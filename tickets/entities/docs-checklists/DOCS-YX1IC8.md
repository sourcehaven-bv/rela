---
id: DOCS-YX1IC8
type: docs-checklist
title: 'Docs: Type picker and fuzzy ranking for the editor''s @ mention completion menu'
status: done
---

## Code Documentation

- [x] Public functions/types have doc comments

`mentionRanking.ts`, `mentionKeymap.ts` and the new `useMentionMenu` surface all
carry doc comments that name the defect each guard exists for, rather than
restating what the code does.

- [x] Non-obvious decisions explained with WHY

Four decisions are recorded at the declaration site because a future change
would otherwise silently undo them:

1. **uFuzzy stays on its defaults.** `intraIns: 1` looks like a free typo-tolerance
win and costs an exponential main-thread stall (measured 55 s at a 64-character
needle). The comment carries the measurements and records that a length bound
was considered and rejected, because the blowup starts near 16 characters.
2. **The highlight is an identity, not an index.** The comment lists the four ways
the index version was wrong, including that Enter fell through to ProseMirror
and inserted a paragraph break.
3. **The query is sent unmodified.** Splitting it reproduces BUG-O09QUC on bleve
and matches nothing on the linear/postgres backends, and the single-token shape
is what keeps `@status:open` inert.
4. **The type is absent from the haystack.** It is a filter, not a scoring
dimension.

- [x] Complex algorithms have explanatory comments

`tokenizerSawWholeNeedle` explains why the coverage comparison needs its
explicit `terms.length === 0` branch: an all-separator query has zero meaningful
characters, so a bare comparison reads as "fully covered" and falls through to a
filter that matches nothing. That was a real regression during implementation.

## Project Documentation

- [x] `docs/data-entry.md` updated

§ The Markdown Body Editor now covers: fuzzy matching across title and ID
together, the type picker and its progressive disclosure, that picking a type
narrows rather than inserts, how to clear the scope, and that a space still
closes the menu. Written for the user, without naming internals.

- [x] `frontend/CLAUDE.md` updated

Records the invariants a future change could silently break, each with the
defect that motivated it: the query reaches `/_search` unmodified;
rank-don't-filter; guard the partly-tokenizable query (not just the fully
untokenizable one); leave uFuzzy on its defaults; the highlight is an identity;
and assert on the pre-response window, because `runSearch` resets the highlight
on arrival so a test that settles first cannot see that class of bug.

- [x] ~~`docs/metamodel.md`~~ (N/A: no metamodel change)
- [x] ~~`docs/cli-reference.md`~~ (N/A: no CLI change)
- [x] ~~`docs/postgres-backend.md`~~ (N/A: frontend-only; no backend change)
- [x] ~~`README.md`~~ (N/A: not a project-level change)

## External Documentation

- [x] ~~API documentation~~ (N/A: no API change — the ticket deliberately reuses
the existing `?type=` parameter on `/_search`)
- [x] ~~Migration notes~~ (N/A: no migration; no persisted shape changed)
- [x] ~~Changelog~~ (N/A: not maintained per-ticket in this repo)

## Accuracy

- [x] Documentation matches the shipped behaviour

Re-read after the review fixes, which changed two documented claims: the
tokenizer guard is no longer "empty split" but "did not see the whole needle",
and uFuzzy no longer runs with `intraIns: 1`. Both were corrected in
`frontend/CLAUDE.md`; the user-facing text in `docs/data-entry.md` was
unaffected, since typo tolerance was never promised there.

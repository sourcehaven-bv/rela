---
id: TKT-L59KUM
type: ticket
title: 'pgstore: push field-visibility match provenance into SQL (perf follow-up to TKT-GGQ0JT)'
kind: enhancement
priority: low
effort: m
status: backlog
---

Perf follow-up to TKT-GGQ0JT (which `TKT-GGQ0JT` names as "a tracked performance
follow-up, not a correctness gap" but did not actually ticket).

## Context

TKT-GGQ0JT closed the `/_search` match-on-hidden-field oracle. The search is now
given a per-request `search.FieldScope` for each type (`TypeScope.Fields`),
compiled from the `visible:` grants. The pgstore path
(`internal/store/pgstore/visiblesearch.go`, `SearchVisible`) splits the work:

- SQL runs the trigram pre-filter and the visibility predicates, and reports
per row which field grants hold (the `field_grants` column, built by
`fieldGrantColumn`).
- Go judges which fields the text matched (`fieldCheck`, using
`search.MatchTextFields`). A row that no visible property admits has its body
read in one batch per page.

## Opportunity

Judge the matched fields in SQL too, so hidden-field text never reaches the app.
The trigram predicate narrows candidates cheaply; a per-field expression then
runs server-side over the small candidate set.

## Constraints / non-goals

- The SQL match must fold case exactly as Go's `strings.ToLower` does, which
is how `search_text` is written. PostgreSQL's `lower()` does not: it folds 'İ'
to "i" plus U+0307, Go to "i". A gate judged with `lower()` admitted a row whose
only real match was a hidden field. That is why the match moved to Go.
`storetest.RunFieldScopeSearchTests` (`FoldDiffersBetweenGoAndSQL`) pins it.
- `search_text` is a single concatenated column today
(`internal/store/pgstore/entity.go` `entitySearchText`), so per-field SQL
matching needs either per-field Go-folded columns or JSONB per-key matching.
- Correctness must stay identical: `storetest.RunFieldScopeSearchTests` is the
guard, and pgstore runs it at the default and at a two-row page size.

## Reference files

- `internal/store/pgstore/visiblesearch.go`: `SearchVisible`, `visiblePages`,
`fieldGrantColumn`, `fieldCheck`, `buildVisibleSearchSQL`
- `internal/store/pgstore/entity.go`: `entitySearchText`, `search_text`
- `internal/store/storetest/visiblesearch.go`: `RunFieldScopeSearchTests`

Informed by RES-H5AB7S.

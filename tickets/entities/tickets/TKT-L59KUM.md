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

TKT-GGQ0JT closed the `/_search` match-on-hidden-field oracle. The
pgstore-native path (`internal/store/pgstore/visiblesearch.go`,
`SearchVisibleFields` / `fieldVisibleForEntity`) computes per-field match
provenance **in Go** over the already-scanned candidate rows, using
`search.MatchTextFields`. Correct and cheap (the row is already in hand from the
visibility scan), but it does the per-field match in the app, not the DB.

## Opportunity

For the **static** (predicate-free) `visible:` case, the visible-field set is
known per (principal, type) and could be pushed into the SQL as a
column-projection / per-field match expression — a two-stage query: the
tsvector/trgm predicate narrows candidates cheaply (uses the index), then a
per-visible-field expression runs server-side over the small candidate set. This
avoids shipping hidden-field text to the app at all and keeps the match faithful
to Postgres's own operators.

## Constraints / non-goals

- `search_text` is a single concatenated column today
(`internal/store/pgstore/entity.go` `entitySearchText`), so per-field SQL
matching needs either per-field columns/tsvectors or JSONB per-key matching —
real schema/query work.
- **Predicate-conditioned** (`when:`) visibility is entity-dependent and can't
be a static projection; it stays on the Go per-candidate path. `log()` /
document the fallback, mirroring the bleve-10k-floor disclosure in TKT-BA8BSX.
- Correctness must stay identical — `storetest.RunVisibleFieldSearchTests` is
the guard; any pushdown must keep it green (and match the Go ground truth).

## Reference files

- `internal/store/pgstore/visiblesearch.go` — `SearchVisibleFields`,
`fieldVisibleForEntity`, `buildVisibleSearchSQL`
- `internal/store/pgstore/entity.go` — `entitySearchText`, `search_text`
- `internal/store/storetest/visiblesearch.go` — `RunVisibleFieldSearchTests`

Informed by RES-H5AB7S.

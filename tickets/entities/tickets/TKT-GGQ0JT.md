---
id: TKT-GGQ0JT
type: ticket
title: 'ACL read-side: close the /_search match-on-hidden-field oracle (drop hits matching only visible:-hidden fields)'
kind: enhancement
priority: medium
effort: m
status: backlog
---

## Problem

`/_search` already redacts the *body* of a `visible:`-hidden property, but the
search backends **flatten all string properties into one searchable blob**:

- bleve: `entityToDoc` joins every string property into a single `properties`
field + an `all` catch-all (`internal/search/bleveindex/bleveindex.go:302-326`).
- pgstore: `search_text = "id\ncontent\n<string props>"` in one column,
substring-matched (`internal/store/pgstore/entity.go:624`).
- linear: `MatchText` loops `e.Properties` returning a bool
(`internal/search/filter.go:86`).

So a query whose only match is a hidden field's text still returns the entity as
a hit — its *presence* confirms the hidden value even though the value is
stripped from the response body (postcode → hidden-address guess oracle).
Entity-level gating (`executeQuery`) can't catch this: the entity is readable;
only one of its properties is hidden.

**The root obstacle:** the concat destroys field provenance *before* the query
runs, so at match time no backend can say *which field* produced the hit.

Established during RES-H5AB7S; related deferred note RR-WX77.

## Design: backend-reported match provenance + ACL intersection at the seam

Rather than have the ACL seam re-implement text matching (which would forever
chase each backend's analyzer and risk false drops), make **match provenance a
`search.Backend` responsibility**. Extend the backend contract so a search can
report, per hit, **which fields matched**. Each backend answers using its *own*
matching machinery, so provenance stays faithful to the actual search operation.

The seam then becomes a dumb, backend-agnostic set intersection:

```
hit, matchedFields := backend.<provenance op>(q)
visibleFields      := redactor.VisibleFields(principal, hit)   // ACL verdict
keep  <=>  (matchedFields intersect visibleFields) is non-empty
```

A hit that matched *only* on fields the principal cannot see is dropped.

### Per-backend method

Candidate generation stays coarse/fast; provenance is computed lazily, only on
candidates, with the backend's own matcher.

- **bleve** — keep the index as-is (do NOT split into per-property fields — that
bloats a sparse, metamodel-open index and rebuilds ranking). Use the existing
index for candidate generation (the fast full-text filter), then for each
candidate **reuse bleve's own analyzer** (`index.Mapping().AnalyzeText` / the
`standard` analyzer already imported) to compute per-field matches on the
individual property values in-process. Same analysis the index used, no index
change.
- **pgstore** — a **two-stage query**: the tsvector/trgm full-text predicate
narrows candidates cheaply (uses the index), then a second, deliberately
inefficient per-field expression runs *only over that small candidate set* to
report which fields matched. Postgres does the cheap indexed filter first, the
precise per-field work last, over few rows.
- **linear (memstore)** — trivial: `MatchText` already loops fields; return the
set of matching field names instead of a bool. This is the ground-truth
semantics the conformance suite pins against.

### Interface sketch

Extend `search.Backend` (currently just `Search(text, limit) []string`) with a
provenance-returning form, e.g.:

```
// MatchedFields reports, per returned id, which logical fields the query
// matched (e.g. "id", "content", "prop:<name>"). Backends compute this with
// their own matcher so provenance == real match semantics.
type ProvenanceBackend interface {
    Backend
    SearchWithFields(text string, limit int) ([]FieldHit, error) // {ID, Fields}
}
```

`search.VisibleSearcher` consumes it: resolve `VisibleFields` per (principal,
hit) from the affordance resolver, intersect, drop empties. A backend that
doesn't implement the provenance form degrades to entity-level only (documented,
fail-closed if a `visible:` block exists for the type).

### Conformance

Extend `storetest.RunVisibleSearchTests` with field-visibility cases: a hit
matching only a hidden field is dropped; a hit matching a visible field (or
id/content) survives even if it *also* matched a hidden one; same no-leak +
ordered-subsequence invariants over the visible-field projection. Run x3
(generic+memstore/linear = ground truth, generic+bleve, pgstore-native
DB-gated). `MatchText`-over-visible-projection is the semantic oracle the
backends are checked against.

### Performance contract

`VisibleFields` verdict cached per (principal, type) per request (same
memoisation shape as `acl.Request` GlobalRoles), except where a `when:`
predicate makes visibility entity-dependent -> per-entity eval (already the
affordance resolver's contract). Provenance is computed only over the bounded
candidate set (bleve 10k floor / pg candidate rows), not the corpus. pgstore's
second stage is inefficient by design but bounded — `log()` if the candidate set
is large, mirroring the bleve-10k-floor disclosure in TKT-BA8BSX.

## Open questions for implementation

- Exact `FieldHit.Fields` vocabulary: `"prop:<name>"` vs bare property name; how
`id`/`content`/`primary` are represented (they're never `visible:`-gated, so
always in the "visible" set).
- bleve: confirm `AnalyzeText` reproduces the per-word/prefix/ID query behavior
closely enough that the per-field pass never rejects a candidate the index
legitimately surfaced from a visible field (bias: a per-field pass that can't
confirm keeps the hit — no false drops).
- Whether the provenance op replaces `Backend.Search` or sits beside it (prefer
beside + capability check, so non-provenance backends still compile).

## Reference files

- `internal/search/types.go` — `Backend`, `VisibleSearcher`, `TypeScope`
- `internal/search/bleveindex/bleveindex.go` — `entityToDoc`, `Search`, mapping
- `internal/search/linearsearch.go`, `internal/search/filter.go` — `MatchText`
- `internal/search/visible.go` — generic VisibleSearcher (entity-level today)
- `internal/store/pgstore/search.go`, `entity.go:624` — `search_text` + backend
- `internal/store/storetest/visiblesearch.go` — conformance suite
- `internal/dataentry/helpers.go` (`executeQuery`), `api_v1.go`
(`handleV1Search`)
- `internal/affordances/resolver.go` — the visible-field verdict source

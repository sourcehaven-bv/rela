---
id: RR-B0L1L0
type: review-response
title: The entity type is not searchable on ANY backend, including bleve
finding: 'The ticket''s headline feature — typing `fancy` to reach an entity of type `FancyReport` — is impossible on every backend as things stand, not just on postgres as RR-79QZA3 stated. On bleve, entityToDoc DOES store a `Type` field (bleveindex.go:655-681), but `boostedFields` (bleveindex.go:412-419) searches only `primary`, `properties`, `content` and `all`, and the `all` composite is built from `id + primary + props + content` — the TYPE IS NOT IN IT. So no free-text word query can ever match the type. MEASURED against a real bleve index holding one entity of type FancyReport titled ''Some word in title'': query ''fancyreport'' -> [], ''FancyReport'' -> [], ''fancy'' -> [], while ''some word'' -> [FR-0001]. My earlier cross-field probe appeared to succeed only because the TITLE words matched; the type token contributed nothing. This invalidates the premise that this is a frontend-only ticket: no client-side reranker can order candidates that were never returned. A backend change is REQUIRED on bleve, and separately on postgres (RR-79QZA3); sqlite needs none because the sqlite build wires bleve as its Searcher (appbuild_sqlite.go:32-38), so it inherits the bleve fix.'
severity: critical
resolution: Superseded by RR-77AMZO. The operator chose a visible type-picker UI over inferred type matching, so the entity type is never matched as free text and never needs to be indexed as such. The finding remains TRUE and worth knowing (the type genuinely is not free-text searchable on any backend), but it is no longer a blocker for this ticket. Worth filing as a standalone backend ticket if free-text type matching is ever wanted for the search page or the API.
reason: 'Not fixed because the design changed so that the finding no longer describes a defect in this ticket''s path. The finding is TRUE and stays on record: the entity type genuinely is not free-text searchable on any backend, verified by measurement against a real bleve index. But the operator chose a visible type-picker UI (RR-77AMZO) over inferring a type from the query text, so the type is now applied as an indexed FILTER through the existing ?type= request parameter and is never matched as free text. Making it free-text searchable would therefore buy this ticket nothing, while costing a full-table backfill under an advisory lock, a five-site change including the MatchText conformance ground truth, and a false-positive blowup where a short prefix of a common type name matches that entire type (RR-V9R8S9 has the measurements). Fixing it here would be pure cost. It remains worth doing on its own merits for the search page and the public API, where a user CAN type a type name as free text and reasonably expect a hit; that belongs in a separate backend ticket with its own migration plan, not bolted onto a frontend menu change.'
status: wont-fix
---

## Evidence

`internal/search/bleveindex/bleveindex.go:672` — the `all` composite omits the
type:

```go
all := strings.Join([]string{e.ID, primary, props, e.Content}, " ")
```

`bleveindex.go:412-419` — the fields a word query actually searches:

```go
var boostedFields = []struct {
    field string
    boost float64
}{
    {"primary", boostPrimary},
    {"properties", boostProperties},
    {"content", boostContent},
    {"all", boostContent},
}
```

`Type` is stored (it backs the `type:` filter and world resolution) but is never
a free-text target.

### Measured

One entity, `FR-0001`, type `FancyReport`, title "Some word in title":

```
query "fancyreport"      -> []
query "FancyReport"      -> []
query "fancy"            -> []
query "fancy some word"  -> [FR-0001]
query "some word"        -> [FR-0001]
```

The last two succeed on the *title* alone. The type token is inert.

## Why this reframes the ticket

The plan is written as a frontend-only change on the premise that recall already
exists and only ordering is wrong. For the type dimension that premise is false
on every backend. A client-side scorer cannot rank a row the server never
returned.

Backend coverage after this correction:

| backend | searcher | type searchable today | needs a change |
|---|---|---|---|
| default (fs) | bleve | **no** | yes |
| sqlite | **bleve** (`appbuild_sqlite.go:32-38`) | **no** | inherits the bleve fix |
| postgres | pgstore SQL LIKE | **no** (RR-79QZA3) | yes |
| memorybackend | LinearSearch | no | test-only, accept |

The operator has asked for postgres and sqlite coverage. Sqlite is free — it
uses bleve — so the real work is **two** changes: bleve and pgstore.

## Required plan change

Move the scope boundary explicitly. Add the entity type to the searchable text
on both engines:

- **bleve** — include `e.Type` in the `all` composite, or add `type` to
`boostedFields` with its own boost. Prefer the latter: it keeps the boost
tunable and avoids re-weighting `all`. Requires a **reindex**, which the fs
build does at startup, so the migration cost is low.
- **pgstore** — add the type to `entitySearchText` (RR-79QZA3), which needs a
backfill of existing rows.

Both need a test asserting a type-name query returns the entity, at the
`bleveindex_test.go` and pgstore conformance levels. Consider adding the case to
the shared `storetest` search conformance suite so every backend is held to it.

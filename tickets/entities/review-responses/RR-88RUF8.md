---
id: RR-88RUF8
type: review-response
title: Splitting the query breaks exact and prefix ID matching in bleve
finding: 'The plan''s half 1 (split the typed query on separators before sending it to /_search) reproduces the exact regression BUG-O09QUC was filed to fix. bleveindex.Search builds its two ID queries from `idText := strings.TrimSpace(text)` — the WHOLE query string, not the per-word list (bleveindex.go:455-470), and the words are re-joined with spaces upstream (helpers.go:598-604). Splitting `TKT-6MZ42J` into `TKT 6MZ42J` makes idText the literal "TKT 6MZ42J", so the exact TermQuery (boost 8.0) and PrefixQuery (boost 6.0) both miss. MEASURED against a throwaway bleve index with 8 title-decoys: `TKT-6MZ42J` returns [TKT-6MZ42J] alone, while `TKT 6MZ42J` returns [RR-0000 RR-0001 RR-0002 RR-0003 RR-0004] — the target entity is pushed out of the top 5 entirely, buried under entities whose titles merely contain the word ''tkt''. That is BUG-O09QUC''s symptom verbatim (''a BUG- search returned zero BUG-* entities in the top 8''). Note recall survives (the per-word fuzzy pass still finds it deeper in the list); it is RANKING that collapses, which is exactly what the mention menu cares about since it shows 20 rows.'
severity: critical
resolution: The query split is removed from the plan entirely. The typed query is sent to /_search unmodified, exactly as today, so bleve's idText remains the full query string and the boost-8.0 exact-ID and boost-6.0 prefix queries fire as they do now. Cross-field reach is provided by the type-picker UI (RR-77AMZO) instead of by query rewriting. AC 7 pins that TKT-AB still returns AND ranks TKT-ABCD.
status: addressed
---

## Evidence

`internal/search/bleveindex/bleveindex.go:443-475`:

```go
words := strings.Fields(text)
...
idText := strings.TrimSpace(text)        // <-- the whole query, not a word
idExact := bleve.NewTermQuery(idText)    // boost 8.0
idPrefix := bleve.NewPrefixQuery(idText) // boost 6.0
for _, word := range words {
    queries = append(queries, buildBoostedWordQuery(strings.ToLower(word)))
}
```

`internal/dataentry/helpers.go:598-604` re-joins the parsed words with `" "`
before handing them to the searcher, so a client-side split survives all the way
down.

### Measured

A temporary test against a real bleve index (8 `RR-*` decoys titled "tkt ranking
discussion about tkt work", plus two real `TKT-*` entities):

```
query "TKT-"         top5 -> [TKT-AAAAAA TKT-6MZ42J RR-0000 RR-0001 RR-0002]
query "TKT-6MZ42J"   top5 -> [TKT-6MZ42J]
query "TKT 6MZ42J"   top5 -> [RR-0000 RR-0001 RR-0002 RR-0003 RR-0004]
```

The split form loses the target from the top 5. The code comment at
`bleveindex.go:450-454` predicts this: the per-word pass tokenizes a dashed ID
into `tkt`/`6mz42j` and "can't bridge a partial-ID query back to the original
token".

## Required plan change

Split **conditionally on query shape**. If the query looks like an entity ID,
send it unsplit so the boosted ID queries fire; otherwise split. The ID grammar
already exists client-side in `entityRefIdGrammar.ts` — reuse it rather than
inventing a second definition.

Add an acceptance criterion and test: `@TKT-6MZ42J` and `@TKT-` must rank the
matching entity first, asserted both at the `useMentionMenu` level (which query
string is sent) and, if the Go side is touched, in `bleveindex_test.go` beside
the existing `TestIndex_SearchByIDPrefix`.

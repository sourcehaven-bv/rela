---
id: RR-DLL40G
type: review-response
title: Splitting the query returns zero results on the default/memory search backend
finding: 'The plan assumes splitting the query widens recall because free-text words are OR''d. That holds for bleve only. LinearSearch (the memorybackend build, and the ground-truth matcher the conformance suite checks the other backends against) matches the ENTIRE query text as one case-insensitive substring: MatchTextFields does `strings.Contains(strings.ToLower(e.ID), lower)` with `lower = strings.ToLower(text)` for the whole text (internal/search/filter.go:129-149), called per entity from LinearSearch.Search via MatchText (linearsearch.go:163). It never tokenizes. So `fancy some word in title` becomes a single 24-char substring needle and matches nothing. Splitting makes recall strictly WORSE there. Conversely — and this VINDICATES the need for half 1 on bleve — a measured probe shows the unsplit `fancy-some-word-in-title` returns [] from bleve while the split `fancy some word in title` returns [RI-0002 FR-0001 TKT-6MZ42J]. So the two backends want OPPOSITE query shapes, and the plan must say which it targets and what the other degrades to.'
severity: critical
resolution: 'Resolved by the same change as RR-88RUF8: the query is no longer rewritten, so every backend receives exactly what it receives today and none of them can diverge. The finding''s analysis of LinearSearch and pgstore whole-string substring matching remains accurate and is preserved as the reason the split was abandoned.'
status: addressed
---

## Evidence

`internal/search/filter.go:129-149` — the whole `text` is lowercased once and
used as a single `strings.Contains` needle:

```go
func MatchTextFields(e *entity.Entity, text string) map[string]struct{} {
    lower := strings.ToLower(text)
    if strings.Contains(strings.ToLower(e.ID), lower) { ... }
    if strings.Contains(strings.ToLower(e.Content), lower) { ... }
    for name, v := range e.Properties {
        if s, ok := v.(string); ok && strings.Contains(strings.ToLower(s), lower) { ... }
    }
}
```

`linearsearch.go:163` gates every candidate on `MatchText(e, text)`; the doc
comment at `linearsearch.go:185-188` confirms it is "plain case-insensitive
substring" and the conformance ground truth.

### Measured on bleve

```
query "fancy-some-word-in-title" -> []
query "fancy some word in title" -> [RI-0002 FR-0001 TKT-6MZ42J]
```

So on bleve the split is **required** — the plan's half 1 is not optional, and
the target entity is unreachable without it. On LinearSearch the same split is
what breaks it.

## The real conflict

The two backends want opposite query shapes:

| query form | bleve | LinearSearch |
|---|---|---|
| `fancy-some-word-in-title` (unsplit) | no hits | substring hit if the title literally contains it |
| `fancy some word in title` (split) | hits | no hits |

## "Send both forms" is ruled out — measured

The obvious compromise (send `"<raw> <seg1> <seg2> …"`, so the raw token serves
the substring backend and the segments serve bleve) **does not work**, because
`idText` is the *entire* query string. Appending anything to the raw ID destroys
the ID queries just as splitting did:

```
"TKT-6MZ42J TKT 6MZ42J"  -> [RR-0000 RR-0001 RR-0002 RR-0003 RR-0004]
"TKT- TKT"               -> [RR-0000 RR-0001 RR-0002 RR-0003 RR-0004]
"fancy-some-word-in-title fancy some word in title"
                         -> [RI-0002 FR-0001 TKT-6MZ42J RR-0000 RR-0001]
```

The cross-field case works, but both ID cases lose the target from the top 5.

## Required plan change

The only client-side option that satisfies both is a **conditional split on
query shape**: send an ID-shaped query unsplit (preserving the boost-8.0/6.0
queries), and split everything else. Reuse the existing ID grammar in
`entityRefIdGrammar.ts` rather than defining a second one. This is the same
conclusion RR-88RUF8 reaches; the two findings share one fix.

For LinearSearch, state in the plan that the memory build degrades on a
multi-term query and that this is accepted — the root CLAUDE.md documents
`memorybackend` as "tests / experiments", not a shipping configuration. Do not
leave it to be discovered later.

Add a test asserting the exact query string sent for both shapes, and verify AC
1 manually on the default build before the ticket is done.

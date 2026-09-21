---
id: RR-66JTAL
type: review-response
title: Non-Latin queries return zero matches and filter() returns null, not an empty array
finding: 'Two related defects in the same code path. (1) uFuzzy''s default `alpha` is a-z and `interSplit` is `[^A-Za-z\d'']+`, so any CJK, Cyrillic, Greek, Hebrew or Arabic query produces an empty split and no matches. Measured: split(''日本語'')=[], filter(hay,''日本'')=null, split(''Проект'')=[]. Today `rankByIdMatch` never filters, so those queries currently work (the backend matches, the client only reorders) — this would be a hard regression, not a missing nicety. The repo has Dutch corpora in its own tests and diacritics are already asymmetric: ''café'' matches both Café and Cafe, while ''cafe'' matches only Cafe, so `latinize` is needed too. The plan files this under Edge Cases as ''decide during implementation whether to enable it'', which treats a total-loss regression as a tuning preference. (2) `uf.filter()` returns null rather than [] when nothing matches — measured for ''日本'', ''---'' and ''''. Passing null to `uf.info()` throws. Since mentionQuery.ts:27 terminates only on whitespace and backticks, ''@---'' is a live query above MIN_SEARCH_LEN, so this is reachable: the throw lands in the existing catch and surfaces as ''Search failed'', a misleading error for harmless input.'
severity: significant
resolution: 'Both halves are now explicit acceptance criteria rather than implementation-time decisions. (1) AC 8: when uf.split() yields no terms — every CJK, Cyrillic, Greek, Hebrew and Arabic query — the ranker passes the server''s rows through unmodified, so today''s behaviour is preserved rather than regressed. (2) The approach includes an explicit `idxs === null` check before uf.info(), since filter() returns null rather than an empty array. The latinize/diacritics decision is called out for implementation with a test required either way.'
status: addressed
---

## Evidence

Measured against the installed `@leeoniya/ufuzzy` 1.0.19:

```
"日本"    split=[]        filter=NULL
"日本語"  split=[]        filter=NULL
"Проект" split=[]        filter=NULL
"café"   split=["caf"]   filter=[1]
"cafe"   split=["cafe"]  filter=[]
"---"    split=[]        filter=NULL
""       split=[]        filter=NULL
```

## Required plan change

Make both explicit acceptance criteria, not implementation-time decisions:

1. **Never filter when the needle cannot be split.** When
`uf.split(query).length === 0`, return the server's rows in the server's order,
unmodified. That preserves today's behaviour for every non-Latin script and for
an all-separator query, and it is one line.
2. **Null-check `filter()`** at every call site before passing its result to
`info()`. The return type is `HaystackIdxs | null`.
3. Decide whether to enable `latinize` for the diacritic asymmetry, and pin the
decision with a test either way.

Add a test with a CJK title and a Cyrillic title asserting the rows survive, and
a test that `@---` yields "No matches" rather than "Search failed".

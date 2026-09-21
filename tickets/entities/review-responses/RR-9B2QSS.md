---
id: RR-9B2QSS
type: review-response
title: An all-separator query makes the quality floor admit every candidate
finding: The quality floor is `info.terms[o] >= need` where `need = uf.split(query).length`. Measured against @leeoniya/ufuzzy 1.0.19, `uf.split('---')` returns `[]` and `uf.split('-')` returns `[]`, so `need` is 0 and the floor degenerates to `terms >= 0`, which every candidate satisfies. A user who types `@---` (or `@-`, reachable in two keystrokes and above MIN_SEARCH_LEN=2) would be shown an unranked dump of whatever the backend returned, presented as if it were a match. The plan's Edge Cases section notes this in passing ('a floor of terms >= 0 would admit everything, so guard this explicitly') but leaves it as an implementation note rather than an acceptance criterion with a test, and the guard is not in the Technical Approach code sketch.
severity: significant
resolution: 'Guarded explicitly in the approach: when uf.split(query).length === 0 (all-separator input such as @---), the ranker returns the server''s rows in the server''s order rather than applying a degenerate floor. Promoted from an edge-case note to AC 9 with its own test asserting ''No matches'' rather than ''Search failed''.'
status: addressed
---

## Evidence

Measured against the installed package:

```
"---" split= []
"-"   split= []
""    split= []
"a"   split= ["a"]
```

`MIN_SEARCH_LEN = 2` (`useMentionMenu.ts:26`) does not help: `--` is two
characters and passes the length gate.

## Required plan change

Promote the guard from an edge-case note to an explicit early return in the
approach, and give it a test:

```ts
const terms = uf.split(query)
if (terms.length === 0) return []   // nothing rankable; show "No matches"
```

Decide and record which state this shows. Showing "No matches" is right; showing
the "type to search" prompt would also be defensible. What must not happen is an
unfiltered list. Add it as an acceptance criterion so it is verified rather than
assumed.

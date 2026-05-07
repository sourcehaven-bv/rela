---
id: RR-XY8HC
type: review-response
title: 'Adjacent IDs without separator: second ID gets replaced incorrectly'
finding: 'rewriteText re-runs the regex on rem[searchFrom:] after a non-boundary failure. The regex''s \A alternative anchors at the start of the SUBSLICE, not the absolute start. So for input ''TKT-1TKT-2'' with both keys mapped: first match at pos 0 (TKT-1) fails trailing-boundary check (next char is T, word). searchFrom advances past TKT-1. Re-search anchors \A at the new subslice start, treating TKT-2 as if it''s at BOL. Result: ''TKT-1[B](#b)'' instead of unchanged ''TKT-1TKT-2''. Verified with reproducing test.'
severity: critical
resolution: Replaced regex+manual-boundary scanner with a Unicode-aware byte-walker. The scanner now tries each key (sorted longest-first) at every word-boundary position, with no regex involved — eliminating the \A-on-subslice bug class. Added tests for 'TKT-1TKT-2', 'TKT-1TKT-1', 'see TKT-1TKT-2 here'; all pass with no rewriting (verified).
status: addressed
---

# Finding

`rewriteText` (`internal/lua/markdown.go:1559-1629`) has a `\A`-anchor bug.

When the regex does not match at the trailing boundary, the code does
`searchFrom = idEnd - i; continue`. The next iteration calls
`re.FindStringSubmatchIndex(rem[searchFrom:])`. The regex's leading boundary
alternative `\A` matches the start of *that subslice*, which is no longer the
start of the original input. So a subsequent ID in the text matches against `\A`
even though there's a word character (the preceding ID's trailing digit)
immediately before it.

**Reproduction (verified):**

```
"TKT-1TKT-2"            → "TKT-1[B](#b)"  // wrong
"see TKT-1TKT-2 here"   → "see TKT-1[B](#b) here"  // wrong
"TKT-1TKT-1"            → "TKT-1[A](#a)"  // wrong
```

Expected: all three should be unchanged because the IDs are not
boundary-separated.

# Resolution

Two viable approaches:

1. After a non-boundary failure, when retrying, suppress the `\A`
alternative — e.g. compile a second regex without `\A` for retries, or
post-check that `loc[2] != loc[3]` (group 1 must be non-empty, meaning a real
boundary char matched).
2. Replace the regex+manual-boundary scanner with a byte-walker
(Aho-Corasick or sequential `strings.Index`) that handles boundaries directly.
This eliminates the whole `\A`-on-subslice class of bug.

Going with option (1): track whether the current scan position is at absolute
start; pass that info to the regex search and gate the `\A` arm.

Add tests:

- `"TKT-1TKT-2"` with both keys → unchanged
- `"TKT-1TKT-1"` with TKT-1 → unchanged
- `"abcTKT-1"` with TKT-1 → unchanged (already covered)
- `"TKT-1abc"` with TKT-1 → unchanged (already covered as AC6)

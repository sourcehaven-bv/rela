---
id: RR-3S0PV
type: review-response
title: 'UTF-8 boundary handling: byte-oriented isBoundary mishandles continuation bytes'
finding: 'isBoundary at markdown.go:1452-1462 and the regex char class [^A-Za-z0-9_-] both treat input as bytes. UTF-8 continuation bytes for non-ASCII letters are ''non-word'' under this scheme. Result: ''éTKT-1'' matches at TKT-1 even though semantically it''s adjacent to a letter. Same root cause as the titleSlug Unicode issue. Either fix to be Unicode-aware or document loudly as a known limitation.'
severity: significant
resolution: 'Boundary checks (boundaryBefore/boundaryAfter) now decode the rune and use unicode.IsLetter/IsDigit. The byte-walker scanner uses these instead of byte-level checks. Added test ''Unicode boundary: éTKT-1 unchanged'' verifying that ''caféTKT-1'' is left alone while ''café TKT-1'' (with space separator) is rewritten.'
status: addressed
---

# Finding

`isBoundary` and the regex `[^A-Za-z0-9_-]` are byte-oriented. UTF-8
continuation bytes for non-ASCII letters look like "non-word" bytes, so
non-ASCII letters adjacent to an ID act like word boundaries instead of
"in-word" characters.

Examples:

```
"éTKT-1"   → matches (the \xa9 byte before T is "non-word")
"caféTKT-1" → matches (same)
```

Users would expect the trailing letter (`é`) to suppress the match the same way
a trailing ASCII letter does.

# Resolution

Two paths:

1. **Make boundaries Unicode-aware.** Decode the rune at position `i`
(or just before, scanning back to a UTF-8 start byte) and use `unicode.IsLetter
   || unicode.IsDigit || == '_' || == '-'` as the word-class. Replace
`[^A-Za-z0-9_-]` with a Unicode-aware predicate in code (regex can't easily
express it; do the boundary check entirely in Go after each match).
2. **Document the limitation explicitly.** Add to docs: "The boundary
class is byte-level and does not understand UTF-8. Non-ASCII letters adjacent to
an ID may be treated as word boundaries." Less work but a sharp edge.

Going with (1) since it pairs with the titleSlug Unicode fix.

---
id: RR-FA0JG
type: review-response
title: Word-boundary semantics need precise spec; pin down the boundary char class
finding: Plan uses '\b(KEY1|KEY2|...)\b' with hand-wavy 'word boundary' wording. RE2 \b uses [A-Za-z0-9_] as the word class, so it fires between digit and letter (good) but treats '-' as a non-word char (so it fires between - and letter — problematic for distinguishing 'TKT-1' from a leading 'X-'). Need explicit non-capturing boundary classes around the alternation that include the - character, plus a documented constraint that IDs start/end with [A-Za-z0-9].
severity: significant
resolution: 'Plan documents an explicit boundary contract using char class [^A-Za-z0-9_-] (NOT RE2''s \b which treats - as word boundary). Form: (?:\A|[^A-Za-z0-9_-])(KEYS)(?=\z|[^A-Za-z0-9_-]). Trailing side uses lookahead. IDs constrained to start/end with [A-Za-z0-9]; violation raises a Lua error. AC6 covers XTKT-1, prefix-TKT-1, TKT-1-suffix, TKT-1abc, TKT-12 negatives.'
status: addressed
---

# Finding

The plan says "matches at word boundaries" and "build `\b(KEY1|...)\b`." Two
problems:

1. **`\b` in RE2 uses `[A-Za-z0-9_]` as the word class.** That means `\b`
fires between `-` and a letter. So `\bTKT-1\b` matches inside
`prefix-TKT-1-suffix` even though the surrounding `-` strongly suggests it's
*part* of a larger token. We probably want `-` to count as "in-word" for this
purpose so that hyphenated identifiers can't accidentally contain a match.
2. **IDs whose last char is non-word** (legacy oddities) would have weird
trailing-boundary semantics. Manual IDs in this metamodel are constrained but
the contract should pin it down.

# Resolution

Replace `\b` with explicit boundary classes:

```
(?:\A|[^A-Za-z0-9_-])(KEYS)(?:\z|[^A-Za-z0-9_-])
```

RE2 doesn't support lookbehind, so the leading "boundary char" is consumed by
the match. Use a numbered submatch group for the ID; on splice, re-emit the
leading char (or use lookahead at the trailing side: RE2 supports `(?=...)`).

A clean form:

```
(?P<L>\A|[^A-Za-z0-9_-])(?P<id>KEYS)(?=\z|[^A-Za-z0-9_-])
```

Match `id`, replace just the `id` range, leave `L` in place.

**Document the constraint:** IDs must start and end with `[A-Za-z0-9]`.
`resolve_refs` will refuse keys that don't satisfy this (Lua error).

**Boundary tests** (replace AC5/AC6 with explicit cases):

- `XTKT-1` → no match.
- `prefix-TKT-1` → no match (boundary class includes `-`).
- `TKT-1-suffix` → no match.
- `TKT-1abc` → no match.
- `TKT-12` (map has `TKT-1`, not `TKT-12`) → no match.
- `(TKT-1)` → match.
- `TKT-1.`, `TKT-1, TKT-2`, BOL/EOL → match.

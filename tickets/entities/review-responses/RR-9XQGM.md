---
id: RR-9XQGM
type: review-response
title: Splice cursor advancement must be specified to prevent replacement-of-replacement
finding: 'Plan says ''continue past the inserted text (do not rescan the splice).'' Specify precisely: after replacing range [match_start, match_end) with value, advance to match_start + len(value) in the new string. Add a negative test where the splice text contains another map key to prove no rescanning.'
severity: minor
resolution: 'Plan Approach step 5 specifies splice cursor advancement: after replacing [match_start, match_end) with value, advance cursor to match_start + len(value) in the new string. Splice text is never rescanned. AC17 verifies with a value containing another map key.'
status: addressed
---

# Finding

Plan says "continue past the inserted text (do not rescan the splice)" but
doesn't specify precisely what "past" means. If the splice value contains
characters matching another key, vague semantics could cause double replacement
or infinite loops.

# Resolution

Pin down:

> **Splice cursor.** After replacing matched range `[match_start,
> match_end)` with `value`, advance the scan cursor to `match_start +
> len(value)` in the *new* string. Splice text is never rescanned.

Negative test:

- Map: `{["TKT-1"] = "[See TKT-2](#tkt-2)", ["TKT-2"] = "[X](#x)"}`
- Input: `text TKT-1 end`
- Expected: `text [See TKT-2](#tkt-2) end` — `TKT-2` in the splice is NOT
re-resolved.

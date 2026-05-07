---
id: RR-J3IIA
type: review-response
title: Naive backtick toggle mishandles multi-backtick code spans
finding: 'Plan says scanner toggles in_code_span on every backtick. CommonMark allows multi-backtick code spans like ``foo`bar`` where a single backtick inside is literal. A single-backtick toggle would mis-segment. Need run-length matching: open with N backticks, close with the next run of exactly N (or, conservatively, N-or-more).'
severity: significant
resolution: 'Plan replaces single-backtick toggle with run-length matching: an opening run of N backticks closes at the next run of exactly N. Malformed (no closer) → opening run treated as literal text. AC4 specifically tests a multi-backtick span containing an ID followed by an outside ID.'
status: addressed
---

# Finding

The plan says the scanner walks left-to-right "tracking an `in_code_span` flag
toggled on every backtick." That is wrong for multi-backtick code spans:
CommonMark allows ` `foo`bar`` `` (open and close with two backticks; a single `
inside is literal). A simple toggle treats the inner backtick as a closer.

`extractInlineText` preserves backticks verbatim from the source, so a
multi-backtick span lands in our `text` field as-is.

# Resolution

**Run-length matching.** When the scanner hits a run of N backticks, find the
next run of *exactly* N backticks and skip the whole span. If no matching closer
exists (malformed), treat the opening run as literal text and continue.

Add tests:

- ` `foo TKT-1 bar` ` followed by `TKT-1` outside → only the second is
rewritten.
- Single ` ` `with no closer (literal) followed by`TKT-1 `→`TKT-1 ` IS
rewritten (no open span).
- `  `triple inline`` `TKT-1` `` mixed nesting (rare in inline; verify
behavior is consistent).

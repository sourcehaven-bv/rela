---
id: RR-XOR7D
type: review-response
title: Docs claim about run-length matching is misleading for parser-produced ASTs
finding: The parser normalizes multi-backtick spans to single-backtick form. Docs should clarify run-length matching is only relevant for hand-constructed text fields (e.g. via paragraph()), not for parse() output.
severity: minor
resolution: 'Added clarifying note to docs: parse() normalizes multi-backtick spans to single-backtick form, so run-length matching is mostly relevant for hand-constructed text fields (e.g. via paragraph() with a literal containing multiple backticks).'
status: addressed
---

# Finding

The docs talk up run-length matching but don't note that `parse()` already
normalizes multi-backtick spans. Run-length matching only matters when scripts
construct text fields by hand (e.g. via `paragraph()` with a
literal-backtick-containing string).

# Resolution

Add to docs: "Note: `parse()` normalizes multi-backtick code spans to
single-backtick form before they reach `resolve_refs`. The run-length scanner is
only relevant when a script constructs text fields by hand (for example, by
calling `paragraph()` with a literal containing multiple backticks)."

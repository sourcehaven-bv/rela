---
id: RR-RYV9V
type: review-response
title: Value cap counts UTF-16 code units; doc implied codepoints
finding: '`target.length` is UTF-16 code units, not codepoints, so astral characters (emoji, rarer CJK) count double — the effective limit is ~5k glyphs for such a field. The docstring''s claim that ''10k is far above any genuine form field'' does unexamined work for a CJK/emoji-heavy field.'
severity: nit
resolution: Added a clause to the MAX_MATCH_VALUE_LENGTH docstring stating the measure is UTF-16 code units, that astral characters count double (~5k glyphs effective), and that this is deliberate — the scan cost really is per code unit, and both figures remain far above a real value.
status: addressed
---

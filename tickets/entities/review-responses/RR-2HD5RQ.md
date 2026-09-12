---
id: RR-2HD5RQ
type: review-response
title: 'Rune set: \v, \f, U+0085, U+2028 and U+2029 survive flattening'
finding: 'flattenToLine maps only \n and \r (to a space) and NUL (dropped). \v, \f, U+0085, U+2028 and U+2029 all pass through unchanged. The question is whether any of them can do the thing the threat model is about — forge a heading out of a producer-supplied value.'
severity: minor
resolution: 'Tested empirically rather than reasoned from the spec: for all five survivors, appending "boom<sep>## Pwned" to a section produced NO new heading — ExtractHeaders returned the original headings in every case. CommonMark defines a line ending as \n, \r or \r\n and nothing else, and markdown.AppendToSection splits on \n, so the current rune set is exactly sufficient and widening it would be cargo-culting. Recorded the reasoning in flattenToLine''s doc comment under an "Only \n and \r, deliberately" heading, including the caveat that the safety comes from the PARSER''s definition of a line ending rather than from the function — so a future path splitting on Unicode line boundaries (notably anything using unicode.IsSpace, which does match \v, \f and U+0085) would put the survivors back in play. Not verified: the client-side renderer (marked, frontend/src/utils/markdown.ts) was not executed. It is CommonMark-based and shares the line-ending definition, so this is high confidence by specification rather than measurement.'
status: addressed
---

The survivors are real but inert. The write path parses with goldmark and
splits on `\n`; none of the five is a line ending in CommonMark, so none of them
can start the heading that the threat model is about.

What is worth writing down is *why* the narrow set is sufficient, since that
reason lives in the parser rather than in this function.

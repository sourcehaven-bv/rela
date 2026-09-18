---
id: RR-DWO2KJ
type: review-response
title: Two wrapper tests were serialization-string or tautological
finding: The first wrapper test asserted four toContain() substring checks against serialized HTML, pinning formatting rather than structure - the one style this file's own BUG-SQSV6V header warns against. The 'leaves content without tables untouched' test asserted identity on input that short-circuits at the includes() guard, so it would pass even if the function body were 'return html'.
severity: minor
resolution: First test rewritten to parse the output and assert on node attributes. Added a table-free case that actually reaches the parser ('<p>write &lt;table&gt; ...</p>'), plus nested-table, blockquote/list-item and whitespace-collapsing cases. 90 tests in the file, up from 86.
status: addressed
---

# Finding

Two of the nine new tests did not pin what they appeared to.

1. **Serialization-string assertions.** The first test used four `toContain()`
checks against the serialized HTML string. Those pin *formatting*, not
structure, and cannot distinguish "the wrapper has these attributes" from "these
strings appear somewhere in the output". This file's own header documents
(BUG-SQSV6V) that HTML serialization differs between DOM implementations — so
asserting on serialized text is precisely the style it has reason to avoid.
2. **Tautological identity check.** `leaves content without tables untouched`
passed `'<p>no tables here</p>'`, which short-circuits at the
`includes('<table')` guard and returns the argument by reference. It would pass
if the function body were `return html`.

# Resolution

The first test now parses the output and asserts on node attributes
(`getAttribute('tabindex')`, `role`, `aria-label`) plus `:scope > table`
containment, matching the shape the second test already used correctly. A shared
`parse()` helper was added to the describe block.

The identity test is kept (the fast path is worth pinning) but joined by a case
that **actually reaches the parser** and still returns unwrapped: `'<p>write
&lt;table&gt; to start one</p>'` — the entity form, which the `includes` guard
does not catch.

Also added while fixing the critical findings: nested-table, blockquote,
list-item, and whitespace-collapsing cases. The file is now at 90 tests, up from
86.

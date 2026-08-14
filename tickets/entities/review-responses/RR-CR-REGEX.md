---
id: RR-CR-REGEX
type: review-response
title: 'wrapCss regex corrupted CSS containing a brace in a comment, string, or nested rule'
finding: "The `@layer` wrap used a regex over `:root...\\{[^}]*\\}`, which cannot see comments, strings, url() literals, or CSS nesting. VERIFIED by direct probe: input ':root{/* } */--a:1}.b{c:d}' produced '@layer rela;:root{/* }\\n@layer rela {\\n */--a:1}.b{c:d}\\n}' — the regex stopped at the brace inside the comment, hoisted half a declaration block out as a token block, and spliced the '@layer rela {' opener into the middle of a comment. Brace count 3 open / 4 close: unbalanced garbage. CSS nesting (':root{--a:1;& .x{color:red}}') corrupted similarly. The whole eager index-*.css would stop applying from that byte onward. The existing test suite passed throughout, because every assertion was a toContain() substring check and none asserted the output was valid CSS."
severity: critical
status: addressed
resolution: "Replaced the regex with a real postcss parse/walk/stringify (`postcss` added as an explicit devDependency; it was already transitive via Vite). Carve-out is now `node.parent.type === 'root'` + a selector regex, which is correct by construction for nesting, comments and strings. Verified: all 14 adversarial inputs now parse cleanly and round-trip stably."
---

Raised by `/code-review` of the TKT-3DBK6I implementation.

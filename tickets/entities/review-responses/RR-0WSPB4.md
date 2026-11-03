---
id: RR-0WSPB4
type: review-response
title: Override moves mermaid's katex outside its declared range
finding: katex goes from 0.16 to 0.18, outside mermaid 11's declared ^0.16.45. The API mermaid calls still works, but no automated test renders a mermaid diagram with a $$ label. Remove the override once mermaid depends on katex 0.18 or later.
severity: minor
reason: The only call mermaid makes (renderToString with MathML output) was smoke-tested against 0.18.10. The 0.18 breaking change is CSS class prefixes, and rela loads no KaTeX CSS. A mermaid render test needs a real browser layout and is out of scope for this triage. The override is removed when rela moves to mermaid 12, which requires katex ^0.18.0.
status: deferred
---

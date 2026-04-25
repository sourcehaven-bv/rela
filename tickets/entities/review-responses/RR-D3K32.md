---
id: RR-D3K32
type: review-response
title: hrefRegex misses non-adjacent id attributes — emits duplicate id on re-rewrite
finding: 'document.go:362 hrefRegex = `(id="[^"]*" )?href="([^"]*)"`. Optional group requires id= IMMEDIATELY before href= with exactly one space. If source markdown or future rewriter passes emit `<a id="mine" class="x" href="/form/x/TKT-1">` or reverse attribute order, regex matches only href=, leaves pre-existing id= in the text, then prepends a new id — output has TWO id attributes. Browsers accept this but click handler''s document.getElementById picks whichever appears first in DOM parse order, which may not be the rewriter''s scroll target. Docstring claims rewriter ''owns the scroll-anchor id'' — it doesn''t, under adversarial input. Fix: either strip ALL id attributes within the tag, or switch to a small HTML tokenizer (golang.org/x/net/html).'
severity: significant
resolution: 'Replaced hrefRegex (match `href=...`) with anchorStartTagRegex (match whole `<a ...>`) + an attribute-level parser (parseAttrs/serializeAttrs). The rewriter now: (a) parses all attributes inside the tag in any order, (b) strips ALL pre-existing id attributes unconditionally (rewriter owns id on form routes), (c) preserves other attributes in source order, (d) re-serializes with normalized spacing. Added TestRewriteDocumentLinks_AttributeShapes with 8 cases covering: author-planted id before href, class before href, id interleaved between class and href, href-before-id, extra data-* attributes, single-quoted href, excess whitespace, href-less anchor. All pass.'
status: addressed
---

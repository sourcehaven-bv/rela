---
id: RR-YMJOEO
type: review-response
title: Text part unescaped AFTER stripping tags, re-materializing entity-encoded markup into the delivered body
finding: 'stripHTML did html.UnescapeString(tagRe.ReplaceAllString(s)) — the order is backwards. Entity-encoded markup passes the tag regex untouched and is then decoded into live markup. Verified: a table cell of ''&lt;script&gt;alert(1)&lt;/script&gt;'' arrived in the text/plain part as a literal <script> tag, and ''&lt;a href=...&gt;'' as a live anchor. Reachable from any entity property or body containing HTML entities, which markdown routinely does and an attacker controls directly. The function''s own doc comment claimed the opposite behaviour. Related bugs in the same file: ReplaceAll on emphasis markers corrupted ordinary text, index arithmetic truncated URLs containing parens, and section titles/subjects bypassed sanitization entirely.'
severity: critical
resolution: text.go rewritten to walk goldmark's AST instead of hand-rolling string munging — the parser already knows what is emphasis, a link, or literal text. Bare values (subject, labels, cells) go through stripMarkup, which decodes BEFORE stripping and repeats to a bounded fixed point so double-encoded input cannot survive. RawHTML/HTMLBlock nodes are dropped rather than echoed. Pinned by TestRender_TextPartStripsEncodedMarkup (5 encodings across subject, title and cell) and TestRender_LinkWithParensInURL. As a bonus the GFM table in prose now renders as readable columns instead of raw pipe syntax.
status: addressed
---

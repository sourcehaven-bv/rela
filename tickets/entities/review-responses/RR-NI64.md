---
id: RR-NI64
type: review-response
title: Inline marker preservation is inconsistent across constructs
finding: 'Strikethrough is preserved but code spans, links, autolinks, raw HTML, bold, italic are silently stripped. This affects ALL extracted-text sites: paragraphs, headings, blockquotes, table cells, list items. A user expecting ''GFM markers preserved'' will be burned by `code` next to ~~strike~~. PR description acknowledged paragraphs and headings but not tables and blockquotes.'
severity: critical
resolution: Added explicit inline marker preservation policy doc comment on extractInlineText. Code spans (`...`) now also preserved alongside strikethrough. Bold/italic/links remain dropped per documented policy. New TestMdInlineTextPolicy covers all preservation cases across paragraph/heading/blockquote/table-cell/list-item/code-block contexts (12 sub-tests). The asymmetry is now explicit, intentional, and tested.
status: addressed
---

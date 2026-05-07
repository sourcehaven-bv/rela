---
id: RR-JUXC1
type: review-response
title: Code span fence width not preserved — silent meaning shift on backtick-containing content
finding: 'renderInlineNode and flattenInlineNode unconditionally wrapped code-span content in single backticks. Per CommonMark 6.1, this corrupts spans whose content contains backticks: `` ` `` (literal backtick) renders as `` ``` `` which won''t re-parse as a span at all.'
severity: critical
resolution: Replaced single-backtick wrap with writeCodeSpan helper that picks the smallest fence (length = max-internal-run + 1) and adds leading/trailing space when content starts/ends with backtick. Both renderInlineNode and flattenInlineNode use it. Added regression test `synthetic-11` (code span containing literal backtick).
status: addressed
---

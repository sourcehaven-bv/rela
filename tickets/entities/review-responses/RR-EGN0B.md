---
id: RR-EGN0B
type: review-response
title: Hard/soft breaks rendered inside heading split the heading
finding: renderInlineNode emits `\n` (soft) or `  \n` (hard) for break inlines unconditionally. Goldmark never produces breaks inside heading text on parse, but a script that copies inlines from a paragraph into a heading would break the document.
severity: critical
resolution: renderHeading now post-processes the rendered inlines string to collapse `  \n` and `\n` to single space. Same defense applies to table cells via escapeTableCell which collapses `\n` to space.
status: addressed
---

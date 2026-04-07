---
id: RR-4A6G
type: review-response
title: 'Test gap: strikethrough in non-list contexts'
finding: Only TestMdParagraphStrikethrough tests strikethrough outside list items. No coverage for headings, table cells (uses same extractText path), blockquotes, or that code blocks correctly do NOT activate strikethrough.
severity: nit
resolution: 'TestMdInlineTextPolicy includes coverage for strikethrough in: paragraph, heading, blockquote, table cell, task item. Plus an explicit test that strikethrough does NOT activate inside fenced code blocks.'
status: addressed
---

---
id: RR-R8IV
type: review-response
title: 'Test coverage gap: no test for ID code spans inside tables, blockquotes, list items'
finding: 'internal/dataentry/mentions_test.go covers paragraph context, fenced/indented code blocks (excluded), link text (excluded), multi-token spans (excluded). It does NOT cover: (a) a code span inside a GFM table cell, (b) a code span inside a blockquote, (c) a code span inside a list item, (d) a code span inside emphasis/strong (`**\`TKT-LXYHQ\`**`). All four ARE valid markdown that the SPA could render — the recursive ast.Walk handles them implicitly, but without tests a future refactor (e.g. switching to a non-recursive walk or restricting to top-level nodes) wouldn''t notice the regression. Add cases for each, both server-side (mentions_test.go) and client-side (markdown.test.ts). The frontend tests cover NONE of these either.'
severity: minor
resolution: Added 'code span inside list item is collected', 'code span inside blockquote is collected', 'code span inside GFM table cell is collected' to mentions_test.go. The table case motivated wiring the GFM extensions (RR-07K2).
status: addressed
---

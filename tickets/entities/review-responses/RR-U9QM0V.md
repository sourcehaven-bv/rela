---
id: RR-U9QM0V
type: review-response
title: Goldmark lacks GFM table extension
finding: quotefind uses goldmark.New() without extensions while marked uses gfm:true; a table is one paragraph to goldmark so its segment straddles cells.
severity: critical
resolution: 'Plan revised: quotefind Segments and RenderedTextWithMapping use goldmark with GFM Table, TaskList, Strikethrough; vitest renders a table through marked and asserts one mark per cell.'
status: addressed
---

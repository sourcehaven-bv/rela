---
id: RR-AT2I
type: review-response
title: Strikethrough markers stripped from list item text
finding: The extractInlineText function (markdown.go:563) recurses into emphasis/strikethrough/link nodes and only emits raw text, dropping the ~~ markers. This is critical because the checklist validation layer at internal/markdown/content.go uses strikethrough as the 'skip' marker — Lua scripts that round-trip checklists would silently strip skip markers, breaking validation.
severity: significant
resolution: 'Plan now includes: (1) enable extension.Strikethrough in luaMdParse alongside TaskList, (2) update extractInlineText to wrap *east.Strikethrough nodes with literal ~~...~~ markers around the recursed inner text. Other inline formatting (bold, italic, links) remains out of scope.'
status: addressed
---

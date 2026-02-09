---
challenges: Markdown parsing complexity, performance with many queries, consistent rendering across TUI/web
description: 'Allow markdown content to include: (1) ```rela-query blocks that render as live tables, (2) [[ID]] references that become links with hover cards, (3) {{property}} interpolation for dynamic text. Queries would use existing filter syntax. Would need markdown processor extension and rendering in both TUI and web UI.'
effort: medium
id: embedded-queries
prerequisites: None - can build incrementally
rationale: Makes entities self-documenting, reduces context switching, enables rich documentation that stays in sync.
status: validated
summary: Dataview-style queries and inline references in markdown
title: Embedded Queries & Live Content
type: future-concept
value: critical
---

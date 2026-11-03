---
id: TKT-ZKPA1E
type: ticket
title: Relation filter controls match on target id instead of display title
kind: enhancement
priority: medium
effort: s
status: backlog
---

## Description

Make relation filter controls match on the target id instead of its display
title. Part of RES-8CKUNJ.

Today the server matches on the neighbour's display title, eq/ne only
(`internal/dataentry/api_v1.go:566-640`). Two targets with the same title (two
statuses both called "Done") cannot be told apart, and renaming a target breaks
saved filter URLs.

## Scope

- The filter value is the target id. The control still shows titles.
- Keep accepting title values for existing URLs, with a deprecation path.
- Push the predicate down to postgres where possible (FEAT-R012DX).

## Acceptance criteria

- Two same-titled targets filter separately.
- Renaming a target does not change filter results.

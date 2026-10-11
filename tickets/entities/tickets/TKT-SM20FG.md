---
id: TKT-SM20FG
type: ticket
title: External-ref property type and Lua 3-way merge helper
kind: enhancement
priority: medium
effort: l
started: "2026-10-08"
completed: "2026-10-08"
status: done
---

## Description

Add the two building blocks a sync connector needs (FEAT-XYQMUB):

1. **External-ref property type**: system, external id and URL, with
uniqueness per system. The base for the next merge is the version tagged
`sync/<system>` (TKT-VO6VG9), which the connector moves after every pull or
push. The ref itself holds no version number: writing one would change the
entity and so make a new version.
2. **Lua 3-way merge helper**: a pure function over base, ours and theirs that
returns per field what to write in rela, what to push, and which fields
conflict.

## Rules

- Loading a schema with sync-managed refs fails on a backend without
versioning (bare fs).
- Loop prevention is by convergence: push only fields that differ from the
base, and move the base after every pull or push.
- If the base tag is missing or its version was purged, the helper reports
"base unknown" rather than merging against nothing.

---
id: TKT-Y7ORJ3L
type: ticket
title: Portable condition tree for form conditions
kind: enhancement
priority: medium
effort: m
status: backlog
---

## Description

`utils/conditions.ts` is a second, hand-written engine for `visible_when` and
`required_when`, next to `internal/predicate`. Native clients would need a third
and a fourth. Have the server parse and type-check the conditions and send a
small normalized expression tree in the form payload. Clients only evaluate it,
so live form feedback needs no round trip. Delete the SPA parser.

Part of FEAT-QRI3CFR (step 3.2).

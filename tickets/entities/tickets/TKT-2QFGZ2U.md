---
id: TKT-2QFGZ2U
type: ticket
title: 'SPA: recount sidebar status and items only for entity changes that can affect them'
kind: enhancement
priority: medium
effort: m
status: ready
---

## Description

`useNavStatus` and `useNavItems` refetch after EVERY `entity:changed` SSE event
(debounced 1 s), in every open tab, whatever the changed type. One write fans
out to one `_nav_status` and one `_nav_items` per open tab.

Filter by the changed type where that is safe. It is not trivially safe: a list
`condition:` or a status rule may traverse relations to other types
(`related(...)`), and a relation change may be reported under the type of either
endpoint. The server could publish, per status/items entry, the set of types
that can move it.

Found while working on TKT-YLG695.

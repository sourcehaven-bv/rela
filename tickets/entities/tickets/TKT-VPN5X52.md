---
id: TKT-VPN5X52
type: ticket
title: Kanban move and group intents
kind: enhancement
priority: medium
effort: l
status: backlog
---

## Description

A card drop makes the SPA build the PATCH or relation change, and wall group
creation (`useWallGrouping.ts`) takes up to four requests with partial states in
between. Give each card its allowed moves (columns and swimlanes, from ACL and
transition guards, evaluated per page) and add `POST /_kanbans/{name}/move` and
`POST /_kanbans/{name}/group`. The server turns each into writes through
`entitymanager`. The client no longer builds writes for the board.

Part of FEAT-QRI3CFR (step 2.2).

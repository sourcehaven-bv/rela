---
id: TKT-GYGUJO
type: ticket
title: 'Per-face visible: blocks (type@face keys)'
kind: enhancement
priority: low
status: backlog
---

## Description

TKT-RSVJKS accepts `type@face` keys in `fields:` and `options:` blocks.
`visible:` and `relations:` still refuse them at load. A per-face `visible:`
block needs `SearchFieldScope` to lower the face into the grant query, so search
keeps applying `visible:` inside the store query. Follow-up from GitHub #1760.

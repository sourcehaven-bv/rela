---
id: TKT-M3TFMZG
type: ticket
title: Records panel on the entity page
kind: enhancement
priority: medium
effort: m
status: backlog
---

## Description

Split from TKT-EV8U8T4. A records panel on the entity page: a table of the
owner's rows per record type and a line chart, reusing the `display: line` card
from TKT-RK810V7 and `/api/v1/_records/{type}/_aggregate`.

## Acceptance

- The panel shows only record types whose owner type is the entity's type.
- The table pages with the list cursor; the chart uses the aggregate endpoint.
